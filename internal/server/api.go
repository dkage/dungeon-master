package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/history"
	"github.com/dkage/dungeon-master/internal/launcher"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/session"
)

const (
	maxBody   = 64 << 10
	maxOpener = 4096
	defCols   = 120
	defRows   = 32
)

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/github", s.handleGitHub)

	mux.HandleFunc("POST /api/campaigns", s.handleCampaignAdd)
	mux.HandleFunc("PATCH /api/campaigns/{id}", s.handleCampaignUpdate)
	mux.HandleFunc("DELETE /api/campaigns/{id}", s.handleCampaignRemove)
	mux.HandleFunc("GET /api/campaigns/{id}/quests", s.handleQuests)
	mux.HandleFunc("GET /api/campaigns/{id}/quests/{number}", s.handleQuest)

	mux.HandleFunc("POST /api/sessions", s.handleCreate)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDelete)
	mux.HandleFunc("GET /api/sessions/{id}/term", s.handleTerm)
}

type heroJSON struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type sessionJSON struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"` // "hero" or "shell"
	Hero     string    `json:"hero,omitempty"`
	Title    string    `json:"title"`
	Campaign string    `json:"campaign"`
	Dir      string    `json:"dir"`
	Created  time.Time `json:"created"`
	Exited   bool      `json:"exited"`
	ExitCode int       `json:"exitCode"`
	// Lost is a session that was running when Dungeon Master last stopped; its
	// terminal is gone and only the record remains.
	Lost bool `json:"lost,omitempty"`
}

type stateJSON struct {
	Version   string              `json:"version"`
	Claude    claudeJSON          `json:"claude"`
	Defaulted bool                `json:"defaultHeroes"`
	Heroes    []heroJSON          `json:"heroes"`
	Campaigns []campaign.Campaign `json:"campaigns"`
	Sessions  []sessionJSON       `json:"sessions"`
}

type claudeJSON struct {
	Found bool   `json:"found"`
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}

// sessionViews merges the persisted records with the live sessions, which are
// the authority on whether a process is still running.
func (s *Server) sessionViews() []sessionJSON {
	recs := s.cfg.History.List()
	out := make([]sessionJSON, 0, len(recs))
	for _, r := range recs {
		v := sessionJSON{
			ID: r.ID, Kind: r.Kind, Hero: r.Hero, Title: r.Title, Campaign: r.Campaign,
			Dir: r.Dir, Created: r.Created, Lost: r.Lost,
			Exited: r.Ended != nil, ExitCode: r.ExitCode,
		}
		if live, ok := s.cfg.Sessions.Get(r.ID); ok {
			v.Exited, v.ExitCode = live.Exited()
		}
		out = append(out, v)
	}
	return out
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	st := stateJSON{
		Version:   s.cfg.Version,
		Claude:    claudeJSON{Found: s.cfg.Claude != "", Path: s.cfg.Claude, Error: s.cfg.ClaudeErr},
		Defaulted: s.cfg.Party.Defaulted,
		Heroes:    []heroJSON{},
		Campaigns: s.cfg.Campaigns.List(),
		Sessions:  s.sessionViews(),
	}
	for _, h := range s.cfg.Party.Heroes {
		st.Heroes = append(st.Heroes, heroJSON{ID: h.ID, Label: h.Label})
	}
	writeJSON(w, http.StatusOK, st)
}

// --- campaigns ---

type campaignReq struct {
	Dir    string   `json:"dir"`
	Name   string   `json:"name"`
	Remote string   `json:"remote"`
	Labels []string `json:"labels"`
}

