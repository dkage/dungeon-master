package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dkage/dungeon-master/internal/briefing"
	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/fsutil"
	"github.com/dkage/dungeon-master/internal/history"
	"github.com/dkage/dungeon-master/internal/launcher"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/platform"
	"github.com/dkage/dungeon-master/internal/quests"
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
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/heroes/config", s.handleHeroesConfig)
	mux.HandleFunc("PUT /api/heroes/config", s.handleSaveHeroesConfig)
	mux.HandleFunc("POST /api/heroes/adopt-starters", s.handleAdoptStarterHeroes)
	mux.HandleFunc("GET /api/github", s.handleGitHub)
	mux.HandleFunc("GET /api/onboarding/suggestions", s.handleSuggestions)
	mux.HandleFunc("POST /api/onboarding/sample", s.handleSampleCampaign)
	mux.HandleFunc("POST /api/pick-folder", s.handlePickFolder)

	mux.HandleFunc("POST /api/campaigns", s.handleCampaignAdd)
	mux.HandleFunc("PATCH /api/campaigns/{id}", s.handleCampaignUpdate)
	mux.HandleFunc("DELETE /api/campaigns/{id}", s.handleCampaignRemove)
	mux.HandleFunc("POST /api/campaigns/{id}/open-folder", s.handleOpenFolder)
	mux.HandleFunc("PUT /api/campaigns/{id}/scope", s.handleScopeSet)
	mux.HandleFunc("GET /api/campaigns/{id}/scope/options", s.handleScopeOptions)
	mux.HandleFunc("GET /api/campaigns/{id}/quests", s.handleQuests)
	mux.HandleFunc("GET /api/campaigns/{id}/quests/{number}", s.handleQuest)
	mux.HandleFunc("POST /api/campaigns/{id}/quests/{number}/launch-preview", s.handleQuestLaunchPreview)

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
	Quest    int       `json:"quest,omitempty"`
	Title    string    `json:"title"`
	Campaign string    `json:"campaign"`
	Dir      string    `json:"dir"`
	Created  time.Time `json:"created"`
	Exited   bool      `json:"exited"`
	ExitCode int       `json:"exitCode"`
	// Activity is working, idle, needs-you or ended; only live sessions have one.
	Activity string `json:"activity,omitempty"`
	// Lost is a session that was running when Dungeon Master last stopped; its
	// terminal is gone and only the record remains.
	Lost bool `json:"lost,omitempty"`
}

type stateJSON struct {
	Version     string              `json:"version"`
	Claude      claudeJSON          `json:"claude"`
	Defaulted   bool                `json:"defaultHeroes"`
	DefaultHero string              `json:"defaultHero"`
	PartyError  string              `json:"partyError,omitempty"`
	Heroes      []heroJSON          `json:"heroes"`
	Campaigns   []campaign.Campaign `json:"campaigns"`
	Sessions    []sessionJSON       `json:"sessions"`
}

type heroConfigJSON struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Args  []string `json:"args"`
	Env   []string `json:"env"`
}

type heroesConfigJSON struct {
	DefaultHero string           `json:"defaultHero"`
	Heroes      []heroConfigJSON `json:"heroes"`
}

type saveHeroesReq struct {
	DefaultHero string           `json:"defaultHero"`
	Heroes      []heroConfigJSON `json:"heroes"`
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
			ID: r.ID, Kind: r.Kind, Hero: r.Hero, Quest: r.Quest, Title: r.Title, Campaign: r.Campaign,
			Dir: r.Dir, Created: r.Created, Lost: r.Lost,
			Exited: r.Ended != nil, ExitCode: r.ExitCode,
		}
		if live, ok := s.cfg.Sessions.Get(r.ID); ok {
			v.Exited, v.ExitCode = live.Exited()
			v.Activity = string(live.Activity())
		}
		out = append(out, v)
	}
	return out
}

