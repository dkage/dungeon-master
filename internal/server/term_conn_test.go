package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
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

// The page asks with a plain GET why a reattach failed: each answer must tell
// the cases apart without the token appearing in it.
func TestTermProbeAnswers(t *testing.T) {
	e, _ := connServer(t, time.Hour, time.Hour)
	sj := e.create(map[string]any{"shell": true})
	for name, tc := range map[string]struct {
		path string
		auth string
		want int
	}{
		"live session": {"/api/sessions/" + sj.ID + "/term", "Bearer " + e.token, http.StatusUpgradeRequired},
		"unknown":      {"/api/sessions/snope/term", "Bearer " + e.token, http.StatusNotFound},
		"wrong token":  {"/api/sessions/" + sj.ID + "/term", "Bearer wrong", http.StatusUnauthorized},
	} {
		req, _ := http.NewRequest("GET", e.base+tc.path, nil)
		req.Header.Set("Authorization", tc.auth)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("%s: got %d, want %d", name, resp.StatusCode, tc.want)
		}
		if strings.Contains(string(body), e.token) {
			t.Errorf("%s: body leaks the token", name)
		}
	}
}

type attachFrame struct {
	Type   string
	Start  uint64
	Resume bool
}

// dialFrom attaches asking to resume at stream position from (negative: no position).
func (e *env) dialFrom(t *testing.T, id string, from int64) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	u := "ws" + strings.TrimPrefix(e.base, "http") + "/api/sessions/" + id + "/term"
	if from >= 0 {
		u += "?from=" + strconv.FormatInt(from, 10)
	}
	c, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPHeader: http.Header{
		"Authorization": {"Bearer " + e.token}, "Origin": {"http://" + e.host}}})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// readAttach reads the attach frame that opens every connection.
func readAttach(t *testing.T, c *websocket.Conn) attachFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	typ, data, err := c.Read(ctx)
	if err != nil || typ != websocket.MessageText {
		t.Fatalf("attach frame: %v %v", typ, err)
	}
	var f attachFrame
	if json.Unmarshal(data, &f) != nil || f.Type != "attach" {
		t.Fatalf("attach frame: %s", data)
	}
	return f
}

// A page that was cut mid-output reattaches with the position it reached and
// receives only the bytes it missed: no gap and no repeat.
func TestTermReattachReplaysOnlyWhatWasMissed(t *testing.T) {
	e, _ := connServer(t, time.Hour, time.Hour)
	sj := e.create(map[string]any{"shell": true})
	c := e.dialFrom(t, sj.ID, -1)
	first := readAttach(t, c)
	if first.Resume {
		t.Fatal("a fresh attach must not claim to resume")
	}
	send(t, c, "echo before-cut\n")
	// The output line, not the echo of the typed command (which also holds the word).
	seen := readUntil(t, c, "\r\nbefore-cut\r\n")
	pos := first.Start + uint64(len(seen))
	c.CloseNow()

	// Output the page misses while it is away.
	c2 := e.dialFrom(t, sj.ID, -1)
	readAttach(t, c2)
	send(t, c2, "echo while-away\n")
	readUntil(t, c2, "\r\nwhile-away\r\n")
	c2.CloseNow()

	c3 := e.dialFrom(t, sj.ID, int64(pos))
	defer c3.CloseNow()
	f := readAttach(t, c3)
	if !f.Resume || f.Start != pos {
		t.Fatalf("attach frame %+v, want a resume at %d", f, pos)
	}
	got := readUntil(t, c3, "while-away")
	if strings.Contains(got, "before-cut") {
		t.Errorf("replay repeats output the page already had: %q", got)
	}

	// A position the scrollback no longer holds (or never had) gets the whole scrollback.
	c4 := e.dialFrom(t, sj.ID, 1<<40)
	defer c4.CloseNow()
	if f := readAttach(t, c4); f.Resume {
		t.Errorf("an unknown position must not resume: %+v", f)
	}
	readUntil(t, c4, "before-cut")
}
