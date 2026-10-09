package quests

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
)

type fakeSource struct {
	name      string
	calls     atomic.Int32
	countsN   atomic.Int32
	getCalls  atomic.Int32
	list      []Quest
	total     int
	truncated bool
	counts    Counts
	err       error
	countsErr error
	gate      chan struct{} // when set, List blocks until it is closed
	gotRepo   Repo
	gotScope  campaign.Scope
	getN      int
}

func (f *fakeSource) List(ctx context.Context, repo Repo, scope campaign.Scope) (Listing, error) {
	f.calls.Add(1)
	f.gotRepo, f.gotScope = repo, scope
	if f.gate != nil {
		<-f.gate
	}
	if f.err != nil {
		return Listing{}, f.err
	}
	return Listing{Quests: append([]Quest(nil), f.list...), TotalOpen: f.total, Truncated: f.truncated}, nil
}

func (f *fakeSource) Get(_ context.Context, repo Repo, n int) (Detail, error) {
	f.getCalls.Add(1)
	f.gotRepo, f.getN = repo, n
	return Detail{Quest: Quest{Number: n, Title: f.name}}, f.err
}

func (f *fakeSource) Counts(_ context.Context, repo Repo) (Counts, error) {
	f.countsN.Add(1)
	f.gotRepo = repo
	return f.counts, f.countsErr
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func setup() (*Service, *fakeSource, *fakeSource, *clock) {
	gh := &fakeSource{name: "gh", total: 312, list: []Quest{
		Finish(Quest{Number: 1, Labels: []Label{{Name: "tracked"}}, Updated: time.Unix(10, 0)}),
		Finish(Quest{Number: 2, Labels: []Label{{Name: "tracked"}, {Name: "bug"}}, Updated: time.Unix(20, 0)}),
		Finish(Quest{Number: 3, Closed: true, Updated: time.Unix(30, 0)}),
	}}
	dm := &fakeSource{name: "demo", total: 11, list: []Quest{Finish(Quest{Number: 100})}}
	clk := &clock{t: time.Unix(1000, 0)}
	return NewService(ServiceOptions{GitHub: gh, Demo: dm, Now: clk.now}), gh, dm, clk
}

var tracked = campaign.Scope{Mode: campaign.ScopeLabels, Labels: []string{"tracked"}}

func gh(scope *campaign.Scope) campaign.Campaign {
	return campaign.Campaign{ID: "c1", Repo: "acme/widgets", Scope: scope}
}

func nums(r Result) []int {
	out := []int{}
	for _, q := range r.Quests {
		out = append(out, q.Number)
	}
	return out
}

func TestSourceSelection(t *testing.T) {
	s, ghs, dm, _ := setup()
	ctx := context.Background()

	r, err := s.List(ctx, gh(&tracked), false)
	if err != nil || r.Source != "github" || r.Repo != "acme/widgets" || ghs.gotRepo != (Repo{Owner: "acme", Name: "widgets"}) {
		t.Fatalf("github: %+v %v (repo %v)", r, err, ghs.gotRepo)
	}
	r, err = s.List(ctx, campaign.Campaign{ID: "c2", Demo: true, Repo: "acme/widgets"}, false)
	if err != nil || r.Source != "demo" || r.Repo != "" || r.Scope != nil || len(r.Quests) != 1 || r.Quests[0].Number != 100 {
		t.Fatalf("demo: %+v %v", r, err)
	}
	if dm.calls.Load() != 1 {
		t.Fatalf("demo calls %d", dm.calls.Load())
	}
	for name, c := range map[string]campaign.Campaign{
		"no repo":  {ID: "c3"},
		"bad repo": {ID: "c4", Repo: "not-a-repo"},
	} {
		if _, err := s.List(ctx, c, false); !errors.Is(err, ErrNoRepo) {
			t.Errorf("%s: %v", name, err)
		}
		if _, err := s.Detail(ctx, c, 1); !errors.Is(err, ErrNoRepo) {
			t.Errorf("%s detail: %v", name, err)
		}
		if _, err := s.Counts(ctx, c); !errors.Is(err, ErrNoRepo) {
			t.Errorf("%s counts: %v", name, err)
		}
	}
}

func TestTheScopeGoesToTheSourceAndIsReported(t *testing.T) {
	s, ghs, _, _ := setup()
	ctx := context.Background()
	r, err := s.List(ctx, gh(&tracked), false)
	if err != nil {
		t.Fatal(err)
	}
	if ghs.gotScope.Mode != campaign.ScopeLabels || len(ghs.gotScope.Labels) != 1 || ghs.gotScope.Labels[0] != "tracked" {
		t.Fatalf("the source was asked for %+v", ghs.gotScope)
	}
	if r.Scope == nil || r.Scope.Mode != campaign.ScopeLabels || r.TotalOpen != 312 || r.Truncated {
		t.Fatalf("result: %+v", r)
	}
	// Truncation is passed on, so the panel can say "narrow the scope".
	ghs.truncated = true
	all := campaign.Scope{Mode: campaign.ScopeAll, Labels: []string{}}
	if r, _ := s.List(ctx, gh(&all), false); !r.Truncated || r.Scope.Mode != campaign.ScopeAll {
		t.Fatalf("%+v", r)
	}
}

func TestAnUnchosenScopeAsksTheQuestion(t *testing.T) {
	s, ghs, _, _ := setup()
	if _, err := s.List(context.Background(), gh(nil), false); !errors.Is(err, ErrScopeNeeded) {
		t.Fatalf("%v", err)
	}
	if ghs.calls.Load() != 0 {
		t.Fatal("GitHub was asked for issues before the user chose a scope")
	}
	// But opening one issue, and the counts for the question itself, need no scope.
	if d, err := s.Detail(context.Background(), gh(nil), 5); err != nil || d.Number != 5 || ghs.getN != 5 {
		t.Fatalf("%+v %v", d, err)
	}
	if _, err := s.Counts(context.Background(), gh(nil)); err != nil {
		t.Fatal(err)
	}
	// Demo data never asks.
	if _, err := s.List(context.Background(), campaign.Campaign{Demo: true}, false); err != nil {
		t.Fatalf("demo: %v", err)
	}
}

func TestDetailIsReusedByPreviewAndLaunchWithinTTL(t *testing.T) {
	s, ghs, _, clk := setup()
	campaign := gh(&tracked)
	for range 3 {
		if _, err := s.Detail(context.Background(), campaign, 5); err != nil {
			t.Fatal(err)
		}
	}
	if got := ghs.getCalls.Load(); got != 1 {
		t.Fatalf("detail source called %d times within cache lifetime", got)
	}
	clk.add(61 * time.Second)
	if _, err := s.Detail(context.Background(), campaign, 5); err != nil {
		t.Fatal(err)
	}
	if got := ghs.getCalls.Load(); got != 2 {
		t.Fatalf("expired detail cache made %d source calls, want 2", got)
	}
}

func TestEachScopeHasItsOwnCacheEntry(t *testing.T) {
	s, ghs, _, _ := setup()
	ctx := context.Background()
	assigned := campaign.Scope{Mode: campaign.ScopeAssigned, Labels: []string{}}
	s.List(ctx, gh(&tracked), false)
	s.List(ctx, gh(&tracked), false)
	if ghs.calls.Load() != 1 {
		t.Fatalf("same scope: %d calls", ghs.calls.Load())
	}
	s.List(ctx, gh(&assigned), false)
	if ghs.calls.Load() != 2 || ghs.gotScope.Mode != campaign.ScopeAssigned {
		t.Fatalf("a different scope must fetch: %d calls, scope %+v", ghs.calls.Load(), ghs.gotScope)
	}
	// Switching back does not refetch.
	s.List(ctx, gh(&tracked), false)
	if ghs.calls.Load() != 2 {
		t.Fatalf("%d calls", ghs.calls.Load())
	}
	// Label order and case do not matter.
	a := campaign.Scope{Mode: campaign.ScopeLabels, Labels: []string{"Bug", "tracked"}}
	b := campaign.Scope{Mode: campaign.ScopeLabels, Labels: []string{"tracked", "bug"}}
	s.List(ctx, gh(&a), false)
	s.List(ctx, gh(&b), false)
	if ghs.calls.Load() != 3 {
		t.Fatalf("equivalent label sets fetched twice: %d calls", ghs.calls.Load())
	}
}

func TestCachingTTLAndRefreshThrottle(t *testing.T) {
	s, ghs, _, clk := setup()
	ctx := context.Background()
	s.List(ctx, gh(&tracked), false)
	s.List(ctx, gh(&tracked), false)
	if ghs.calls.Load() != 1 {
		t.Fatalf("within TTL: %d calls", ghs.calls.Load())
	}
	// An explicit refresh straight away is throttled…
	clk.add(2 * time.Second)
	s.List(ctx, gh(&tracked), true)
	if ghs.calls.Load() != 1 {
		t.Fatalf("refresh inside MinRefresh: %d calls", ghs.calls.Load())
	}
	// …but honoured after the minimum gap, even inside the TTL.
	clk.add(4 * time.Second)
	r, _ := s.List(ctx, gh(&tracked), true)
	if ghs.calls.Load() != 2 || !r.FetchedAt.Equal(clk.now()) {
		t.Fatalf("refresh after MinRefresh: %d calls, fetched %v", ghs.calls.Load(), r.FetchedAt)
	}
	// A plain read after the TTL asks again.
	clk.add(61 * time.Second)
	s.List(ctx, gh(&tracked), false)
	if ghs.calls.Load() != 3 {
		t.Fatalf("after TTL: %d calls", ghs.calls.Load())
	}
}

func TestCampaignsOfOneRepoWithTheSameScopeShareAFetch(t *testing.T) {
	s, ghs, _, _ := setup()
	ctx := context.Background()
	a := gh(&tracked)
	b := campaign.Campaign{ID: "c2", Repo: "acme/widgets", Scope: &tracked}
	s.List(ctx, a, false)
	s.List(ctx, b, false)
	if ghs.calls.Load() != 1 {
		t.Fatalf("%d calls", ghs.calls.Load())
	}
}

func TestResultsAreSortedAndDoNotAliasTheCache(t *testing.T) {
	s, _, _, _ := setup()
	ctx := context.Background()
	r, _ := s.List(ctx, gh(&tracked), false)
	if got := nums(r); got[0] != 2 || got[1] != 1 || got[2] != 3 { // open newest first, cleared last
		t.Fatalf("order %v", got)
	}
	r.Quests[0].Title = "mutated"
	r2, _ := s.List(ctx, gh(&tracked), false)
	if r2.Quests[0].Title == "mutated" {
		t.Fatal("caller could mutate the cache")
	}
}

func TestFailureServesStaleDataOrReportsTheError(t *testing.T) {
	s, ghs, _, clk := setup()
	ctx := context.Background()
	// No cache yet: the error comes through, and is not cached.
	ghs.err = &RateLimitError{}
	if _, err := s.List(ctx, gh(&tracked), false); err == nil {
		t.Fatal("expected an error")
	}
	ghs.err = nil
	if r, err := s.List(ctx, gh(&tracked), false); err != nil || r.Stale || len(r.Quests) != 3 {
		t.Fatalf("recovery: %+v %v", r, err)
	}
	// With a cache: a failing refresh keeps showing the old data, flagged.
	clk.add(2 * time.Minute)
	ghs.err = ErrUnauthorized
	r, err := s.List(ctx, gh(&tracked), false)
	if err != nil || !r.Stale || r.Warning == "" || len(r.Quests) != 3 || r.TotalOpen != 312 {
		t.Fatalf("stale: %+v %v", r, err)
	}
	ghs.err = nil
	if r, _ := s.List(ctx, gh(&tracked), false); r.Stale {
		t.Fatal("stale flag stuck after recovery")
	}
}

func TestConcurrentCallersShareOneFetch(t *testing.T) {
	s, ghs, _, _ := setup()
	ghs.gate = make(chan struct{})
	var wg sync.WaitGroup
	results := make([]Result, 6)
	errs := make([]error, 6)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = s.List(context.Background(), gh(&tracked), false)
		}()
	}
	for ghs.calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // let the others queue up behind it
	close(ghs.gate)
	wg.Wait()
	if ghs.calls.Load() != 1 {
		t.Fatalf("%d fetches for 6 concurrent callers", ghs.calls.Load())
	}
	for i := range results {
		if errs[i] != nil || len(results[i].Quests) != 3 {
			t.Errorf("caller %d: %+v %v", i, results[i], errs[i])
		}
	}
}

