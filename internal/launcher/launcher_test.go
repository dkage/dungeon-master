package launcher

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/party"
)

func TestCommandOrder(t *testing.T) {
	h := party.Hero{ID: "opus", Args: []string{"--model", "claude-opus-5-5"}}
	got := Command("/bin/claude", h, Opts{
		Name:               "quest 7",
		AppendSystemPrompt: "be careful",
		AddDirs:            []string{"/a", "/b"},
		Opener:             "/wayfinder https://example.test/1",
	})
	want := []string{
		"/bin/claude", "--model", "claude-opus-5-5",
		"--name", "quest 7",
		"--append-system-prompt", "be careful",
		"--add-dir", "/a", "--add-dir", "/b",
		"--", "/wayfinder https://example.test/1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestCommandMinimal(t *testing.T) {
	got := Command("claude", party.Hero{}, Opts{})
	if !reflect.DeepEqual(got, []string{"claude"}) {
		t.Fatalf("got %q", got)
	}
}

func TestOpenerCannotBecomeAFlag(t *testing.T) {
	got := Command("claude", party.Hero{}, Opts{Opener: "--dangerously-skip-permissions"})
	if len(got) != 3 || got[1] != "--" || got[2] != "--dangerously-skip-permissions" {
		t.Fatalf("opener not isolated behind --: %q", got)
	}
}

func TestEnvironScrubsParentSession(t *testing.T) {
	base := []string{
		"PATH=/usr/bin", "HOME=/h", "TERM=dumb",
		"CLAUDECODE=1", "CLAUDE_CODE_CHILD_SESSION=1", "CLAUDE_CODE_SSE_PORT=9",
		"CLAUDE_CODE_USE_BEDROCK=1", "ANTHROPIC_API_KEY=k",
	}
	h := party.Hero{Env: []string{"HERO_VAR=yes", "HOME=/override"}}
	got := Environ(base, h)
	has := func(kv string) bool {
		for _, e := range got {
			if e == kv {
				return true
			}
		}
		return false
	}
	for _, gone := range []string{"CLAUDECODE=1", "CLAUDE_CODE_CHILD_SESSION=1", "CLAUDE_CODE_SSE_PORT=9", "TERM=dumb"} {
		if has(gone) {
			t.Errorf("%s leaked into child env", gone)
		}
	}
	for _, kept := range []string{"PATH=/usr/bin", "CLAUDE_CODE_USE_BEDROCK=1", "ANTHROPIC_API_KEY=k", "TERM=xterm-256color", "HERO_VAR=yes"} {
		if !has(kept) {
			t.Errorf("%s missing from child env", kept)
		}
	}
	// Hero entries come last so they win over inherited values.
	if got[len(got)-1] != "HOME=/override" {
		t.Errorf("hero env not last: %q", got)
	}
}

func writeExe(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolve(t *testing.T) {
	ctx := context.Background()
	bin := t.TempDir()
	want := writeExe(t, bin, "claude")

	// Explicit path.
	if got, err := resolve(ctx, want, "", "", nil); err != nil || got != want {
		t.Fatalf("explicit: %q %v", got, err)
	}
	// Explicit path that is not executable.
	plain := filepath.Join(t.TempDir(), "claude")
	_ = os.WriteFile(plain, []byte("x"), 0o644)
	if _, err := resolve(ctx, plain, "", "", nil); err == nil {
		t.Fatal("non-executable accepted")
	}
	// A directory named like the binary.
	d := t.TempDir()
	_ = os.Mkdir(filepath.Join(d, "claude"), 0o755)
	if _, err := resolve(ctx, "claude", d, "", nil); err == nil {
		t.Fatal("directory accepted")
	}
	// PATH lookup.
	if got, err := resolve(ctx, "claude", "/nonexistent:"+bin, "", nil); err != nil || got != want {
		t.Fatalf("PATH: %q %v", got, err)
	}
	// Not found anywhere.
	if _, err := resolve(ctx, "claude", "/nonexistent", "", nil); err == nil {
		t.Fatal("found a binary that does not exist")
	}
	if _, err := resolve(ctx, "", "", "", nil); err == nil {
		t.Fatal("empty name accepted")
	}
}

func TestResolveFallsBackToLoginShellPath(t *testing.T) {
	ctx := context.Background()
	bin := t.TempDir()
	want := writeExe(t, bin, "claude")

	// A fake "login shell" that prints a banner and then its PATH.
	shell := filepath.Join(t.TempDir(), "fakeshell")
	script := "#!/bin/sh\necho 'welcome banner'\nprintf '%s' '" + bin + "'\n"
	if err := os.WriteFile(shell, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolve(ctx, "claude", "/nonexistent", shell, nil)
	if err != nil || got != want {
		t.Fatalf("login PATH fallback: %q %v", got, err)
	}
	// A broken shell must not turn into a crash, just "not found".
	if _, err := resolve(ctx, "claude", "/nonexistent", "/nonexistent/shell", nil); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("broken shell: %v", err)
	}
}

func TestLoginShellWithStrayChildrenDoesNotHang(t *testing.T) {
	shell := filepath.Join(t.TempDir(), "slowshell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nsleep 30 &\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := loginPath(ctx, shell); err == nil {
		t.Fatal("expected an error")
	}
	if time.Since(start) > 8*time.Second {
		t.Fatalf("hung for %v", time.Since(start))
	}
}

func TestResolveExceptSkipsRejectedCandidates(t *testing.T) {
	ctx := context.Background()
	shim, real := t.TempDir(), t.TempDir()
	writeExe(t, shim, "gh")
	wantReal := writeExe(t, real, "gh")
	path := shim + string(os.PathListSeparator) + real

	// Without a filter the first gh on PATH wins, which here is the shim.
	if got, err := resolve(ctx, "gh", path, "", nil); err != nil || filepath.Dir(got) != shim {
		t.Fatalf("%q %v", got, err)
	}
	// Rejecting the shim finds the real one behind it.
	reject := func(p string) bool { return filepath.Dir(p) == shim }
	if got, err := resolve(ctx, "gh", path, "", reject); err != nil || got != wantReal {
		t.Fatalf("%q %v", got, err)
	}
	// Rejecting everything is "not found", not a crash.
	if _, err := resolve(ctx, "gh", path, "", func(string) bool { return true }); err == nil {
		t.Fatal("found something although everything was rejected")
	}
}

func TestResolveExceptUsesTheEnvironmentPath(t *testing.T) {
	bin := t.TempDir()
	want := writeExe(t, bin, "ghx")
	t.Setenv("PATH", bin)
	t.Setenv("SHELL", "")
	if got, err := ResolveExcept(context.Background(), "ghx", nil); err != nil || got != want {
		t.Fatalf("%q %v", got, err)
	}
}
