// Package quartermaster is the one gatekeeper for the app's GitHub traffic.
// It is an http.RoundTripper, so no caller can bypass it. It sends requests
// one at a time, remembers what GitHub says is left of each budget (the
// x-ratelimit-* headers are authoritative and include every other tool using
// the same token), refuses requests that would eat into a reserve, and goes
// into camp, stopping everything, when GitHub asks it to slow down.
package quartermaster

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Class says how important a request is, which decides how much of the budget
// it may use.
type Class int

const (
	// Interactive is a request a person is waiting for (the default).
	Interactive Class = iota
	// Background is polling nobody is waiting for; it leaves the most in reserve.
	Background
)

type classKey struct{}

// WithClass marks the requests made with ctx.
func WithClass(ctx context.Context, c Class) context.Context {
	return context.WithValue(ctx, classKey{}, c)
}

// ClassOf returns the class of ctx, Interactive when unset.
func ClassOf(ctx context.Context) Class {
	if c, ok := ctx.Value(classKey{}).(Class); ok {
		return c
	}
	return Interactive
}

// Clock is the time source; tests replace it.
type Clock interface{ Now() time.Time }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Options configures a Quartermaster. Zero values pick the defaults.
type Options struct {
	Clock Clock
	Store Store
	// Floors is the fraction of a budget kept in reserve per class: a request
	// is refused when what is left falls below it. Defaults: Background 0.20,
	// Interactive 0.05.
	Floors map[Class]float64
	// MinCamp is the shortest pause after a limit signal. Default 1 minute.
	MinCamp time.Duration
	// MaxCamp caps the exponential growth of repeated pauses. Default 1 hour.
	MaxCamp time.Duration
	// Recovery is how long without a limit signal resets the growth. Default
	// 30 minutes.
	Recovery time.Duration
}

// Quartermaster implements the gatekeeping.
type Quartermaster struct {
	clock    Clock
	store    Store
	floors   map[Class]float64
	minCamp  time.Duration
	maxCamp  time.Duration
	recovery time.Duration

	slot chan struct{} // one request in flight

	mu      sync.Mutex
	rations map[string]Ration
	state   State
}

// New builds a Quartermaster and loads any saved pause.
func New(o Options) (*Quartermaster, error) {
	if o.Clock == nil {
		o.Clock = realClock{}
	}
	if o.Store == nil {
		o.Store = &MemStore{}
	}
	floors := map[Class]float64{Background: 0.20, Interactive: 0.05}
	for c, f := range o.Floors {
		floors[c] = f
	}
	if o.MinCamp <= 0 {
		o.MinCamp = time.Minute
	}
	if o.MaxCamp <= 0 {
		o.MaxCamp = time.Hour
	}
	if o.Recovery <= 0 {
		o.Recovery = 30 * time.Minute
	}
	st, err := o.Store.Load()
	if err != nil {
		return nil, err
	}
	return &Quartermaster{
		clock: o.Clock, store: o.Store, floors: floors,
		minCamp: o.MinCamp, maxCamp: o.MaxCamp, recovery: o.Recovery,
		slot: make(chan struct{}, 1), rations: map[string]Ration{}, state: st,
	}, nil
}

// Transport wraps next so that every request passes the Quartermaster.
func (q *Quartermaster) Transport(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &transport{q: q, next: next}
}

type transport struct {
	q    *Quartermaster
	next http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	q := t.q
	select {
	case q.slot <- struct{}{}:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	release := sync.OnceFunc(func() { <-q.slot })

	if err := q.admit(req); err != nil {
		release()
		return nil, err
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		release()
		return nil, err
	}
	q.observe(req, resp)
	// The slot is held until the body is finished, so requests stay serial
	// even while a caller is still reading the previous response.
	resp.Body = &releasingBody{ReadCloser: resp.Body, release: release}
	return resp, nil
}

type releasingBody struct {
	io.ReadCloser
	release func()
}

func (b *releasingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.release()
	}
	return n, err
}

func (b *releasingBody) Close() error {
	err := b.ReadCloser.Close()
	b.release()
	return err
}

// resourceOf names the budget a request counts against. GitHub's own
// x-ratelimit-resource header overrides this once a response arrives.
func resourceOf(req *http.Request) string {
	if strings.HasSuffix(req.URL.Path, "/graphql") {
		return "graphql"
	}
	return "core"
}

// admit decides, before anything is sent, whether the request may go.
func (q *Quartermaster) admit(req *http.Request) error {
	now := q.clock.Now()
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.state.CampUntil.After(now) {
		return &CampError{Until: q.state.CampUntil, Reason: q.state.Reason}
	}
	r, ok := q.rations[resourceOf(req)]
	if !ok || r.Limit <= 0 || !now.Before(r.ResetsAt) {
		return nil // nothing known, or the window has rolled over
	}
	floor := int(float64(r.Limit) * q.floors[ClassOf(req.Context())])
	if r.Remaining < floor {
		return &RationError{Resource: r.Resource, Remaining: r.Remaining, Floor: floor, Reset: r.ResetsAt}
	}
	return nil
}

