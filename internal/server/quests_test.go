package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
	"github.com/dkage/dungeon-master/internal/quartermaster"
	"github.com/dkage/dungeon-master/internal/quests"
)

// fakeGit makes every directory a repository whose origin is
// github.com/acme/widgets, except directories named "plain".
type fakeGit struct{}

func (fakeGit) Toplevel(_ context.Context, dir string) (string, error) {
	if filepath.Base(dir) == "plain" {
		return "", errors.New("not a git repository")
	}
	return dir, nil
}

func (fakeGit) RemoteURL(context.Context, string, string) (string, error) {
	return "git@github.com:acme/widgets.git", nil
}

// fakeQuests is the GitHub source: settable data and errors.
type fakeQuests struct {
	class     quartermaster.Class
	mu        sync.Mutex
	list      []quests.Quest
	total     int
	truncated bool
	err       error
	detail    quests.Detail
	counts    quests.Counts
	countsErr error
	repo      quests.Repo
	scope     campaign.Scope
	number    int
	getCalls  int
}

func (f *fakeQuests) set(list []quests.Quest, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.list, f.err = list, err
}

func (f *fakeQuests) List(ctx context.Context, repo quests.Repo, scope campaign.Scope) (quests.Listing, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repo, f.scope = repo, scope
	f.class = quartermaster.ClassOf(ctx)
	if f.err != nil {
		return quests.Listing{}, f.err
	}
	total := f.total
	if total == 0 {
		total = len(f.list)
	}
	return quests.Listing{Quests: append([]quests.Quest(nil), f.list...), TotalOpen: total, Truncated: f.truncated}, nil
}

func (f *fakeQuests) Counts(_ context.Context, repo quests.Repo) (quests.Counts, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repo = repo
	return f.counts, f.countsErr
}

func (f *fakeQuests) Get(_ context.Context, repo quests.Repo, n int) (quests.Detail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repo, f.number = repo, n
	f.getCalls++
	d := f.detail
	d.Number = n
	return d, f.err
}

func q(n int, labels ...string) quests.Quest {
	var ls []quests.Label
	for _, l := range labels {
		ls = append(ls, quests.Label{Name: l})
	}
	return quests.Finish(quests.Quest{Number: n, Title: fmt.Sprintf("quest %d", n), Labels: ls, Updated: time.Unix(int64(n), 0)})
}

func (e *env) quests(path string) (*http.Response, []byte) {
	return e.req("GET", "/api/campaigns/"+e.campaign+"/quests"+path, nil, nil)
}

func decodeResult(t *testing.T, out []byte) quests.Result {
	t.Helper()
	var r quests.Result
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return r
}

func TestQuestsFromGitHub(t *testing.T) {
	e := startServer(t, nil)
	e.gh.set([]quests.Quest{q(1), q(2)}, nil)

	resp, out := e.quests("")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
	r := decodeResult(t, out)
	if r.Source != "github" || r.Repo != "acme/widgets" || len(r.Quests) != 2 || r.Quests[0].Number != 2 || r.Stale {
		t.Fatalf("result: %s", out)
	}
	if e.gh.repo != (quests.Repo{Owner: "acme", Name: "widgets"}) {
		t.Fatalf("source asked for %v", e.gh.repo)
	}
	// Empty is an empty list, not null.
	e.gh.set(nil, nil)
	_, out = e.quests("?refresh=1")
	if !strings.Contains(string(out), `"quests":[]`) {
		t.Fatalf("empty list encoded as %s", out)
	}
}

