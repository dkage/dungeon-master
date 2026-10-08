package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/history"
	"github.com/dkage/dungeon-master/internal/party"
	"github.com/dkage/dungeon-master/internal/quests"
	"github.com/dkage/dungeon-master/internal/quests/demo"
	"github.com/dkage/dungeon-master/internal/session"
	"github.com/dkage/dungeon-master/internal/testutil"
)

// env is a real server on a loopback port, wired to the fake claude.
type env struct {
	t        *testing.T
	base     string
	host     string
	dir      string // the directory of the pre-registered campaign
	campaign string // that campaign's id
	token    string
	cfg      Config
	gh       *fakeQuests // the GitHub source behind the quest service
}

func startServer(t *testing.T, mutate func(*Config)) *env {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		Token:   testToken,
		Version: "test",
		Party: party.Party{
			Claude: "claude",
			Heroes: []party.Hero{{
				ID: "opus", Label: "Opus",
				Args: []string{"--model", "m1"},
				Env:  []string{"HERO_VAR=yes"},
			}},
		},
		Claude: testutil.FakeClaude(t),
		Shell:  "/bin/sh",
		BaseEnv: func() []string {
			return append(os.Environ(), "CLAUDE_CODE_CHILD_SESSION=1")
		},
	}
	cfg.Sessions = session.NewManager()
	// Campaigns resolve to github.com/acme/widgets unless the directory is
	// named "plain", so tests need no real repository.
	cfg.Campaigns, _ = campaign.Open(context.Background(), "", fakeGit{})
	gh := &fakeQuests{}
	cfg.Quests = quests.NewService(quests.ServiceOptions{
		GitHub: gh, Demo: demo.New(), TTL: time.Nanosecond, MinRefresh: time.Nanosecond,
	})
	if mutate != nil {
		mutate(&cfg)
	}
	cfg = withDefaults(cfg)
	camp, err := cfg.Campaigns.Add(context.Background(), campaign.Input{Dir: dir, Name: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := Handler(cfg, ln.Addr().(*net.TCPAddr).Port)
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: h}}
	srv.Start()
	t.Cleanup(func() {
		cfg.Sessions.Shutdown()
		srv.Close()
	})
	return &env{t: t, base: srv.URL, host: ln.Addr().String(), dir: dir, campaign: camp.ID, token: testToken, cfg: cfg, gh: gh}
}

func (e *env) req(method, path string, body any, mutate func(*http.Request)) (*http.Response, []byte) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.base+path, rd)
	req.Header.Set("Authorization", "Bearer "+e.token)
	if body != nil || method != http.MethodGet {
		// The guard demands JSON on every write, DELETE included.
		req.Header.Set("Content-Type", "application/json")
	}
	if mutate != nil {
		mutate(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

func (e *env) create(body map[string]any) sessionJSON {
	e.t.Helper()
	if _, ok := body["campaign"]; !ok {
		body["campaign"] = e.campaign
	}
	resp, out := e.req("POST", "/api/sessions", body, nil)
	if resp.StatusCode != http.StatusCreated {
		e.t.Fatalf("create: %d %s", resp.StatusCode, out)
	}
	var sj sessionJSON
	if err := json.Unmarshal(out, &sj); err != nil {
		e.t.Fatal(err)
	}
	return sj
}

func (e *env) dial(id string, hdr http.Header) (*websocket.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if hdr == nil {
		hdr = http.Header{}
	}
	if hdr.Get("Authorization") == "" {
		hdr.Set("Authorization", "Bearer "+e.token)
	}
	if _, ok := hdr["Origin"]; !ok {
		hdr.Set("Origin", "http://"+e.host)
	}
	return websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.base, "http")+"/api/sessions/"+id+"/term",
		&websocket.DialOptions{HTTPHeader: hdr})
}

// readUntil reads terminal output until it contains want.
func readUntil(t *testing.T, c *websocket.Conn, want string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var out bytes.Buffer
	for !strings.Contains(out.String(), want) {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %q: %v (got %q)", want, err, out.String())
		}
		if typ == websocket.MessageBinary {
			out.Write(data)
		}
	}
	return out.String()
}

func send(t *testing.T, c *websocket.Conn, s string) {
	t.Helper()
	if err := c.Write(context.Background(), websocket.MessageBinary, []byte(s)); err != nil {
		t.Fatal(err)
	}
}

func TestStateListsHeroesAndClaude(t *testing.T) {
	e := startServer(t, nil)
	_, out := e.req("GET", "/api/state", nil, nil)
	var st stateJSON
	if err := json.Unmarshal(out, &st); err != nil {
		t.Fatal(err)
	}
	if !st.Claude.Found || len(st.Heroes) != 1 || st.Heroes[0].ID != "opus" || len(st.Sessions) != 0 || st.Version != "test" {
		t.Fatalf("state: %s", out)
	}
}