// observe records the budget headers and trips camp on a limit signal.
func (q *Quartermaster) observe(req *http.Request, resp *http.Response) {
	now := q.clock.Now()
	h := resp.Header
	resource := h.Get("X-Ratelimit-Resource")
	if resource == "" {
		resource = resourceOf(req)
	}
	limit, hasLimit := atoi(h.Get("X-Ratelimit-Limit"))
	remaining, hasRemaining := atoi(h.Get("X-Ratelimit-Remaining"))
	used, _ := atoi(h.Get("X-Ratelimit-Used"))
	var reset time.Time
	if sec, ok := atoi64(h.Get("X-Ratelimit-Reset")); ok {
		reset = time.Unix(sec, 0)
	}
	if hasLimit && hasRemaining {
		q.mu.Lock()
		q.rations[resource] = Ration{
			Resource: resource, Limit: limit, Remaining: remaining, Used: used, ResetsAt: reset, SeenAt: now,
		}
		q.mu.Unlock()
	}

	if !q.limited(resp, remaining, hasRemaining) {
		return
	}
	// Prefer Retry-After, then the reset time when the budget is spent;
	// otherwise the minimum.
	var wait time.Duration
	reason := "GitHub reported a rate limit"
	if sec, ok := atoi(h.Get("Retry-After")); ok && sec > 0 {
		wait = time.Duration(sec) * time.Second
		reason = "GitHub sent Retry-After"
	} else if hasRemaining && remaining == 0 && reset.After(now) {
		wait = reset.Sub(now)
		reason = "the hourly budget is spent"
	}
	q.trip(now, wait, reason)
}

// limited reports whether a response is a rate-limit signal. A 403 body is
// peeked at (and put back) because secondary limits come without a header.
func (q *Quartermaster) limited(resp *http.Response, remaining int, hasRemaining bool) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		if resp.Header.Get("Retry-After") != "" || (hasRemaining && remaining == 0) {
			return true
		}
		head, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(head), resp.Body), resp.Body}
		s := strings.ToLower(string(head))
		return strings.Contains(s, "rate limit") || strings.Contains(s, "abuse")
	}
	return false
}

// Trip puts the Quartermaster in camp for at least the minimum, or until, if
// that is later. Callers use it when they learn of a limit the transport
// could not see (for example a GraphQL error inside a 200 response).
func (q *Quartermaster) Trip(until time.Time, reason string) {
	now := q.clock.Now()
	wait := until.Sub(now)
	if wait < 0 {
		wait = 0
	}
	q.trip(now, wait, reason)
}

func (q *Quartermaster) trip(now time.Time, wait time.Duration, reason string) {
	q.mu.Lock()
	// Repeated trips grow exponentially; a quiet spell resets the growth. The
	// quiet spell is counted from the end of the last camp, not from the trip:
	// waiting out a long camp is not recovery, and failing again right after
	// it must make the next pause longer.
	quietSince := q.state.LastTrip
	if q.state.CampUntil.After(quietSince) {
		quietSince = q.state.CampUntil
	}
	if q.state.LastTrip.IsZero() || now.Sub(quietSince) > q.recovery {
		q.state.Level = 0
	} else {
		q.state.Level++
	}
	floor := q.minCamp << min(q.state.Level, 20)
	if wait < floor {
		wait = floor
	}
	if wait > q.maxCamp {
		wait = q.maxCamp
	}
	until := now.Add(wait)
	if until.After(q.state.CampUntil) {
		q.state.CampUntil = until
	}
	q.state.Reason = reason
	q.state.LastTrip = now
	snapshot := q.state
	q.mu.Unlock()
	_ = q.store.Save(snapshot) // best effort: the pause still holds in memory
}

// Status is a snapshot for the UI.
func (q *Quartermaster) Status() Status {
	now := q.clock.Now()
	q.mu.Lock()
	defer q.mu.Unlock()
	// graphql and core first (the ones the UI shows), then anything else
	// GitHub reported, by name.
	st := Status{Rations: []Ration{}}
	names := make([]string, 0, len(q.rations))
	for name := range q.rations {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		rank := func(n string) int {
			switch n {
			case "graphql":
				return 0
			case "core":
				return 1
			}
			return 2
		}
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra - rb
		}
		return strings.Compare(a, b)
	})
	for _, name := range names {
		st.Rations = append(st.Rations, q.rations[name])
	}
	if q.state.CampUntil.After(now) {
		st.Camp = Camp{Active: true, Until: q.state.CampUntil, Reason: q.state.Reason, Level: q.state.Level}
	}
	return st
}

// Ration is what GitHub last said about one budget.
type Ration struct {
	Resource  string    `json:"resource"`
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Used      int       `json:"used"`
	ResetsAt  time.Time `json:"resetsAt"`
	SeenAt    time.Time `json:"seenAt"`
}

// Camp is the pause state.
type Camp struct {
	Active bool      `json:"active"`
	Until  time.Time `json:"until,omitempty"`
	Reason string    `json:"reason,omitempty"`
	Level  int       `json:"level,omitempty"`
}

// Status is what the UI shows.
type Status struct {
	Rations []Ration `json:"rations"`
	Camp    Camp     `json:"camp"`
}

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return n, err == nil
}

func atoi64(s string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n, err == nil
}
