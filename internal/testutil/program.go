package testutil

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

type ProgramOptions struct {
	Mode, Log, Output, Error string
	Exit                     int
	SleepMS                  int
}

var fixtureOnce sync.Once
var fixtureBytes []byte
var fixtureError error

// Program builds the stub once and copies it into each test's private directory.
func Program(t testing.TB, name string, options ProgramOptions) string {
	t.Helper()
	fixtureOnce.Do(func() {
		dir, err := os.MkdirTemp("", "dungeon-fixture-")
		if err != nil {
			fixtureError = err
			return
		}
		defer os.RemoveAll(dir)
		_, here, _, _ := runtime.Caller(0)
		goExe := filepath.Join(runtime.GOROOT(), "bin", "go")
		if runtime.GOOS == "windows" {
			goExe += ".exe"
		}
		bin := filepath.Join(dir, "fixture")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}
		cmd := exec.Command(goExe, "build", "-o", bin, ".")
		cmd.Dir = filepath.Join(filepath.Dir(here), "cmd", "fixture")
		if out, err := cmd.CombinedOutput(); err != nil {
			fixtureError = &buildError{err, string(out)}
			return
		}
		fixtureBytes, fixtureError = os.ReadFile(bin)
	})
	if fixtureError != nil {
		t.Fatal(fixtureError)
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(bin, fixtureBytes, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin+".json", data, 0o600); err != nil {
		t.Fatal(err)
	}
	return bin
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.output }
