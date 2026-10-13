package server

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type logBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func (l *logBuf) waitFor(t *testing.T, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := l.String(); strings.Contains(s, want) {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("log never contained %q:\n%s", want, l.String())
	return ""
}

func connServer(t *testing.T, ping, timeout time.Duration) (*env, *logBuf) {
	lb := &logBuf{}
	e := startServer(t, func(c *Config) {
		c.Log = slog.New(slog.NewTextHandler(lb, nil))
		c.TermPing, c.TermPingTimeout = ping, timeout
	})
	return e, lb
}

// An idle connection that answers pings stays open well past the ping
// interval, and still reaches the session.
func TestTermIdleConnectionStaysOpen(t *testing.T) {
	e, lb := connServer(t, 40*time.Millisecond, time.Second)
	sj := e.create(map[string]any{"shell": true})
	c, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	// Read in the background so pongs are answered while idle.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	}()
	time.Sleep(400 * time.Millisecond)
	if strings.Contains(lb.String(), "terminal connection closed") {
		t.Fatalf("idle connection was closed:\n%s", lb.String())
	}
	send(t, c, "echo still-here\n")
}

// A peer that never answers pings is closed, and the session survives and
// can be attached again with its scrollback.
func TestTermSilentPeerClosedSessionSurvives(t *testing.T) {
	e, lb := connServer(t, 30*time.Millisecond, 100*time.Millisecond)
	sj := e.create(map[string]any{"shell": true})
	c, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	send(t, c, "echo marker-one\n") // written but never read back: the peer is silent
	line := lb.waitFor(t, "peer stopped answering pings")
	if !strings.Contains(line, "session="+sj.ID) {
		t.Errorf("log lacks the session id: %s", line)
	}
	c2, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.CloseNow()
	readUntil(t, c2, "marker-one")
}

func TestTermCloseCausesAreLogged(t *testing.T) {
	e, lb := connServer(t, time.Hour, time.Hour)

	sj := e.create(map[string]any{"shell": true})
	c, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	secret := "typed-secret-text"
	send(t, c, "echo "+secret+"\n")
	readUntil(t, c, secret)
	_ = c.Close(websocket.StatusNormalClosure, "bye")
	out := lb.waitFor(t, "cause=\"client closed\"")
	for _, bad := range []string{secret, e.token} {
		if strings.Contains(out, bad) {
			t.Errorf("log leaks %q:\n%s", bad, out)
		}
	}

	// Abrupt drop: no close frame.
	c, _, err = e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	readUntil(t, c, secret)
	c.CloseNow()
	lb.waitFor(t, "cause=\"read error\"")

	// The process ends while attached.
	sj2 := e.create(map[string]any{"shell": true})
	c, _, err = e.dial(sj2.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	send(t, c, "echo ready-now\n")
	readUntil(t, c, "ready-now")
	send(t, c, "exit 3\n")
	go func() { // answer the close handshake
		for {
			if _, _, err := c.Read(context.Background()); err != nil {
				return
			}
		}
	}()
	lb.waitFor(t, "cause=\"session exited\"")
	if !strings.Contains(lb.String(), "lasted=") || !strings.Contains(lb.String(), "silent=") {
		t.Errorf("log lacks durations:\n%s", lb.String())
	}
}
