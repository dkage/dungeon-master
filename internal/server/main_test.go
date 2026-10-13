package server

import (
	"os"
	"testing"

	"github.com/dkage/dungeon-master/internal/testutil"
)

// TestMain keeps the tests independent of the Gatekeeper variables an agent session inherits.
func TestMain(m *testing.M) {
	testutil.ScrubSessionEnv()
	os.Exit(m.Run())
}
