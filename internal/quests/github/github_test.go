package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/quartermaster"
	"github.com/dkage/dungeon-master/internal/quests"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type gqlReq struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// fakeGitHub serves handler on a test server and returns a Client pointed at it.
func fakeGitHub(t *testing.T, token TokenFunc, handler func(w http.ResponseWriter, r *http.Request, q gqlReq)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q gqlReq
		_ = json.NewDecoder(r.Body).Decode(&q)
		handler(w, r, q)
	}))
	t.Cleanup(srv.Close)
	if token == nil {
		token = func(context.Context) (string, error) { return "tok-123", nil }
	}
	return New(Options{Endpoint: srv.URL, Token: token})
}

var demoRepo = quests.Repo{Owner: "dkage", Name: "dungeon-master"}

func TestListMapsARealResponse(t *testing.T) {
	list := fixture(t, "list.json")
	empty := []byte(`{"data":{"repository":{"issues":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}`)
	var sawAuth, sawUA string
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		sawAuth, sawUA = r.Header.Get("Authorization"), r.Header.Get("User-Agent")
		if q.Variables["owner"] != "dkage" || q.Variables["name"] != "dungeon-master" {
			t.Errorf("variables: %v", q.Variables)
		}
		if states, _ := q.Variables["states"].([]any); len(states) == 1 && states[0] == "CLOSED" {
			w.Write(empty)
			return
		}
		w.Write(list)
	})
	got, err := c.List(context.Background(), demoRepo)
	if err != nil {
		t.Fatal(err)
	}
	if sawAuth != "Bearer tok-123" || sawUA != "dungeon-master" {
		t.Errorf("auth %q ua %q", sawAuth, sawUA)
	}
	by := map[int]quests.Quest{}
	for _, q := range got {
		by[q.Number] = q
	}
	if len(by) != 5 {
		t.Fatalf("got %d quests", len(by))
	}
	// #3: sub-issue of #1, blocked by the open #2 → locked.
	q3 := by[3]
	if q3.Parent != 1 || len(q3.BlockedBy) != 1 || q3.BlockedBy[0].Number != 2 || q3.BlockedBy[0].Closed ||
		q3.State != quests.StateLocked || q3.Comments != 1 {
		t.Errorf("#3: %+v", q3)
	}
	// #1: parent of two.
	if q := by[1]; q.SubTotal != 2 || q.SubDone != 0 || q.State != quests.StateOpen {
		t.Errorf("#1: %+v", q)
	}
	// #5: assigned bug → claimed, label kept with its colour.
	q5 := by[5]
	if q5.State != quests.StateClaimed || len(q5.Assignees) != 1 || q5.Assignees[0] != "dkage" ||
		len(q5.Labels) != 1 || q5.Labels[0].Name != "bug" || q5.Labels[0].Color == "" {
		t.Errorf("#5: %+v", q5)
	}
	if q := by[4]; q.State != quests.StateOpen || q.URL == "" || q.Updated.IsZero() {
		t.Errorf("#4: %+v", q)
	}
	for _, q := range got {
		if q.Labels == nil || q.Assignees == nil || q.BlockedBy == nil || q.PRs == nil {
			t.Errorf("#%d has nil slices", q.Number)
		}
	}
}

func TestListPagesThroughOpenIssuesAndAddsClosed(t *testing.T) {
	var openCalls, closedCalls atomic.Int32
	page := func(num int, next bool, cursor string) []byte {
		b, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{"issues": map[string]any{
			"pageInfo": map[string]any{"hasNextPage": next, "endCursor": cursor},
			"nodes":    []map[string]any{{"number": num, "title": "t", "state": "OPEN"}},
		}}}})
		return b
	}
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		states, _ := q.Variables["states"].([]any)
		if states[0] == "CLOSED" {
			closedCalls.Add(1)
			b, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{"issues": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": false},
				"nodes":    []map[string]any{{"number": 99, "title": "old", "state": "CLOSED", "stateReason": "COMPLETED"}},
			}}}})
			w.Write(b)
			return
		}
		switch openCalls.Add(1) {
		case 1:
			if _, has := q.Variables["after"]; has {
				t.Error("first page must not send a cursor")
			}
			w.Write(page(1, true, "c1"))
		case 2:
			if q.Variables["after"] != "c1" {
				t.Errorf("second page cursor: %v", q.Variables["after"])
			}
			w.Write(page(2, false, ""))
		}
	})
	got, err := c.List(context.Background(), demoRepo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Number != 1 || got[1].Number != 2 || got[2].Number != 99 || !got[2].Closed || got[2].Reason != "completed" || got[2].State != quests.StateCleared {
		t.Fatalf("%+v", got)
	}
	if openCalls.Load() != 2 || closedCalls.Load() != 1 {
		t.Fatalf("open pages %d, closed pages %d", openCalls.Load(), closedCalls.Load())
	}
}

