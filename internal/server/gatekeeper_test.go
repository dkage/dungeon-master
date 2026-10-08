package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/gatekeeper"
	"github.com/dkage/dungeon-master/internal/quartermaster"
)

// withGate starts a server whose sessions get the shim, paced by a real
// Quartermaster with short intervals so the tests stay fast.
func withGate(t *testing.T, qopts ...func(*quartermaster.Options)) (*env, *quartermaster.Quartermaster) {
	t.Helper()
	o := quartermaster.Options{WriteGap: 80 * time.Millisecond, WriteJitter: -1, MaxWait: 2 * time.Second}
	for _, f := range qopts {
		f(&o)
	}
	qm, err := quartermaster.New(o)
	if err != nil {
		t.Fatal(err)
	}
	e := startServer(t, func(c *Config) {
		c.Quartermaster = qm
		c.Gatekeeper = &Gatekeeper{BinDir: "/gk/bin", RealGH: "/real/gh"}
	})
	return e, qm
}

var tokenRe = regexp.MustCompile(`gate-token=(gate_[0-9a-f]+)`)

// heroToken starts a hero session and returns the session and its shim token.
func heroToken(t *testing.T, e *env) (sessionJSON, string, string) {
	t.Helper()
	sj := e.create(map[string]any{"hero": "opus"})
	c, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	out := readUntil(t, c, "real-gh=")
	m := tokenRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no gate token in the session's environment:\n%s", out)
	}
	return sj, m[1], out
}

// gate sends a request to the Gatekeeper's socket, the way the shim does.
func (e *env) gate(method, path string, body any, token string, mutate func(*http.Request)) (*http.Response, []byte) {
	e.t.Helper()
	client, base := gatekeeper.Dialer(gatekeeper.URLPrefix+e.gateSock, nil)
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, base+path, rd)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

func TestSessionsGetTheShimFirstOnThePathAndTheRule(t *testing.T) {
	e, _ := withGate(t)
	_, tok, out := heroToken(t, e)
	for _, want := range []string{
		"path=/gk/bin" + string(os.PathListSeparator),   // the shim directory is first
		"gate-url=" + gatekeeper.URLPrefix + e.gateSock, // sessions reach the private socket, not the web port
		"real-gh=/real/gh",
		"arg=[--append-system-prompt]",
		"arg=[" + gatekeeper.Rule + "]", // the rule is in the hero's instructions
	} {
		if !strings.Contains(out, want) {
			t.Errorf("session output lacks %q:\n%s", want, out)
		}
	}
	if len(tok) < 40 {
		t.Fatalf("token %q", tok)
	}
}

func TestEverySessionGetsItsOwnToken(t *testing.T) {
	e, _ := withGate(t)
	_, t1, _ := heroToken(t, e)
	_, t2, _ := heroToken(t, e)
	if t1 == t2 {
		t.Fatal("two sessions share a token")
	}
}

func TestShellSessionsGetTheShimToo(t *testing.T) {
	e, _ := withGate(t)
	sj := e.create(map[string]any{"shell": true})
	c, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	send(t, c, "echo URL=$DUNGEON_GATE_URL BIN=$DUNGEON_GATE_BIN REAL=$DUNGEON_REAL_GH\n")
	out := readUntil(t, c, "REAL=/real/gh")
	if !strings.Contains(out, "URL="+gatekeeper.URLPrefix+e.gateSock) || !strings.Contains(out, "BIN=/gk/bin") {
		t.Fatalf("%s", out)
	}
	send(t, c, "echo FIRST=${PATH%%:*}\n")
	readUntil(t, c, "FIRST=/gk/bin")
}

func TestWithoutTheGatekeeperSessionsAreUntouched(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"hero": "opus"})
	c, _, err := e.dial(sj.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	out := readUntil(t, c, "real-gh=")
	for _, want := range []string{"gate-url=unset", "gate-token=unset", "real-gh=unset"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "--append-system-prompt") || strings.Contains(out, "path=/gk/bin") {
		t.Fatalf("a session got the Gatekeeper although it is off:\n%s", out)
	}
}

