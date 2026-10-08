package quartermaster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

// Sleep advances the clock instead of waiting, so an hour of pacing runs
// instantly. It still honours cancellation.
func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.Add(d)
	return nil
}

var t0 = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// github is a fake GitHub whose next response is chosen by the test.
type github struct {
	srv      *httptest.Server
	hits     atomic.Int32
	inflight atomic.Int32
	maxSeen  atomic.Int32
	mu       sync.Mutex
	respond  func(w http.ResponseWriter, r *http.Request)
	delay    time.Duration
}

func newGitHub(t *testing.T) *github {
	g := &github{}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.hits.Add(1)
		n := g.inflight.Add(1)
		for {
			m := g.maxSeen.Load()
			if n <= m || g.maxSeen.CompareAndSwap(m, n) {
				break
			}
		}
		defer g.inflight.Add(-1)
		if g.delay > 0 {
			time.Sleep(g.delay)
		}
		g.mu.Lock()
		f := g.respond
		g.mu.Unlock()
		if f == nil {
			w.Write([]byte("{}"))
			return
		}
		f(w, r)
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *github) set(f func(w http.ResponseWriter, r *http.Request)) {
	g.mu.Lock()
	g.respond = f
	g.mu.Unlock()
}

func budgetHeaders(w http.ResponseWriter, limit, remaining int, reset time.Time, resource string) {
	h := w.Header()
	h.Set("X-Ratelimit-Limit", strconv.Itoa(limit))
	h.Set("X-Ratelimit-Remaining", strconv.Itoa(remaining))
	h.Set("X-Ratelimit-Used", strconv.Itoa(limit-remaining))
	h.Set("X-Ratelimit-Reset", strconv.FormatInt(reset.Unix(), 10))
	h.Set("X-Ratelimit-Resource", resource)
}

func budget(limit, remaining int, reset time.Time, resource string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		budgetHeaders(w, limit, remaining, reset, resource)
		w.Write([]byte("{}"))
	}
}

func newQM(t *testing.T, clk *fakeClock, opts ...func(*Options)) *Quartermaster {
	t.Helper()
	o := Options{Clock: clk}
	for _, f := range opts {
		f(&o)
	}
	q, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func get(t *testing.T, q *Quartermaster, g *github, path string, class Class) (*http.Response, error) {
	t.Helper()
	req, _ := http.NewRequestWithContext(WithClass(context.Background(), class), "GET", g.srv.URL+path, nil)
	resp, err := (&http.Client{Transport: q.Transport(nil)}).Do(req)
	if err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	return resp, err
}

func TestRecordsBudgetFromHeaders(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	reset := t0.Add(40 * time.Minute)
	g.set(budget(5000, 4936, reset, "graphql"))
	q := newQM(t, clk)
	if _, err := get(t, q, g, "/graphql", Interactive); err != nil {
		t.Fatal(err)
	}
	st := q.Status()
	if len(st.Rations) != 1 {
		t.Fatalf("%+v", st)
	}
	r := st.Rations[0]
	if r.Resource != "graphql" || r.Limit != 5000 || r.Remaining != 4936 || r.Used != 64 || !r.ResetsAt.Equal(reset) || !r.SeenAt.Equal(t0) {
		t.Fatalf("%+v", r)
	}
	if st.Camp.Active {
		t.Fatal("camp from a normal response")
	}
}

func TestResourceComesFromGitHubWhenGiven(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	// A path that looks like core, answered as "search": GitHub's header wins.
	g.set(budget(30, 29, t0.Add(time.Minute), "search"))
	get(t, q, g, "/search/issues", Interactive)
	// Without the header the path decides.
	g.set(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Limit", "5000")
		w.Header().Set("X-Ratelimit-Remaining", "4000")
		w.Header().Set("X-Ratelimit-Reset", strconv.FormatInt(t0.Add(time.Hour).Unix(), 10))
	})
	get(t, q, g, "/graphql", Interactive)
	get(t, q, g, "/repos/a/b/issues", Interactive)
	got := map[string]int{}
	var order []string
	for _, r := range q.Status().Rations {
		got[r.Resource] = r.Remaining
		order = append(order, r.Resource)
	}
	if got["search"] != 29 || got["graphql"] != 4000 || got["core"] != 4000 {
		t.Fatalf("%v", got)
	}
	if strings.Join(order, ",") != "graphql,core,search" {
		t.Fatalf("order %v", order)
	}
}

