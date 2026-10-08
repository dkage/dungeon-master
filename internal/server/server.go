// Package server is the local HTTP surface of Dungeon Master. It binds to loopback
// only and treats every request as untrusted until it passes the guard.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/history"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/quests"
	"github.com/dkage/dungeon-master/internal/quests/demo"
	"github.com/dkage/dungeon-master/internal/session"
)

// Config is everything New needs.
type Config struct {
	// Addr is a loopback host:port. Port 0 picks a free port.
	Addr string
	// Token is the per-launch secret exchanged for a session cookie.
	Token string
	// Version is reported by the health endpoint.
	Version string
	// Assets is the built frontend. May be nil or lack index.html, in which
	// case a built-in page is served.
	Assets fs.FS

	// Party is the loaded hero set.
	Party party.Party
	// Claude is the resolved claude binary; empty when it was not found, in
	// which case ClaudeErr says why and hero launches are refused.
	Claude    string
	ClaudeErr string
	// Shell is the user's shell for free terminals.
	Shell string
	// Sessions runs the PTYs. New creates one when nil.
	Sessions *session.Manager
	// BaseEnv is the environment children inherit. Defaults to os.Environ.
	BaseEnv func() []string
	// Campaigns is the registry of working directories. New creates an
	// in-memory one when nil.
	Campaigns *campaign.Registry
	// History records sessions across restarts. In-memory when nil.
	History *history.Store
	// Quests supplies issues. When nil, only the demo quests are available.
	Quests *quests.Service
}

// Server wires the handler, the guard and the listener together.
type Server struct {
	cfg  Config
	http *http.Server
	ln   net.Listener
	g    *guard
}

// NewToken returns a fresh random token.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// New validates the config. It does not listen yet.
func New(cfg Config) (*Server, error) {
	if cfg.Token == "" {
		return nil, errors.New("server: empty token")
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("server: bad address %q: %w", cfg.Addr, err)
	}
	if !isLoopbackHost(host) {
		return nil, fmt.Errorf("server: %q is not a loopback address", host)
	}
	cfg = withDefaults(cfg)
	return &Server{cfg: cfg}, nil
}

func withDefaults(cfg Config) Config {
	if cfg.Sessions == nil {
		cfg.Sessions = session.NewManager()
	}
	if cfg.BaseEnv == nil {
		cfg.BaseEnv = os.Environ
	}
	if cfg.Campaigns == nil {
		cfg.Campaigns, _ = campaign.Open(context.Background(), "", nil) // in-memory: cannot fail
	}
	if cfg.History == nil {
		cfg.History, _ = history.Open("") // in-memory: cannot fail
	}
	if cfg.Quests == nil {
		cfg.Quests = quests.NewService(quests.ServiceOptions{GitHub: noGitHub{}, Demo: demo.New()})
	}
	return cfg
}

// Listen binds the address and finishes building the handler, now that the
// real port is known.
func (s *Server) Listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return nil, err
	}
	s.ln = ln
	s.g = newGuard(ln.Addr().(*net.TCPAddr).Port, s.cfg.Token)
	s.http = &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return ln, nil
}

// Serve blocks serving on ln until Shutdown.
func (s *Server) Serve(ln net.Listener) error { return s.http.Serve(ln) }

// Shutdown stops the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	s.cfg.Sessions.Shutdown()
	return s.http.Shutdown(ctx)
}

// Handler returns the guarded handler for a listener bound on the given
// port. It exists so tests can drive the server without a real socket.
func Handler(cfg Config, port int) http.Handler {
	s := &Server{cfg: withDefaults(cfg), g: newGuard(port, cfg.Token)}
	return s.handler()
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.cfg.Version})
	})
	s.routes(mux)
	mux.Handle("/api/", http.NotFoundHandler())
	mux.Handle("/", s.static())
	return s.g.wrap(mux)
}

func (s *Server) static() http.Handler {
	if s.cfg.Assets != nil {
		if _, err := fs.Stat(s.cfg.Assets, "index.html"); err == nil {
			return http.FileServerFS(s.cfg.Assets)
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, fallbackPage)
	})
}

const fallbackPage = `<!doctype html><html lang="en"><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Dungeon Master</title>
<body style="font:16px/1.5 system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 16px">
<h1>Dungeon Master</h1><p>The server is running, but the frontend is not built. Run <code>make web</code> and restart.</p>`

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func isLoopbackHost(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// noGitHub is the GitHub source of a server started without one.
type noGitHub struct{}

func (noGitHub) List(context.Context, quests.Repo) ([]quests.Quest, error) {
	return nil, quests.ErrNoToken
}

func (noGitHub) Get(context.Context, quests.Repo, int) (quests.Detail, error) {
	return quests.Detail{}, quests.ErrNoToken
}
