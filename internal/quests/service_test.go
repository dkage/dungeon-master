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
	name    string
	calls   atomic.Int32
	list    []Quest
	err     error
	gate    chan struct{} // when set, List blocks until it is closed
	gotRepo Repo
	getN    int
}

func (f *fakeSource) List(ctx context.Context, repo Repo) ([]Quest, error) {
	f.calls.Add(1)
	f.gotRepo = repo
	if f.gate != nil {
		<-f.gate
	}
	if f.err != nil {
		return nil, f.err
	}
	return append([]Quest(nil), f.list...), nil
}

func (f *fakeSource) Get(_ context.Context, repo Repo, n int) (Detail, error) {
	f.gotRepo, f.getN = repo, n
	return Detail{Quest: Quest{Number: n, Title: f.name}}, f.err
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func setup() (*Service, *fakeSource, *fakeSource, *clock) {
	gh := &fakeSource{name: "gh", list: []Quest{
		Finish(Quest{Number: 1, Labels: []Label{{Name: "area:api"}}, Updated: time.Unix(10, 0)}),
		Finish(Quest{Number: 2, Labels: []Label{{Name: "area:web"}}, Updated: time.Unix(20, 0)}),
		Finish(Quest{Number: 3, Closed: true, Updated: time.Unix(30, 0)}),
	}}
	dm := &fakeSource{name: "demo", list: []Quest{Finish(Quest{Number: 100})}}
	clk := &clock{t: time.Unix(1000, 0)}
	return NewService(ServiceOptions{GitHub: gh, Demo: dm, Now: clk.now}), gh, dm, clk
}

var ghCampaign = campaign.Campaign{ID: "c1", Repo: "acme/widgets", Labels: []string{}}

func nums(r Result) []int {
	out := []int{}
	for _, q := range r.Quests {
		out = append(out, q.Number)
	}
	return out
}

func TestSourceSelection(t *testing.T) {
	s, gh, dm, _ := setup()
	ctx := context.Background()

	r, err := s.List(ctx, ghCampaign, false)
	if err != nil || r.Source != "github" || r.Repo != "acme/widgets" || gh.gotRepo != (Repo{Owner: "acme", Name: "widgets"}) {
		t.Fatalf("github: %+v %v (repo %v)", r, err, gh.gotRepo)
	}
	r, err = s.List(ctx, campaign.Campaign{ID: "c2", Demo: true, Repo: "acme/widgets"}, false)
	if err != nil || r.Source != "demo" || r.Repo != "" || len(r.Quests) != 1 || r.Quests[0].Number != 100 {
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
	}
}

func TestCachingTTLAndRefreshThrottle(t *testing.T) {
	s, gh, _, clk := setup()
	ctx := context.Background()
	s.List(ctx, ghCampaign, false)
	s.List(ctx, ghCampaign, false)
	if gh.calls.Load() != 1 {
		t.Fatalf("within TTL: %d calls", gh.calls.Load())
	}
	// An explicit refresh straight away is throttled…
	clk.add(2 * time.Second)
	s.List(ctx, ghCampaign, true)
	if gh.calls.Load() != 1 {
		t.Fatalf("refresh inside MinRefresh: %d calls", gh.calls.Load())
	}
	// …but honoured after the minimum gap, even inside the TTL.
	clk.add(4 * time.Second)
	r, _ := s.List(ctx, ghCampaign, true)
	if gh.calls.Load() != 2 || !r.FetchedAt.Equal(clk.now()) {
		t.Fatalf("refresh after MinRefresh: %d calls, fetched %v", gh.calls.Load(), r.FetchedAt)
	}
	// A plain read after the TTL asks again.
	clk.add(61 * time.Second)
	s.List(ctx, ghCampaign, false)
	if gh.calls.Load() != 3 {
		t.Fatalf("after TTL: %d calls", gh.calls.Load())
	}
}

func TestLabelFilterIsPerCampaignButSharesTheFetch(t *testing.T) {
	s, gh, _, _ := setup()
	ctx := context.Background()
	api := campaign.Campaign{ID: "a", Repo: "acme/widgets", Labels: []string{"area:api"}}
	web := campaign.Campaign{ID: "b", Repo: "acme/widgets", Labels: []string{"area:web"}}
	ra, _ := s.List(ctx, api, false)
	rw, _ := s.List(ctx, web, false)
	rall, _ := s.List(ctx, ghCampaign, false)
	if got := nums(ra); len(got) != 1 || got[0] != 1 {
		t.Errorf("api: %v", got)
	}
	if got := nums(rw); len(got) != 1 || got[0] != 2 {
		t.Errorf("web: %v", got)
	}
	if got := nums(rall); len(got) != 3 {
		t.Errorf("unfiltered: %v", got)
	}
	if gh.calls.Load() != 1 {
		t.Fatalf("campaigns of one repo should share a fetch: %d calls", gh.calls.Load())
	}
}

func TestResultsAreSortedAndDoNotAliasTheCache(t *testing.T) {
	s, _, _, _ := setup()
	ctx := context.Background()
	r, _ := s.List(ctx, ghCampaign, false)
	if got := nums(r); got[0] != 2 || got[1] != 1 || got[2] != 3 { // open newest first, cleared last
		t.Fatalf("order %v", got)
	}
	r.Quests[0].Title = "mutated"
	r2, _ := s.List(ctx, ghCampaign, false)
	if r2.Quests[0].Title == "mutated" {
		t.Fatal("caller could mutate the cache")
	}
}

func TestFailureServesStaleDataOrReportsTheError(t *testing.T) {
	s, gh, _, clk := setup()
	ctx := context.Background()
	// No cache yet: the error comes through, and is not cached.
	gh.err = &RateLimitError{}
	if _, err := s.List(ctx, ghCampaign, false); err == nil {
		t.Fatal("expected an error")
	}
	gh.err = nil
	if r, err := s.List(ctx, ghCampaign, false); err != nil || r.Stale || len(r.Quests) != 3 {
		t.Fatalf("recovery: %+v %v", r, err)
	}
	// With a cache: a failing refresh keeps showing the old data, flagged.
	clk.add(2 * time.Minute)
	gh.err = ErrUnauthorized
	r, err := s.List(ctx, ghCampaign, false)
	if err != nil || !r.Stale || r.Warning == "" || len(r.Quests) != 3 {
		t.Fatalf("stale: %+v %v", r, err)
	}
	gh.err = nil
	if r, _ := s.List(ctx, ghCampaign, false); r.Stale {
		t.Fatal("stale flag stuck after recovery")
	}
}

func TestConcurrentCallersShareOneFetch(t *testing.T) {
	s, gh, _, _ := setup()
	gh.gate = make(chan struct{})
	var wg sync.WaitGroup
	results := make([]Result, 6)
	errs := make([]error, 6)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = s.List(context.Background(), ghCampaign, false)
		}()
	}
	for gh.calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // let the others queue up behind it
	close(gh.gate)
	wg.Wait()
	if gh.calls.Load() != 1 {
		t.Fatalf("%d fetches for 6 concurrent callers", gh.calls.Load())
	}
	for i := range results {
		if errs[i] != nil || len(results[i].Quests) != 3 {
			t.Errorf("caller %d: %+v %v", i, results[i], errs[i])
		}
	}
}

