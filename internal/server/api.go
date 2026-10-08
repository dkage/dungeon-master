package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"dungeon/internal/launcher"
	"dungeon/internal/party"
	"dungeon/internal/session"
)

const (
	maxBody   = 64 << 10
	maxOpener = 4096
	defCols   = 120
	defRows   = 32
)

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", s.handleState)
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
	Dir      string    `json:"dir"`
	Created  time.Time `json:"created"`
	Exited   bool      `json:"exited"`
	ExitCode int       `json:"exitCode"`
}

type stateJSON struct {
	Version   string        `json:"version"`
	Claude    claudeJSON    `json:"claude"`
	Defaulted bool          `json:"defaultHeroes"`
	Heroes    []heroJSON    `json:"heroes"`
	Sessions  []sessionJSON `json:"sessions"`
}

type claudeJSON struct {
	Found bool   `json:"found"`
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}

func toSessionJSON(s *session.Session) sessionJSON {
	exited, code := s.Exited()
	return sessionJSON{
		ID: s.ID, Kind: s.Meta["kind"], Hero: s.Meta["hero"], Title: s.Meta["title"],
		Dir: s.Dir, Created: s.Created, Exited: exited, ExitCode: code,
	}
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	st := stateJSON{
		Version:   s.cfg.Version,
		Claude:    claudeJSON{Found: s.cfg.Claude != "", Path: s.cfg.Claude, Error: s.cfg.ClaudeErr},
		Defaulted: s.cfg.Party.Defaulted,
		Heroes:    []heroJSON{},
		Sessions:  []sessionJSON{},
	}
	for _, h := range s.cfg.Party.Heroes {
		st.Heroes = append(st.Heroes, heroJSON{ID: h.ID, Label: h.Label})
	}
	for _, sess := range s.cfg.Sessions.List() {
		st.Sessions = append(st.Sessions, toSessionJSON(sess))
	}
	writeJSON(w, http.StatusOK, st)
}

type createReq struct {
	// Hero is a hero id. Empty with Shell true starts a plain shell.
	Hero   string `json:"hero"`
	Shell  bool   `json:"shell"`
	Dir    string `json:"dir"`
	Opener string `json:"opener"`
	Cols   int    `json:"cols"`
	Rows   int    `json:"rows"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := checkDir(req.Dir); err != nil {
		httpErr(w, http.StatusBadRequest, err.Error())
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
	spec.Dir, spec.Cols, spec.Rows = req.Dir, cols, rows

	sess, err := s.cfg.Sessions.Spawn(spec)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toSessionJSON(sess))
}

func heroSpec(cfg Config, h party.Hero, opener string) session.Spec {
	return session.Spec{
		Argv: launcher.Command(cfg.Claude, h, launcher.Opts{Opener: opener}),
		Env:  launcher.Environ(cfg.BaseEnv(), h),
		Meta: map[string]string{"kind": "hero", "hero": h.ID, "title": h.Label},
	}
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Sessions.Remove(r.PathValue("id")) {
		httpErr(w, http.StatusNotFound, "no such session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
