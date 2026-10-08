// Package quests turns issues into the app's quests: a plain model, the
// state derived from it, and the Source interface that GitHub and the demo
// data both implement. Nothing here talks to the network.
package quests

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// State is a quest's derived state. It is computed from the issue, never
// stored.
type State string

const (
	// StateOpen is an open, unclaimed, unblocked issue: the frontier.
	StateOpen State = "open"
	// StateLocked is an open issue with at least one open blocker.
	StateLocked State = "locked"
	// StateClaimed is an open, unblocked issue somebody is assigned to.
	StateClaimed State = "claimed"
	// StateCleared is a closed issue.
	StateCleared State = "cleared"
)

// Repo names a GitHub repository.
type Repo struct{ Owner, Name string }

func (r Repo) String() string { return r.Owner + "/" + r.Name }

// ParseRepo splits "owner/name".
func ParseRepo(s string) (Repo, error) {
	owner, name, ok := strings.Cut(s, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return Repo{}, fmt.Errorf("quests: %q is not owner/name", s)
	}
	return Repo{Owner: owner, Name: name}, nil
}

// Label is an issue label.
type Label struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// Ref points at another issue, with whether it is still open.
type Ref struct {
	Number int    `json:"number"`
	Title  string `json:"title,omitempty"`
	Closed bool   `json:"closed"`
}

// PullRequest is a pull request that closes (or would close) the issue.
type PullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"` // "open", "closed", "merged"
}

// Quest is one issue.
type Quest struct {
	Number    int           `json:"number"`
	Title     string        `json:"title"`
	State     State         `json:"state"`
	Closed    bool          `json:"closed"`
	Reason    string        `json:"reason,omitempty"` // "completed", "not planned"
	URL       string        `json:"url"`
	Labels    []Label       `json:"labels"`
	Assignees []string      `json:"assignees"`
	Milestone string        `json:"milestone,omitempty"`
	Parent    int           `json:"parent,omitempty"`
	BlockedBy []Ref         `json:"blockedBy"`
	SubTotal  int           `json:"subTotal"`
	SubDone   int           `json:"subDone"`
	Comments  int           `json:"comments"`
	PRs       []PullRequest `json:"prs"`
	Updated   time.Time     `json:"updated"`
}

// Comment is one comment on an issue.
type Comment struct {
	Author  string    `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
	URL     string    `json:"url,omitempty"`
}

// Detail is a quest with the parts fetched only when it is opened.
type Detail struct {
	Quest
	Author      string    `json:"author"`
	Body        string    `json:"body"`
	Created     time.Time `json:"created"`
	Blocking    []Ref     `json:"blocking"`
	SubIssues   []Ref     `json:"subIssues"`
	ParentTitle string    `json:"parentTitle,omitempty"`
	CommentList []Comment `json:"commentList"`
}

// Source supplies quests for a repository.
type Source interface {
	List(ctx context.Context, repo Repo) ([]Quest, error)
	Get(ctx context.Context, repo Repo, number int) (Detail, error)
}

var (
	// ErrNotFound means the repository or issue does not exist, or the token
	// cannot see it (GitHub answers both the same way).
	ErrNotFound = errors.New("quests: not found")
	// ErrUnauthorized means GitHub rejected the credentials.
	ErrUnauthorized = errors.New("quests: GitHub rejected the credentials")
	// ErrNoToken means no GitHub credentials could be found.
	ErrNoToken = errors.New("quests: no GitHub credentials found")
)

// RateLimitError is returned when GitHub's rate limit is exhausted.
type RateLimitError struct{ Reset time.Time }

func (e *RateLimitError) Error() string {
	if e.Reset.IsZero() {
		return "quests: GitHub rate limit reached"
	}
	return "quests: GitHub rate limit reached, resets at " + e.Reset.Format(time.Kitchen)
}

// Derive computes a quest's state from its own fields.
func Derive(q Quest) State {
	switch {
	case q.Closed:
		return StateCleared
	case hasOpenBlocker(q.BlockedBy):
		return StateLocked
	case len(q.Assignees) > 0:
		return StateClaimed
	default:
		return StateOpen
	}
}

func hasOpenBlocker(refs []Ref) bool {
	for _, r := range refs {
		if !r.Closed {
			return true
		}
	}
	return false
}

// Finish sets derived fields and makes slices non-nil so they encode as [].
func Finish(q Quest) Quest {
	if q.Labels == nil {
		q.Labels = []Label{}
	}
	if q.Assignees == nil {
		q.Assignees = []string{}
	}
	if q.BlockedBy == nil {
		q.BlockedBy = []Ref{}
	}
	if q.PRs == nil {
		q.PRs = []PullRequest{}
	}
	q.State = Derive(q)
	return q
}

// FilterLabels keeps quests that carry at least one of the labels
// (case-insensitive). No labels means no filtering.
func FilterLabels(qs []Quest, labels []string) []Quest {
	if len(labels) == 0 {
		return qs
	}
	want := map[string]bool{}
	for _, l := range labels {
		want[strings.ToLower(l)] = true
	}
	out := make([]Quest, 0, len(qs))
	for _, q := range qs {
		for _, l := range q.Labels {
			if want[strings.ToLower(l.Name)] {
				out = append(out, q)
				break
			}
		}
	}
	return out
}

// Sort orders quests for display: open work first (frontier, then claimed,
// then locked), cleared last; newest activity first within a group.
func Sort(qs []Quest) {
	rank := map[State]int{StateOpen: 0, StateClaimed: 1, StateLocked: 2, StateCleared: 3}
	sort.SliceStable(qs, func(i, j int) bool {
		if ri, rj := rank[qs[i].State], rank[qs[j].State]; ri != rj {
			return ri < rj
		}
		if !qs[i].Updated.Equal(qs[j].Updated) {
			return qs[i].Updated.After(qs[j].Updated)
		}
		return qs[i].Number < qs[j].Number
	})
}
