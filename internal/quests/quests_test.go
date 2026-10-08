package quests

import (
	"testing"
	"time"
)

func TestParseRepo(t *testing.T) {
	r, err := ParseRepo("acme/widgets")
	if err != nil || r.Owner != "acme" || r.Name != "widgets" || r.String() != "acme/widgets" {
		t.Fatalf("%+v %v", r, err)
	}
	for _, bad := range []string{"", "acme", "/x", "x/", "a/b/c"} {
		if _, err := ParseRepo(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestDerive(t *testing.T) {
	open, closed := Ref{Number: 1}, Ref{Number: 2, Closed: true}
	cases := []struct {
		name string
		q    Quest
		want State
	}{
		{"plain", Quest{}, StateOpen},
		{"assigned", Quest{Assignees: []string{"a"}}, StateClaimed},
		{"open blocker", Quest{BlockedBy: []Ref{open}}, StateLocked},
		{"closed blocker only", Quest{BlockedBy: []Ref{closed}}, StateOpen},
		{"mixed blockers", Quest{BlockedBy: []Ref{closed, open}}, StateLocked},
		{"locked beats claimed", Quest{BlockedBy: []Ref{open}, Assignees: []string{"a"}}, StateLocked},
		{"closed", Quest{Closed: true}, StateCleared},
		{"closed beats everything", Quest{Closed: true, BlockedBy: []Ref{open}, Assignees: []string{"a"}}, StateCleared},
	}
	for _, c := range cases {
		if got := Derive(c.q); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestFinishSetsStateAndNonNilSlices(t *testing.T) {
	q := Finish(Quest{Number: 1})
	if q.State != StateOpen || q.Labels == nil || q.Assignees == nil || q.BlockedBy == nil || q.PRs == nil {
		t.Fatalf("%+v", q)
	}
}

func TestFilterLabels(t *testing.T) {
	qs := []Quest{
		{Number: 1, Labels: []Label{{Name: "area:api"}}},
		{Number: 2, Labels: []Label{{Name: "Area:Web"}, {Name: "bug"}}},
		{Number: 3},
	}
	if got := FilterLabels(qs, nil); len(got) != 3 {
		t.Fatalf("no filter: %d", len(got))
	}
	if got := FilterLabels(qs, []string{"area:api"}); len(got) != 1 || got[0].Number != 1 {
		t.Fatalf("one label: %+v", got)
	}
	if got := FilterLabels(qs, []string{"AREA:WEB", "area:api"}); len(got) != 2 {
		t.Fatalf("any-of, case-insensitive: %+v", got)
	}
	if got := FilterLabels(qs, []string{"nope"}); len(got) != 0 {
		t.Fatalf("no match: %+v", got)
	}
}

func TestSortOrder(t *testing.T) {
	now := time.Now()
	qs := []Quest{
		{Number: 1, State: StateCleared, Updated: now},
		{Number: 2, State: StateLocked, Updated: now},
		{Number: 3, State: StateOpen, Updated: now.Add(-time.Hour)},
		{Number: 4, State: StateClaimed, Updated: now},
		{Number: 5, State: StateOpen, Updated: now},
		{Number: 6, State: StateOpen, Updated: now},
	}
	Sort(qs)
	got := []int{}
	for _, q := range qs {
		got = append(got, q.Number)
	}
	want := []int{5, 6, 3, 4, 2, 1} // open (newest first, ties by number), claimed, locked, cleared
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestRateLimitErrorMessage(t *testing.T) {
	if (&RateLimitError{}).Error() == "" {
		t.Fatal("empty message")
	}
	e := &RateLimitError{Reset: time.Date(2026, 1, 1, 15, 4, 0, 0, time.UTC)}
	if got := e.Error(); got == (&RateLimitError{}).Error() {
		t.Fatalf("reset time not mentioned: %q", got)
	}
}
