package session

import (
	"bytes"
	"regexp"
	"strings"
	"time"
)

// Activity is what a session is doing, judged only from its output.
type Activity string

const (
	// Working: output arrived recently.
	Working Activity = "working"
	// Idle: quiet, and the last thing on screen is not a question.
	Idle Activity = "idle"
	// NeedsYou: quiet, and the screen ends in something that asks the user.
	NeedsYou Activity = "needs-you"
	// Ended: the process has exited.
	Ended Activity = "ended"
)

const (
	// QuietAfter is how long output must stop before a session is no longer
	// "working". Agents pause between tool calls, so it is not too short.
	QuietAfter = 3 * time.Second
	// tailBytes is how much recent output is kept to look for a question.
	tailBytes = 4096
	// askLines is how many non-blank lines at the end of the screen are read.
	askLines = 8
)

// activity tracks the facts the state is derived from. Session guards it.
type activity struct {
	started time.Time
	last    time.Time
	tail    *ring
}

func newActivity(now time.Time) *activity {
	return &activity{started: now, tail: newRing(tailBytes)}
}

func (a *activity) note(chunk []byte, now time.Time) {
	a.last = now
	// A full-screen clear wipes what was on screen, question included.
	if i := bytes.LastIndex(chunk, clearScreen); i >= 0 {
		a.tail = newRing(tailBytes)
		chunk = chunk[i+len(clearScreen):]
	}
	a.tail.write(chunk)
}

// state derives the activity at the given time. Pure: tests drive it with a
// fake clock.
func (a *activity) state(now time.Time, exited bool) Activity {
	if exited {
		return Ended
	}
	since := a.last
	if since.IsZero() {
		since = a.started
	}
	if now.Sub(since) < QuietAfter {
		return Working
	}
	if asksUser(a.tail.snapshot()) {
		return NeedsYou
	}
	return Idle
}

var clearScreen = []byte{0x1b, '[', '2', 'J'}

var (
	ansiSeq = regexp.MustCompile(`\x1b(\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[@-Z\\-_])`)
	// Wording that means "I am waiting for an answer". Provisional: to be
	// checked against real captured output (issue #52).
	askPatterns = regexp.MustCompile(`(?i)(\(y/n\)|\[y/n\]|\[yes/no\]|do you want to |would you like to |press enter to continue|esc to cancel|❯\s*1\.\s)`)
)

// asksUser reports whether the end of the output looks like a question or a
// choice put to the user.
func asksUser(tail []byte) bool {
	text := ansiSeq.ReplaceAllString(string(tail), "")
	text = strings.ReplaceAll(text, "\r", "\n")
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(strings.Trim(l, "│┃|╭╮╰╯─━ \t")); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > askLines {
		lines = lines[len(lines)-askLines:]
	}
	for _, l := range lines {
		if askPatterns.MatchString(l) {
			return true
		}
	}
	return len(lines) > 0 && strings.HasSuffix(lines[len(lines)-1], "?")
}