func TestStateWithoutClaude(t *testing.T) {
	e := startServer(t, func(c *Config) { c.Claude, c.ClaudeErr = "", "not on PATH" })
	_, out := e.req("GET", "/api/state", nil, nil)
	if !strings.Contains(string(out), `"found":false`) || !strings.Contains(string(out), "not on PATH") {
		t.Fatalf("state: %s", out)
	}
	resp, out := e.req("POST", "/api/sessions", map[string]any{"hero": "opus", "campaign": e.campaign}, nil)
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(out), "not on PATH") {
		t.Fatalf("hero launch without claude: %d %s", resp.StatusCode, out)
	}
	// A plain shell does not need claude.
	e.create(map[string]any{"shell": true})
}

func TestHeroSessionEndToEnd(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"hero": "opus", "opener": "/some-skill arg"})
	if sj.Kind != "hero" || sj.Hero != "opus" || sj.Title != "Opus" || sj.Exited || sj.Campaign != e.campaign || sj.Dir != e.dir {
		t.Fatalf("session: %+v", sj)
	}

	c, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	out := readUntil(t, c, "hero-env=")
	wantCwd, _ := filepath.EvalSymlinks(e.dir)
	for _, want := range []string{
		"FAKE-CLAUDE",
		"arg=[--model]", "arg=[m1]", // hero args
		"arg=[--]", "arg=[/some-skill arg]", // opener behind the separator
		"cwd=" + wantCwd,     // started in the requested directory
		"child-marker=unset", // parent-session marker scrubbed
		"hero-env=yes",       // hero env applied
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	// Keystrokes reach the process and come back.
	send(t, c, "ping\n")
	readUntil(t, c, "echo:ping")

	// A second viewer gets the scrollback replayed.
	c2, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.CloseNow()
	if replay := readUntil(t, c2, "echo:ping"); !strings.Contains(replay, "FAKE-CLAUDE") {
		t.Fatalf("replay incomplete: %q", replay)
	}

	// The session shows up in state, then deleting it ends it.
	_, stOut := e.req("GET", "/api/state", nil, nil)
	if !strings.Contains(string(stOut), sj.ID) {
		t.Fatalf("session missing from state: %s", stOut)
	}
	if resp, _ := e.req("DELETE", "/api/sessions/"+sj.ID, nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	if resp, _ := e.req("DELETE", "/api/sessions/"+sj.ID, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second delete: %d", resp.StatusCode)
	}
	if resp, _ := e.req("GET", "/api/sessions/"+sj.ID+"/term", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("terminal of a removed session: %d", resp.StatusCode)
	}
}

func TestShellResizeAndExitFrame(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"shell": true, "cols": 80, "rows": 24})
	if sj.Kind != "shell" {
		t.Fatalf("kind %q", sj.Kind)
	}
	c, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	msg, _ := json.Marshal(map[string]any{"type": "resize", "cols": 100, "rows": 50})
	if err := c.Write(context.Background(), websocket.MessageText, msg); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	send(t, c, "stty size\n")
	readUntil(t, c, "50 100")

	// Exiting the shell yields an exit frame carrying the code.
	send(t, c, "exit 7\n")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("no exit frame: %v", err)
		}
		if typ == websocket.MessageText {
			var m struct {
				Type string
				Code int
			}
			if json.Unmarshal(data, &m) != nil || m.Type != "exit" || m.Code != 7 {
				t.Fatalf("exit frame: %s", data)
			}
			break
		}
	}
	// The finished session stays listed, marked exited.
	time.Sleep(100 * time.Millisecond)
	_, out := e.req("GET", "/api/state", nil, nil)
	if !strings.Contains(string(out), `"exited":true`) || !strings.Contains(string(out), `"exitCode":7`) {
		t.Fatalf("state after exit: %s", out)
	}
}

func TestTerminalSocketSecurity(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"shell": true})

	// A browser-style upgrade with no Origin header is refused outright.
	req, _ := http.NewRequest("GET", e.base+"/api/sessions/"+sj.ID+"/term", nil)
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("no origin: got %d, want 403", resp.StatusCode)
	}

	for name, tc := range map[string]struct {
		hdr  http.Header
		want int
	}{
		"foreign origin": {http.Header{"Origin": {"http://evil.test"}}, http.StatusForbidden},
		"wrong token":    {http.Header{"Authorization": {"Bearer wrong"}}, http.StatusUnauthorized},
	} {
		c, resp, err := e.dial(sj.ID, tc.hdr)
		if err == nil {
			c.CloseNow()
			t.Errorf("%s: connected", name)
			continue
		}
		if resp == nil || resp.StatusCode != tc.want {
			t.Errorf("%s: got %v, want %d", name, resp, tc.want)
		}
	}
}

