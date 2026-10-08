package quests

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
)

// ErrNoRepo means the campaign has no GitHub repository to read issues from.
var ErrNoRepo = errors.New("quests: this campaign has no GitHub repository")

// Result is a campaign's quests plus where they came from.
type Result struct {
	// Source is "github" or "demo".
	Source string  `json:"source"`
	Repo   string  `json:"repo,omitempty"`
	Quests []Quest `json:"quests"`
	// FetchedAt is when the data was read from the source.
	FetchedAt time.Time `json:"fetchedAt"`
	// Stale is set when a refresh failed and older data is shown instead;
	// Warning says why.
	Stale   bool   `json:"stale"`
	Warning string `json:"warning,omitempty"`
}

// Service chooses the source for a campaign, caches its answers and shares
// one fetch between concurrent callers.
type Service struct {
	github, demo Source
	ttl          time.Duration
	minRefresh   time.Duration
	now          func() time.Time

	mu       sync.Mutex
	cache    map[string]entry
	inflight map[string]*call
}

type entry struct {
	quests  []Quest
	fetched time.Time
}

type call struct {
	done   chan struct{}
	quests []Quest
	err    error
}

// ServiceOptions configures a Service. Zero values pick sensible defaults.
type ServiceOptions struct {
	GitHub, Demo Source
	// TTL is how long a list is served without asking again. Default 60s.
	TTL time.Duration
	// MinRefresh is the shortest gap honoured for an explicit refresh, so a
	// button cannot hammer GitHub. Default 5s.
	MinRefresh time.Duration
	Now        func() time.Time
}

// NewService builds a Service.
func NewService(o ServiceOptions) *Service {
	if o.TTL == 0 {
		o.TTL = 60 * time.Second
	}
	if o.MinRefresh == 0 {
		o.MinRefresh = 5 * time.Second
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{
		github: o.GitHub, demo: o.Demo, ttl: o.TTL, minRefresh: o.MinRefresh, now: o.Now,
		cache: map[string]entry{}, inflight: map[string]*call{},
	}
}

// pick returns the source, cache key and repo for a campaign.
func (s *Service) pick(c campaign.Campaign) (src Source, name, key string, repo Repo, err error) {
	if c.Demo {
		return s.demo, "demo", "demo", Repo{}, nil
	}
	if c.Repo == "" {
		return nil, "", "", Repo{}, ErrNoRepo
	}
	repo, err = ParseRepo(c.Repo)
	if err != nil {
		return nil, "", "", Repo{}, ErrNoRepo
	}
	return s.github, "github", "github:" + c.Repo, repo, nil
}

// List returns the campaign's quests, filtered by its labels and sorted.
// refresh asks for fresh data (rate-limited by MinRefresh).
func (s *Service) List(ctx context.Context, c campaign.Campaign, refresh bool) (Result, error) {
	src, name, key, repo, err := s.pick(c)
	if err != nil {
		return Result{}, err
	}
	now := s.now()
	s.mu.Lock()
	cached, have := s.cache[key]
	age := now.Sub(cached.fetched)
	if have && ((!refresh && age < s.ttl) || (refresh && age < s.minRefresh)) {
		s.mu.Unlock()
		return s.result(name, c, cached, false, ""), nil
	}
	s.mu.Unlock()

	qs, ferr := s.fetch(ctx, key, src, repo)
	if ferr != nil {
		if have {
			return s.result(name, c, cached, true, ferr.Error()), nil
		}
		return Result{}, ferr
	}
	e := entry{quests: qs, fetched: s.now()}
	s.mu.Lock()
	s.cache[key] = e
	s.mu.Unlock()
	return s.result(name, c, e, false, ""), nil
}

// fetch runs one source call per key at a time; concurrent callers share it.
func (s *Service) fetch(ctx context.Context, key string, src Source, repo Repo) ([]Quest, error) {
	s.mu.Lock()
	if cl, ok := s.inflight[key]; ok {
		s.mu.Unlock()
		select {
		case <-cl.done:
			return cl.quests, cl.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	cl := &call{done: make(chan struct{})}
	s.inflight[key] = cl
	s.mu.Unlock()

	// The shared fetch must not die with one caller's request, so it gets its
	// own bounded context.
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	cl.quests, cl.err = src.List(fctx, repo)

	s.mu.Lock()
	delete(s.inflight, key)
	s.mu.Unlock()
	close(cl.done)
	return cl.quests, cl.err
}

func (s *Service) result(source string, c campaign.Campaign, e entry, stale bool, warning string) Result {
	qs := FilterLabels(append([]Quest(nil), e.quests...), c.Labels)
	Sort(qs)
	repo := c.Repo
	if source == "demo" {
		repo = ""
	}
	return Result{Source: source, Repo: repo, Quests: qs, FetchedAt: e.fetched, Stale: stale, Warning: warning}
}

// Detail returns one quest in full. It is not cached: opening an issue is a
// deliberate act and costs a single request.
func (s *Service) Detail(ctx context.Context, c campaign.Campaign, number int) (Detail, error) {
	src, _, _, repo, err := s.pick(c)
	if err != nil {
		return Detail{}, err
	}
	return src.Get(ctx, repo, number)
}