func TestTheGateSocketServesOnlyTheShimsTwoRoutes(t *testing.T) {
	e, _ := withGate(t)
	_, tok, _ := heroToken(t, e)

	if resp, out := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "issue create"}, tok, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("slot: %d %s", resp.StatusCode, out)
	}
	if resp, _ := e.gate("POST", "/gatekeeper/report", map[string]any{"message": "x"}, tok, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("report: %d", resp.StatusCode)
	}
	// Nothing else exists there, not even the app's API with the session's token.
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/state"}, {"GET", "/api/github"}, {"POST", "/api/sessions"}, {"DELETE", "/api/sessions/x"},
		{"POST", "/api/campaigns"}, {"GET", "/api/campaigns/" + e.campaign + "/quests"}, {"GET", "/"},
	} {
		resp, _ := e.gate(tc.method, tc.path, map[string]any{}, tok, nil)
		if resp.StatusCode < 400 {
			t.Errorf("%s %s on the gate socket: %d", tc.method, tc.path, resp.StatusCode)
		}
	}
	// Garbage and missing tokens are refused.
	for name, bad := range map[string]string{"unknown": "gate_deadbeef", "empty": "", "no prefix": "x"} {
		if resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "x"}, bad, nil); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s token: %d", name, resp.StatusCode)
		}
	}
	// The app's own access token is not a gate token.
	if resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "x"}, testToken, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("access token on the gate: %d", resp.StatusCode)
	}
}

// The reverse matters more: the port a browser can reach must not honour a
// gate token at all.
func TestTheWebPortIgnoresGateTokens(t *testing.T) {
	e, _ := withGate(t)
	_, tok, _ := heroToken(t, e)
	for _, path := range []string{"/api/gatekeeper/slot", "/api/gatekeeper/report", "/gatekeeper/slot", "/api/state", "/api/github"} {
		req, _ := http.NewRequest("POST", e.base+path, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			t.Errorf("web port accepted a gate token on %s: %d", path, resp.StatusCode)
		}
	}
}

func TestTheGateSocketIsPrivateAndGoesAwayWithTheServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modes do not exist on Windows; the directory ACL protects the socket")
	}
	qm, _ := quartermaster.New(quartermaster.Options{})
	s := handlerServer(Config{Gatekeeper: &Gatekeeper{BinDir: "/b", RealGH: "/g"}, Quartermaster: qm}, 1)
	if err := s.startGate(); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(s.gateSock)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("socket mode %v, want 0600", fi.Mode().Perm())
	}
	di, _ := os.Stat(filepath.Dir(s.gateSock))
	if di.Mode().Perm() != 0o700 {
		t.Errorf("directory mode %v, want 0700", di.Mode().Perm())
	}
	s.stopGate(context.Background())
	if _, err := os.Stat(s.gateSock); err == nil {
		t.Error("the socket file outlived the server")
	}
}

func TestAGateWithoutTheGatekeeperHasNoSocket(t *testing.T) {
	e := startServer(t, nil)
	if e.gateSock != "" {
		t.Fatalf("socket %q although the Gatekeeper is off", e.gateSock)
	}
}

func TestAnOversizedGateRequestIsRefused(t *testing.T) {
	e, _ := withGate(t)
	_, tok, _ := heroToken(t, e)
	big := map[string]any{"command": strings.Repeat("x", 1<<20)}
	if resp, _ := e.gate("POST", "/gatekeeper/slot", big, tok, nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("1 MiB body: %d", resp.StatusCode)
	}
}

