// Package demo is a Source with built-in sample quests, so the quest list
// and the map can be explored without GitHub.
package demo

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/quests"
)

// Source serves a fixed set of sample quests. Times are relative to Now so
// the data always looks recent.
type Source struct {
	Now func() time.Time
}

// New returns a demo source using the real clock.
func New() *Source { return &Source{Now: time.Now} }

type item struct {
	n         int
	title     string
	closed    bool
	reason    string
	labels    []string
	assignees []string
	parent    int
	blockedBy []int
	prs       []quests.PullRequest
	ageHours  int
	body      string
	comments  []quests.Comment
	milestone string
}

// The sample is shaped to show the map (docs/design/map.md §8): two epics,
// the second blocked by a quest of the first, a hall inside the second, a
// chain of four ending in a boss, strays for the Tavern, a quest without an
// effort label, one assigned elsewhere and a blocker outside the scope.
var items = []item{
	{n: 1, title: "Ship the onboarding flow", labels: []string{"epic", "effort:extreme"}, milestone: "v1", ageHours: 3,
		body: "Everything a new user needs from first visit to first success.\n\n## Done when\n\n- a visitor can sign up\n- their address is checked\n- they receive a welcome email\n"},
	{n: 2, title: "Design the welcome screen", closed: true, reason: "completed", labels: []string{"design", "effort:trivial"}, parent: 1, ageHours: 50,
		prs:  []quests.PullRequest{{Number: 21, URL: "https://example.test/pull/21", State: "merged"}},
		body: "Mockups for the first screen a new user sees.\n\nApproved in review; see the linked pull request."},
	{n: 3, title: "Build the signup form", labels: []string{"feature", "effort:moderate"}, parent: 1, blockedBy: []int{2}, ageHours: 5,
		body:     "Collect an email and a password.\n\n```ts\ninterface Signup {\n  email: string\n  password: string\n}\n```\n\n1. render the form\n2. submit to `/api/signup`\n3. show errors inline\n",
		comments: []quests.Comment{{Author: "alice", Body: "The design is approved, so this is unblocked.", Created: time.Time{}}}},
	{n: 4, title: "Validate email addresses", labels: []string{"feature", "effort:moderate"}, parent: 1, blockedBy: []int{3}, ageHours: 6,
		body: "Reject malformed addresses on the client **and** the server."},
	{n: 5, title: "Send the welcome email", labels: []string{"feature", "effort:high", "priority:urgent"}, parent: 1, blockedBy: []int{4, 7}, ageHours: 8,
		body: "Sent once, right after a successful signup. Needs an email provider (see the open question)."},
	{n: 6, title: "Login button unresponsive on Safari", labels: []string{"bug", "effort:low", "priority:high"}, assignees: []string{"alice"}, ageHours: 1,
		prs:  []quests.PullRequest{{Number: 34, URL: "https://example.test/pull/34", State: "open"}},
		body: "## Steps\n\n1. Open the login page in Safari 17\n2. Click **Log in**\n\n## Expected\n\nThe form submits.\n\n## Actual\n\nNothing happens.\n",
		comments: []quests.Comment{
			{Author: "bob", Body: "Reproduced. The click handler is attached to a `div`.", Created: time.Time{}},
			{Author: "alice", Body: "On it, fix is in the linked PR.", Created: time.Time{}},
		}},
	{n: 7, title: "Which email provider should we use?", labels: []string{"question"}, ageHours: 12,
		body: "Options: **A** (cheap, slow), **B** (fast, pricey). Needs a decision before the welcome email."},
	{n: 8, title: "Write the privacy policy", closed: true, reason: "not planned", labels: []string{"docs", "effort:low"}, ageHours: 200,
		body: "Handled by legal; out of scope for this repository."},
	{n: 9, title: "Evaluate rate-limit libraries", labels: []string{"research", "effort:moderate"}, ageHours: 30,
		body: "Compare three libraries for the signup endpoint and write down the trade-offs."},
	{n: 10, title: "Upgrade Node to the latest LTS", labels: []string{"chore", "effort:trivial"}, assignees: []string{"bob"}, ageHours: 20,
		body: "Bump the runtime and the CI matrix."},
	{n: 11, title: "Build the admin dashboard", labels: []string{"epic", "effort:high"}, milestone: "v2", ageHours: 70,
		body: "Internal tools for support staff."},
	{n: 12, title: "List users", labels: []string{"feature", "effort:low"}, parent: 11, blockedBy: []int{3}, ageHours: 69,
		body: "A paginated table of users with search. Needs the signup form, so there are users to list."},
	{n: 13, title: "Suspend a user", labels: []string{"feature", "effort:moderate"}, parent: 11, blockedBy: []int{12, 40}, ageHours: 68,
		body: "Needs the user list first, and the user table migration from the platform team."},
	{n: 14, title: "Keep an audit trail", labels: []string{"feature", "effort:high"}, parent: 11, ageHours: 66,
		body: "Every admin action is recorded and can be reviewed."},
	{n: 15, title: "Record admin actions", labels: []string{"feature", "effort:moderate"}, parent: 14, ageHours: 65,
		body: "Write who did what, and when, for every admin action."},
	{n: 16, title: "Show the audit trail", labels: []string{"feature", "effort:low"}, parent: 14, blockedBy: []int{15}, ageHours: 64,
		body: "A read-only page listing recorded actions, newest first."},
}

