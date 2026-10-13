package demo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/quests"
)

var fixed = func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }

func list(t *testing.T) map[int]quests.Quest {
	t.Helper()
	l, err := (&Source{Now: fixed}).List(context.Background(), quests.Repo{}, campaign.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	m := map[int]quests.Quest{}
	for _, q := range l.Quests {
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
		12: quests.StateLocked,  // blocked by 3, in the other epic
		13: quests.StateLocked,
		15: quests.StateOpen,
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
			if _, ok := m[b.Number]; !ok && b.Number != 40 {
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
	if len(d.Blocking) != 2 || d.Blocking[0].Number != 4 || d.Blocking[1].Number != 12 {
		t.Errorf("#3 should block #4 and #12: %+v", d.Blocking)
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
	l2, _ := later.List(context.Background(), quests.Repo{}, campaign.Scope{})
	for _, q := range l2.Quests {
		if q.Number == 6 && !q.Updated.After(a[6].Updated) {
			t.Fatal("updated time did not move with the clock")
		}
	}
}

func TestDemoTotalsAndCounts(t *testing.T) {
	s := &Source{Now: fixed}
	l, _ := s.List(context.Background(), quests.Repo{}, campaign.Scope{Mode: campaign.ScopeAssigned})
	open := 0
	for _, q := range l.Quests {
		if !q.Closed {
			open++
		}
	}
	if l.TotalOpen != open || l.TotalOpen == 0 || l.Truncated {
		t.Fatalf("totals: %+v", l)
	}
	c, err := s.Counts(context.Background(), quests.Repo{})
	if err != nil || c.OpenTotal != l.TotalOpen || c.Assigned != 1 || c.Login != "alice" || len(c.Labels) < 4 {
		t.Fatalf("counts: %+v %v", c, err)
	}
	for i := 1; i < len(c.Labels); i++ {
		if c.Labels[i-1].Name >= c.Labels[i].Name {
			t.Fatalf("labels not sorted: %v", c.Labels)
		}
	}
	for _, lc := range c.Labels {
		if lc.Open <= 0 {
			t.Fatalf("label with nothing open: %+v", lc)
		}
	}
}

// The sample is the map's showcase (docs/design/map.md §8), so its shape is
// part of the contract.
func TestDemoShowsTheMapShape(t *testing.T) {
	m := list(t)
	hasChildren := map[int]bool{}
	for _, q := range m {
		if q.Parent != 0 {
			hasChildren[q.Parent] = true
		}
	}
	var dungeons, tavern []int
	for n, q := range m {
		switch {
		case q.Parent == 0 && hasChildren[n]:
			dungeons = append(dungeons, n)
		case q.Parent == 0:
			tavern = append(tavern, n)
		}
	}
	if len(dungeons) != 2 {
		t.Errorf("want two dungeons (top-level epics), got %v", dungeons)
	}
	if len(tavern) < 2 {
		t.Errorf("want at least two quests for the Tavern, got %v", tavern)
	}
	if m[14].Parent != 11 || !hasChildren[14] {
		t.Error("#14 should be a hall: a sub-epic of #11 with its own children")
	}
	if b := m[12].BlockedBy; len(b) != 1 || b[0].Number != 3 || m[3].Parent != 1 {
		t.Error("#12 (admin) should be blocked by #3 (onboarding), drawing a line between the dungeons")
	}
	if _, listed := m[40]; listed {
		t.Error("#40 is outside the scope and must not be listed")
	}
	if b := m[13].BlockedBy; len(b) != 2 || b[1].Number != 40 || b[1].Title == "" || b[1].Closed {
		t.Errorf("#13 should be blocked by #40, open and outside the scope: %+v", b)
	}
	effortless := 0
	for _, q := range m {
		e := quests.ResolveEffort(q.Labels, nil, "")
		if e.Tier == "" {
			effortless++
		}
	}
	if effortless != 1 || quests.ResolveEffort(m[7].Labels, nil, "").Tier != "" {
		t.Errorf("exactly one quest (#7) should have no effort label, got %d", effortless)
	}
	if p, _ := quests.ResolvePriority(m[5].Labels); p != "urgent" {
		t.Errorf("#5 priority: %s", p)
	}
}

func TestDemoOutsideIssueCanBeOpened(t *testing.T) {
	d, err := (&Source{Now: fixed}).Get(context.Background(), quests.Repo{}, 40)
	if err != nil || d.Title != "Migrate the user table" || d.Body == "" {
		t.Fatalf("outside issue: %+v %v", d, err)
	}
	if len(d.Blocking) != 1 || d.Blocking[0].Number != 13 {
		t.Errorf("#40 should say it blocks #13: %+v", d.Blocking)
	}
}
