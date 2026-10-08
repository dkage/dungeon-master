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
	"regexp"
	"strings"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/fsutil"
	"github.com/dkage/dungeon-master/internal/gatekeeper"
	"github.com/dkage/dungeon-master/internal/history"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/quartermaster"
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
	// Quartermaster is the gatekeeper for GitHub traffic; its status is
	// served here and it paces the writes of sessions' gh commands. May be nil.
	Quartermaster *quartermaster.Quartermaster
	// Gatekeeper puts the gh shim in front of the sessions it starts. Nil
	// means sessions run with the plain gh.
	Gatekeeper *Gatekeeper
}

// Gatekeeper configures the gh shim for sessions.
type Gatekeeper struct {
	// BinDir holds the shim, and goes first on every session's PATH.
	BinDir string
	// RealGH is the gh the shim runs.
	RealGH string
	// Tokens are the per-session credentials; created when nil.
	Tokens *gatekeeper.Tokens
}

// Server wires the handler, the guard and the listener together.
type Server struct {
	cfg  Config
	http *http.Server
	ln   net.Listener
	g    *guard
	url  string // where this server can be reached, set once it listens

	// The Gatekeeper answers the shim on its own private socket, never on
	// the browser-facing port.
	gateSrv  *http.Server
	gateLn   net.Listener
	gateSock string
}

// TokenFile is where the access token is kept inside the config directory.
const TokenFile = "token"

var tokenPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// LoadOrCreateToken returns the access token stored at path, creating one if
// there is none, or replacing it when rotate is set or the file is not a
// valid token. Keeping the token between runs means a browser tab and its
// cookie survive a restart; the file is private to the user (0600), like the
// rest of Dungeon Master's state. created reports whether a new token was
// written.
func LoadOrCreateToken(path string, rotate bool) (token string, created bool, err error) {
	if !rotate {
		if b, rerr := os.ReadFile(path); rerr == nil {
			if t := strings.TrimSpace(string(b)); tokenPattern.MatchString(t) {
				return t, false, nil
			}
		} else if !errors.Is(rerr, os.ErrNotExist) {
			return "", false, rerr
		}
	}
	token, err = NewToken()
	if err != nil {
		return "", false, err
	}
	if err := fsutil.WriteFileAtomic(path, []byte(token+"\n")); err != nil {
		return "", false, err
	}
	return token, true, nil
}

// NewToken returns a fresh random token.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CheckAddr reports whether addr is a host:port on a loopback address. Callers
// use it to refuse a bad address before they touch any state.
func CheckAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("server: bad address %q: %w", addr, err)
	}
	if !isLoopbackHost(host) {
		return fmt.Errorf("server: %q is not a loopback address", host)
	}
	return nil
}

// New validates the config. It does not listen yet.
func New(cfg Config) (*Server, error) {
	if cfg.Token == "" {
		return nil, errors.New("server: empty token")
	}
	if err := CheckAddr(cfg.Addr); err != nil {
		return nil, err
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
	if cfg.Gatekeeper != nil && cfg.Gatekeeper.Tokens == nil {
		g := *cfg.Gatekeeper
		g.Tokens = gatekeeper.NewTokens()
		cfg.Gatekeeper = &g
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
	s.url = "http://" + ln.Addr().String()
	s.g = newGuard(ln.Addr().(*net.TCPAddr).Port, s.cfg.Token)
	if err := s.startGate(); err != nil {
		// Never a reason not to start: sessions just run the plain gh.
		fmt.Fprintln(os.Stderr, "warning: gh writes in sessions will not be paced:", err)
		s.cfg.Gatekeeper = nil
	}
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
	s.stopGate(ctx)
	return s.http.Shutdown(ctx)
}

// Handler returns the guarded handler for a listener bound on the given
// port. It exists so tests can drive the server without a real socket.
func Handler(cfg Config, port int) http.Handler { return handlerServer(cfg, port).handler() }

func handlerServer(cfg Config, port int) *Server {
	return &Server{cfg: withDefaults(cfg), g: newGuard(port, cfg.Token), url: fmt.Sprintf("http://127.0.0.1:%d", port)}
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

func (noGitHub) List(context.Context, quests.Repo, campaign.Scope) (quests.Listing, error) {
	return quests.Listing{}, quests.ErrNoToken
}

func (noGitHub) Counts(context.Context, quests.Repo) (quests.Counts, error) {
	return quests.Counts{}, quests.ErrNoToken
}

func (noGitHub) Get(context.Context, quests.Repo, int) (quests.Detail, error) {
	return quests.Detail{}, quests.ErrNoToken
}