func TestGateRequestsAreStrict(t *testing.T) {
	e, _ := withGate(t)
	_, tok, _ := heroToken(t, e)
	if resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "x", "extra": 1}, tok, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown field: %d", resp.StatusCode)
	}
	if resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{}, tok, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }); resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("not JSON: %d", resp.StatusCode)
	}
	if resp, _ := e.gate("GET", "/gatekeeper/slot", nil, tok, nil); resp.StatusCode == http.StatusOK {
		t.Errorf("a GET must not be a way to take a slot: %d", resp.StatusCode)
	}
	// Anything that looks like a web page is refused, whatever its Origin.
	for _, origin := range []string{"http://evil.test", e.base, "null"} {
		if resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "x"}, tok, func(r *http.Request) { r.Header.Set("Origin", origin) }); resp.StatusCode != http.StatusForbidden {
			t.Errorf("origin %q: %d", origin, resp.StatusCode)
		}
	}
}

func TestTokensStopWorkingWhenTheirSessionEnds(t *testing.T) {
	e, _ := withGate(t)
	s1, t1, _ := heroToken(t, e)
	_, t2, _ := heroToken(t, e)
	if resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "x"}, t1, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("live session: %d", resp.StatusCode)
	}
	e.req("DELETE", "/api/sessions/"+s1.ID, nil, nil)
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "x"}, t1, nil)
		if resp.StatusCode == http.StatusUnauthorized {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a finished session's token is still valid")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "x"}, t2, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("another session's token was revoked: %d", resp.StatusCode)
	}
}

func TestSlotsArePacedThroughTheQuartermaster(t *testing.T) {
	e, qm := withGate(t)
	_, tok, _ := heroToken(t, e)
	slot := func() (waited int64) {
		resp, out := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "issue create"}, tok, nil)
		var body struct {
			OK       bool  `json:"ok"`
			WaitedMS int64 `json:"waitedMs"`
		}
		if err := json.Unmarshal(out, &body); err != nil || resp.StatusCode != http.StatusOK || !body.OK {
			t.Fatalf("%d %s", resp.StatusCode, out)
		}
		return body.WaitedMS
	}
	if w := slot(); w != 0 {
		t.Fatalf("the first write waited %d ms", w)
	}
	if w := slot(); w < 40 {
		t.Fatalf("the second write waited only %d ms, the gap is 80", w)
	}
	if got := qm.Status().Writes.LastMinute; got != 2 {
		t.Fatalf("writes counted: %d", got)
	}
}

func TestWritesAreRefusedDuringCampAndWhenTheBudgetIsUsedUp(t *testing.T) {
	e, qm := withGate(t, func(o *quartermaster.Options) { o.WritesPerHour = 1; o.MaxWait = 50 * time.Millisecond })
	_, tok, _ := heroToken(t, e)
	post := func() (*http.Response, []byte) {
		return e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "issue create"}, tok, nil)
	}
	if resp, _ := post(); resp.StatusCode != http.StatusOK {
		t.Fatal(resp.StatusCode)
	}
	resp, out := post()
	var body map[string]string
	json.Unmarshal(out, &body)
	if resp.StatusCode != http.StatusTooManyRequests || body["code"] != "too_busy" || !strings.Contains(body["error"], "next slot is in") {
		t.Fatalf("budget used up: %d %s", resp.StatusCode, out)
	}
	// Camp beats everything and says until when.
	qm.Trip(time.Now().Add(10*time.Minute), "GitHub sent Retry-After")
	resp, out = post()
	body = nil
	json.Unmarshal(out, &body)
	if resp.StatusCode != http.StatusTooManyRequests || body["code"] != "camp" || !strings.Contains(body["error"], "paused until") {
		t.Fatalf("camp: %d %s", resp.StatusCode, out)
	}
}