func TestStatusOrdersGraphQLFirstAndIsNeverNil(t *testing.T) {
	q := newQM(t, &fakeClock{t: t0})
	if st := q.Status(); st.Rations == nil || len(st.Rations) != 0 {
		t.Fatalf("%+v", st)
	}
}

func TestRequestsAreSerial(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	g.delay = 20 * time.Millisecond
	q := newQM(t, clk)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := get(t, q, g, "/graphql", Interactive); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if g.hits.Load() != 12 || g.maxSeen.Load() != 1 {
		t.Fatalf("hits %d, max concurrent %d", g.hits.Load(), g.maxSeen.Load())
	}
}

func TestSlotIsHeldUntilTheBodyIsFinished(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	g.set(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", 100))) })
	q := newQM(t, clk)
	c := &http.Client{Transport: q.Transport(nil)}
	resp, err := c.Get(g.srv.URL + "/graphql")
	if err != nil {
		t.Fatal(err)
	}
	// A second request must wait while the first body is unread.
	done := make(chan struct{})
	go func() {
		r2, err := c.Get(g.srv.URL + "/graphql")
		if err == nil {
			r2.Body.Close()
		}
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("second request overtook an unfinished response")
	case <-time.After(150 * time.Millisecond):
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("second request never ran after the first finished")
	}
}

func TestWaitingForTheSlotHonoursCancellation(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	c := &http.Client{Transport: q.Transport(nil)}
	held, err := c.Get(g.srv.URL + "/graphql") // keeps the slot
	if err != nil {
		t.Fatal(err)
	}
	defer held.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", g.srv.URL+"/graphql", nil)
	if _, err := c.Do(req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err %v", err)
	}
}

func TestFloorsByClass(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	reset := t0.Add(30 * time.Minute)
	q := newQM(t, clk)

	// 1,500 of 5,000 left (30%): everything may go.
	g.set(budget(5000, 1500, reset, "graphql"))
	get(t, q, g, "/graphql", Interactive)
	for _, c := range []Class{Background, Interactive} {
		if _, err := get(t, q, g, "/graphql", c); err != nil {
			t.Fatalf("class %d refused at 30%%: %v", c, err)
		}
	}
	// 900 left (18%): below the 20% reserve, so background polling stops; people still get through.
	g.set(budget(5000, 900, reset, "graphql"))
	get(t, q, g, "/graphql", Interactive)
	hits := g.hits.Load()
	_, err := get(t, q, g, "/graphql", Background)
	var re *RationError
	if !errors.As(err, &re) || re.Resource != "graphql" || re.Remaining != 900 || re.Floor != 1000 || !re.Reset.Equal(reset) {
		t.Fatalf("background at 18%%: %v", err)
	}
	if g.hits.Load() != hits {
		t.Fatal("a refused request still reached GitHub")
	}
	if _, err := get(t, q, g, "/graphql", Interactive); err != nil {
		t.Fatalf("interactive at 18%%: %v", err)
	}
	// 200 left (4%): below the 5% reserve, so even interactive requests stop.
	g.set(budget(5000, 200, reset, "graphql"))
	get(t, q, g, "/graphql", Interactive)
	if _, err := get(t, q, g, "/graphql", Interactive); !errors.As(err, &re) {
		t.Fatalf("interactive at 4%%: %v", err)
	}
	// The other bucket is unaffected.
	if _, err := get(t, q, g, "/repos/x/y/issues", Interactive); err != nil {
		t.Fatalf("core request refused because graphql is low: %v", err)
	}
}

func TestFloorLiftsWhenTheWindowResets(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	g.set(budget(5000, 100, t0.Add(10*time.Minute), "graphql"))
	get(t, q, g, "/graphql", Interactive)
	if _, err := get(t, q, g, "/graphql", Interactive); err == nil {
		t.Fatal("expected a refusal below the floor")
	}
	clk.Add(11 * time.Minute) // the hour rolled over: what we knew is stale
	g.set(budget(5000, 4999, clk.Now().Add(time.Hour), "graphql"))
	if _, err := get(t, q, g, "/graphql", Interactive); err != nil {
		t.Fatalf("still refused after the reset time: %v", err)
	}
}

func TestCustomFloors(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk, func(o *Options) { o.Floors = map[Class]float64{Interactive: 0.5} })
	g.set(budget(100, 60, t0.Add(time.Hour), "graphql"))
	get(t, q, g, "/graphql", Interactive)
	if _, err := get(t, q, g, "/graphql", Interactive); err != nil {
		t.Fatalf("60%% left is above a 50%% floor: %v", err)
	}
	g.set(budget(100, 40, t0.Add(time.Hour), "graphql"))
	get(t, q, g, "/graphql", Interactive)
	if _, err := get(t, q, g, "/graphql", Interactive); err == nil {
		t.Fatal("40% left is below a 50% floor")
	}
}

