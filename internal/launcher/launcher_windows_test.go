package launcher

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsResolvedExecutableRuns(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	want := filepath.Join(bin, "claude.exe")
	if err := os.WriteFile(want, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	for _, name := range []string{"claude", "claude.exe", want, filepath.ToSlash(want)} {
		got, err := resolve(context.Background(), name, bin, "", nil)
		if err != nil {
			t.Fatalf("resolve %q: %v", name, err)
		}
		cmd := exec.Command(got, "-test.run=^TestWindowsExecutableHelper$")
		cmd.Env = append(os.Environ(), "DUNGEON_LAUNCHER_TEST_HELPER=1")
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "launcher helper ran") {
			t.Fatalf("execute %q: %v: %s", got, err, out)
		}
	}
}

func TestWindowsExecutableHelper(t *testing.T) {
	if os.Getenv("DUNGEON_LAUNCHER_TEST_HELPER") == "1" {
		_, _ = os.Stdout.WriteString("launcher helper ran\n")
		os.Exit(0)
	}
}