func TestTheCampaignsScopeGoesToGitHubAndIsReported(t *testing.T) {
	e := startServer(t, nil)
	e.gh.set([]quests.Quest{q(1, "tracked"), q(2, "tracked")}, nil)
	e.gh.total = 312

	// The default test campaign shows the tracked label.
	_, out := e.quests("")
	r := decodeResult(t, out)
	if e.gh.scope.Mode != campaign.ScopeLabels || len(e.gh.scope.Labels) != 1 || e.gh.scope.Labels[0] != "tracked" {
		t.Fatalf("the source was asked for %+v", e.gh.scope)
	}
	if r.Scope == nil || r.Scope.Mode != campaign.ScopeLabels || r.TotalOpen != 312 || r.Truncated || len(r.Quests) != 2 {
		t.Fatalf("result: %s", out)
	}

	// Changing the scope changes the query, not a filter on what came back.
	resp, out := e.req("PUT", "/api/campaigns/"+e.campaign+"/scope", map[string]any{"mode": "assigned", "labels": []string{}}, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(out), `"mode":"assigned"`) {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
	e.gh.truncated = true
	_, out = e.quests("")
	r = decodeResult(t, out)
	if e.gh.scope.Mode != campaign.ScopeAssigned || r.Scope.Mode != campaign.ScopeAssigned || !r.Truncated {
		t.Fatalf("assigned: %+v / %s", e.gh.scope, out)
	}
}

func TestChoosingAScopeIsValidated(t *testing.T) {
	e := startServer(t, nil)
	put := func(body any) (*http.Response, []byte) {
		return e.req("PUT", "/api/campaigns/"+e.campaign+"/scope", body, nil)
	}
	for name, body := range map[string]any{
		"unknown mode":    map[string]any{"mode": "whatever"},
		"empty":           map[string]any{},
		"labels but none": map[string]any{"mode": "labels", "labels": []string{}},
		"too many labels": map[string]any{"mode": "labels", "labels": []string{"a", "b", "c", "d", "e", "f"}},
		"comma label":     map[string]any{"mode": "labels", "labels": []string{"a,b"}},
		"labels on all":   map[string]any{"mode": "all", "labels": []string{"x"}},
		"unknown field":   map[string]any{"mode": "all", "extra": 1},
	} {
		if resp, out := put(body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, resp.StatusCode, out)
		}
	}
	// Nothing invalid was stored.
	if c, _ := e.cfg.Campaigns.Get(e.campaign); c.Scope == nil || c.Scope.Mode != campaign.ScopeLabels {
		t.Fatalf("a rejected scope changed the campaign: %+v", c.Scope)
	}
	if resp, _ := e.req("PUT", "/api/campaigns/cnope/scope", map[string]any{"mode": "all"}, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown campaign: %d", resp.StatusCode)
	}
	if resp, _ := e.req("PUT", "/api/campaigns/"+e.campaign+"/scope", map[string]any{"mode": "all"}, func(r *http.Request) { r.Header.Del("Authorization") }); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("without auth: %d", resp.StatusCode)
	}
	if resp, _ := put(map[string]any{"mode": "labels", "labels": []string{" Tracked ", "bug"}}); resp.StatusCode != http.StatusOK {
		t.Errorf("a valid scope was refused: %d", resp.StatusCode)
	}
}