func TestListStopsAtThePageCap(t *testing.T) {
	var openCalls atomic.Int32
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		states, _ := q.Variables["states"].([]any)
		more := true
		if states[0] == "CLOSED" {
			more = false
		} else {
			openCalls.Add(1)
		}
		b, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{"issues": map[string]any{
			"pageInfo": map[string]any{"hasNextPage": more, "endCursor": "again"}, "nodes": []any{},
		}}}})
		w.Write(b)
	})
	if _, err := c.List(context.Background(), demoRepo); err != nil {
		t.Fatal(err)
	}
	if openCalls.Load() != maxOpenPages {
		t.Fatalf("fetched %d open pages, cap is %d", openCalls.Load(), maxOpenPages)
	}
}

func TestGetMapsARealResponse(t *testing.T) {
	detail := fixture(t, "detail.json")
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		if q.Variables["number"] != float64(3) {
			t.Errorf("number variable: %v", q.Variables["number"])
		}
		w.Write(detail)
	})
	d, err := c.Get(context.Background(), demoRepo, 3)
	if err != nil {
		t.Fatal(err)
	}
	if d.Number != 3 || d.Author != "dkage" || d.Body == "" || d.Parent != 1 || d.ParentTitle == "" ||
		d.Created.IsZero() || d.State != quests.StateLocked {
		t.Errorf("detail: %+v", d)
	}
	if len(d.CommentList) != 1 || d.CommentList[0].Author != "dkage" || !strings.Contains(d.CommentList[0].Body, "layout sketch") ||
		d.CommentList[0].Created.IsZero() {
		t.Errorf("comments: %+v", d.CommentList)
	}
	if d.Blocking == nil || d.SubIssues == nil {
		t.Error("relation slices must be non-nil")
	}
}

func TestGetUnknownIssueIsNotFound(t *testing.T) {
	for name, body := range map[string]string{
		"null issue":      `{"data":{"repository":{"issue":null}}}`,
		"null repository": `{"data":{"repository":null}}`,
		"not found error": `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","message":"Could not resolve to a Repository"}]}`,
	} {
		c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) { w.Write([]byte(body)) })
		if _, err := c.Get(context.Background(), demoRepo, 1); !errors.Is(err, quests.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
		if _, err := c.List(context.Background(), demoRepo); !errors.Is(err, quests.ErrNotFound) && name != "null issue" {
			t.Errorf("%s (list): %v", name, err)
		}
	}
}

func TestMissingCredentials(t *testing.T) {
	var hits atomic.Int32
	c := fakeGitHub(t, func(context.Context) (string, error) { return "", quests.ErrNoToken }, func(w http.ResponseWriter, r *http.Request, q gqlReq) { hits.Add(1) })
	if _, err := c.List(context.Background(), demoRepo); !errors.Is(err, quests.ErrNoToken) {
		t.Fatalf("err %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("a request was made without a token")
	}
}

func TestUnauthorizedRefreshesTheTokenOnce(t *testing.T) {
	var provided atomic.Int32
	token := func(context.Context) (string, error) {
		if provided.Add(1) == 1 {
			return "stale", nil
		}
		return "fresh", nil
	}
	var seen []string
	c := fakeGitHub(t, token, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		auth := r.Header.Get("Authorization")
		seen = append(seen, auth)
		if auth != "Bearer fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"data":{"repository":{"issue":{"number":1,"title":"x","state":"OPEN"}}}}`))
	})
	if _, err := c.Get(context.Background(), demoRepo, 1); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != "Bearer stale" || seen[1] != "Bearer fresh" {
		t.Fatalf("requests: %v", seen)
	}
}

