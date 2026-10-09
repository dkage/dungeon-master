package quests

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dkage/dungeon-master/internal/campaign"
)

var (
	// ErrNoRepo means the campaign has no GitHub repository to read issues from.
	ErrNoRepo = errors.New("quests: this campaign has no GitHub repository")
	// ErrScopeNeeded means the user has not yet chosen which issues to show.
	ErrScopeNeeded = errors.New("quests: choose which issues to show")
)

// Result is a campaign's quests plus where they came from.
type Result struct {
	// Source is "github" or "demo".
	Source string `json:"source"`
	Repo   string `json:"repo,omitempty"`
	// Scope is which issues were asked for; nil for demo data.
	Scope  *campaign.Scope `json:"scope,omitempty"`
	Quests []Quest         `json:"quests"`
	// TotalOpen is how many issues are open in the whole repository.
	TotalOpen int `json:"totalOpen"`
	// Truncated means more issues matched than were fetched: narrow the scope.
	Truncated bool `json:"truncated"`
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
	countsTTL    time.Duration
	minRefresh   time.Duration
	now          func() time.Time

	mu       sync.Mutex
	cache    map[string]entry
	inflight map[string]*call
	counts   map[string]countsEntry
	details  map[string]detailEntry
}

type entry struct {
	listing Listing
	fetched time.Time
}

type countsEntry struct {
	counts  Counts
	fetched time.Time
}

type detailEntry struct {
	detail  Detail
	fetched time.Time
}

const maxCachedDetails = 100

type call struct {
	done    chan struct{}
	listing Listing
	err     error
}

// defaultCountsTTL is how long the numbers behind the scope picker are reused.
const defaultCountsTTL = 5 * time.Minute