func TestSessionAPIRequiresAuthAndJSON(t *testing.T) {
	e := startServer(t, nil)
	noAuth := func(r *http.Request) { r.Header.Del("Authorization") }
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/state"}, {"POST", "/api/sessions"}, {"DELETE", "/api/sessions/s1"},
	} {
		if resp, _ := e.req(tc.method, tc.path, map[string]any{}, noAuth); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: %d", tc.method, tc.path, resp.StatusCode)
		}
	}
	resp, _ := e.req("POST", "/api/sessions", map[string]any{"shell": true, "campaign": e.campaign}, func(r *http.Request) {
		r.Header.Set("Content-Type", "text/plain")
	})
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain POST: %d", resp.StatusCode)
	}
}

func TestCreateValidation(t *testing.T) {
	e := startServer(t, nil)
	cases := map[string]map[string]any{
		"unknown hero":     {"hero": "ghost"},
		"unknown campaign": {"hero": "opus", "campaign": "cnope"},
		"empty campaign":   {"hero": "opus", "campaign": ""},
		"hero and shell":   {"hero": "opus", "shell": true},
		"neither":          {},
		"dir not accepted": {"hero": "opus", "dir": "/etc"},
		"unknown field":    {"hero": "opus", "argv": []string{"rm"}},
		"opener too long":  {"hero": "opus", "opener": strings.Repeat("x", maxOpener+1)},
	}
	for name, body := range cases {
		if _, ok := body["campaign"]; !ok {
			body["campaign"] = e.campaign
		}
		resp, out := e.req("POST", "/api/sessions", body, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: got %d %s, want 400", name, resp.StatusCode, out)
		}
	}
	// Malformed JSON.
	req, _ := http.NewRequest("POST", e.base+"/api/sessions", strings.NewReader("{nope"))
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed JSON: %d", resp.StatusCode)
	}
	// Oversized body.
	big := bytes.Repeat([]byte("a"), maxBody+10)
	req, _ = http.NewRequest("POST", e.base+"/api/sessions", bytes.NewReader(big))
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("oversized body: %d", resp.StatusCode)
	}
	// None of the rejected requests may have spawned or recorded anything.
	_, out := e.req("GET", "/api/state", nil, nil)
	if !strings.Contains(string(out), `"sessions":[]`) {
		t.Fatalf("rejected requests created sessions: %s", out)
	}
}

func TestSessionRefusedWhenCampaignDirectoryDisappears(t *testing.T) {
	e := startServer(t, nil)
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	c, err := e.cfg.Campaigns.Add(context.Background(), campaign.Input{Dir: gone})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	resp, out := e.req("POST", "/api/sessions", map[string]any{"shell": true, "campaign": c.ID}, nil)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(out), "no longer available") {
		t.Fatalf("got %d %s", resp.StatusCode, out)
	}
}