// outside are issues the sample's quests refer to but that the scope leaves
// out: they appear as blockers and can be opened, but are never listed.
var outside = []item{
	{n: 40, title: "Migrate the user table", labels: []string{"platform"}, ageHours: 90,
		body: "Owned by the platform team; tracked outside this project's scope."},
}

func (s *Source) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// quests returns the listed sample quests.
func (s *Source) quests() []quests.Quest { return s.build(items) }

// everything also includes the issues outside the scope, for Get.
func (s *Source) everything() []quests.Quest { return s.build(combined) }

// combined lists the sample and the issues outside its scope; byNumber finds any of them.
// Neither is ever changed, so they are built once.
var (
	combined = slices.Concat(items, outside)
	byNumber = func() map[int]item {
		m := make(map[int]item, len(combined))
		for _, it := range combined {
			m[it.n] = it
		}
		return m
	}()
)

func (s *Source) build(list []item) []quests.Quest {
	now := s.now()
	subs := map[int][2]int{} // parent → total, done
	for _, it := range list {
		if it.parent != 0 {
			c := subs[it.parent]
			c[0]++
			if it.closed {
				c[1]++
			}
			subs[it.parent] = c
		}
	}
	out := make([]quests.Quest, 0, len(list))
	for _, it := range list {
		q := quests.Quest{
			Number:    it.n,
			Title:     it.title,
			Closed:    it.closed,
			Reason:    it.reason,
			URL:       "https://example.test/issues/" + strconv.Itoa(it.n),
			Assignees: it.assignees,
			Milestone: it.milestone,
			Parent:    it.parent,
			SubTotal:  subs[it.n][0],
			SubDone:   subs[it.n][1],
			Comments:  len(it.comments),
			PRs:       it.prs,
			Updated:   now.Add(-time.Duration(it.ageHours) * time.Hour),
		}
		for _, l := range it.labels {
			q.Labels = append(q.Labels, quests.Label{Name: l, Color: colors[l]})
		}
		for _, b := range it.blockedBy {
			q.BlockedBy = append(q.BlockedBy, quests.Ref{Number: b, Title: byNumber[b].title, Closed: byNumber[b].closed})
		}
		out = append(out, quests.Finish(q))
	}
	return out
}

var colors = map[string]string{
	"epic": "5319e7", "design": "c5def5", "feature": "0e8a16", "bug": "d73a4a",
	"question": "d876e3", "docs": "0075ca", "research": "fbca04", "chore": "ededed", "platform": "bfd4f2",
	"effort:trivial": "d4c5f9", "effort:low": "c2b2f0", "effort:moderate": "a48fe0",
	"effort:high": "8566cc", "effort:extreme": "6440b8",
	"priority:high": "f9a825", "priority:urgent": "b60205",
}

// List returns every sample quest, regardless of repo and scope.
func (s *Source) List(_ context.Context, _ quests.Repo, _ campaign.Scope) (quests.Listing, error) {
	qs := s.quests()
	open := 0
	for _, q := range qs {
		if !q.Closed {
			open++
		}
	}
	return quests.Listing{Quests: qs, TotalOpen: open}, nil
}

// Counts describes the sample data.
func (s *Source) Counts(_ context.Context, _ quests.Repo) (quests.Counts, error) {
	c := quests.Counts{Login: "alice", Labels: []quests.LabelCount{}}
	byLabel := map[string]int{}
	for _, it := range items {
		if it.closed {
			continue
		}
		c.OpenTotal++
		for _, a := range it.assignees {
			if a == "alice" {
				c.Assigned++
			}
		}
		for _, l := range it.labels {
			byLabel[l]++
		}
	}
	for name, n := range byLabel {
		c.Labels = append(c.Labels, quests.LabelCount{Name: name, Color: colors[name], Open: n})
	}
	sort.Slice(c.Labels, func(i, j int) bool { return c.Labels[i].Name < c.Labels[j].Name })
	return c, nil
}

// Get returns one sample quest with its body, comments and relations.
func (s *Source) Get(_ context.Context, _ quests.Repo, number int) (quests.Detail, error) {
	all := s.everything()
	var base *quests.Quest
	for i := range all {
		if all[i].Number == number {
			base = &all[i]
		}
	}
	if base == nil {
		return quests.Detail{}, quests.ErrNotFound
	}
	it := byNumber[number]
	now := s.now()
	d := quests.Detail{
		Quest:       *base,
		Author:      "alice",
		Body:        it.body,
		Created:     base.Updated.Add(-24 * time.Hour),
		Blocking:    []quests.Ref{},
		SubIssues:   []quests.Ref{},
		CommentList: []quests.Comment{},
	}
	for _, o := range all {
		if o.Parent == number {
			d.SubIssues = append(d.SubIssues, quests.Ref{Number: o.Number, Title: o.Title, Closed: o.Closed})
		}
		for _, b := range o.BlockedBy {
			if b.Number == number {
				d.Blocking = append(d.Blocking, quests.Ref{Number: o.Number, Title: o.Title, Closed: o.Closed})
			}
		}
		if o.Number == base.Parent {
			d.ParentTitle = o.Title
		}
	}
	for i, c := range it.comments {
		c.Created = now.Add(-time.Duration(len(it.comments)-i) * time.Hour)
		d.CommentList = append(d.CommentList, c)
	}
	return d, nil
}
