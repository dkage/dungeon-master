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
	class  quartermaster.Class
	mu     sync.Mutex
	list   []quests.Quest
	err    error
	detail quests.Detail
	repo   quests.Repo
	number int
}

func (f *fakeQuests) set(list []quests.Quest, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.list, f.err = list, err
}

func (f *fakeQuests) List(ctx context.Context, repo quests.Repo) ([]quests.Quest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repo = repo
	f.class = quartermaster.ClassOf(ctx)
	return append([]quests.Quest(nil), f.list...), f.err
}

func (f *fakeQuests) Get(_ context.Context, repo quests.Repo, n int) (quests.Detail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repo, f.number = repo, n
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

func TestQuestsAreFilteredByCampaignLabels(t *testing.T) {
	e := startServer(t, nil)
	e.gh.set([]quests.Quest{q(1, "area:api"), q(2, "area:web"), q(3)}, nil)
	e.req("PATCH", "/api/campaigns/"+e.campaign, map[string]any{"labels": []string{"area:api"}}, nil)
	_, out := e.quests("")
	if r := decodeResult(t, out); len(r.Quests) != 1 || r.Quests[0].Number != 1 {
		t.Fatalf("filtered: %s", out)
	}
	e.req("PATCH", "/api/campaigns/"+e.campaign, map[string]any{"labels": []string{}}, nil)
	_, out = e.quests("")
	if r := decodeResult(t, out); len(r.Quests) != 3 {
		t.Fatalf("unfiltered: %s", out)
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
	var d quests.Detail
	_ = json.Unmarshal(out, &d)
	if resp.StatusCode != http.StatusOK || d.Number != 42 || d.Body == "" || len(d.CommentList) != 1 || e.gh.number != 42 {
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