func TestAWaitingCallerCanGiveUpWithoutAffectingTheFetch(t *testing.T) {
	s, ghs, _, _ := setup()
	ghs.gate = make(chan struct{})
	first := make(chan Result, 1)
	go func() {
		r, _ := s.List(context.Background(), gh(&tracked), false)
		first <- r
	}()
	for ghs.calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.List(ctx, gh(&tracked), false); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting caller: %v", err)
	}
	close(ghs.gate)
	if r := <-first; len(r.Quests) != 3 {
		t.Fatalf("the shared fetch was disturbed: %+v", r)
	}
}

func TestDetailUsesTheCampaignSource(t *testing.T) {
	s, ghs, dm, _ := setup()
	d, err := s.Detail(context.Background(), gh(&tracked), 7)
	if err != nil || d.Number != 7 || d.Title != "gh" || ghs.getN != 7 || ghs.gotRepo.Name != "widgets" {
		t.Fatalf("%+v %v", d, err)
	}
	d, err = s.Detail(context.Background(), campaign.Campaign{Demo: true}, 3)
	if err != nil || d.Title != "demo" || dm.getN != 3 {
		t.Fatalf("%+v %v", d, err)
	}
	ghs.err = ErrNotFound
	if _, err := s.Detail(context.Background(), gh(&tracked), 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("%v", err)
	}
}