func TestACampaignWithoutAScopeAsksTheQuestion(t *testing.T) {
	e := startServer(t, nil)
	other := t.TempDir()
	c, err := e.cfg.Campaigns.Add(context.Background(), campaign.Input{Dir: other})
	if err != nil {
		t.Fatal(err)
	}
	e.gh.set([]quests.Quest{q(1)}, nil)
	before := e.gh.repo
	resp, out := e.req("GET", "/api/campaigns/"+c.ID+"/quests", nil, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), `"code":"scope_needed"`) {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
	if e.gh.repo != before {
		t.Fatal("GitHub was asked for issues before a scope was chosen")
	}
	// Opening a single issue needs no scope.
	if resp, out := e.req("GET", "/api/campaigns/"+c.ID+"/quests/7", nil, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("detail without scope: %d %s", resp.StatusCode, out)
	}
	// Choosing one unlocks the list.
	e.req("PUT", "/api/campaigns/"+c.ID+"/scope", map[string]any{"mode": "all"}, nil)
	if resp, out := e.req("GET", "/api/campaigns/"+c.ID+"/quests", nil, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
}

func TestScopeOptionsReportTheCounts(t *testing.T) {
	e := startServer(t, nil)
	e.gh.counts = quests.Counts{OpenTotal: 312, Assigned: 3, Login: "me", Labels: []quests.LabelCount{
		{Name: "tracked", Open: 14, Color: "123456"}, {Name: "bug", Open: 40},
	}}
	resp, out := e.req("GET", "/api/campaigns/"+e.campaign+"/scope/options", nil, nil)
	var got quests.Counts
	if err := json.Unmarshal(out, &got); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("%v %d %s", err, resp.StatusCode, out)
	}
	if got.OpenTotal != 312 || got.Assigned != 3 || got.Login != "me" || len(got.Labels) != 2 || got.Labels[0].Open != 14 {
		t.Fatalf("%s", out)
	}
	// Empty labels encode as [], never null.
	e.gh.counts = quests.Counts{}
	if _, out := e.req("GET", "/api/campaigns/"+e.campaign+"/scope/options", nil, nil); !strings.Contains(string(out), `"labels":[]`) {
		t.Fatalf("%s", out)
	}
	// Errors use the same actionable mapping as the quest list.
	e.gh.countsErr = quests.ErrNoToken
	if resp, out := e.req("GET", "/api/campaigns/"+e.campaign+"/scope/options", nil, nil); resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(out), "no_token") {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
	if resp, _ := e.req("GET", "/api/campaigns/cnope/scope/options", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown campaign: %d", resp.StatusCode)
	}
	if resp, _ := e.req("GET", "/api/campaigns/"+e.campaign+"/scope/options", nil, func(r *http.Request) { r.Header.Del("Authorization") }); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without auth: %d", resp.StatusCode)
	}
}

func TestScopeOptionsForCampaignsWithoutARepoOrOnDemoData(t *testing.T) {
	e := startServer(t, nil)
	plain := filepath.Join(t.TempDir(), "plain")
	os.Mkdir(plain, 0o755)
	c, _ := e.cfg.Campaigns.Add(context.Background(), campaign.Input{Dir: plain})
	if resp, out := e.req("GET", "/api/campaigns/"+c.ID+"/scope/options", nil, nil); resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), "no_repo") {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
	e.req("PATCH", "/api/campaigns/"+c.ID, map[string]any{"demo": true}, nil)
	resp, out := e.req("GET", "/api/campaigns/"+c.ID+"/scope/options", nil, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(out), `"openTotal":11`) {
		t.Fatalf("demo: %d %s", resp.StatusCode, out)
	}
}

func TestDemoQuestsToggle(t *testing.T) {
	e := startServer(t, nil)
	e.gh.set([]quests.Quest{q(1)}, nil)

	resp, out := e.req("PATCH", "/api/campaigns/"+e.campaign, map[string]any{"demo": true}, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(out), `"demo":true`) {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
	_, out = e.quests("")
	r := decodeResult(t, out)
	if r.Source != "demo" || r.Repo != "" || len(r.Quests) < 10 {
		t.Fatalf("demo result: %s", out)
	}
	// Detail also comes from the demo data.
	resp, out = e.req("GET", "/api/campaigns/"+e.campaign+"/quests/3", nil, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(out), "Build the signup form") {
		t.Fatalf("demo detail: %d %s", resp.StatusCode, out)
	}
	// Back to real issues.
	e.req("PATCH", "/api/campaigns/"+e.campaign, map[string]any{"demo": false}, nil)
	_, out = e.quests("")
	if r := decodeResult(t, out); r.Source != "github" || len(r.Quests) != 1 {
		t.Fatalf("back to github: %s", out)
	}
}

func TestDemoWorksWithoutAnyRepositoryOrCredentials(t *testing.T) {
	e := startServer(t, nil)
	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	c, err := e.cfg.Campaigns.Add(context.Background(), campaign.Input{Dir: plain})
	if err != nil {
		t.Fatal(err)
	}
	get := func() (*http.Response, []byte) {
		return e.req("GET", "/api/campaigns/"+c.ID+"/quests", nil, nil)
	}
	resp, out := get()
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(out), `"code":"no_repo"`) {
		t.Fatalf("no repo: %d %s", resp.StatusCode, out)
	}
	e.req("PATCH", "/api/campaigns/"+c.ID, map[string]any{"demo": true}, nil)
	if resp, out := get(); resp.StatusCode != http.StatusOK || decodeResult(t, out).Source != "demo" {
		t.Fatalf("demo: %d %s", resp.StatusCode, out)
	}
}