// ServiceOptions configures a Service. Zero values pick sensible defaults.
type ServiceOptions struct {
	GitHub, Demo Source
	// TTL is how long a list is served without asking again. Default 60s.
	TTL time.Duration
	// CountsTTL is how long the scope picker's numbers are reused. Default 5 minutes.
	CountsTTL time.Duration
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
	if o.CountsTTL == 0 {
		o.CountsTTL = defaultCountsTTL
	}
	if o.MinRefresh == 0 {
		o.MinRefresh = 5 * time.Second
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{
		github: o.GitHub, demo: o.Demo, ttl: o.TTL, countsTTL: o.CountsTTL, minRefresh: o.MinRefresh, now: o.Now,
		cache: map[string]entry{}, inflight: map[string]*call{}, counts: map[string]countsEntry{}, details: map[string]detailEntry{},
	}
}

// target is where a campaign's quests come from.
type target struct {
	src    Source
	name   string // "github" or "demo"
	key    string // cache key
	repo   Repo
	scope  campaign.Scope
	hasRep bool
}

// pick returns the source, cache key, repo and scope for a campaign.
func (s *Service) pick(c campaign.Campaign) (target, error) {
	if c.Demo {
		return target{src: s.demo, name: "demo", key: "demo"}, nil
	}
	if c.Repo == "" {
		return target{}, ErrNoRepo
	}
	repo, err := ParseRepo(c.Repo)
	if err != nil {
		return target{}, ErrNoRepo
	}
	if c.Scope == nil {
		return target{}, ErrScopeNeeded
	}
	return target{
		src: s.github, name: "github", repo: repo, scope: *c.Scope, hasRep: true,
		key: "github:" + c.Repo + "|" + scopeKey(*c.Scope),
	}, nil
}

// scopeKey identifies a scope for caching: labels are order-insensitive.
func scopeKey(sc campaign.Scope) string {
	labels := make([]string, 0, len(sc.Labels))
	for _, l := range sc.Labels {
		labels = append(labels, strings.ToLower(l))
	}
	slices.Sort(labels)
	return sc.Mode + ":" + strings.Join(labels, ",")
}

// List returns the campaign's quests inside its scope, sorted. refresh asks
// for fresh data (rate-limited by MinRefresh).
func (s *Service) List(ctx context.Context, c campaign.Campaign, refresh bool) (Result, error) {
	t, err := s.pick(c)
	if err != nil {
		return Result{}, err
	}
	now := s.now()
	s.mu.Lock()
	cached, have := s.cache[t.key]
	age := now.Sub(cached.fetched)
	if have && ((!refresh && age < s.ttl) || (refresh && age < s.minRefresh)) {
		s.mu.Unlock()
		return s.result(t, c, cached, false, ""), nil
	}
	s.mu.Unlock()

	l, ferr := s.fetch(ctx, t)
	if ferr != nil {
		if have {
			return s.result(t, c, cached, true, Friendly(ferr)), nil
		}
		return Result{}, ferr
	}
	e := entry{listing: l, fetched: s.now()}
	s.mu.Lock()
	s.cache[t.key] = e
	s.mu.Unlock()
	return s.result(t, c, e, false, ""), nil
}

// fetch runs one source call per key at a time; concurrent callers share it.
func (s *Service) fetch(ctx context.Context, t target) (Listing, error) {
	s.mu.Lock()
	if cl, ok := s.inflight[t.key]; ok {
		s.mu.Unlock()
		select {
		case <-cl.done:
			return cl.listing, cl.err
		case <-ctx.Done():
			return Listing{}, ctx.Err()
		}
	}
	cl := &call{done: make(chan struct{})}
	s.inflight[t.key] = cl
	s.mu.Unlock()

	// The shared fetch must not die with one caller's request, so it gets its
	// own bounded context.
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	cl.listing, cl.err = t.src.List(fctx, t.repo, t.scope)

	s.mu.Lock()
	delete(s.inflight, t.key)
	s.mu.Unlock()
	close(cl.done)
	return cl.listing, cl.err
}

func (s *Service) result(t target, c campaign.Campaign, e entry, stale bool, warning string) Result {
	qs := append([]Quest(nil), e.listing.Quests...)
	Sort(qs)
	r := Result{
		Source: t.name, Quests: qs, TotalOpen: e.listing.TotalOpen, Truncated: e.listing.Truncated,
		FetchedAt: e.fetched, Stale: stale, Warning: warning,
	}
	if t.name == "github" {
		r.Repo = c.Repo
		sc := t.scope
		r.Scope = &sc
	}
	return r
}

// Counts returns what each scope choice would show, for the first-connection
// question. It does not need a scope, and is reused for a few minutes.
func (s *Service) Counts(ctx context.Context, c campaign.Campaign) (Counts, error) {
	src, repo, key := Source(s.demo), Repo{}, "demo"
	if !c.Demo {
		if c.Repo == "" {
			return Counts{}, ErrNoRepo
		}
		r, err := ParseRepo(c.Repo)
		if err != nil {
			return Counts{}, ErrNoRepo
		}
		src, repo, key = s.github, r, "github:"+c.Repo
	}
	now := s.now()
	s.mu.Lock()
	if e, ok := s.counts[key]; ok && now.Sub(e.fetched) < s.countsTTL {
		s.mu.Unlock()
		return e.counts, nil
	}
	s.mu.Unlock()
	counts, err := src.Counts(ctx, repo)
	if err != nil {
		return Counts{}, err
	}
	s.mu.Lock()
	s.counts[key] = countsEntry{counts: counts, fetched: s.now()}
	s.mu.Unlock()
	return counts, nil
}

// Detail returns one quest in full. The short cache lets an explicit Brief
// preview and Start action reuse the request made when the issue was opened.
func (s *Service) Detail(ctx context.Context, c campaign.Campaign, number int) (Detail, error) {
	t, err := s.pick(c)
	if err != nil && !errors.Is(err, ErrScopeNeeded) {
		return Detail{}, err
	}
	if t.src == nil {
		// Opening one issue needs no scope: the repository is enough.
		repo, perr := ParseRepo(c.Repo)
		if perr != nil {
			return Detail{}, ErrNoRepo
		}
		t = target{src: s.github, repo: repo, name: "github", key: "github:" + c.Repo}
	}
	key := t.key + "|detail:" + strconv.Itoa(number)
	now := s.now()
	s.mu.Lock()
	if cached, ok := s.details[key]; ok && now.Sub(cached.fetched) < s.ttl {
		s.mu.Unlock()
		return cached.detail, nil
	}
	s.mu.Unlock()
	d, err := t.src.Get(ctx, t.repo, number)
	if err != nil {
		return Detail{}, err
	}
	s.mu.Lock()
	s.details[key] = detailEntry{detail: d, fetched: s.now()}
	for len(s.details) > maxCachedDetails {
		oldKey := ""
		var oldest time.Time
		for candidate, item := range s.details {
			if oldKey == "" || item.fetched.Before(oldest) {
				oldKey, oldest = candidate, item.fetched
			}
		}
		delete(s.details, oldKey)
	}
	s.mu.Unlock()
	return d, nil
}