func TestShellUnavailable(t *testing.T) {
	e := startServer(t, func(c *Config) { c.Shell = "" })
	resp, _ := e.req("POST", "/api/sessions", map[string]any{"shell": true, "campaign": e.campaign}, nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func decodeState(t *testing.T, out []byte) stateJSON {
	t.Helper()
	var st stateJSON
	if err := json.Unmarshal(out, &st); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return st
}

func TestCampaignLifecycleOverHTTP(t *testing.T) {
	e := startServer(t, nil)
	other := t.TempDir()

	// The pre-registered campaign is listed.
	_, out := e.req("GET", "/api/state", nil, nil)
	if st := decodeState(t, out); len(st.Campaigns) != 1 || st.Campaigns[0].ID != e.campaign || st.Campaigns[0].Name != "demo" {
		t.Fatalf("state: %s", out)
	}

	// Add.
	resp, out := e.req("POST", "/api/campaigns", map[string]any{
		"dir": other, "name": "Second", "labels": []string{"area:web"},
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add: %d %s", resp.StatusCode, out)
	}
	var c campaign.Campaign
	_ = json.Unmarshal(out, &c)
	if c.Name != "Second" || c.Dir != other || c.Remote != "origin" || len(c.Labels) != 1 {
		t.Fatalf("added: %+v", c)
	}

	// Duplicate, invalid and unknown-field requests.
	for name, tc := range map[string]struct {
		body map[string]any
		want int
	}{
		"duplicate":     {map[string]any{"dir": other}, http.StatusConflict},
		"relative":      {map[string]any{"dir": "rel"}, http.StatusBadRequest},
		"missing":       {map[string]any{"dir": filepath.Join(other, "nope")}, http.StatusBadRequest},
		"bad remote":    {map[string]any{"dir": t.TempDir(), "remote": "--x"}, http.StatusBadRequest},
		"unknown field": {map[string]any{"dir": t.TempDir(), "gitRoot": "/"}, http.StatusBadRequest},
	} {
		if resp, out := e.req("POST", "/api/campaigns", tc.body, nil); resp.StatusCode != tc.want {
			t.Errorf("%s: got %d %s, want %d", name, resp.StatusCode, out, tc.want)
		}
	}

	// Update.
	resp, out = e.req("PATCH", "/api/campaigns/"+c.ID, map[string]any{"name": "Renamed", "labels": []string{}}, nil)
	_ = json.Unmarshal(out, &c)
	if resp.StatusCode != http.StatusOK || c.Name != "Renamed" || len(c.Labels) != 0 {
		t.Fatalf("update: %d %s", resp.StatusCode, out)
	}
	if resp, _ := e.req("PATCH", "/api/campaigns/cnope", map[string]any{"name": "x"}, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("update unknown: %d", resp.StatusCode)
	}
	if resp, _ := e.req("PATCH", "/api/campaigns/"+c.ID, map[string]any{"remote": "--evil"}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("update bad remote: %d", resp.StatusCode)
	}

	// Remove.
	if resp, _ := e.req("DELETE", "/api/campaigns/"+c.ID, nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("remove: %d", resp.StatusCode)
	}
	if resp, _ := e.req("DELETE", "/api/campaigns/"+c.ID, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("remove twice: %d", resp.StatusCode)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("removing a campaign must not touch its directory")
	}
}

func TestCampaignWithRunningSessionCannotBeRemoved(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"shell": true})

	resp, out := e.req("DELETE", "/api/campaigns/"+e.campaign, nil, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), "running sessions") {
		t.Fatalf("got %d %s", resp.StatusCode, out)
	}
	// Once the session is closed, removal works and its history goes with it.
	e.req("DELETE", "/api/sessions/"+sj.ID, nil, nil)
	e.create(map[string]any{"shell": true}) // a fresh one to leave a record behind
	e.cfg.Sessions.Shutdown()
	time.Sleep(200 * time.Millisecond)
	if resp, out := e.req("DELETE", "/api/campaigns/"+e.campaign, nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("remove after close: %d %s", resp.StatusCode, out)
	}
	_, stOut := e.req("GET", "/api/state", nil, nil)
	if st := decodeState(t, stOut); len(st.Sessions) != 0 || len(st.Campaigns) != 0 {
		t.Fatalf("leftovers: %s", stOut)
	}
}

func TestFinishedSessionStaysListedUntilClosed(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"shell": true})
	c, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	send(t, c, "exit 4\n")
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, out := e.req("GET", "/api/state", nil, nil)
		st := decodeState(t, out)
		if len(st.Sessions) == 1 && st.Sessions[0].Exited && st.Sessions[0].ExitCode == 4 {
			if rec, ok := e.cfg.History.Get(sj.ID); !ok || rec.Ended == nil || rec.ExitCode != 4 {
				t.Fatalf("history not updated: %+v", rec)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("session never reported as exited: %s", out)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if resp, _ := e.req("DELETE", "/api/sessions/"+sj.ID, nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("close finished session: %d", resp.StatusCode)
	}
	_, out := e.req("GET", "/api/state", nil, nil)
	if st := decodeState(t, out); len(st.Sessions) != 0 {
		t.Fatalf("still listed: %s", out)
	}
}

func TestSessionsFromAPreviousRunAreLost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	prev, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var campaignID string
	// Simulate a previous run that was killed while a session was running.
	e1 := startServer(t, func(c *Config) { c.History = prev })
	campaignID = e1.campaign
	_ = prev.Add(history.Record{ID: "sdeadbeef", Campaign: campaignID, Kind: "hero", Hero: "opus", Title: "Opus", Dir: e1.dir, Created: time.Now().UTC()})

	reopened, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	e2 := startServer(t, func(c *Config) { c.History = reopened })
	_, out := e2.req("GET", "/api/state", nil, nil)
	st := decodeState(t, out)
	if len(st.Sessions) != 1 || !st.Sessions[0].Lost || !st.Sessions[0].Exited || st.Sessions[0].ExitCode != -1 {
		t.Fatalf("lost session not reported: %s", out)
	}
	// Its terminal is gone for good: 410, distinct from "never existed".
	if resp, _ := e2.req("GET", "/api/sessions/sdeadbeef/term", nil, nil); resp.StatusCode != http.StatusGone {
		t.Fatalf("terminal of lost session: %d", resp.StatusCode)
	}
	if resp, _ := e2.req("GET", "/api/sessions/snever/term", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("terminal of unknown session: %d", resp.StatusCode)
	}
	// It can be dismissed.
	if resp, _ := e2.req("DELETE", "/api/sessions/sdeadbeef", nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("dismiss: %d", resp.StatusCode)
	}
	if got := reopened.List(); len(got) != 0 {
		t.Fatalf("history after dismiss: %+v", got)
	}
}
