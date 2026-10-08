// Package github is a quests.Source backed by GitHub's GraphQL API. One
// request returns a page of issues together with labels, assignees, parent,
// blockers and linked pull requests, so listing costs one rate-limit point
// instead of one request per issue.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/quartermaster"
	"github.com/dkage/dungeon-master/internal/quests"
)

const (
	// DefaultEndpoint is GitHub's GraphQL endpoint.
	DefaultEndpoint = "https://api.github.com/graphql"

	pageSize      = 100
	maxOpenPages  = 3  // at most 300 open issues per query
	closedAll     = 50 // recently closed issues shown when everything is in scope
	closedScoped  = 20 // ... and when a label or assignee narrows it
	maxResponse   = 8 << 20
	tokenLifetime = 5 * time.Minute
	loginLifetime = 10 * time.Minute
)

// Options configures a Client.
type Options struct {
	Endpoint string
	HTTP     *http.Client
	Token    TokenFunc
	Now      func() time.Time
	// OnRateLimit is told about a limit that only shows up inside a 200
	// response (a GraphQL error), which the transport cannot see.
	OnRateLimit func(reset time.Time)
}

// Client implements quests.Source for GitHub.
type Client struct {
	endpoint string
	http     *http.Client
	token    *cached
	now      func() time.Time
	onLimit  func(reset time.Time)

	loginMu    sync.Mutex
	loginName  string
	loginUntil time.Time
}

var _ quests.Source = (*Client)(nil)

// New builds a Client.
func New(o Options) *Client {
	if o.Endpoint == "" {
		o.Endpoint = DefaultEndpoint
	}
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 20 * time.Second}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Token == nil {
		o.Token = func(context.Context) (string, error) { return "", quests.ErrNoToken }
	}
	return &Client{
		endpoint: o.Endpoint,
		http:     o.HTTP,
		now:      o.Now,
		onLimit:  o.OnRateLimit,
		token:    &cached{src: o.Token, ttl: tokenLifetime, now: o.Now},
	}
}

// listQuery reads one page of issues. The scope is part of the query: $labels
// and $filter narrow it on GitHub's side, so nothing is cut before filtering.
// `total` is the number of open issues in the whole repository.
const listQuery = `
query($owner:String!,$name:String!,$states:[IssueState!],$first:Int!,$after:String,$labels:[String!],$filter:IssueFilters){
  repository(owner:$owner,name:$name){
    total: issues(states:OPEN, first:1){totalCount}
    issues(first:$first, after:$after, states:$states, labels:$labels, filterBy:$filter, orderBy:{field:UPDATED_AT,direction:DESC}){
      pageInfo{hasNextPage endCursor}
      nodes{
        number title state stateReason url updatedAt
        labels(first:20){nodes{name color}}
        assignees(first:5){nodes{login}}
        milestone{title}
        parent{number}
        blockedBy(first:20){nodes{number title state}}
        subIssuesSummary{total completed}
        closedByPullRequestsReferences(first:3){nodes{number state url merged}}
        comments{totalCount}
      }
    }
  }
}`

// countsQuery reads what each scope choice would show. The marker label is
// asked for by name so it is found even in a repository with over 100 labels.
const countsQuery = `
query($owner:String!,$name:String!,$login:String!,$marker:String!){
  repository(owner:$owner,name:$name){
    total: issues(states:OPEN, first:1){totalCount}
    assigned: issues(states:OPEN, first:1, filterBy:{assignee:$login}){totalCount}
    marker: label(name:$marker){name color issues(states:OPEN, first:1){totalCount}}
    labels(first:100, orderBy:{field:NAME,direction:ASC}){
      nodes{name color issues(states:OPEN, first:1){totalCount}}
    }
  }
}`

const viewerQuery = `query{viewer{login}}`

const detailQuery = `
query($owner:String!,$name:String!,$number:Int!){
  repository(owner:$owner,name:$name){
    issue(number:$number){
      number title state stateReason url updatedAt createdAt body
      author{login}
      labels(first:20){nodes{name color}}
      assignees(first:5){nodes{login}}
      milestone{title}
      parent{number title}
      blockedBy(first:20){nodes{number title state}}
      blocking(first:20){nodes{number title state}}
      subIssues(first:50){nodes{number title state}}
      subIssuesSummary{total completed}
      closedByPullRequestsReferences(first:3){nodes{number state url merged}}
      comments(last:50){totalCount nodes{author{login} body createdAt url}}
    }
  }
}`

// --- response shapes ---

type nodes[T any] struct {
	Nodes      []T `json:"nodes"`
	TotalCount int `json:"totalCount"`
}

type ref struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