// handleSessions is the cheap list the browser polls to see which sessions are
// working or waiting; it touches no campaigns, quests or GitHub.
func (s *Server) handleSessions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.sessionViews())
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	s.partyMu.RLock()
	partyState := s.cfg.Party
	partyError := s.cfg.PartyError
	s.partyMu.RUnlock()
	st := stateJSON{
		Version:     s.cfg.Version,
		Claude:      claudeJSON{Found: s.cfg.Claude != "", Path: s.cfg.Claude, Error: s.cfg.ClaudeErr},
		Defaulted:   partyState.Defaulted,
		DefaultHero: partyState.DefaultHero,
		PartyError:  partyError,
		Heroes:      []heroJSON{},
		Campaigns:   s.cfg.Campaigns.List(),
		Sessions:    s.sessionViews(),
	}
	for _, h := range partyState.Heroes {
		st.Heroes = append(st.Heroes, heroJSON{ID: h.ID, Label: h.Label})
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleHeroesConfig(w http.ResponseWriter, _ *http.Request) {
	s.partyMu.RLock()
	p := s.cfg.Party
	s.partyMu.RUnlock()
	out := heroesConfigJSON{DefaultHero: p.DefaultHero, Heroes: make([]heroConfigJSON, 0, len(p.Heroes))}
	for _, h := range p.Heroes {
		masked := make([]string, 0, len(h.Env))
		for _, entry := range h.Env {
			key, _, _ := strings.Cut(entry, "=")
			masked = append(masked, key+"=")
		}
		out.Heroes = append(out.Heroes, heroConfigJSON{ID: h.ID, Label: h.Label, Args: h.Args, Env: masked})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSaveHeroesConfig(w http.ResponseWriter, r *http.Request) {
	var req saveHeroesReq
	if !decode(w, r, &req) {
		return
	}
	if len(req.Heroes) == 0 {
		httpErr(w, http.StatusBadRequest, "at least one hero is required")
		return
	}
	s.partyMu.Lock()
	defer s.partyMu.Unlock()
	if s.cfg.PartyPath == "" {
		httpErr(w, http.StatusServiceUnavailable, "hero settings cannot be saved in this server configuration")
		return
	}
	// An empty value means "keep the saved value" for a key that already
	// exists. The browser never receives saved environment values.
	previous := make(map[string]map[string]string, len(s.cfg.Party.Heroes))
	for _, h := range s.cfg.Party.Heroes {
		values := make(map[string]string, len(h.Env))
		for _, entry := range h.Env {
			key, value, _ := strings.Cut(entry, "=")
			values[key] = value
		}
		previous[h.ID] = values
	}
	updated := party.Party{Claude: s.cfg.Party.Claude, DefaultHero: req.DefaultHero, Heroes: make([]party.Hero, 0, len(req.Heroes))}
	for _, h := range req.Heroes {
		values := previous[h.ID]
		env := make([]string, 0, len(h.Env))
		for _, entry := range h.Env {
			key, value, _ := strings.Cut(entry, "=")
			if value == "" {
				if saved, ok := values[key]; ok {
					value = saved
				}
			}
			env = append(env, key+"="+value)
		}
		updated.Heroes = append(updated.Heroes, party.Hero{ID: h.ID, Label: h.Label, Args: h.Args, Env: env})
	}
	data, err := party.Encode(updated)
	if err != nil {
		httpErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := fsutil.WriteFileAtomic(s.cfg.PartyPath, data); err != nil {
		httpErr(w, http.StatusInternalServerError, "could not save hero settings")
		return
	}
	updated.Defaulted = false
	s.cfg.Party = updated
	s.cfg.PartyError = ""
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) handleAdoptStarterHeroes(w http.ResponseWriter, _ *http.Request) {
	s.partyMu.Lock()
	defer s.partyMu.Unlock()
	if !s.cfg.Party.Defaulted {
		httpErr(w, http.StatusConflict, "starter heroes are no longer active")
		return
	}
	if s.cfg.PartyPath == "" {
		httpErr(w, http.StatusServiceUnavailable, "hero settings cannot be saved in this server configuration")
		return
	}
	data, err := party.Encode(s.cfg.Party)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "could not prepare starter hero settings")
		return
	}
	if err := fsutil.CreateFileAtomic(s.cfg.PartyPath, data); err != nil {
		if errors.Is(err, os.ErrExist) {
			httpErr(w, http.StatusConflict, "hero settings already exist; restart Dungeon Master to reload them")
			return
		}
		httpErr(w, http.StatusInternalServerError, "could not save starter hero settings")
		return
	}
	s.cfg.Party.Defaulted = false
	s.cfg.PartyError = ""
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

// --- campaigns ---

type campaignReq struct {
	Dir    string `json:"dir"`
	Name   string `json:"name"`
	Remote string `json:"remote"`
}

func (s *Server) handleCampaignAdd(w http.ResponseWriter, r *http.Request) {
	var req campaignReq
	if !decode(w, r, &req) {
		return
	}
	c, err := s.cfg.Campaigns.Add(r.Context(), campaign.Input{
		Dir: req.Dir, Name: req.Name, Remote: req.Remote,
	})
	if err != nil {
		if errors.Is(err, campaign.ErrDuplicate) {
			if wanted, statErr := os.Stat(req.Dir); statErr == nil {
				for _, existing := range s.cfg.Campaigns.List() {
					if current, currentErr := os.Stat(existing.Dir); currentErr == nil && os.SameFile(wanted, current) {
						writeJSON(w, http.StatusOK, existing)
						return
					}
				}
			}
		}
		campaignErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if !decode(w, r, &req) {
		return
	}
	if !s.pickerMu.TryLock() {
		httpErrCode(w, http.StatusConflict, "picker_busy", "A folder picker is already open.")
		return
	}
	defer s.pickerMu.Unlock()
	pick := s.cfg.PickFolder
	if pick == nil {
		pick = pickFolder
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	path, err := pick(ctx)
	if errors.Is(err, errPickerCancelled) {
		writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
		return
	}
	if errors.Is(err, errPickerUnavailable) {
		httpErrCode(w, http.StatusNotImplemented, "picker_unavailable", "The native folder picker is unavailable. Type a project path instead.")
		return
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			httpErrCode(w, http.StatusGatewayTimeout, "picker_timeout", "The folder picker timed out.")
		} else {
			httpErr(w, http.StatusInternalServerError, "could not open the folder picker")
		}
		return
	}
	path, err = filepath.Abs(path)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "the folder picker returned an invalid path")
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		httpErr(w, http.StatusBadRequest, "the selected path is not an existing directory")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (s *Server) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if !decode(w, r, &req) {
		return
	}
	c, ok := s.cfg.Campaigns.Get(r.PathValue("id"))
	if !ok {
		httpErr(w, http.StatusNotFound, "no such campaign")
		return
	}
	info, err := os.Stat(c.Dir)
	if err != nil || !info.IsDir() {
		httpErr(w, http.StatusBadRequest, "campaign folder is no longer available")
		return
	}
	open := s.cfg.OpenFolder
	if open == nil {
		open = openFolder
	}
	if err := open(r.Context(), c.Dir); err != nil {
		httpErr(w, http.StatusServiceUnavailable, "could not open this folder in the file manager")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCampaignUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   *string         `json:"name"`
		Remote *string         `json:"remote"`
		Scope  *campaign.Scope `json:"scope"`
		Demo   *bool           `json:"demo"`
	}
	if !decode(w, r, &req) {
		return
	}
	c, err := s.cfg.Campaigns.Update(r.Context(), r.PathValue("id"), campaign.Patch{
		Name: req.Name, Remote: req.Remote, Scope: req.Scope, Demo: req.Demo,
	})
	if err != nil {
		campaignErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleCampaignRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	camp, ok := s.cfg.Campaigns.Get(id)
	if !ok {
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
	// Only the app-created scratch directory is removed with the sample campaign.
	// Ordinary campaign directories are never touched when they are unregistered.
	if camp.Sample && s.cfg.SampleDir != "" && platform.SamePath(camp.Dir, s.cfg.SampleDir) {
		if err := os.RemoveAll(s.cfg.SampleDir); err != nil {
			httpErr(w, http.StatusInternalServerError, "campaign removed, but the sample workspace could not be cleaned up")
			return
		}
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
	Quest  int    `json:"quest,omitempty"`
	// BriefingID refers only to a preview created by this server for the same
	// campaign, quest, and hero; it is never interpreted as a filesystem path.
	BriefingID string `json:"briefingId,omitempty"`
	Cols       int    `json:"cols"`
	Rows       int    `json:"rows"`
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
	if req.Quest < 0 || (req.Quest == 0 && req.BriefingID != "") {
		httpErr(w, http.StatusBadRequest, "invalid quest briefing reference")
		return
	}
	if req.Quest > 0 && (req.Hero == "" || req.Shell) {
		httpErr(w, http.StatusBadRequest, "a quest session requires a hero")
		return
	}
	if req.Quest > 0 {
		// Serialize the check and spawn so two browser requests cannot race
		// past the one-active-quest-session-per-worktree rule.
		s.questLaunchMu.Lock()
		defer s.questLaunchMu.Unlock()
	}
	cols, rows := req.Cols, req.Rows
	if cols <= 0 || rows <= 0 || cols > 1000 || rows > 1000 {
		cols, rows = defCols, defRows
	}

	var spec session.Spec
	questNumber := 0
	switch {
	case req.Shell && req.Hero == "":
		if s.cfg.Shell == "" {
			httpErr(w, http.StatusServiceUnavailable, "no shell configured")
			return
		}
		spec = s.shellSpecWithGate()
	case req.Hero != "" && !req.Shell:
		s.partyMu.RLock()
		h, ok := s.cfg.Party.Hero(req.Hero)
		s.partyMu.RUnlock()
		if !ok {
			httpErr(w, http.StatusBadRequest, "unknown hero")
			return
		}
		if s.cfg.Claude == "" {
			httpErr(w, http.StatusServiceUnavailable, "claude not found: "+s.cfg.ClaudeErr)
			return
		}
		if req.Quest > 0 {
			if req.Opener != "" {
				httpErr(w, http.StatusBadRequest, "quest sessions use their generated briefing opener")
				return
			}
			d, err := s.cfg.Quests.Detail(r.Context(), camp, req.Quest)
			if err != nil {
				if errors.Is(err, quests.ErrNotFound) {
					httpErrCode(w, http.StatusNotFound, "not_found", "That issue was not found.")
					return
				}
				questsErr(w, err, camp.Repo)
				return
			}
			if reason := questStartBlock(d); reason != "" {
				httpErrCode(w, http.StatusConflict, "quest_not_startable", reason)
				return
			}
			if existing := s.activeQuestSessionFor(camp); existing != nil {
				httpErrCode(w, http.StatusConflict, "quest_session_active", fmt.Sprintf("%s is already working in this Git working tree.", existing.Title))
				return
			}
			plan, err := s.questLaunchPlan(camp, d, h, req.BriefingID)
			if err != nil {
				if req.BriefingID != "" {
					httpErrCode(w, http.StatusConflict, "preview_expired", "The briefing preview is no longer available. Preview it again before starting.")
				} else {
					httpErr(w, http.StatusInternalServerError, "could not prepare the private task briefing")
				}
				return
			}
			spec = s.heroSpecWithOpts(h, launcher.Opts{
				Opener: plan.opener, AppendSystemPrompt: briefing.Instructions,
				AddDirs: []string{filepath.Dir(plan.path)},
			})
			spec.Meta["title"] = fmt.Sprintf("#%d %s", d.Number, d.Title)
			spec.Meta["quest"] = strconv.Itoa(d.Number)
			questNumber = d.Number
		} else {
			spec = s.heroSpecWithGate(h, req.Opener)
		}
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
		Quest: questNumber, Title: spec.Meta["title"], Dir: camp.Dir, Created: sess.Created.UTC(),
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
		if g := s.cfg.Gatekeeper; g != nil {
			g.Tokens.Revoke(sess.ID) // a finished session's shim token stops working
		}
	}()

	for _, v := range s.sessionViews() {
		if v.ID == sess.ID {
			writeJSON(w, http.StatusCreated, v)
			return
		}
	}
	httpErr(w, http.StatusInternalServerError, "session vanished")
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
