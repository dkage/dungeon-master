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