type issueNode struct {
	Number      int                            `json:"number"`
	Title       string                         `json:"title"`
	State       string                         `json:"state"`
	StateReason string                         `json:"stateReason"`
	URL         string                         `json:"url"`
	UpdatedAt   time.Time                      `json:"updatedAt"`
	CreatedAt   time.Time                      `json:"createdAt"`
	Body        string                         `json:"body"`
	Author      *struct{ Login string }        `json:"author"`
	Labels      nodes[quests.Label]            `json:"labels"`
	Assignees   nodes[struct{ Login string }]  `json:"assignees"`
	Milestone   *struct{ Title string }        `json:"milestone"`
	Parent      *ref                           `json:"parent"`
	BlockedBy   nodes[ref]                     `json:"blockedBy"`
	Blocking    nodes[ref]                     `json:"blocking"`
	SubIssues   nodes[ref]                     `json:"subIssues"`
	SubSummary  struct{ Total, Completed int } `json:"subIssuesSummary"`
	PRs         nodes[struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		URL    string `json:"url"`
		Merged bool   `json:"merged"`
	}] `json:"closedByPullRequestsReferences"`
	Comments struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			Author    *struct{ Login string } `json:"author"`
			Body      string                  `json:"body"`
			CreatedAt time.Time               `json:"createdAt"`
			URL       string                  `json:"url"`
		} `json:"nodes"`
	} `json:"comments"`
}

type listData struct {
	Repository *struct {
		Total struct {
			TotalCount int `json:"totalCount"`
		} `json:"total"`
		Issues struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []issueNode `json:"nodes"`
		} `json:"issues"`
	} `json:"repository"`
}

type detailData struct {
	Repository *struct {
		Issue *issueNode `json:"issue"`
	} `json:"repository"`
}

// List returns the issues inside the scope: open ones (up to 300 per query)
// plus the most recently closed. Labels are queried one at a time and merged,
// because GitHub ANDs a list of labels.
func (c *Client) List(ctx context.Context, repo quests.Repo, scope campaign.Scope) (quests.Listing, error) {
	switch scope.Mode {
	case campaign.ScopeLabels:
		if len(scope.Labels) == 0 {
			return quests.Listing{}, errors.New("quests: no labels in the scope")
		}
		var out quests.Listing
		seen := map[int]bool{}
		for _, label := range scope.Labels {
			part, err := c.collect(ctx, repo, []string{label}, "", closedScoped)
			if err != nil {
				return quests.Listing{}, err
			}
			out.TotalOpen = part.TotalOpen
			out.Truncated = out.Truncated || part.Truncated
			for _, q := range part.Quests {
				if !seen[q.Number] {
					seen[q.Number] = true
					out.Quests = append(out.Quests, q)
				}
			}
		}
		return out, nil
	case campaign.ScopeAssigned:
		login, err := c.login(ctx)
		if err != nil {
			return quests.Listing{}, err
		}
		return c.collect(ctx, repo, nil, login, closedScoped)
	case campaign.ScopeAll:
		return c.collect(ctx, repo, nil, "", closedAll)
	}
	return quests.Listing{}, fmt.Errorf("quests: unknown scope %q", scope.Mode)
}

// collect reads one query's worth: open pages, then recently closed.
func (c *Client) collect(ctx context.Context, repo quests.Repo, labels []string, assignee string, closed int) (quests.Listing, error) {
	var out quests.Listing
	var after string
	for page := 0; page < maxOpenPages; page++ {
		got, total, next, err := c.listPage(ctx, repo, []string{"OPEN"}, pageSize, after, labels, assignee)
		if err != nil {
			return quests.Listing{}, err
		}
		out.Quests = append(out.Quests, got...)
		out.TotalOpen = total
		if next == "" {
			break
		}
		if page == maxOpenPages-1 {
			out.Truncated = true // more matched than we are willing to fetch
		}
		after = next
	}
	recent, _, _, err := c.listPage(ctx, repo, []string{"CLOSED"}, closed, "", labels, assignee)
	if err != nil {
		return quests.Listing{}, err
	}
	out.Quests = append(out.Quests, recent...)
	return out, nil
}