func TestPersistentUnauthorizedGivesUp(t *testing.T) {
	var hits atomic.Int32
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	})
	if _, err := c.Get(context.Background(), demoRepo, 1); !errors.Is(err, quests.ErrUnauthorized) {
		t.Fatalf("err %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("requests: %d, want exactly one retry", hits.Load())
	}
}

func TestRateLimits(t *testing.T) {
	reset := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	cases := map[string]func(w http.ResponseWriter){
		"primary 403": func(w http.ResponseWriter) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", itoa(reset.Unix()))
			w.WriteHeader(http.StatusForbidden)
		},
		"secondary 429": func(w http.ResponseWriter) {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
		},
		"graphql error": func(w http.ResponseWriter) {
			w.Write([]byte(`{"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`))
		},
	}
	for name, respond := range cases {
		c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) { respond(w) })
		_, err := c.Get(context.Background(), demoRepo, 1)
		var rl *quests.RateLimitError
		if !errors.As(err, &rl) {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if name == "primary 403" && !rl.Reset.Equal(reset) {
			t.Errorf("reset %v, want %v", rl.Reset, reset)
		}
		if name == "secondary 429" && rl.Reset.IsZero() {
			t.Error("Retry-After not turned into a reset time")
		}
	}
}

func TestOtherFailuresAreDescriptiveAndNeverLeakTheToken(t *testing.T) {
	const secret = "ghp_SUPERSECRET"
	cases := map[string]func(w http.ResponseWriter){
		"forbidden": func(w http.ResponseWriter) { w.WriteHeader(403); w.Write([]byte("Resource not accessible\nmore")) },
		"server":    func(w http.ResponseWriter) { w.WriteHeader(502) },
		"teapot":    func(w http.ResponseWriter) { w.WriteHeader(418) },
		"not json":  func(w http.ResponseWriter) { w.Write([]byte("<html>")) },
		"gql error": func(w http.ResponseWriter) { w.Write([]byte(`{"errors":[{"type":"X","message":"boom"}]}`)) },
		"scopes": func(w http.ResponseWriter) {
			w.Write([]byte(`{"errors":[{"type":"INSUFFICIENT_SCOPES","message":"needs repo"}]}`))
		},
		"wrong shape": func(w http.ResponseWriter) { w.Write([]byte(`{"data":{"repository":"nope"}}`)) },
	}
	for name, respond := range cases {
		c := fakeGitHub(t, func(context.Context) (string, error) { return secret, nil }, func(w http.ResponseWriter, r *http.Request, q gqlReq) { respond(w) })
		_, err := c.Get(context.Background(), demoRepo, 1)
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("%s: error leaks the token: %v", name, err)
		}
	}
	// A refused connection names the problem without the token either.
	c := New(Options{Endpoint: "http://127.0.0.1:1", Token: func(context.Context) (string, error) { return secret, nil }})
	if _, err := c.Get(context.Background(), demoRepo, 1); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("connection error: %v", err)
	}
}