func TestCampOnRetryAfter(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	g.set(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "300")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	resp, err := get(t, q, g, "/graphql", Interactive)
	if err != nil || resp.StatusCode != 429 {
		t.Fatalf("the response must still reach the caller: %v %v", resp, err)
	}
	st := q.Status()
	if !st.Camp.Active || !st.Camp.Until.Equal(t0.Add(5*time.Minute)) {
		t.Fatalf("camp: %+v", st.Camp)
	}
	// Everything stops, without touching the network.
	hits := g.hits.Load()
	for _, class := range []Class{Interactive, Background} {
		_, err := get(t, q, g, "/graphql", class)
		var ce *CampError
		if !errors.As(err, &ce) || !ce.Until.Equal(t0.Add(5*time.Minute)) {
			t.Fatalf("class %d: %v", class, err)
		}
	}
	if _, err := get(t, q, g, "/repos/a/b/issues", Interactive); err == nil {
		t.Fatal("camp must stop the other bucket too")
	}
	if g.hits.Load() != hits {
		t.Fatal("requests reached GitHub during camp")
	}
	// Camp ends on time.
	clk.Add(5*time.Minute + time.Second)
	g.set(budget(5000, 4000, clk.Now().Add(time.Hour), "graphql"))
	if _, err := get(t, q, g, "/graphql", Interactive); err != nil {
		t.Fatalf("still in camp after it ended: %v", err)
	}
	if q.Status().Camp.Active {
		t.Fatal("camp still reported")
	}
}

func TestCampWhenTheBudgetIsSpent(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	reset := t0.Add(17 * time.Minute)
	g.set(func(w http.ResponseWriter, r *http.Request) {
		budgetHeaders(w, 5000, 0, reset, "core")
		w.WriteHeader(http.StatusForbidden)
	})
	get(t, q, g, "/repos/a/b/issues", Interactive)
	st := q.Status()
	if !st.Camp.Active || !st.Camp.Until.Equal(reset) || !strings.Contains(st.Camp.Reason, "spent") {
		t.Fatalf("camp: %+v", st.Camp)
	}
}

func TestCampOnASecondaryLimitMessage(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	body := `{"message":"You have exceeded a secondary rate limit. Please wait a few minutes before you try again."}`
	g.set(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(body))
	})
	req, _ := http.NewRequest("POST", g.srv.URL+"/graphql", nil)
	resp, err := (&http.Client{Transport: q.Transport(nil)}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	// The caller still gets the whole body: peeking must not eat it.
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(got) != body {
		t.Fatalf("body altered: %q", got)
	}
	if st := q.Status(); !st.Camp.Active || !st.Camp.Until.Equal(t0.Add(time.Minute)) {
		t.Fatalf("camp for the minimum when GitHub gives no time: %+v", st.Camp)
	}
}

