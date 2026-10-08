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

	"github.com/dkage/dungeon-master/internal/campaign"
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

var everything = campaign.Scope{Mode: campaign.ScopeAll, Labels: []string{}}

// replay serves recorded responses, chosen by what the request asks for.
func replay(t *testing.T, calls *[]gqlReq) func(w http.ResponseWriter, r *http.Request, q gqlReq) {
	return func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		if calls != nil {
			*calls = append(*calls, q)
		}
		switch {
		case strings.Contains(q.Query, "viewer{login}"):
			w.Write(fixture(t, "viewer.json"))
		case strings.Contains(q.Query, "assigned: issues"):
			w.Write(fixture(t, "counts.json"))
		default:
			states, _ := q.Variables["states"].([]any)
			closed := len(states) == 1 && states[0] == "CLOSED"
			kind := "all"
			if _, ok := q.Variables["labels"]; ok {
				kind = "labels"
			} else if _, ok := q.Variables["filter"]; ok {
				kind = "assigned"
			}
			which := "open"
			if closed {
				which = "closed"
			}
			w.Write(fixture(t, "list_"+kind+"_"+which+".json"))
		}
	}
}

func TestListMapsARealResponse(t *testing.T) {
	var sawAuth, sawUA string
	var calls []gqlReq
	inner := replay(t, &calls)
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		sawAuth, sawUA = r.Header.Get("Authorization"), r.Header.Get("User-Agent")
		if q.Variables["owner"] != "dkage" || q.Variables["name"] != "dungeon-master" {
			t.Errorf("variables: %v", q.Variables)
		}
		inner(w, r, q)
	})
	listing, err := c.List(context.Background(), demoRepo, campaign.Scope{Mode: campaign.ScopeLabels, Labels: []string{"enhancement"}})
	if err != nil {
		t.Fatal(err)
	}
	if sawAuth != "Bearer tok-123" || sawUA != "dungeon-master" {
		t.Errorf("auth %q ua %q", sawAuth, sawUA)
	}
	// The scope is part of the query: the label goes to GitHub, not applied afterwards.
	if len(calls) != 2 {
		t.Fatalf("%d requests (one open page, one closed)", len(calls))
	}
	for _, q := range calls {
		labels, _ := q.Variables["labels"].([]any)
		if len(labels) != 1 || labels[0] != "enhancement" {
			t.Errorf("labels variable: %v", q.Variables["labels"])
		}
		if _, has := q.Variables["filter"]; has {
			t.Error("a label scope must not send an assignee filter")
		}
	}
	if first, _ := calls[1].Variables["first"].(float64); first != 20 {
		t.Errorf("recently closed in a narrowed scope: first=%v, want 20", first)
	}
	if listing.TotalOpen != 5 || listing.Truncated {
		t.Fatalf("listing: total %d truncated %v", listing.TotalOpen, listing.Truncated)
	}
	by := map[int]quests.Quest{}
	for _, q := range listing.Quests {
		by[q.Number] = q
	}
	if len(by) != 3 {
		t.Fatalf("got %d quests: %v", len(by), by)
	}
	// #3: sub-issue of #1, blocked by the open #2, so locked.
	q3 := by[3]
	if q3.Parent != 1 || len(q3.BlockedBy) != 1 || q3.BlockedBy[0].Number != 2 || q3.BlockedBy[0].Closed ||
		q3.State != quests.StateLocked || q3.Comments != 1 {
		t.Errorf("#3: %+v", q3)
	}
	if q := by[1]; q.SubTotal != 2 || q.SubDone != 0 || q.State != quests.StateOpen {
		t.Errorf("#1: %+v", q)
	}
	for _, q := range listing.Quests {
		if q.Labels == nil || q.Assignees == nil || q.BlockedBy == nil || q.PRs == nil {
			t.Errorf("#%d has nil slices", q.Number)
		}
	}
}

