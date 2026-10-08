// Package testutil holds fixtures shared by tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// FakeClaude writes a stand-in for the claude CLI and returns its path. It
// prints how it was invoked, then echoes each line it receives, so tests can
// observe argv, working directory and environment through a real PTY.
func FakeClaude(t testing.TB) string {
	t.Helper()
	const script = `#!/bin/sh
echo "FAKE-CLAUDE"
for a in "$@"; do echo "arg=[$a]"; done
echo "cwd=$(pwd)"
echo "child-marker=${CLAUDE_CODE_CHILD_SESSION:-unset}"
echo "hero-env=${HERO_VAR:-unset}"
echo "path=$PATH"
echo "gate-url=${DUNGEON_GATE_URL:-unset}"
echo "gate-token=${DUNGEON_GATE_TOKEN:-unset}"
echo "real-gh=${DUNGEON_REAL_GH:-unset}"
while IFS= read -r line; do echo "echo:$line"; done
`
	p := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}
