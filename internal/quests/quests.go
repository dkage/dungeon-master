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

	"github.com/dkage/dungeon-master/internal/campaign"
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

	// Set by Annotate from the labels and the campaign's local settings.
	Effort           string   `json:"effort,omitempty"`
	EffortSource     string   `json:"effortSource,omitempty"`
	EffortConflict   []string `json:"effortConflict,omitempty"`
	Priority         string   `json:"priority,omitempty"`
	PriorityConflict []string `json:"priorityConflict,omitempty"`
	Creature         string   `json:"creature,omitempty"`
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

// MarkerLabel is the default label that marks an issue as part of the work
// done here. It is a plain, neutral GitHub label; the scope picker suggests it.
const MarkerLabel = "tracked"

// Listing is what a source returns for a scope.
type Listing struct {
	Quests []Quest
	// TotalOpen is how many issues are open in the whole repository, whatever
	// the scope, so the panel can say "14 of 312 open".
	TotalOpen int
	// Truncated means more issues matched than were fetched.
	Truncated bool
}

// LabelCount is a label with how many open issues carry it.
type LabelCount struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
	Open  int    `json:"open"`
}

// Counts are the numbers behind the first-connection question: how many
// issues each choice would show.
type Counts struct {
	OpenTotal int          `json:"openTotal"`
	Assigned  int          `json:"assigned"`
	Login     string       `json:"login,omitempty"`
	Labels    []LabelCount `json:"labels"`
}

// Source supplies quests for a repository.
type Source interface {
	// List returns the quests inside the scope. The scope is part of the
	// query, never applied to a truncated list afterwards.
	List(ctx context.Context, repo Repo, scope campaign.Scope) (Listing, error)
	Get(ctx context.Context, repo Repo, number int) (Detail, error)
	// Counts returns what each scope choice would show.
	Counts(ctx context.Context, repo Repo) (Counts, error)
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

// RateLimitError is returned when GitHub's rate limit is exhausted, or when
// the app is holding back requests so that it does not get there.
type RateLimitError struct {
	Reset time.Time
	// Paused is set when the app itself is not sending requests: GitHub asked
	// it to slow down earlier, or the remaining budget is below the reserve.
	Paused bool
	// Reason says why, in words fit for the user ("GitHub asked this app to
	// slow down"). Empty means the default for the case.
	Reason string
}

func (e *RateLimitError) Error() string {
	switch {
	case e.Paused && !e.Reset.IsZero():
		return "quests: requests to GitHub are paused until " + e.Reset.Local().Format("15:04")
	case e.Reset.IsZero():
		return "quests: GitHub rate limit reached"
	}
	return "quests: GitHub rate limit reached, resets at " + e.Reset.Local().Format("15:04")
}

// UserMessage is the same fact in plain words for the interface: what
// happened and until when.
func (e *RateLimitError) UserMessage() string {
	if e.Paused {
		reason := e.Reason
		if reason == "" {
			reason = "GitHub asked this app to slow down"
		}
		if e.Reset.IsZero() {
			return "Paused: " + reason + "."
		}
		return "Paused until " + e.Reset.Local().Format("15:04") + ": " + reason + "."
	}
	if e.Reset.IsZero() {
		return "GitHub's rate limit is reached."
	}
	return "GitHub's rate limit is reached; it resets at " + e.Reset.Local().Format("15:04") + "."
}

// Friendly turns an error into a short sentence for the interface, without
// Go-style prefixes.
func Friendly(err error) string {
	var rl *RateLimitError
	switch {
	case errors.As(err, &rl):
		return rl.UserMessage()
	case errors.Is(err, ErrNoToken):
		return "no GitHub credentials were found."
	case errors.Is(err, ErrUnauthorized):
		return "GitHub rejected the credentials."
	case errors.Is(err, ErrNotFound):
		return "GitHub could not find it."
	case errors.Is(err, context.DeadlineExceeded):
		return "GitHub took too long to answer."
	}
	return strings.TrimPrefix(err.Error(), "quests: ")
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