func (c *Client) listPage(ctx context.Context, repo quests.Repo, states []string, first int, after string, labels []string, assignee string) ([]quests.Quest, int, string, error) {
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "states": states, "first": first}
	if after != "" {
		vars["after"] = after
	}
	if len(labels) > 0 {
		vars["labels"] = labels
	}
	if assignee != "" {
		vars["filter"] = map[string]any{"assignee": assignee}
	}
	var data listData
	if err := c.do(ctx, listQuery, vars, &data); err != nil {
		return nil, 0, "", err
	}
	if data.Repository == nil {
		return nil, 0, "", quests.ErrNotFound
	}
	conn := data.Repository.Issues
	qs := make([]quests.Quest, 0, len(conn.Nodes))
	for _, n := range conn.Nodes {
		qs = append(qs, n.quest())
	}
	next := ""
	if conn.PageInfo.HasNextPage {
		next = conn.PageInfo.EndCursor
	}
	return qs, data.Repository.Total.TotalCount, next, nil
}

// login returns the GitHub user the token belongs to, remembered for a while.
func (c *Client) login(ctx context.Context) (string, error) {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	if c.loginName != "" && c.now().Before(c.loginUntil) {
		return c.loginName, nil
	}
	var data struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	}
	if err := c.do(ctx, viewerQuery, nil, &data); err != nil {
		return "", err
	}
	if data.Viewer.Login == "" {
		return "", quests.ErrUnauthorized
	}
	c.loginName, c.loginUntil = data.Viewer.Login, c.now().Add(loginLifetime)
	return c.loginName, nil
}

// Counts returns how many open issues each scope choice would show.
func (c *Client) Counts(ctx context.Context, repo quests.Repo) (quests.Counts, error) {
	login, err := c.login(ctx)
	if err != nil {
		return quests.Counts{}, err
	}
	var data struct {
		Repository *struct {
			Total    struct{ TotalCount int } `json:"total"`
			Assigned struct{ TotalCount int } `json:"assigned"`
			Marker   *labelNode               `json:"marker"`
			Labels   struct {
				Nodes []labelNode `json:"nodes"`
			} `json:"labels"`
		} `json:"repository"`
	}
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "login": login, "marker": quests.MarkerLabel}
	if err := c.do(ctx, countsQuery, vars, &data); err != nil {
		return quests.Counts{}, err
	}
	if data.Repository == nil {
		return quests.Counts{}, quests.ErrNotFound
	}
	out := quests.Counts{
		OpenTotal: data.Repository.Total.TotalCount, Assigned: data.Repository.Assigned.TotalCount,
		Login: login, Labels: []quests.LabelCount{},
	}
	seen := map[string]bool{}
	add := func(l labelNode) {
		if !seen[l.Name] {
			seen[l.Name] = true
			out.Labels = append(out.Labels, quests.LabelCount{Name: l.Name, Color: l.Color, Open: l.Issues.TotalCount})
		}
	}
	// The marker is always offered, even with nothing in it yet or beyond the first 100 labels.
	if m := data.Repository.Marker; m != nil {
		add(*m)
	} else {
		seen[quests.MarkerLabel] = true
		out.Labels = append(out.Labels, quests.LabelCount{Name: quests.MarkerLabel, Open: 0})
	}
	for _, l := range data.Repository.Labels.Nodes {
		add(l)
	}
	return out, nil
}

type labelNode struct {
	Name   string `json:"name"`
	Color  string `json:"color"`
	Issues struct {
		TotalCount int `json:"totalCount"`
	} `json:"issues"`
}

// Get returns one issue with its body, comments and relations.
func (c *Client) Get(ctx context.Context, repo quests.Repo, number int) (quests.Detail, error) {
	var data detailData
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number}
	if err := c.do(ctx, detailQuery, vars, &data); err != nil {
		return quests.Detail{}, err
	}
	if data.Repository == nil || data.Repository.Issue == nil {
		return quests.Detail{}, quests.ErrNotFound
	}
	n := data.Repository.Issue
	d := quests.Detail{
		Quest:       n.quest(),
		Body:        n.Body,
		Created:     n.CreatedAt,
		Blocking:    toRefs(n.Blocking.Nodes),
		SubIssues:   toRefs(n.SubIssues.Nodes),
		CommentList: []quests.Comment{},
	}
	if n.Author != nil {
		d.Author = n.Author.Login
	}
	if n.Parent != nil {
		d.ParentTitle = n.Parent.Title
	}
	for _, cm := range n.Comments.Nodes {
		author := "ghost" // GitHub's name for deleted accounts
		if cm.Author != nil {
			author = cm.Author.Login
		}
		d.CommentList = append(d.CommentList, quests.Comment{Author: author, Body: cm.Body, Created: cm.CreatedAt, URL: cm.URL})
	}
	return d, nil
}