func TestAWaitingCallerCanGiveUpWithoutAffectingTheFetch(t *testing.T) {
	s, gh, _, _ := setup()
	gh.gate = make(chan struct{})
	first := make(chan Result, 1)
	go func() {
		r, _ := s.List(context.Background(), ghCampaign, false)
		first <- r
	}()
	for gh.calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.List(ctx, ghCampaign, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting caller: %v", err)
	}
	close(gh.gate)
	if r := <-first; len(r.Quests) != 3 {
		t.Fatalf("the shared fetch was disturbed: %+v", r)
	}
}

func TestDetailUsesTheCampaignSource(t *testing.T) {
	s, gh, dm, _ := setup()
	d, err := s.Detail(context.Background(), ghCampaign, 7)
	if err != nil || d.Number != 7 || d.Title != "gh" || gh.getN != 7 || gh.gotRepo.Name != "widgets" {
		t.Fatalf("%+v %v", d, err)
	}
	d, err = s.Detail(context.Background(), campaign.Campaign{Demo: true}, 3)
	if err != nil || d.Title != "demo" || dm.getN != 3 {
		t.Fatalf("%+v %v", d, err)
	}
	gh.err = ErrNotFound
	if _, err := s.Detail(context.Background(), ghCampaign, 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("%v", err)
	}
}

func TestServiceDefaults(t *testing.T) {
	s := NewService(ServiceOptions{})
	if s.ttl != 60*time.Second || s.minRefresh != 5*time.Second || s.now == nil {
		t.Fatalf("%+v", s)
	}
}
