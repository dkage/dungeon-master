package demo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/quests"
)

var fixed = func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }

func list(t *testing.T) map[int]quests.Quest {
	t.Helper()
	qs, err := (&Source{Now: fixed}).List(context.Background(), quests.Repo{})
	if err != nil {
		t.Fatal(err)
	}
	m := map[int]quests.Quest{}
	for _, q := range qs {
		m[q.Number] = q
	}
	return m
}

func TestDemoShowsEveryState(t *testing.T) {
	m := list(t)
	want := map[int]quests.State{
		1:  quests.StateOpen,    // epic, unassigned, unblocked
		2:  quests.StateCleared, // closed
		3:  quests.StateOpen,    // its blocker (2) is closed: the frontier
		4:  quests.StateLocked,  // blocked by 3 (open)
		5:  quests.StateLocked,  // blocked by 4 and 7
		6:  quests.StateClaimed, // assigned
		12: quests.StateOpen,
		13: quests.StateLocked,
	}
	for n, s := range want {
		if m[n].State != s {
			t.Errorf("#%d: got %s want %s", n, m[n].State, s)
		}
	}
	seen := map[quests.State]bool{}
	for _, q := range m {
		seen[q.State] = true
	}
	for _, s := range []quests.State{quests.StateOpen, quests.StateLocked, quests.StateClaimed, quests.StateCleared} {
		if !seen[s] {
			t.Errorf("demo data never shows state %s", s)
		}
	}
}

func TestDemoRelationshipsAreConsistent(t *testing.T) {
	m := list(t)
	if q := m[1]; q.SubTotal != 4 || q.SubDone != 1 {
		t.Errorf("epic sub-issue counts: %d/%d", q.SubDone, q.SubTotal)
	}
	if q := m[3]; q.Parent != 1 || len(q.BlockedBy) != 1 || !q.BlockedBy[0].Closed || q.BlockedBy[0].Title == "" {
		t.Errorf("#3: %+v", q)
	}
	for _, q := range m {
		for _, b := range q.BlockedBy {
			if _, ok := m[b.Number]; !ok {
				t.Errorf("#%d is blocked by unknown #%d", q.Number, b.Number)
			}
		}
		if q.Parent != 0 {
			if _, ok := m[q.Parent]; !ok {
				t.Errorf("#%d has unknown parent #%d", q.Number, q.Parent)
			}
		}
		if q.Labels == nil || q.Assignees == nil || q.BlockedBy == nil || q.PRs == nil {
			t.Errorf("#%d has nil slices", q.Number)
		}
	}
}

func TestDemoDetail(t *testing.T) {
	s := &Source{Now: fixed}
	d, err := s.Get(context.Background(), quests.Repo{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if d.Body == "" || d.ParentTitle == "" || len(d.CommentList) != 1 || d.CommentList[0].Created.IsZero() {
		t.Fatalf("detail: %+v", d)
	}
	if len(d.Blocking) != 1 || d.Blocking[0].Number != 4 {
		t.Errorf("#3 should block #4: %+v", d.Blocking)
	}
	epic, _ := s.Get(context.Background(), quests.Repo{}, 1)
	if len(epic.SubIssues) != 4 {
		t.Errorf("epic sub-issues: %+v", epic.SubIssues)
	}
	if _, err := s.Get(context.Background(), quests.Repo{}, 999); !errors.Is(err, quests.ErrNotFound) {
		t.Errorf("unknown quest: %v", err)
	}
}

func TestDemoTimesFollowTheClock(t *testing.T) {
	a := list(t)
	later := &Source{Now: func() time.Time { return fixed().Add(48 * time.Hour) }}
	qs, _ := later.List(context.Background(), quests.Repo{})
	for _, q := range qs {
		if q.Number == 6 && !q.Updated.After(a[6].Updated) {
			t.Fatal("updated time did not move with the clock")
		}
	}
}
