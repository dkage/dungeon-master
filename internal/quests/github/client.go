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
	"time"

	"github.com/dkage/dungeon-master/internal/quartermaster"
	"github.com/dkage/dungeon-master/internal/quests"
)

const (
	// DefaultEndpoint is GitHub's GraphQL endpoint.
	DefaultEndpoint = "https://api.github.com/graphql"

	pageSize      = 100
	maxOpenPages  = 3 // at most 300 open issues
	closedRecent  = 50
	maxResponse   = 8 << 20
	tokenLifetime = 5 * time.Minute
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

const listQuery = `
query($owner:String!,$name:String!,$states:[IssueState!],$first:Int!,$after:String){
  repository(owner:$owner,name:$name){
    issues(first:$first, after:$after, states:$states, orderBy:{field:UPDATED_AT,direction:DESC}){
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

// List returns open issues (up to 300) and the most recently updated closed
// ones, already finished (state derived).
func (c *Client) List(ctx context.Context, repo quests.Repo) ([]quests.Quest, error) {
	var out []quests.Quest
	var after string
	for page := 0; page < maxOpenPages; page++ {
		got, next, err := c.listPage(ctx, repo, []string{"OPEN"}, pageSize, after)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
		if next == "" {
			break
		}
		after = next
	}
	closed, _, err := c.listPage(ctx, repo, []string{"CLOSED"}, closedRecent, "")
	if err != nil {
		return nil, err
	}
	return append(out, closed...), nil
}

func (c *Client) listPage(ctx context.Context, repo quests.Repo, states []string, first int, after string) ([]quests.Quest, string, error) {
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "states": states, "first": first}
	if after != "" {
		vars["after"] = after
	}
	var data listData
	if err := c.do(ctx, listQuery, vars, &data); err != nil {
		return nil, "", err
	}
	if data.Repository == nil {
		return nil, "", quests.ErrNotFound
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
	return qs, next, nil
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