func TestQuestErrorsMapToActionableResponses(t *testing.T) {
	e := startServer(t, nil)
	cases := []struct {
		name   string
		err    error
		status int
		code   string
		hint   string
	}{
		{"no token", quests.ErrNoToken, http.StatusServiceUnavailable, "no_token", "gh auth login"},
		{"rejected", quests.ErrUnauthorized, http.StatusBadGateway, "github_unauthorized", "gh auth login"},
		{"rate limit", &quests.RateLimitError{Reset: time.Now().Add(time.Hour)}, http.StatusTooManyRequests, "rate_limited", "rate limit"},
		{"not found", quests.ErrNotFound, http.StatusNotFound, "repo_not_found", "acme/widgets"},
		{"timeout", context.DeadlineExceeded, http.StatusGatewayTimeout, "timeout", "too long"},
		{"other", errors.New("kaboom"), http.StatusBadGateway, "upstream", "kaboom"},
	}
	for _, c := range cases {
		e.gh.set(nil, c.err)
		resp, out := e.quests("?refresh=1")
		var body map[string]string
		_ = json.Unmarshal(out, &body)
		if resp.StatusCode != c.status || body["code"] != c.code || !strings.Contains(body["error"], c.hint) {
			t.Errorf("%s: %d %s", c.name, resp.StatusCode, out)
		}
	}
	// The same mapping applies to opening a single issue.
	e.gh.set(nil, quests.ErrNoToken)
	if resp, out := e.req("GET", "/api/campaigns/"+e.campaign+"/quests/1", nil, nil); resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(out), "no_token") {
		t.Errorf("detail error: %d %s", resp.StatusCode, out)
	}
}

func TestQuestsServeStaleDataWhenGitHubFails(t *testing.T) {
	e := startServer(t, nil)
	e.gh.set([]quests.Quest{q(1)}, nil)
	e.quests("")
	e.gh.set(nil, &quests.RateLimitError{})
	resp, out := e.quests("?refresh=1")
	r := decodeResult(t, out)
	if resp.StatusCode != http.StatusOK || !r.Stale || r.Warning == "" || len(r.Quests) != 1 {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
}

func TestQuestDetail(t *testing.T) {
	e := startServer(t, nil)
	e.gh.detail = quests.Detail{
		Quest: quests.Finish(quests.Quest{Title: "Fix it"}), Body: "## Steps\n1. go", Author: "alice",
		CommentList: []quests.Comment{{Author: "bob", Body: "me too"}},
	}
	resp, out := e.req("GET", "/api/campaigns/"+e.campaign+"/quests/42", nil, nil)
	var d struct {
		quests.Detail
		Briefing struct {
			Content       string `json:"content"`
			CommentsShown int    `json:"commentsShown"`
		} `json:"briefing"`
	}
	_ = json.Unmarshal(out, &d)
	if resp.StatusCode != http.StatusOK || d.Number != 42 || d.Body == "" || len(d.CommentList) != 1 || e.gh.number != 42 || !strings.Contains(d.Briefing.Content, "Fix it") || d.Briefing.CommentsShown != 1 {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
	// Not found.
	e.gh.err = quests.ErrNotFound
	if resp, out := e.req("GET", "/api/campaigns/"+e.campaign+"/quests/43", nil, nil); resp.StatusCode != http.StatusNotFound || !strings.Contains(string(out), `"code":"not_found"`) {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
}

func TestQuestEndpointValidation(t *testing.T) {
	e := startServer(t, nil)
	for _, n := range []string{"0", "-3", "abc", "1.5", "99999999999999999999"} {
		if resp, _ := e.req("GET", "/api/campaigns/"+e.campaign+"/quests/"+n, nil, nil); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("number %q: %d", n, resp.StatusCode)
		}
	}
	for _, path := range []string{"/api/campaigns/cnope/quests", "/api/campaigns/cnope/quests/1"} {
		if resp, _ := e.req("GET", path, nil, nil); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d", path, resp.StatusCode)
		}
	}
	noAuth := func(r *http.Request) { r.Header.Del("Authorization") }
	for _, path := range []string{"/api/campaigns/" + e.campaign + "/quests", "/api/campaigns/" + e.campaign + "/quests/1"} {
		if resp, _ := e.req("GET", path, nil, noAuth); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s without auth: %d", path, resp.StatusCode)
		}
	}
}