func TestEveryLabelIsQueriedAndMergedWithoutDuplicates(t *testing.T) {
	var calls []gqlReq
	inner := replay(t, &calls)
	c := fakeGitHub(t, nil, inner)
	// Both labels return the same recorded issues: they must be merged, not doubled.
	listing, err := c.List(context.Background(), demoRepo, campaign.Scope{Mode: campaign.ScopeLabels, Labels: []string{"enhancement", "tracked"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("%d requests, want open+closed per label", len(calls))
	}
	if got := calls[0].Variables["labels"].([]any)[0]; got != "enhancement" {
		t.Errorf("first query label %v", got)
	}
	if got := calls[2].Variables["labels"].([]any)[0]; got != "tracked" {
		t.Errorf("second query label %v", got)
	}
	seen := map[int]bool{}
	for _, q := range listing.Quests {
		if seen[q.Number] {
			t.Errorf("#%d returned twice", q.Number)
		}
		seen[q.Number] = true
	}
	if len(listing.Quests) != 3 {
		t.Fatalf("%d quests", len(listing.Quests))
	}
}

func TestAssignedScopeAsksWhoYouAreOnceAndFiltersOnGitHub(t *testing.T) {
	var calls []gqlReq
	c := fakeGitHub(t, nil, replay(t, &calls))
	scope := campaign.Scope{Mode: campaign.ScopeAssigned, Labels: []string{}}
	listing, err := c.List(context.Background(), demoRepo, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Quests) != 1 || listing.Quests[0].Number != 5 || listing.Quests[0].Assignees[0] != "dkage" {
		t.Fatalf("%+v", listing.Quests)
	}
	// viewer, open page, closed page.
	if len(calls) != 3 || !strings.Contains(calls[0].Query, "viewer{login}") {
		t.Fatalf("%d requests: the first must ask who the user is", len(calls))
	}
	filter, _ := calls[1].Variables["filter"].(map[string]any)
	if filter["assignee"] != "dkage" {
		t.Fatalf("assignee filter %v", calls[1].Variables["filter"])
	}
	if _, has := calls[1].Variables["labels"]; has {
		t.Error("an assigned scope sends no label filter")
	}
	// The login is remembered: the second listing does not ask again.
	calls = nil
	if _, err := c.List(context.Background(), demoRepo, scope); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("%d requests on the second listing, want 2 (no viewer query)", len(calls))
	}
}

func TestAssignedScopeNeedsAKnownUser(t *testing.T) {
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		w.Write([]byte(`{"data":{"viewer":{"login":""}}}`))
	})
	_, err := c.List(context.Background(), demoRepo, campaign.Scope{Mode: campaign.ScopeAssigned})
	if !errors.Is(err, quests.ErrUnauthorized) {
		t.Fatalf("%v", err)
	}
}

func TestEverythingScopeSendsNoFilterAndShowsMoreClosedIssues(t *testing.T) {
	var calls []gqlReq
	c := fakeGitHub(t, nil, replay(t, &calls))
	listing, err := c.List(context.Background(), demoRepo, everything)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("%d requests", len(calls))
	}
	for _, q := range calls {
		if _, has := q.Variables["labels"]; has {
			t.Error("labels sent for the everything scope")
		}
		if _, has := q.Variables["filter"]; has {
			t.Error("filter sent for the everything scope")
		}
	}
	if first, _ := calls[1].Variables["first"].(float64); first != 50 {
		t.Errorf("closed page size %v, want 50", first)
	}
	if len(listing.Quests) != 6 { // 5 open plus the closed probe issue
		t.Fatalf("%d quests", len(listing.Quests))
	}
}

func TestUnknownOrEmptyScopesAreRefusedWithoutAskingGitHub(t *testing.T) {
	var hits int
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) { hits++ })
	for name, sc := range map[string]campaign.Scope{
		"empty mode":      {},
		"unknown mode":    {Mode: "everything"},
		"labels, no list": {Mode: campaign.ScopeLabels},
	} {
		if _, err := c.List(context.Background(), demoRepo, sc); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if hits != 0 {
		t.Fatalf("%d requests for refused scopes", hits)
	}
}

func TestListPagesThroughOpenIssuesAndAddsClosed(t *testing.T) {
	var openCalls, closedCalls atomic.Int32
	page := func(num int, next bool, cursor string) []byte {
		b, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{
			"total": map[string]any{"totalCount": 312},
			"issues": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": next, "endCursor": cursor},
				"nodes":    []map[string]any{{"number": num, "title": "t", "state": "OPEN"}},
			}}}})
		return b
	}
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		states, _ := q.Variables["states"].([]any)
		if states[0] == "CLOSED" {
			closedCalls.Add(1)
			b, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{
				"total": map[string]any{"totalCount": 312},
				"issues": map[string]any{
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
	got, err := c.List(context.Background(), demoRepo, everything)
	if err != nil {
		t.Fatal(err)
	}
	qs := got.Quests
	if len(qs) != 3 || qs[0].Number != 1 || qs[1].Number != 2 || qs[2].Number != 99 || !qs[2].Closed || qs[2].Reason != "completed" || qs[2].State != quests.StateCleared {
		t.Fatalf("%+v", qs)
	}
	if got.TotalOpen != 312 || got.Truncated {
		t.Fatalf("total %d truncated %v", got.TotalOpen, got.Truncated)
	}
	if openCalls.Load() != 2 || closedCalls.Load() != 1 {
		t.Fatalf("open pages %d, closed pages %d", openCalls.Load(), closedCalls.Load())
	}
}

func TestListStopsAtThePageCapAndSaysItWasCut(t *testing.T) {
	var openCalls atomic.Int32
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		states, _ := q.Variables["states"].([]any)
		more := true
		if states[0] == "CLOSED" {
			more = false
		} else {
			openCalls.Add(1)
		}
		b, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{
			"total": map[string]any{"totalCount": 900},
			"issues": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": more, "endCursor": "again"}, "nodes": []any{},
			}}}})
		w.Write(b)
	})
	got, err := c.List(context.Background(), demoRepo, everything)
	if err != nil {
		t.Fatal(err)
	}
	if openCalls.Load() != maxOpenPages {
		t.Fatalf("fetched %d open pages, cap is %d", openCalls.Load(), maxOpenPages)
	}
	if !got.Truncated || got.TotalOpen != 900 {
		t.Fatalf("a cut list must say so: %+v", got)
	}
	// Exactly filling the cap with nothing more is not a cut.
	openCalls.Store(0)
	c2 := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		states, _ := q.Variables["states"].([]any)
		n := openCalls.Add(1)
		more := states[0] != "CLOSED" && n < int32(maxOpenPages)
		b, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{
			"total":  map[string]any{"totalCount": 300},
			"issues": map[string]any{"pageInfo": map[string]any{"hasNextPage": more, "endCursor": "x"}, "nodes": []any{}}}}})
		w.Write(b)
	})
	if got, _ := c2.List(context.Background(), demoRepo, everything); got.Truncated {
		t.Fatal("a list that ended exactly at the cap was reported as cut")
	}
}