func (n issueNode) quest() quests.Quest {
	q := quests.Quest{
		Number:    n.Number,
		Title:     n.Title,
		Closed:    n.State == "CLOSED",
		Reason:    reason(n.StateReason),
		URL:       n.URL,
		Labels:    n.Labels.Nodes,
		SubTotal:  n.SubSummary.Total,
		SubDone:   n.SubSummary.Completed,
		Comments:  n.Comments.TotalCount,
		Updated:   n.UpdatedAt,
		BlockedBy: toRefs(n.BlockedBy.Nodes),
	}
	for _, a := range n.Assignees.Nodes {
		q.Assignees = append(q.Assignees, a.Login)
	}
	if n.Milestone != nil {
		q.Milestone = n.Milestone.Title
	}
	if n.Parent != nil {
		q.Parent = n.Parent.Number
	}
	for _, p := range n.PRs.Nodes {
		state := strings.ToLower(p.State)
		if p.Merged {
			state = "merged"
		}
		q.PRs = append(q.PRs, quests.PullRequest{Number: p.Number, URL: p.URL, State: state})
	}
	return quests.Finish(q)
}

func reason(s string) string {
	switch s {
	case "COMPLETED":
		return "completed"
	case "NOT_PLANNED":
		return "not planned"
	}
	return ""
}

func toRefs(in []ref) []quests.Ref {
	out := make([]quests.Ref, 0, len(in))
	for _, r := range in {
		out = append(out, quests.Ref{Number: r.Number, Title: r.Title, Closed: r.State == "CLOSED"})
	}
	return out
}

// --- transport ---

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"errors"`
}

// do runs one GraphQL request. A 401 drops the cached token and retries
// once, since the CLI may have refreshed it.
func (c *Client) do(ctx context.Context, query string, vars map[string]any, out any) error {
	err := c.once(ctx, query, vars, out)
	if errors.Is(err, quests.ErrUnauthorized) {
		c.token.forget()
		err = c.once(ctx, query, vars, out)
	}
	return err
}

func (c *Client) once(ctx context.Context, query string, vars map[string]any, out any) error {
	tok, err := c.token.get(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dungeon-master")

	resp, err := c.http.Do(req)
	if err != nil {
		// The Quartermaster refuses requests without sending them while GitHub
		// has asked us to slow down, or while the budget is under its reserve.
		var camp *quartermaster.CampError
		if errors.As(err, &camp) {
			return &quests.RateLimitError{Reset: camp.Until, Paused: true}
		}
		var ration *quartermaster.RationError
		if errors.As(err, &ration) {
			return &quests.RateLimitError{Reset: ration.Reset, Paused: true}
		}
		// The error text of net/http can include the URL but never headers.
		return fmt.Errorf("quests: reaching GitHub: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return fmt.Errorf("quests: reading GitHub's response: %w", err)
	}
	if len(raw) > maxResponse {
		return errors.New("quests: GitHub's response was unexpectedly large")
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return quests.ErrUnauthorized
	case resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && rateLimited(resp)):
		return &quests.RateLimitError{Reset: resetTime(resp, c.now())}
	case resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("quests: GitHub refused the request (403): %s", firstLine(string(raw)))
	case resp.StatusCode >= 500:
		return fmt.Errorf("quests: GitHub is unavailable (HTTP %d)", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("quests: unexpected GitHub response (HTTP %d)", resp.StatusCode)
	}

	var gr gqlResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return errors.New("quests: GitHub returned something that is not JSON")
	}
	if len(gr.Errors) > 0 {
		for _, e := range gr.Errors {
			switch e.Type {
			case "NOT_FOUND":
				return quests.ErrNotFound
			case "RATE_LIMITED":
				reset := resetTime(resp, c.now())
				if c.onLimit != nil {
					c.onLimit(reset)
				}
				return &quests.RateLimitError{Reset: reset}
			case "FORBIDDEN", "INSUFFICIENT_SCOPES":
				return fmt.Errorf("quests: the token cannot read this repository: %s", e.Message)
			}
		}
		return fmt.Errorf("quests: GitHub: %s", gr.Errors[0].Message)
	}
	if err := json.Unmarshal(gr.Data, out); err != nil {
		return fmt.Errorf("quests: unexpected data shape from GitHub: %w", err)
	}
	return nil
}

func rateLimited(r *http.Response) bool {
	return r.Header.Get("X-RateLimit-Remaining") == "0" || r.Header.Get("Retry-After") != ""
}

// resetTime reads when the limit lifts, from X-RateLimit-Reset (unix
// seconds) or Retry-After (seconds); zero when unknown.
func resetTime(r *http.Response, now time.Time) time.Time {
	if v := r.Header.Get("X-RateLimit-Reset"); v != "" {
		if sec, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(sec, 0)
		}
	}
	if v := r.Header.Get("Retry-After"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil {
			return now.Add(time.Duration(sec) * time.Second)
		}
	}
	return time.Time{}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