func TestOversizedResponseIsRejected(t *testing.T) {
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		w.Write([]byte(strings.Repeat("x", maxResponse+10)))
	})
	if _, err := c.Get(context.Background(), demoRepo, 1); err == nil || !strings.Contains(err.Error(), "large") {
		t.Fatalf("err %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := c.Get(ctx, demoRepo, 1); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDefaultsAreSafe(t *testing.T) {
	c := New(Options{})
	if c.endpoint != DefaultEndpoint || c.http == nil || c.http.Timeout == 0 {
		t.Fatalf("%+v", c)
	}
	if _, err := c.Get(context.Background(), demoRepo, 1); !errors.Is(err, quests.ErrNoToken) {
		t.Fatalf("a client without a token source must refuse: %v", err)
	}
}

func TestStateReasonAndPullRequestMapping(t *testing.T) {
	if reason("COMPLETED") != "completed" || reason("NOT_PLANNED") != "not planned" || reason("REOPENED") != "" || reason("") != "" {
		t.Fatal("reason mapping")
	}
	var n issueNode
	_ = json.Unmarshal([]byte(`{"number":7,"state":"CLOSED","stateReason":"NOT_PLANNED","closedByPullRequestsReferences":{"nodes":[
		{"number":1,"state":"MERGED","url":"u1","merged":true},{"number":2,"state":"OPEN","url":"u2","merged":false}]},
		"milestone":{"title":"v1"},"comments":{"totalCount":4}}`), &n)
	q := n.quest()
	if !q.Closed || q.Reason != "not planned" || q.Milestone != "v1" || q.Comments != 4 || q.State != quests.StateCleared ||
		len(q.PRs) != 2 || q.PRs[0].State != "merged" || q.PRs[1].State != "open" {
		t.Fatalf("%+v", q)
	}
}

// stubTransport returns a fixed error, standing in for the Quartermaster refusing a request.
type stubTransport struct{ err error }

func (s stubTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, s.err }

func TestQuartermasterRefusalsBecomePausedRateLimits(t *testing.T) {
	until := time.Now().Add(7 * time.Minute)
	for name, err := range map[string]error{
		"camp":   &quartermaster.CampError{Until: until, Reason: "x"},
		"ration": &quartermaster.RationError{Resource: "graphql", Remaining: 10, Floor: 250, Reset: until},
	} {
		c := New(Options{
			Endpoint: "http://127.0.0.1:1",
			Token:    func(context.Context) (string, error) { return "tok", nil },
			HTTP:     &http.Client{Transport: stubTransport{err}},
		})
		_, gotErr := c.Get(context.Background(), demoRepo, 1)
		var rl *quests.RateLimitError
		if !errors.As(gotErr, &rl) || !rl.Paused || !rl.Reset.Equal(until) {
			t.Errorf("%s: %v", name, gotErr)
		}
		if _, listErr := c.List(context.Background(), demoRepo); !errors.As(listErr, &rl) || !rl.Paused {
			t.Errorf("%s (list): %v", name, listErr)
		}
	}
}

func TestGraphQLRateLimitInsideA200IsReportedToTheCaller(t *testing.T) {
	var told []time.Time
	reset := time.Now().Add(9 * time.Minute).Truncate(time.Second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", itoa(reset.Unix()))
		w.Write([]byte(`{"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`))
	}))
	defer srv.Close()
	c := New(Options{
		Endpoint:    srv.URL,
		Token:       func(context.Context) (string, error) { return "tok", nil },
		OnRateLimit: func(r time.Time) { told = append(told, r) },
	})
	_, err := c.Get(context.Background(), demoRepo, 1)
	var rl *quests.RateLimitError
	if !errors.As(err, &rl) || rl.Paused {
		t.Fatalf("%v", err)
	}
	if len(told) != 1 || !told[0].Equal(reset) {
		t.Fatalf("OnRateLimit calls: %v", told)
	}
}

func TestHTTPLevelLimitsAreLeftToTheTransport(t *testing.T) {
	// A 429 is seen (and acted on) by the Quartermaster in the transport, so the
	// client must not trip a second time and double the backoff.
	var told int
	c := fakeGitHubWith(t, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}, func(o *Options) { o.OnRateLimit = func(time.Time) { told++ } })
	_, err := c.Get(context.Background(), demoRepo, 1)
	var rl *quests.RateLimitError
	if !errors.As(err, &rl) || told != 0 {
		t.Fatalf("err %v, OnRateLimit calls %d", err, told)
	}
}

func fakeGitHubWith(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, q gqlReq), mutate func(*Options)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q gqlReq
		_ = json.NewDecoder(r.Body).Decode(&q)
		handler(w, r, q)
	}))
	t.Cleanup(srv.Close)
	o := Options{Endpoint: srv.URL, Token: func(context.Context) (string, error) { return "tok", nil }}
	mutate(&o)
	return New(o)
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