func TestAnOrdinary403IsNotALimit(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	g.set(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Limit", "5000")
		w.Header().Set("X-Ratelimit-Remaining", "4000")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Resource not accessible by personal access token"}`))
	})
	resp, _ := get(t, q, g, "/graphql", Interactive)
	if resp.StatusCode != 403 || q.Status().Camp.Active {
		t.Fatal("a permission error must not trigger camp")
	}
	// A 403 with a huge body is peeked at only briefly.
	g.set(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(strings.Repeat("a", 50_000) + "rate limit"))
	})
	get(t, q, g, "/graphql", Interactive)
	if q.Status().Camp.Active {
		t.Fatal("only the first few KB are inspected")
	}
}

func TestRepeatedTripsBackOffExponentiallyAndRecover(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	limit := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }
	g.set(limit)

	var waits []time.Duration
	for i := 0; i < 8; i++ {
		get(t, q, g, "/graphql", Interactive)
		until := q.Status().Camp.Until
		waits = append(waits, until.Sub(clk.Now()))
		clk.Add(until.Sub(clk.Now()) + time.Second) // wait it out, then try again
	}
	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute,
		32 * time.Minute, time.Hour, time.Hour}
	for i := range want {
		if waits[i] != want[i] {
			t.Fatalf("waits %v, want %v", waits, want)
		}
	}
	// A long quiet spell resets the growth.
	clk.Add(2 * time.Hour)
	get(t, q, g, "/graphql", Interactive)
	if got := q.Status().Camp.Until.Sub(clk.Now()); got != time.Minute {
		t.Fatalf("after recovery: %v", got)
	}
}

func TestRetryAfterLongerThanTheBackoffWins(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	g.set(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1200")
		w.WriteHeader(http.StatusForbidden)
	})
	get(t, q, g, "/graphql", Interactive)
	if got := q.Status().Camp.Until.Sub(clk.Now()); got != 20*time.Minute {
		t.Fatalf("%v", got)
	}
	// And the cap applies to absurd values.
	clk.Add(21 * time.Minute)
	g.set(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "999999")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	get(t, q, g, "/graphql", Interactive)
	if got := q.Status().Camp.Until.Sub(clk.Now()); got != time.Hour {
		t.Fatalf("not capped: %v", got)
	}
}

func TestExplicitTrip(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := newQM(t, clk)
	q.Trip(t0.Add(10*time.Minute), "GitHub reported a rate limit")
	st := q.Status()
	if !st.Camp.Active || !st.Camp.Until.Equal(t0.Add(10*time.Minute)) || st.Camp.Reason == "" {
		t.Fatalf("%+v", st.Camp)
	}
	// A time in the past still pauses for the minimum.
	q2 := newQM(t, clk)
	q2.Trip(t0.Add(-time.Hour), "x")
	if got := q2.Status().Camp.Until.Sub(t0); got != time.Minute {
		t.Fatalf("%v", got)
	}
	// A shorter trip never shortens a longer pause.
	q.Trip(t0.Add(time.Minute), "again")
	if !q.Status().Camp.Until.After(t0.Add(9 * time.Minute)) {
		t.Fatal("a later trip shortened the pause")
	}
}

func TestCampSurvivesARestart(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	path := filepath.Join(t.TempDir(), "state", "quartermaster.json")
	q := newQM(t, clk, func(o *Options) { o.Store = FileStore{Path: path} })
	g.set(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	get(t, q, g, "/graphql", Interactive)
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("state file: %v %v", fi, err)
	}

	// "Restart": a new Quartermaster on the same file is still in camp.
	hits := g.hits.Load()
	q2 := newQM(t, clk, func(o *Options) { o.Store = FileStore{Path: path} })
	_, err := get(t, q2, g, "/graphql", Interactive)
	var ce *CampError
	if !errors.As(err, &ce) || !ce.Until.Equal(t0.Add(10*time.Minute)) || g.hits.Load() != hits {
		t.Fatalf("a restart got around the pause: %v", err)
	}
	// And the growth level persisted too: the next trip is longer.
	clk.Add(11 * time.Minute)
	g.set(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTooManyRequests) })
	get(t, q2, g, "/graphql", Interactive)
	if got := q2.Status().Camp.Until.Sub(clk.Now()); got != 2*time.Minute {
		t.Fatalf("level not persisted: %v", got)
	}
}

