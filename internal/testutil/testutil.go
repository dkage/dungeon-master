// Package testutil holds portable fixtures shared by tests.
package testutil

import "testing"

func FakeClaude(t testing.TB) string {
	return Program(t, "claude", ProgramOptions{Mode: "claude"})
}