func TestAReportedLimitPausesTheWholeApp(t *testing.T) {
	e, qm := withGate(t)
	_, tok, _ := heroToken(t, e)
	if qm.Status().Camp.Active {
		t.Fatal("camp before any report")
	}
	if resp, _ := e.gate("POST", "/gatekeeper/report", map[string]any{"message": "secondary rate limit", "retryAfter": 300}, tok, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("%d", resp.StatusCode)
	}
	st := qm.Status().Camp
	if !st.Active || st.Until.Before(time.Now().Add(4*time.Minute)) || !strings.Contains(st.Reason, "gh") {
		t.Fatalf("%+v", st)
	}
	// The status endpoint shows it, and the next slot is refused.
	_, out := e.req("GET", "/api/github", nil, nil)
	if !strings.Contains(string(out), `"active":true`) {
		t.Fatalf("%s", out)
	}
	if resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "issue create"}, tok, nil); resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("slot during camp: %d", resp.StatusCode)
	}
	// A report with no time still pauses for the minimum.
	e2, qm2 := withGate(t)
	_, tok2, _ := heroToken(t, e2)
	e2.gate("POST", "/gatekeeper/report", map[string]any{"message": "x"}, tok2, nil)
	if !qm2.Status().Camp.Active {
		t.Fatal("a report without Retry-After did not pause")
	}
}

func TestWithoutAQuartermasterEveryoneMayWrite(t *testing.T) {
	e := startServer(t, func(c *Config) { c.Gatekeeper = &Gatekeeper{BinDir: "/gk/bin", RealGH: "/real/gh"} })
	_, tok, _ := heroToken(t, e)
	if resp, _ := e.gate("POST", "/gatekeeper/slot", map[string]any{"command": "x"}, tok, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("%d", resp.StatusCode)
	}
	if resp, _ := e.gate("POST", "/gatekeeper/report", map[string]any{"message": "x"}, tok, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("%d", resp.StatusCode)
	}
}

// The whole chain, in one process: the real shim, the real server, the real
// Quartermaster, and a fake gh.
func TestTheShimTalksToTheServerEndToEnd(t *testing.T) {
	e, qm := withGate(t)
	_, tok, _ := heroToken(t, e)

	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	fake := filepath.Join(dir, "gh")
	os.WriteFile(fake, []byte("#!/bin/sh\necho \"$@\" >> "+calls+"\necho created\n"), 0o755)

	envv := map[string]string{
		gatekeeper.EnvGateURL: gatekeeper.URLPrefix + e.gateSock, gatekeeper.EnvGateTok: tok, gatekeeper.EnvRealGH: fake,
	}
	runShim := func(args ...string) int {
		return gatekeeper.Run(context.Background(), gatekeeper.ShimOptions{
			Args:    args,
			Getenv:  func(k string) string { return envv[k] },
			Environ: os.Environ(),
			Stdin:   strings.NewReader(""),
			Stdout:  &strings.Builder{}, Stderr: &strings.Builder{},
			SlotTimeout: 5 * time.Second,
		})
	}
	// Three writes and a read: the writes take slots (and are spaced), the read does not.
	start := time.Now()
	for i := 0; i < 3; i++ {
		if code := runShim("issue", "create", "--title", "n"); code != 0 {
			t.Fatalf("exit %d", code)
		}
	}
	if runShim("issue", "list") != 0 {
		t.Fatal("read failed")
	}
	if el := time.Since(start); el < 140*time.Millisecond {
		t.Fatalf("three writes took only %v, the gap is 80 ms", el)
	}
	if got := qm.Status().Writes.LastMinute; got != 3 {
		t.Fatalf("slots taken: %d, want 3 (the read takes none)", got)
	}
	b, _ := os.ReadFile(calls)
	if strings.Count(string(b), "issue create") != 3 || !strings.Contains(string(b), "issue list") {
		t.Fatalf("gh calls: %s", b)
	}

	// During camp the shim refuses and gh does not run.
	qm.Trip(time.Now().Add(10*time.Minute), "x")
	before, _ := os.ReadFile(calls)
	if code := runShim("issue", "create", "--title", "n"); code != 75 {
		t.Fatalf("exit %d during camp, want 75", code)
	}
	after, _ := os.ReadFile(calls)
	if string(before) != string(after) {
		t.Fatal("gh ran during camp")
	}
}
