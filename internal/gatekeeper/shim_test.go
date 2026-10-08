package gatekeeper

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeGH writes a stand-in for the real gh. It records its arguments and
// working environment, echoes stdin, and exits as FAKE_GH_EXIT says.
func fakeGH(t *testing.T) (path, log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
{
  echo "ARGS: $*"
  echo "PATH: $PATH"
} >> "` + log + `"
if [ -n "$FAKE_GH_STDERR" ]; then echo "$FAKE_GH_STDERR" >&2; fi
if [ "$FAKE_GH_ECHO_STDIN" = 1 ]; then cat; fi
echo "stdout-from-gh"
exit ${FAKE_GH_EXIT:-0}
`
	path = filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, log
}

// gate is a fake app: it answers slot and report requests.
type gate struct {
	srv     *httptest.Server
	mu      sync.Mutex
	slots   int
	reports []map[string]any
	auths   []string
	answer  func(w http.ResponseWriter)
}

func newGate(t *testing.T) *gate {
	g := &gate{}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.auths = append(g.auths, r.Header.Get("Authorization"))
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "json only", http.StatusUnsupportedMediaType)
			return
		}
		switch r.URL.Path {
		case SlotPath:
			g.slots++
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["command"] == "" {
				t.Error("the slot request must say which command")
			}
			if g.answer != nil {
				g.answer(w)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "waitedMs": 0})
		case ReportPath:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			g.reports = append(g.reports, body)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func run(t *testing.T, args []string, env map[string]string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	environ := os.Environ()
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}
	code = Run(context.Background(), ShimOptions{
		Args:        args,
		Getenv:      func(k string) string { return lookup(environ, k) },
		Environ:     environ,
		Stdin:       strings.NewReader(stdin),
		Stdout:      &out,
		Stderr:      &errb,
		HTTP:        &http.Client{Timeout: 5 * time.Second},
		SlotTimeout: 5 * time.Second,
	})
	return code, out.String(), errb.String()
}

func lookup(environ []string, key string) string {
	v := ""
	for _, kv := range environ {
		if k, val, ok := strings.Cut(kv, "="); ok && k == key {
			v = val // the last one wins, as in a real environment
		}
	}
	return v
}

func session(g *gate, real string) map[string]string {
	return map[string]string{EnvGateURL: g.srv.URL, EnvGateTok: "gate_tok", EnvRealGH: real}
}

func read(t *testing.T, path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func TestReadsPassStraightThrough(t *testing.T) {
	real, log := fakeGH(t)
	g := newGate(t)
	code, out, _ := run(t, []string{"issue", "list", "--limit", "5"}, session(g, real), "")
	if code != 0 || !strings.Contains(out, "stdout-from-gh") {
		t.Fatalf("%d %q", code, out)
	}
	if g.slots != 0 {
		t.Fatal("a read asked for a write slot")
	}
	if !strings.Contains(read(t, log), "ARGS: issue list --limit 5") {
		t.Fatalf("gh got: %s", read(t, log))
	}
}

func TestWritesAskForASlotFirstAndThenRun(t *testing.T) {
	real, log := fakeGH(t)
	g := newGate(t)
	code, out, _ := run(t, []string{"issue", "create", "--title", "a secret title"}, session(g, real), "")
	if code != 0 || !strings.Contains(out, "stdout-from-gh") {
		t.Fatalf("%d %q", code, out)
	}
	if g.slots != 1 || g.auths[0] != "Bearer gate_tok" {
		t.Fatalf("slots %d auths %v", g.slots, g.auths)
	}
	if !strings.Contains(read(t, log), "ARGS: issue create --title a secret title") {
		t.Fatalf("gh got: %s", read(t, log))
	}
}

func TestTheSlotRequestNeverContainsTheArguments(t *testing.T) {
	real, _ := fakeGH(t)
	var sent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		sent = string(b)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	run(t, []string{"-R", "o/r", "issue", "create", "--title", "TOP SECRET", "--body", "confidential"}, map[string]string{EnvGateURL: srv.URL, EnvGateTok: "t", EnvRealGH: real}, "")
	if strings.Contains(sent, "SECRET") || strings.Contains(sent, "confidential") || !strings.Contains(sent, "issue create") {
		t.Fatalf("slot request body: %s", sent)
	}
}

func TestAHeldWriteDoesNotRunAndTellsTheAgentWhy(t *testing.T) {
	real, log := fakeGH(t)
	g := newGate(t)
	g.answer = func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "GitHub asked this app to slow down; writes are paused until 15:12.", "code": "camp"})
	}
	code, out, errOut := run(t, []string{"issue", "create", "--title", "x"}, session(g, real), "")
	if code != 75 || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	if !strings.Contains(errOut, "paused until 15:12") || !strings.Contains(errOut, "do not retry in a loop") {
		t.Fatalf("stderr: %q", errOut)
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("gh ran although the write was held")
	}
}

func TestAHeldWriteWithoutAMessageStillSaysSomething(t *testing.T) {
	real, _ := fakeGH(t)
	g := newGate(t)
	g.answer = func(w http.ResponseWriter) { w.WriteHeader(http.StatusTooManyRequests); w.Write([]byte("{}")) }
	code, _, errOut := run(t, []string{"issue", "close", "1"}, session(g, real), "")
	if code != 75 || !strings.Contains(errOut, "paused") {
		t.Fatalf("%d %q", code, errOut)
	}
}

func TestLongWaitsAreMentioned(t *testing.T) {
	real, _ := fakeGH(t)
	g := newGate(t)
	g.answer = func(w http.ResponseWriter) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "waitedMs": 3200}) }
	_, _, errOut := run(t, []string{"issue", "comment", "1", "-b", "x"}, session(g, real), "")
	if !strings.Contains(errOut, "waited 3s for a GitHub write slot") {
		t.Fatalf("%q", errOut)
	}
	g.answer = func(w http.ResponseWriter) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "waitedMs": 200}) }
	if _, _, errOut := run(t, []string{"issue", "comment", "1", "-b", "x"}, session(g, real), ""); errOut != "" {
		t.Fatalf("a short wait should be silent: %q", errOut)
	}
}

func TestItFailsOpenWhenTheAppCannotBeAsked(t *testing.T) {
	real, log := fakeGH(t)
	cases := map[string]map[string]string{
		"app not running":    {EnvGateURL: "http://127.0.0.1:1", EnvGateTok: "t", EnvRealGH: real},
		"no gate configured": {EnvRealGH: real}, // not an app session at all
		"no token":           {EnvGateURL: "http://127.0.0.1:1", EnvRealGH: real},
	}
	for name, env := range cases {
		os.Remove(log)
		code, out, errOut := run(t, []string{"issue", "create", "--title", "x"}, env, "")
		if code != 0 || !strings.Contains(out, "stdout-from-gh") || errOut != "" {
			t.Errorf("%s: exit %d out %q err %q", name, code, out, errOut)
		}
		if !strings.Contains(read(t, log), "ARGS: issue create") {
			t.Errorf("%s: gh did not run", name)
		}
	}
	// Odd answers from the app (revoked token, old version, garbage) also let the command through.
	for name, answer := range map[string]func(w http.ResponseWriter){
		"401":       func(w http.ResponseWriter) { w.WriteHeader(http.StatusUnauthorized) },
		"500":       func(w http.ResponseWriter) { w.WriteHeader(http.StatusInternalServerError) },
		"not json":  func(w http.ResponseWriter) { w.WriteHeader(http.StatusTeapot); w.Write([]byte("<html>")) },
		"empty 200": func(w http.ResponseWriter) {},
	} {
		g := newGate(t)
		g.answer = answer
		os.Remove(log)
		if code, _, _ := run(t, []string{"issue", "create", "--title", "x"}, session(g, real), ""); code != 0 {
			t.Errorf("%s: exit %d", name, code)
		}
		if !strings.Contains(read(t, log), "ARGS: issue create") {
			t.Errorf("%s: gh did not run", name)
		}
	}
}

func TestAnAppThatAnswersTooSlowlyDoesNotBlockWork(t *testing.T) {
	real, log := fakeGH(t)
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(3 * time.Second) }))
	defer slow.Close()
	var out, errb bytes.Buffer
	env := append(os.Environ(), EnvGateURL+"="+slow.URL, EnvGateTok+"=t", EnvRealGH+"="+real)
	start := time.Now()
	code := Run(context.Background(), ShimOptions{
		Args: []string{"issue", "create", "--title", "x"}, Environ: env, Getenv: func(k string) string { return lookup(env, k) },
		Stdout: &out, Stderr: &errb, Stdin: strings.NewReader(""), SlotTimeout: 300 * time.Millisecond,
	})
	if code != 0 || time.Since(start) > 2500*time.Millisecond || !strings.Contains(read(t, log), "ARGS: issue create") {
		t.Fatalf("exit %d after %v", code, time.Since(start))
	}
}

func TestExitCodeOutputAndStdinPassThrough(t *testing.T) {
	real, _ := fakeGH(t)
	g := newGate(t)
	code, out, errOut := run(t, []string{"issue", "view", "3"}, map[string]string{
		EnvGateURL: g.srv.URL, EnvGateTok: "t", EnvRealGH: real, "FAKE_GH_EXIT": "3", "FAKE_GH_STDERR": "boom", "FAKE_GH_ECHO_STDIN": "1",
	}, "typed input\n")
	if code != 3 || !strings.Contains(out, "typed input") || !strings.Contains(out, "stdout-from-gh") || strings.TrimSpace(errOut) != "boom" {
		t.Fatalf("exit %d out %q err %q", code, out, errOut)
	}
	// A write keeps all three as well.
	code, out, errOut = run(t, []string{"issue", "create"}, map[string]string{
		EnvGateURL: g.srv.URL, EnvGateTok: "t", EnvRealGH: real, "FAKE_GH_EXIT": "1", "FAKE_GH_STDERR": "validation failed", "FAKE_GH_ECHO_STDIN": "1",
	}, "body from stdin\n")
	if code != 1 || !strings.Contains(out, "body from stdin") || !strings.Contains(errOut, "validation failed") {
		t.Fatalf("exit %d out %q err %q", code, out, errOut)
	}
}

func TestARateLimitInGhsOutputIsReportedToTheApp(t *testing.T) {
	real, _ := fakeGH(t)
	g := newGate(t)
	code, _, errOut := run(t, []string{"issue", "create", "--title", "x"}, map[string]string{
		EnvGateURL: g.srv.URL, EnvGateTok: "t", EnvRealGH: real, "FAKE_GH_EXIT": "1",
		"FAKE_GH_STDERR": "GraphQL: You have exceeded a secondary rate limit. Please wait a few minutes. Retry-After: 90",
	}, "")
	if code != 1 || !strings.Contains(errOut, "secondary rate limit") {
		t.Fatalf("gh's own message must reach the agent: %d %q", code, errOut)
	}
	if len(g.reports) != 1 {
		t.Fatalf("%d reports", len(g.reports))
	}
	if !strings.Contains(g.reports[0]["message"].(string), "secondary rate limit") || g.reports[0]["retryAfter"].(float64) != 90 {
		t.Fatalf("%v", g.reports[0])
	}
}

func TestNoReportWithoutAFailureOrWithAnUnrelatedOne(t *testing.T) {
	real, _ := fakeGH(t)
	g := newGate(t)
	// The words appear but the command succeeded: nothing to report.
	run(t, []string{"issue", "create"}, map[string]string{EnvGateURL: g.srv.URL, EnvGateTok: "t", EnvRealGH: real, "FAKE_GH_STDERR": "note: rate limit is fine"}, "")
	// It failed for another reason.
	run(t, []string{"issue", "create"}, map[string]string{EnvGateURL: g.srv.URL, EnvGateTok: "t", EnvRealGH: real, "FAKE_GH_EXIT": "1", "FAKE_GH_STDERR": "could not resolve to a Repository"}, "")
	if len(g.reports) != 0 {
		t.Fatalf("%d spurious reports: %v", len(g.reports), g.reports)
	}
	// Reads are never reported on: they cost no slot and the app sees nothing.
	run(t, []string{"issue", "list"}, map[string]string{EnvGateURL: g.srv.URL, EnvGateTok: "t", EnvRealGH: real, "FAKE_GH_EXIT": "1", "FAKE_GH_STDERR": "API rate limit exceeded"}, "")
	if len(g.reports) != 0 {
		t.Fatal("a read was reported")
	}
}

func TestTheShimDirectoryIsRemovedFromGhsOwnPath(t *testing.T) {
	real, log := fakeGH(t)
	g := newGate(t)
	shimDir := t.TempDir()
	run(t, []string{"issue", "list"}, map[string]string{
		EnvGateURL: g.srv.URL, EnvGateTok: "t", EnvRealGH: real, EnvGateBin: shimDir,
		"PATH": shimDir + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin",
	}, "")
	l := read(t, log)
	if strings.Contains(l, shimDir) || !strings.Contains(l, "/usr/bin") {
		t.Fatalf("the real gh saw PATH: %s", l)
	}
}

func TestWhenTheRealGhIsMissing(t *testing.T) {
	g := newGate(t)
	code, _, errOut := run(t, []string{"issue", "list"}, map[string]string{
		EnvGateURL: g.srv.URL, EnvGateTok: "t", EnvRealGH: "/nonexistent/gh", "PATH": t.TempDir(),
	}, "")
	if code != 127 || !strings.Contains(errOut, "gh: command not found") {
		t.Fatalf("%d %q", code, errOut)
	}
}

func TestFallsBackToTheNextGhOnPathAndNeverToItself(t *testing.T) {
	real, _ := fakeGH(t)
	g := newGate(t)
	// No DUNGEON_REAL_GH: the first gh on PATH that is not this program is used.
	dir := filepath.Dir(real)
	code, out, _ := run(t, []string{"issue", "list"}, map[string]string{EnvGateURL: g.srv.URL, EnvGateTok: "t", "PATH": dir}, "")
	if code != 0 || !strings.Contains(out, "stdout-from-gh") {
		t.Fatalf("%d %q", code, out)
	}
}

func TestIsShim(t *testing.T) {
	for in, want := range map[string]bool{
		"gh": true, "/usr/local/bin/gh": true, "/x/y/GH": true, "gh.exe": true, `C:\x\gh.exe`: false, /* backslashes are not separators here */
		"dungeon": false, "/x/ghost": false, "/x/gh2": false, "": false,
	} {
		got := IsShim(in)
		if in == `C:\x\gh.exe` {
			continue // path handling differs by OS; covered by the "gh.exe" case
		}
		if got != want {
			t.Errorf("IsShim(%q)=%v want %v", in, got, want)
		}
	}
}

func TestSummary(t *testing.T) {
	for in, want := range map[string]string{
		"issue create --title x": "issue create", "-R o/r issue edit 3": "issue edit", "api repos/x -X POST": "api repos/x",
		"": "", "issue": "issue",
	} {
		if got := summary(strings.Fields(in)); got != want {
			t.Errorf("summary(%q)=%q want %q", in, got, want)
		}
	}
}
