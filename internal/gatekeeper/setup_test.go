package gatekeeper

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallLinksGhToThisProgram(t *testing.T) {
	if runtime.GOOS == "windows" {
		dir := t.TempDir()
		exe := filepath.Join(dir, "dungeon.exe")
		os.WriteFile(exe, []byte("first"), 0o600)
		bin, err := Install(filepath.Join(dir, "gatekeeper"), exe)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(bin, "gh.exe"))
		if err != nil || string(b) != "first" {
			t.Fatalf("copy: %q %v", b, err)
		}
		os.WriteFile(exe, []byte("second"), 0o600)
		if _, err := Install(filepath.Join(dir, "gatekeeper"), exe); err != nil {
			t.Fatal(err)
		}
		b, _ = os.ReadFile(filepath.Join(bin, "gh.exe"))
		if string(b) != "second" {
			t.Fatalf("copy not replaced: %q", b)
		}
		return
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "dungeon")
	os.WriteFile(exe, []byte("x"), 0o755)

	bin, err := Install(filepath.Join(dir, "gatekeeper"), exe)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(bin) != "bin" {
		t.Fatalf("bin dir %q", bin)
	}
	target, err := os.Readlink(filepath.Join(bin, "gh"))
	if err != nil || target != exe {
		t.Fatalf("link: %q %v", target, err)
	}
	if fi, _ := os.Stat(bin); fi.Mode().Perm() != 0o700 {
		t.Fatalf("bin dir mode %v", fi.Mode().Perm())
	}
	// Installing again, even after the program moved, replaces the link.
	exe2 := filepath.Join(dir, "dungeon2")
	os.WriteFile(exe2, []byte("y"), 0o755)
	if _, err := Install(filepath.Join(dir, "gatekeeper"), exe2); err != nil {
		t.Fatal(err)
	}
	if target, _ := os.Readlink(filepath.Join(bin, "gh")); target != exe2 {
		t.Fatalf("link not replaced: %q", target)
	}
}

func TestInstallFailsCleanly(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	os.WriteFile(blocker, nil, 0o600)
	if _, err := Install(filepath.Join(blocker, "sub"), "/x"); err == nil {
		t.Fatal("expected an error when the directory cannot be made")
	}
}

func TestSessionEnvPutsTheShimFirstAndReplacesOldValues(t *testing.T) {
	env := []string{"HOME=/h", "PATH=/usr/bin:/bin", EnvGateTok + "=old", "OTHER=1", EnvRealGH + "=/old/gh"}
	got := SessionEnv{BinDir: "/state/gk/bin", URL: "http://127.0.0.1:7878", Token: "gate_new", RealGH: "/opt/gh"}.Apply(env)
	m := map[string]string{}
	count := map[string]int{}
	for _, kv := range got {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
		count[k]++
	}
	if m["PATH"] != "/state/gk/bin"+string(os.PathListSeparator)+"/usr/bin:/bin" {
		t.Fatalf("PATH %q", m["PATH"])
	}
	for k, want := range map[string]string{
		EnvGateURL: "http://127.0.0.1:7878", EnvGateTok: "gate_new", EnvRealGH: "/opt/gh", EnvGateBin: "/state/gk/bin",
		"HOME": "/h", "OTHER": "1",
	} {
		if m[k] != want {
			t.Errorf("%s = %q, want %q", k, m[k], want)
		}
	}
	for k, n := range count {
		if n != 1 {
			t.Errorf("%s appears %d times", k, n)
		}
	}
	if len(env) != 5 {
		t.Fatal("the input slice was modified")
	}
}

func TestSessionEnvWithoutAPath(t *testing.T) {
	got := SessionEnv{BinDir: "/b", URL: "u", Token: "t", RealGH: "g"}.Apply([]string{"HOME=/h"})
	for _, kv := range got {
		if strings.HasPrefix(kv, "PATH=") && kv != "PATH=/b" {
			t.Fatalf("%q", kv)
		}
	}
}

func TestTokensBelongToOneSession(t *testing.T) {
	tk := NewTokens()
	a, err := tk.Issue("s1")
	if err != nil || !strings.HasPrefix(a, "gate_") || len(a) < 40 {
		t.Fatalf("%q %v", a, err)
	}
	b, _ := tk.Issue("s2")
	a2, _ := tk.Issue("s1")
	if a == b || a == a2 {
		t.Fatal("tokens are not unique")
	}
	if s, ok := tk.Session(a); !ok || s != "s1" {
		t.Fatalf("%q %v", s, ok)
	}
	if _, ok := tk.Session("gate_nope"); ok {
		t.Fatal("unknown token accepted")
	}
	if _, ok := tk.Session(""); ok {
		t.Fatal("empty token accepted")
	}
	tk.Revoke("s1")
	if _, ok := tk.Session(a); ok {
		t.Fatal("revoked token still valid")
	}
	if _, ok := tk.Session(a2); ok {
		t.Fatal("every token of the session must be revoked")
	}
	if _, ok := tk.Session(b); !ok {
		t.Fatal("another session's token was revoked")
	}
	tk.Revoke("nobody") // harmless
}

func TestTheRuleIsShortAndSaysWhatToDo(t *testing.T) {
	for _, must := range []string{"one at a time", "never in a loop", "wait for them", "stop and tell the user"} {
		if !strings.Contains(Rule, must) {
			t.Errorf("rule lacks %q", must)
		}
	}
	if len(Rule) > 600 {
		t.Fatalf("the rule is %d characters: it goes into every session's instructions", len(Rule))
	}
}