func TestServerWithoutQuestSourceOnlyOffersTheDemo(t *testing.T) {
	// The default service (no GitHub source configured) reports missing
	// credentials rather than crashing.
	e := startServer(t, func(c *Config) { c.Quests = nil })
	resp, out := e.quests("")
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(out), "no_token") {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
	e.req("PATCH", "/api/campaigns/"+e.campaign, map[string]any{"demo": true}, nil)
	if resp, out := e.quests(""); resp.StatusCode != http.StatusOK || decodeResult(t, out).Source != "demo" {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
}

func TestPollingIsMarkedAsBackgroundWork(t *testing.T) {
	e := startServer(t, nil)
	e.gh.set([]quests.Quest{q(1)}, nil)
	e.quests("")
	if e.gh.class != quartermaster.Interactive {
		t.Fatalf("a person's request was classed %v", e.gh.class)
	}
	e.quests("?background=1&refresh=1")
	if e.gh.class != quartermaster.Background {
		t.Fatalf("polling was classed %v", e.gh.class)
	}
}

func TestCampIsReportedWithItsOwnCode(t *testing.T) {
	e := startServer(t, nil)
	until := time.Now().Add(20 * time.Minute)
	e.gh.set(nil, &quests.RateLimitError{Reset: until, Paused: true})
	resp, out := e.quests("?refresh=1")
	var body map[string]string
	_ = json.Unmarshal(out, &body)
	if resp.StatusCode != http.StatusTooManyRequests || body["code"] != "camp" ||
		!strings.Contains(body["error"], until.Local().Format("15:04")) {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}
	// An ordinary rate limit keeps its own code.
	e.gh.set(nil, &quests.RateLimitError{Reset: until})
	if _, out := e.quests("?refresh=1"); !strings.Contains(string(out), `"code":"rate_limited"`) {
		t.Fatalf("%s", out)
	}
	// With data already loaded, camp serves the last copy instead.
	e.gh.set([]quests.Quest{q(1)}, nil)
	e.quests("?refresh=1")
	e.gh.set(nil, &quests.RateLimitError{Reset: until, Paused: true})
	_, out = e.quests("?refresh=1")
	if r := decodeResult(t, out); !r.Stale || len(r.Quests) != 1 {
		t.Fatalf("expected stale data during camp: %s", out)
	}
}

func TestGitHubStatusEndpoint(t *testing.T) {
	// Without a Quartermaster the endpoint still answers, with nothing known.
	e := startServer(t, nil)
	resp, out := e.req("GET", "/api/github", nil, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(out), `"rations":[]`) || !strings.Contains(string(out), `"active":false`) {
		t.Fatalf("%d %s", resp.StatusCode, out)
	}

	qm, err := quartermaster.New(quartermaster.Options{})
	if err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(15 * time.Minute)
	qm.Trip(until, "GitHub sent Retry-After")
	e2 := startServer(t, func(c *Config) { c.Quartermaster = qm })
	resp, out = e2.req("GET", "/api/github", nil, nil)
	var st quartermaster.Status
	if err := json.Unmarshal(out, &st); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("%v %s", err, out)
	}
	if !st.Camp.Active || st.Camp.Reason != "GitHub sent Retry-After" || st.Camp.Until.Before(time.Now()) {
		t.Fatalf("%s", out)
	}
	// It needs the access token like every other API call.
	if resp, _ := e2.req("GET", "/api/github", nil, func(r *http.Request) { r.Header.Del("Authorization") }); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", resp.StatusCode)
	}
}
