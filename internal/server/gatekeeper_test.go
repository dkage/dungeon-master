package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

func (e *env) gate(method, path string, body any, token string, mutate func(*http.Request)) (*http.Response, []byte) {
	return e.req(method, path, body, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+token)
		if mutate != nil {
			mutate(r)
		}
	})
}

func TestSessionsGetTheShimFirstOnThePathAndTheRule(t *testing.T) {
	e, _ := withGate(t)
	_, tok, out := heroToken(t, e)
	for _, want := range []string{
		"path=/gk/bin" + string(os.PathListSeparator), // the shim directory is first
		"gate-url=" + e.base,                          // sessions reach this server
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
	if !strings.Contains(out, "URL="+e.base) || !strings.Contains(out, "BIN=/gk/bin") {
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

func TestAGateTokenOnlyOpensTheGatekeepersEndpoints(t *testing.T) {
	e, _ := withGate(t)
	_, tok, _ := heroToken(t, e)

	// It works for what the shim needs.
	if resp, out := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "issue create"}, tok, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("slot: %d %s", resp.StatusCode, out)
	}
	// The ordinary access token works there as well (the same trust level).
	if resp, _ := e.req("POST", "/api/gatekeeper/slot", map[string]any{"command": "issue create"}, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("access token: %d", resp.StatusCode)
	}
	if resp, _ := e.gate("POST", "/api/gatekeeper/report", map[string]any{"message": "x"}, tok, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("report: %d", resp.StatusCode)
	}
	// And nothing else: it must not read state, start sessions, or touch campaigns.
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/state"}, {"GET", "/api/github"}, {"POST", "/api/sessions"}, {"DELETE", "/api/sessions/" + "x"},
		{"POST", "/api/campaigns"}, {"GET", "/api/campaigns/" + e.campaign + "/quests"}, {"PUT", "/api/campaigns/" + e.campaign + "/scope"},
	} {
		resp, _ := e.gate(tc.method, tc.path, map[string]any{}, tok, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s with a gate token: %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
	}
	// Garbage and missing tokens are refused on the gate endpoints too.
	for name, bad := range map[string]string{"unknown": "gate_deadbeef", "empty": "", "no prefix": "x"} {
		if resp, _ := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "x"}, bad, nil); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s token: %d", name, resp.StatusCode)
		}
	}
}

func TestGateRequestsAreStrict(t *testing.T) {
	e, _ := withGate(t)
	_, tok, _ := heroToken(t, e)
	if resp, _ := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "x", "extra": 1}, tok, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown field: %d", resp.StatusCode)
	}
	if resp, _ := e.gate("POST", "/api/gatekeeper/slot", map[string]any{}, tok, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }); resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("not JSON: %d", resp.StatusCode)
	}
	if resp, _ := e.gate("GET", "/api/gatekeeper/slot", nil, tok, nil); resp.StatusCode == http.StatusOK {
		t.Errorf("a GET must not be a way to take a slot: %d", resp.StatusCode)
	}
	// A browser page cannot ask for slots either: a foreign Origin is refused.
	if resp, _ := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "x"}, tok, func(r *http.Request) { r.Header.Set("Origin", "http://evil.test") }); resp.StatusCode != http.StatusForbidden {
		t.Errorf("foreign origin: %d", resp.StatusCode)
	}
}

func TestTokensStopWorkingWhenTheirSessionEnds(t *testing.T) {
	e, _ := withGate(t)
	s1, t1, _ := heroToken(t, e)
	_, t2, _ := heroToken(t, e)
	if resp, _ := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "x"}, t1, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("live session: %d", resp.StatusCode)
	}
	e.req("DELETE", "/api/sessions/"+s1.ID, nil, nil)
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, _ := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "x"}, t1, nil)
		if resp.StatusCode == http.StatusUnauthorized {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a finished session's token is still valid")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if resp, _ := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "x"}, t2, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("another session's token was revoked: %d", resp.StatusCode)
	}
}

func TestSlotsArePacedThroughTheQuartermaster(t *testing.T) {
	e, qm := withGate(t)
	_, tok, _ := heroToken(t, e)
	slot := func() (waited int64) {
		resp, out := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "issue create"}, tok, nil)
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
		return e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "issue create"}, tok, nil)
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
	if resp, _ := e.gate("POST", "/api/gatekeeper/report", map[string]any{"message": "secondary rate limit", "retryAfter": 300}, tok, nil); resp.StatusCode != http.StatusNoContent {
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
	if resp, _ := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "issue create"}, tok, nil); resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("slot during camp: %d", resp.StatusCode)
	}
	// A report with no time still pauses for the minimum.
	e2, qm2 := withGate(t)
	_, tok2, _ := heroToken(t, e2)
	e2.gate("POST", "/api/gatekeeper/report", map[string]any{"message": "x"}, tok2, nil)
	if !qm2.Status().Camp.Active {
		t.Fatal("a report without Retry-After did not pause")
	}
}

func TestWithoutAQuartermasterEveryoneMayWrite(t *testing.T) {
	e := startServer(t, func(c *Config) { c.Gatekeeper = &Gatekeeper{BinDir: "/gk/bin", RealGH: "/real/gh"} })
	_, tok, _ := heroToken(t, e)
	if resp, _ := e.gate("POST", "/api/gatekeeper/slot", map[string]any{"command": "x"}, tok, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("%d", resp.StatusCode)
	}
	if resp, _ := e.gate("POST", "/api/gatekeeper/report", map[string]any{"message": "x"}, tok, nil); resp.StatusCode != http.StatusNoContent {
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
		gatekeeper.EnvGateURL: e.base, gatekeeper.EnvGateTok: tok, gatekeeper.EnvRealGH: fake,
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
