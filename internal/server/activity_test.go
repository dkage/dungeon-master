package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/testutil"
)

// activityOf reads one session's activity from the state the browser gets.
func (e *env) activityOf(id string) string {
	e.t.Helper()
	_, out := e.req("GET", "/api/state", nil, nil)
	var st struct {
		Sessions []sessionJSON `json:"sessions"`
	}
	if err := json.Unmarshal(out, &st); err != nil {
		e.t.Fatal(err)
	}
	for _, s := range st.Sessions {
		if s.ID == id {
			return s.Activity
		}
	}
	e.t.Fatalf("session %s not in state", id)
	return ""
}

func TestStateReportsWhenASessionNeedsTheUser(t *testing.T) {
	asker := testutil.Program(t, "claude", testutil.ProgramOptions{Mode: "ask", Output: "Delete the branch? (y/n) "})
	e := startServer(t, func(c *Config) { c.Claude = asker })
	sj := e.create(map[string]any{"hero": "opus"})
	if got := e.activityOf(sj.ID); got != "working" {
		t.Fatalf("right after start: %q", got)
	}
	deadline := time.Now().Add(10 * time.Second)
	for e.activityOf(sj.ID) != "needs-you" {
		if time.Now().After(deadline) {
			t.Fatalf("never reported needs-you; last %q", e.activityOf(sj.ID))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestSessionsListMatchesState(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"hero": "opus"})
	resp, out := e.req("GET", "/api/sessions", nil, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list: %d %s", resp.StatusCode, out)
	}
	var list []sessionJSON
	if err := json.Unmarshal(out, &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != sj.ID || list[0].Activity == "" {
		t.Fatalf("list: %+v", list)
	}
}