func (s *Server) handleCampaignAdd(w http.ResponseWriter, r *http.Request) {
	var req campaignReq
	if !decode(w, r, &req) {
		return
	}
	c, err := s.cfg.Campaigns.Add(r.Context(), campaign.Input{
		Dir: req.Dir, Name: req.Name, Remote: req.Remote, Labels: req.Labels,
	})
	if err != nil {
		campaignErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleCampaignUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   *string   `json:"name"`
		Remote *string   `json:"remote"`
		Labels *[]string `json:"labels"`
		Demo   *bool     `json:"demo"`
	}
	if !decode(w, r, &req) {
		return
	}
	c, err := s.cfg.Campaigns.Update(r.Context(), r.PathValue("id"), campaign.Patch{
		Name: req.Name, Remote: req.Remote, Labels: req.Labels, Demo: req.Demo,
	})
	if err != nil {
		campaignErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleCampaignRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.cfg.Campaigns.Get(id); !ok {
		httpErr(w, http.StatusNotFound, "no such campaign")
		return
	}
	for _, v := range s.sessionViews() {
		if v.Campaign == id && !v.Exited {
			httpErr(w, http.StatusConflict, "close this campaign's running sessions first")
			return
		}
	}
	if _, err := s.cfg.Campaigns.Remove(id); err != nil {
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.cfg.History.RemoveCampaign(id)
	w.WriteHeader(http.StatusNoContent)
}

func campaignErr(w http.ResponseWriter, err error) {
	var ve campaign.ValidationError
	switch {
	case errors.As(err, &ve):
		httpErr(w, http.StatusBadRequest, ve.Error())
	case errors.Is(err, campaign.ErrDuplicate):
		httpErr(w, http.StatusConflict, "that directory is already a campaign")
	case errors.Is(err, campaign.ErrNotFound):
		httpErr(w, http.StatusNotFound, "no such campaign")
	default:
		httpErr(w, http.StatusInternalServerError, err.Error())
	}
}

// --- sessions ---

type createReq struct {
	// Campaign is the id of the campaign whose directory the session runs in.
	Campaign string `json:"campaign"`
	// Hero is a hero id. Empty with Shell true starts a plain shell.
	Hero   string `json:"hero"`
	Shell  bool   `json:"shell"`
	Opener string `json:"opener"`
	Cols   int    `json:"cols"`
	Rows   int    `json:"rows"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if !decode(w, r, &req) {
		return
	}
	camp, ok := s.cfg.Campaigns.Get(req.Campaign)
	if !ok {
		httpErr(w, http.StatusBadRequest, "unknown campaign")
		return
	}
	if err := checkDir(camp.Dir); err != nil {
		httpErr(w, http.StatusBadRequest, "campaign directory is no longer available")
		return
	}
	if len(req.Opener) > maxOpener {
		httpErr(w, http.StatusBadRequest, "opener too long")
		return
	}
	cols, rows := req.Cols, req.Rows
	if cols <= 0 || rows <= 0 || cols > 1000 || rows > 1000 {
		cols, rows = defCols, defRows
	}

	var spec session.Spec
	switch {
	case req.Shell && req.Hero == "":
		if s.cfg.Shell == "" {
			httpErr(w, http.StatusServiceUnavailable, "no shell configured")
			return
		}
		spec = session.Spec{
			Argv: []string{s.cfg.Shell},
			Env:  launcher.ShellEnviron(s.cfg.BaseEnv()),
			Meta: map[string]string{"kind": "shell", "title": "shell"},
		}
	case req.Hero != "" && !req.Shell:
		h, ok := s.cfg.Party.Hero(req.Hero)
		if !ok {
			httpErr(w, http.StatusBadRequest, "unknown hero")
			return
		}
		if s.cfg.Claude == "" {
			httpErr(w, http.StatusServiceUnavailable, "claude not found: "+s.cfg.ClaudeErr)
			return
		}
		spec = heroSpec(s.cfg, h, req.Opener)
	default:
		httpErr(w, http.StatusBadRequest, "give either hero or shell")
		return
	}
	spec.Dir, spec.Cols, spec.Rows = camp.Dir, cols, rows

	sess, err := s.cfg.Sessions.Spawn(spec)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	rec := history.Record{
		ID: sess.ID, Campaign: camp.ID, Kind: spec.Meta["kind"], Hero: spec.Meta["hero"],
		Title: spec.Meta["title"], Dir: camp.Dir, Created: sess.Created.UTC(),
	}
	if err := s.cfg.History.Add(rec); err != nil {
		s.cfg.Sessions.Remove(sess.ID)
		httpErr(w, http.StatusInternalServerError, "could not record the session: "+err.Error())
		return
	}
	go func() {
		<-sess.Done()
		_, code := sess.Exited()
		_ = s.cfg.History.Finish(sess.ID, code)
	}()

	for _, v := range s.sessionViews() {
		if v.ID == sess.ID {
			writeJSON(w, http.StatusCreated, v)
			return
		}
	}
	httpErr(w, http.StatusInternalServerError, "session vanished")
}

func heroSpec(cfg Config, h party.Hero, opener string) session.Spec {
	return session.Spec{
		Argv: launcher.Command(cfg.Claude, h, launcher.Opts{Opener: opener}),
		Env:  launcher.Environ(cfg.BaseEnv(), h),
		Meta: map[string]string{"kind": "hero", "hero": h.ID, "title": h.Label},
	}
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	live := s.cfg.Sessions.Remove(id)
	rec, err := s.cfg.History.Remove(id)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !live && !rec {
		httpErr(w, http.StatusNotFound, "no such session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

// decode reads a strict JSON body: size-capped, no unknown fields.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		httpErr(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func checkDir(dir string) error {
	if dir == "" || !filepath.IsAbs(dir) {
		return errors.New("dir must be an absolute path")
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return errors.New("dir is not an existing directory")
	}
	return nil
}

func httpErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// httpErrCode adds a machine-readable code the frontend can act on.
func httpErrCode(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}