func TestCountsFromARealResponse(t *testing.T) {
	var calls []gqlReq
	c := fakeGitHub(t, nil, replay(t, &calls))
	got, err := c.Counts(context.Background(), demoRepo)
	if err != nil {
		t.Fatal(err)
	}
	if got.OpenTotal != 5 || got.Assigned != 1 || got.Login != "dkage" {
		t.Fatalf("%+v", got)
	}
	by := map[string]quests.LabelCount{}
	for _, l := range got.Labels {
		by[l.Name] = l
	}
	if by["enhancement"].Open != 3 || by["bug"].Open != 1 || by["question"].Open != 1 || by["enhancement"].Color == "" {
		t.Fatalf("%+v", got.Labels)
	}
	// The marker label does not exist in this repository, but is still offered, with nothing in it.
	if m, ok := by[quests.MarkerLabel]; !ok || m.Open != 0 || got.Labels[0].Name != quests.MarkerLabel {
		t.Fatalf("marker: %+v (labels %+v)", m, got.Labels)
	}
	// It asks who the user is, then for the counts, naming the marker.
	if len(calls) != 2 || calls[1].Variables["login"] != "dkage" || calls[1].Variables["marker"] != quests.MarkerLabel {
		t.Fatalf("%d requests: %+v", len(calls), calls)
	}
}

func TestMarkerLabelIsFoundEvenWhenItIsNotAmongTheFirstHundred(t *testing.T) {
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		if strings.Contains(q.Query, "viewer{login}") {
			w.Write([]byte(`{"data":{"viewer":{"login":"me"}}}`))
			return
		}
		w.Write([]byte(`{"data":{"repository":{
		  "total":{"totalCount":400},"assigned":{"totalCount":0},
		  "marker":{"name":"tracked","color":"123456","issues":{"totalCount":14}},
		  "labels":{"nodes":[{"name":"a","color":"fff","issues":{"totalCount":2}},{"name":"tracked","color":"123456","issues":{"totalCount":14}}]}}}}`))
	})
	got, err := c.Counts(context.Background(), demoRepo)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range got.Labels {
		if l.Name == "tracked" {
			n++
			if l.Open != 14 || l.Color != "123456" {
				t.Fatalf("%+v", l)
			}
		}
	}
	if n != 1 || got.Labels[0].Name != "tracked" {
		t.Fatalf("the marker must appear once, first: %+v", got.Labels)
	}
}

func TestCountsFailures(t *testing.T) {
	c := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) {
		if strings.Contains(q.Query, "viewer{login}") {
			w.Write([]byte(`{"data":{"viewer":{"login":"me"}}}`))
			return
		}
		w.Write([]byte(`{"data":{"repository":null}}`))
	})
	if _, err := c.Counts(context.Background(), demoRepo); !errors.Is(err, quests.ErrNotFound) {
		t.Fatalf("%v", err)
	}
	c2 := fakeGitHub(t, nil, func(w http.ResponseWriter, r *http.Request, q gqlReq) { w.WriteHeader(http.StatusUnauthorized) })
	if _, err := c2.Counts(context.Background(), demoRepo); !errors.Is(err, quests.ErrUnauthorized) {
		t.Fatalf("%v", err)
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
		if _, err := c.List(context.Background(), demoRepo, everything); !errors.Is(err, quests.ErrNotFound) && name != "null issue" {
			t.Errorf("%s (list): %v", name, err)
		}
	}
}

func TestMissingCredentials(t *testing.T) {
	var hits atomic.Int32
	c := fakeGitHub(t, func(context.Context) (string, error) { return "", quests.ErrNoToken }, func(w http.ResponseWriter, r *http.Request, q gqlReq) { hits.Add(1) })
	if _, err := c.List(context.Background(), demoRepo, everything); !errors.Is(err, quests.ErrNoToken) {
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
		if _, listErr := c.List(context.Background(), demoRepo, everything); !errors.As(listErr, &rl) || !rl.Paused {
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
