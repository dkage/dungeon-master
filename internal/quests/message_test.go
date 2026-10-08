package quests

import (
	"strings"
	"testing"
	"time"
)

func TestUserMessageIsPlainAndSaysWhatAndUntilWhen(t *testing.T) {
	reset := time.Date(2026, 10, 8, 15, 12, 0, 0, time.Local)

	paused := (&RateLimitError{Paused: true, Reset: reset}).UserMessage()
	for _, want := range []string{"Paused until 15:12", "GitHub asked this app to slow down"} {
		if !strings.Contains(paused, want) {
			t.Errorf("paused message %q lacks %q", paused, want)
		}
	}

	custom := (&RateLimitError{Paused: true, Reset: reset, Reason: "the hourly budget is nearly used up"}).UserMessage()
	if !strings.Contains(custom, "nearly used up") {
		t.Errorf("reason lost: %q", custom)
	}

	limited := (&RateLimitError{Reset: reset}).UserMessage()
	if !strings.Contains(limited, "resets at 15:12") {
		t.Errorf("limit message %q lacks the reset time", limited)
	}

	for _, m := range []string{paused, custom, limited, Friendly(&RateLimitError{Paused: true})} {
		low := strings.ToLower(m)
		for _, internal := range []string{"camp", "quartermaster", "rations", "chronicle", "gatekeeper"} {
			if strings.Contains(low, internal) {
				t.Errorf("message %q leaks the internal word %q", m, internal)
			}
		}
	}
}