func TestStateFileProblemsAreNotFatal(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{nope"), 0o600)
	if st, err := (FileStore{Path: bad}).Load(); err != nil || st.CampUntil != (time.Time{}) {
		t.Fatalf("corrupt file: %+v %v", st, err)
	}
	if st, err := (FileStore{Path: filepath.Join(dir, "missing.json")}).Load(); err != nil || st.Level != 0 {
		t.Fatalf("missing file: %+v %v", st, err)
	}
	// A directory where the file should be is a real error at startup...
	os.Mkdir(filepath.Join(dir, "dir"), 0o700)
	if _, err := New(Options{Store: FileStore{Path: filepath.Join(dir, "dir")}}); err == nil {
		t.Fatal("unreadable state accepted")
	}
}

type failingStore struct{ MemStore }

func (*failingStore) Save(State) error { return errors.New("disk full") }

func TestAFailingSaveStillPauses(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := newQM(t, clk, func(o *Options) { o.Store = &failingStore{} })
	q.Trip(t0.Add(5*time.Minute), "x")
	if !q.Status().Camp.Active {
		t.Fatal("the pause must hold in memory")
	}
}

func TestClassContext(t *testing.T) {
	if ClassOf(context.Background()) != Interactive {
		t.Fatal("default class")
	}
	if ClassOf(WithClass(context.Background(), Background)) != Background {
		t.Fatal("class not carried")
	}
}

func TestTransportErrorsReleaseTheSlot(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := newQM(t, clk)
	c := &http.Client{Transport: q.Transport(nil), Timeout: 2 * time.Second}
	for i := 0; i < 3; i++ { // each failure must free the slot for the next
		if _, err := c.Get("http://127.0.0.1:1/graphql"); err == nil {
			t.Fatal("expected a connection error")
		}
	}
	g := newGitHub(t)
	if _, err := c.Get(g.srv.URL + "/graphql"); err != nil {
		t.Fatalf("slot leaked after errors: %v", err)
	}
}

// A long random stream of mixed responses never breaks the invariants: one
// request in flight, and nothing sent while in camp.
func TestInvariantsUnderMixedTraffic(t *testing.T) {
	clk := &fakeClock{t: t0}
	g := newGitHub(t)
	q := newQM(t, clk)
	var n atomic.Int32
	var sentDuringCamp atomic.Int32
	g.set(func(w http.ResponseWriter, r *http.Request) {
		if q.Status().Camp.Active {
			sentDuringCamp.Add(1)
		}
		switch n.Add(1) % 7 {
		case 0:
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
		case 3:
			budget(5000, 120, clk.Now().Add(20*time.Minute), "graphql")(w, r)
		default:
			budget(5000, 4000, clk.Now().Add(20*time.Minute), "graphql")(w, r)
		}
	})
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 60; i++ {
				cl := Class(i % 2)
				get(t, q, g, fmt.Sprintf("/graphql?w=%d&i=%d", w, i), cl)
				clk.Add(7 * time.Second)
			}
		}(w)
	}
	wg.Wait()
	if g.maxSeen.Load() > 1 {
		t.Fatalf("%d requests were in flight at once", g.maxSeen.Load())
	}
	if sentDuringCamp.Load() != 0 {
		t.Fatalf("%d requests were sent during camp", sentDuringCamp.Load())
	}
}

func TestErrorMessages(t *testing.T) {
	ce := &CampError{Until: t0, Reason: "GitHub sent Retry-After"}
	if !strings.Contains(ce.Error(), "slow down") || !strings.Contains(ce.Error(), "Retry-After") {
		t.Fatal(ce.Error())
	}
	re := &RationError{Resource: "graphql", Remaining: 90, Floor: 250, Reset: t0}
	if !strings.Contains(re.Error(), "90") || !strings.Contains(re.Error(), "graphql") {
		t.Fatal(re.Error())
	}
}
