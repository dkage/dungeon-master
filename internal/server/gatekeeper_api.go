package server

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/dkage/dungeon-master/internal/gatekeeper"
	"github.com/dkage/dungeon-master/internal/launcher"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/quartermaster"
	"github.com/dkage/dungeon-master/internal/session"
)

// gateTokenOK reports whether tok is a live Gatekeeper token.
func (s *Server) gateTokenOK(tok string) bool {
	if s.cfg.Gatekeeper == nil || s.cfg.Gatekeeper.Tokens == nil {
		return false
	}
	_, ok := s.cfg.Gatekeeper.Tokens.Session(tok)
	return ok
}

// startGate listens on the Gatekeeper's private socket. A no-op when the
// Gatekeeper is off.
func (s *Server) startGate() error {
	if s.cfg.Gatekeeper == nil {
		return nil
	}
	ln, path, err := gatekeeper.ListenSocket()
	if err != nil {
		return err
	}
	s.gateSock, s.gateLn = path, ln
	s.gateSrv = &http.Server{Handler: s.gateHandler(), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = s.gateSrv.Serve(ln) }()
	return nil
}

// stopGate closes the socket, which removes its file.
func (s *Server) stopGate(ctx context.Context) {
	if s.gateSrv != nil {
		_ = s.gateSrv.Shutdown(ctx)
		_ = s.gateSrv.Close()
		_ = s.gateLn.Close() // Serve may not have started yet; closing removes the file
	}
}

// gateHandler serves only what the shim needs, to a session's token. It has
// no cookie, no access token and no other route, and a request that carries
// an Origin header (that is, one that comes from a web page) is refused.
func (s *Server) gateHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+gatekeeper.SlotPath, s.handleSlot)
	mux.HandleFunc("POST "+gatekeeper.ReportPath, s.handleReport)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || !s.gateTokenOK(tok) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/json" {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) gateURL() string { return gatekeeper.URLPrefix + s.gateSock }

// sessionEnv adds the shim to a session's environment when the Gatekeeper is
// on. Called with the session's id, which names its token.
func (s *Server) sessionEnv(env []string, id string) []string {
	g := s.cfg.Gatekeeper
	if g == nil {
		return env
	}
	tok, err := g.Tokens.Issue(id)
	if err != nil {
		return env // better an unpaced session than none
	}
	return gatekeeper.SessionEnv{BinDir: g.BinDir, URL: s.gateURL(), Token: tok, RealGH: g.RealGH}.Apply(env)
}

// hero and shell specs get the shim and, for heroes, the rule.
func (s *Server) heroSpecWithGate(h party.Hero, opener string) session.Spec {
	opts := launcher.Opts{Opener: opener}
	if s.cfg.Gatekeeper != nil {
		opts.AppendSystemPrompt = gatekeeper.Rule
	}
	base := s.cfg.BaseEnv()
	return session.Spec{
		Argv:   launcher.Command(s.cfg.Claude, h, opts),
		EnvFor: func(id string) []string { return s.sessionEnv(launcher.Environ(base, h), id) },
		Meta:   map[string]string{"kind": "hero", "hero": h.ID, "title": h.Label},
	}
}

func (s *Server) shellSpecWithGate() session.Spec {
	base := s.cfg.BaseEnv()
	return session.Spec{
		Argv:   []string{s.cfg.Shell},
		EnvFor: func(id string) []string { return s.sessionEnv(launcher.ShellEnviron(base), id) },
		Meta:   map[string]string{"kind": "shell", "title": "shell"},
	}
}

type slotReq struct {
	// Command is the first two words of the gh command, for the log.
	Command string `json:"command"`
}

// handleSlot gives a write slot to a gh command in a session, waiting for it.
// The shim runs gh when it answers 200 and stops when it answers 429.
func (s *Server) handleSlot(w http.ResponseWriter, r *http.Request) {
	var req slotReq
	if !decode(w, r, &req) {
		return
	}
	qm := s.cfg.Quartermaster
	if qm == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "waitedMs": 0})
		return
	}
	waited, err := qm.Reserve(r.Context())
	var camp *quartermaster.CampError
	var busy *quartermaster.TooBusyError
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "waitedMs": waited.Milliseconds()})
	case errors.As(err, &camp):
		httpErrCode(w, http.StatusTooManyRequests, "camp",
			"GitHub asked this app to slow down, so GitHub writes are paused until "+camp.Until.Local().Format("15:04")+".")
	case errors.As(err, &busy):
		httpErrCode(w, http.StatusTooManyRequests, "too_busy",
			"The GitHub write budget is used up for now; the next slot is in "+busy.Wait.Round(time.Second).String()+".")
	default:
		// The caller went away while waiting; nothing was reserved.
		httpErr(w, http.StatusServiceUnavailable, err.Error())
	}
}

type reportReq struct {
	Message    string `json:"message"`
	RetryAfter int    `json:"retryAfter"`
}

// handleReport is told by the shim when gh's own output showed a rate limit.
// The app pauses too, so nothing else writes meanwhile.
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	var req reportReq
	if !decode(w, r, &req) {
		return
	}
	if qm := s.cfg.Quartermaster; qm != nil {
		wait := time.Duration(req.RetryAfter) * time.Second
		qm.Trip(time.Now().Add(wait), "a gh command in a session hit a rate limit")
	}
	w.WriteHeader(http.StatusNoContent)
}
