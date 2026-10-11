package platform

import (
	"os"
	"runtime"
	"testing"
)

func TestEnvironmentAndShell(t *testing.T) {
	t.Setenv("SHELL", "/custom/sh")
	t.Setenv("COMSPEC", "C:\\Windows\\System32\\cmd.exe")
	if runtime.GOOS == "windows" {
		if EnvKey("Path") != "PATH" || !SamePath("C:/Bin", "c:/bin") || Shell() != os.Getenv("COMSPEC") {
			t.Fatal("Windows environment rules")
		}
	} else {
		if EnvKey("Path") != "Path" || SamePath("/Bin", "/bin") || Shell() != "/custom/sh" {
			t.Fatal("Unix environment rules")
		}
	}
	t.Setenv("SHELL", "")
	t.Setenv("COMSPEC", "")
	if Shell() == "" {
		t.Fatal("no default shell")
	}
}

func TestSamePathWindowsForms(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive letters and UNC shares only exist on Windows")
	}
	same := [][2]string{
		{`C:\Users\Me\repo`, `c:/users/me/repo/`},
		{`\Server\Share\repo`, `\server\share\repo`},
	}
	for _, p := range same {
		if !SamePath(p[0], p[1]) {
			t.Errorf("%q and %q should be the same folder", p[0], p[1])
		}
	}
	if SamePath(`C:\repo`, `D:\repo`) || SamePath(`C:\repo`, `C:\repo2`) {
		t.Error("different drives or names must differ")
	}
}
