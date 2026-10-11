package session

import (
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/testutil"
)

var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func TestActivityQuietAfterOutput(t *testing.T) {
	a := newActivity(t0)
	a.note([]byte("compiling...\r\n"), t0.Add(time.Second))
	if got := a.state(t0.Add(time.Second+QuietAfter-1), false); got != Working {
		t.Fatalf("just under the threshold: %s", got)
	}
	if got := a.state(t0.Add(time.Second+QuietAfter), false); got != Idle {
		t.Fatalf("at the threshold: %s", got)
	}
}

func TestActivityContinuousOutputStaysWorking(t *testing.T) {
	a := newActivity(t0)
	now := t0
	for range 20 {
		now = now.Add(QuietAfter - time.Second)
		a.note([]byte("tick\n"), now)
		if got := a.state(now, false); got != Working {
			t.Fatalf("at %s: %s", now, got)
		}
	}
}

func TestActivityNoOutputYetCountsFromStart(t *testing.T) {
	a := newActivity(t0)
	if got := a.state(t0.Add(time.Second), false); got != Working {
		t.Fatalf("got %s", got)
	}
	if got := a.state(t0.Add(QuietAfter), false); got != Idle {
		t.Fatalf("got %s", got)
	}
}

func TestActivityNeedsYouWhenQuietAfterQuestion(t *testing.T) {
	a := newActivity(t0)
	a.note([]byte("\x1b[1mDo you want to proceed?\x1b[0m\r\n\x1b[36m❯ 1. Yes\x1b[0m\r\n  2. No\r\n"), t0)
	late := t0.Add(QuietAfter)
	if got := a.state(late, false); got != NeedsYou {
		t.Fatalf("got %s", got)
	}
	// Output resumes: back to working, and idle once it ends without a question.
	a.note([]byte("\x1b[2J\x1b[Hrunning tests\r\n"), late)
	if got := a.state(late, false); got != Working {
		t.Fatalf("resumed: %s", got)
	}
	if got := a.state(late.Add(QuietAfter), false); got != Idle {
		t.Fatalf("after resume: %s", got)
	}
}

func TestActivityExitedWins(t *testing.T) {
	a := newActivity(t0)
	a.note([]byte("Continue? (y/n) "), t0)
	if got := a.state(t0.Add(time.Hour), true); got != Ended {
		t.Fatalf("got %s", got)
	}
}

func TestAsksUser(t *testing.T) {
	for _, tc := range []struct {
		name, out string
		want      bool
	}{
		{"empty", "", false},
		{"yes/no", "Overwrite file? (y/n) ", true},
		{"trailing question", "All done.\nShould I also update the docs?\n", true},
		{"question in a box", "╭──────╮\n│ Run the tests? │\n╰──────╯\n", true},
		{"choice menu", "Pick one\n❯ 1. Yes\n  2. No\n", true},
		{"colour codes do not hide it", "\x1b[31mContinue\x1b[0m? \x1b[1m[Y/n]\x1b[0m", true},
		{"plain statement", "Build finished in 2s\n$ ", false},
		{"old question scrolled away", "Proceed? (y/n)\n1\n2\n3\n4\n5\n6\n7\n8\n9\n", false},
		{"question mark mid-line only", "what? is this\nok\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := asksUser([]byte(tc.out)); got != tc.want {
				t.Fatalf("asksUser(%q) = %v", tc.out, got)
			}
		})
	}
}

// A real process prints a question and waits; the clock is moved instead of
// sleeping.
func TestSessionReportsNeedsYouFromRealPTY(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()
	prog := testutil.Program(t, "asker", testutil.ProgramOptions{Mode: "ask", Output: "Delete the branch? (y/n) "})
	s, err := m.Spawn(Spec{Argv: []string{prog}, Env: Environ()})
	if err != nil {
		t.Fatal(err)
	}
	collect(t, s, "(y/n)")
	if got := s.Activity(); got != Working {
		t.Fatalf("right after output: %s", got)
	}
	now := time.Now().Add(QuietAfter + time.Second)
	s.mu.Lock()
	s.clock = func() time.Time { return now }
	s.mu.Unlock()
	if got := s.Activity(); got != NeedsYou {
		t.Fatalf("quiet after a question: %s", got)
	}
	if _, err := s.Write([]byte("y\r")); err != nil {
		t.Fatal(err)
	}
	<-s.Done()
	if got := s.Activity(); got != Ended {
		t.Fatalf("after exit: %s", got)
	}
}
