// Package testutil holds portable fixtures shared by tests.
package testutil

import (
	"os"
	"testing"
)

func FakeClaude(t testing.TB) string {
	return Program(t, "claude", ProgramOptions{Mode: "claude"})
}

// ScrubSessionEnv removes the variables Dungeon Master sets for the agent sessions it starts. Tests run by an
// agent inside such a session inherit them, and they change what the gatekeeper tests observe. Call it from
// TestMain before m.Run.
func ScrubSessionEnv() {
	for _, k := range []string{"DUNGEON_GATE_URL", "DUNGEON_GATE_TOKEN", "DUNGEON_REAL_GH", "DUNGEON_GATE_BIN"} {
		os.Unsetenv(k)
	}
}