func TestCountsAreCachedAndErrorsAreNot(t *testing.T) {
	s, ghs, dm, clk := setup()
	ctx := context.Background()
	ghs.counts = Counts{OpenTotal: 312, Assigned: 3, Login: "me", Labels: []LabelCount{{Name: "tracked", Open: 14}}}
	dm.counts = Counts{OpenTotal: 11}

	c, err := s.Counts(ctx, gh(nil))
	if err != nil || c.OpenTotal != 312 || c.Assigned != 3 || c.Labels[0].Open != 14 {
		t.Fatalf("%+v %v", c, err)
	}
	s.Counts(ctx, gh(&tracked)) // any scope state shares the numbers
	if ghs.countsN.Load() != 1 {
		t.Fatalf("not cached: %d calls", ghs.countsN.Load())
	}
	clk.add(6 * time.Minute)
	s.Counts(ctx, gh(nil))
	if ghs.countsN.Load() != 2 {
		t.Fatalf("not refreshed after the TTL: %d calls", ghs.countsN.Load())
	}
	// Demo has its own.
	if c, _ := s.Counts(ctx, campaign.Campaign{Demo: true}); c.OpenTotal != 11 {
		t.Fatalf("%+v", c)
	}
	// A failure is returned and not remembered.
	clk.add(6 * time.Minute)
	ghs.countsErr = ErrUnauthorized
	if _, err := s.Counts(ctx, gh(nil)); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("%v", err)
	}
	ghs.countsErr = nil
	if c, err := s.Counts(ctx, gh(nil)); err != nil || c.OpenTotal != 312 {
		t.Fatalf("after recovery: %+v %v", c, err)
	}
}

func TestServiceDefaults(t *testing.T) {
	s := NewService(ServiceOptions{})
	if s.ttl != 60*time.Second || s.countsTTL != 5*time.Minute || s.minRefresh != 5*time.Second || s.now == nil {
		t.Fatalf("%+v", s)
	}
}
