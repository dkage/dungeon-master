package quartermaster

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func pacer(t *testing.T, clk Clock, store Store, mutate ...func(*Options)) *Quartermaster {
	t.Helper()
	o := Options{Clock: clk, Store: store, WriteJitter: -1, Rand: func() float64 { return 0 }}
	for _, f := range mutate {
		f(&o)
	}
	q, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func reserve(t *testing.T, q *Quartermaster) (time.Duration, error) {
	t.Helper()
	return q.Reserve(context.Background())
}

func TestTheFirstWriteIsImmediateAndTheNextOneWaitsTheGap(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := pacer(t, clk, nil)
	if w, err := reserve(t, q); err != nil || w != 0 {
		t.Fatalf("first write waited %v: %v", w, err)
	}
	if w, err := reserve(t, q); err != nil || w != 3*time.Second {
		t.Fatalf("second write waited %v, want the 3 s gap: %v", w, err)
	}
	// Time that passes by itself counts.
	clk.Add(10 * time.Second)
	if w, _ := reserve(t, q); w != 0 {
		t.Fatalf("a write after a long pause waited %v", w)
	}
}

func TestJitterIsAddedToTheGapAndChosenPerWrite(t *testing.T) {
	clk := &fakeClock{t: t0}
	draws := []float64{0.5, 0.0, 1.0 - 1e-9}
	var i int
	q := pacer(t, clk, nil, func(o *Options) {
		o.WriteJitter = time.Second
		o.Rand = func() float64 { v := draws[i%len(draws)]; i++; return v }
	})
	reserve(t, q) // draws 0.5: the gap after it is 3.5 s
	if w, _ := reserve(t, q); w != 3500*time.Millisecond {
		t.Fatalf("waited %v, want 3.5 s", w)
	}
	if w, _ := reserve(t, q); w != 3*time.Second { // the second draw was 0
		t.Fatalf("waited %v, want 3 s", w)
	}
}

func TestAtMostFifteenWritesInAnyMinute(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := pacer(t, clk, nil)
	var at []time.Time
	for i := 0; i < 16; i++ {
		if _, err := reserve(t, q); err != nil {
			t.Fatal(err)
		}
		at = append(at, clk.Now())
	}
	// 0, 3, 6 ... 42 are fifteen writes inside the minute; the sixteenth must wait for the first to age out.
	if got := at[15].Sub(at[0]); got != time.Minute {
		t.Fatalf("the 16th write came %v after the first, want 1m0s", got)
	}
	if got := at[14].Sub(at[0]); got != 42*time.Second {
		t.Fatalf("the 15th write came %v after the first, want 42s", got)
	}
}

func TestTheHourlyCapAndTheTooBusyRefusal(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := pacer(t, clk, nil, func(o *Options) {
		o.WriteGap = time.Second
		o.WritesPerMinute = 1000
		o.WritesPerHour = 10
		o.MaxWait = 2 * time.Minute
	})
	for i := 0; i < 10; i++ {
		if _, err := reserve(t, q); err != nil {
			t.Fatal(err)
		}
	}
	before := q.Status().Writes
	_, err := reserve(t, q)
	var busy *TooBusyError
	if !errors.As(err, &busy) || busy.Wait < 50*time.Minute || busy.Wait > time.Hour {
		t.Fatalf("an 11th write inside the hour: %v", err)
	}
	if got := q.Status().Writes; got != before {
		t.Fatalf("a refused request reserved a slot: %+v -> %+v", before, got)
	}
	if busy.Error() == "" {
		t.Fatal("empty message")
	}
	// Willing to wait? Then it does, and gets the slot when the oldest write ages out.
	patient := pacer(t, &fakeClock{t: t0}, nil, func(o *Options) {
		o.WriteGap = time.Second
		o.WritesPerMinute = 1000
		o.WritesPerHour = 3
		o.MaxWait = 2 * time.Hour
	})
	var firstAt, fourthAt time.Time
	for i := 0; i < 4; i++ {
		if _, err := patient.Reserve(context.Background()); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			firstAt = patient.clock.Now()
		}
		fourthAt = patient.clock.Now()
	}
	if got := fourthAt.Sub(firstAt); got != time.Hour {
		t.Fatalf("the 4th write came %v after the first, want 1h0m0s", got)
	}
}

func TestCampStopsWritesWithoutReservingAnything(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := pacer(t, clk, nil)
	reserve(t, q)
	q.Trip(t0.Add(10*time.Minute), "GitHub sent Retry-After")
	w0 := q.Status().Writes
	_, err := reserve(t, q)
	var ce *CampError
	if !errors.As(err, &ce) {
		t.Fatalf("%v", err)
	}
	if q.Status().Writes != w0 {
		t.Fatal("a refused write was counted")
	}
	// Camp ends: writes resume (the gap since the last write long passed).
	clk.Add(11 * time.Minute)
	if _, err := reserve(t, q); err != nil {
		t.Fatalf("still refused after camp: %v", err)
	}
}

// tripClock trips camp while a write is asleep waiting for its slot.
type tripClock struct {
	fakeClock
	q *Quartermaster
}

func (c *tripClock) Sleep(ctx context.Context, d time.Duration) error {
	c.q.Trip(c.Now().Add(5*time.Minute), "GitHub reported a rate limit")
	return c.fakeClock.Sleep(ctx, d)
}

func TestACampThatStartsWhileWaitingStopsTheWrite(t *testing.T) {
	clk := &tripClock{fakeClock: fakeClock{t: t0}}
	q := pacer(t, clk, nil)
	clk.q = q
	reserve(t, q)
	_, err := reserve(t, q) // sleeps for the gap; camp begins meanwhile
	var ce *CampError
	if !errors.As(err, &ce) {
		t.Fatalf("a write went ahead although GitHub had just asked us to slow down: %v", err)
	}
	if q.Status().Writes.LastMinute != 1 {
		t.Fatalf("writes: %+v", q.Status().Writes)
	}
}

func TestCancellationWhileWaitingReservesNothing(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := pacer(t, clk, nil)
	reserve(t, q)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := q.Reserve(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	if q.Status().Writes.LastHour != 1 {
		t.Fatalf("a cancelled write was counted: %+v", q.Status().Writes)
	}
}

func TestPacingSurvivesARestart(t *testing.T) {
	clk := &fakeClock{t: t0}
	store := &MemStore{}
	q := pacer(t, clk, store)
	for i := 0; i < 15; i++ {
		reserve(t, q)
	}
	// "Restart": a new Quartermaster on the same state, right after the 15th write.
	q2 := pacer(t, clk, store)
	if got := q2.Status().Writes.LastMinute; got != 15 {
		t.Fatalf("recent writes forgotten: %+v", q2.Status().Writes)
	}
	start := clk.Now()
	w, err := reserve(t, q2)
	if err != nil || clk.Now().Sub(start) != w || w < 18*time.Second {
		t.Fatalf("a restart reset the pacing: waited %v: %v", w, err)
	}
}

func TestPacingSurvivesARestartThroughTheFile(t *testing.T) {
	clk := &fakeClock{t: t0}
	store := FileStore{Path: t.TempDir() + "/qm.json"}
	q := pacer(t, clk, store)
	reserve(t, q)
	reserve(t, q)
	q2 := pacer(t, clk, store)
	if got := q2.Status().Writes.LastMinute; got != 2 {
		t.Fatalf("%+v", q2.Status().Writes)
	}
}

func TestOldWritesAreForgottenAndStatusCountsTheWindows(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := pacer(t, clk, nil)
	reserve(t, q)
	clk.Add(30 * time.Second)
	reserve(t, q)
	if got := q.Status().Writes; got.LastMinute != 2 || got.LastHour != 2 {
		t.Fatalf("%+v", got)
	}
	clk.Add(45 * time.Second) // the first is now 75 s old
	if got := q.Status().Writes; got.LastMinute != 1 || got.LastHour != 2 {
		t.Fatalf("%+v", got)
	}
	clk.Add(2 * time.Hour)
	reserve(t, q)
	if n := len(q.state.Writes); n != 1 {
		t.Fatalf("%d writes kept after two hours", n)
	}
}

func TestConcurrentWritersAreSpacedOut(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := pacer(t, clk, nil)
	var wg sync.WaitGroup
	var failures atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := q.Reserve(context.Background()); err != nil {
				t.Error(err)
				failures.Add(1)
			}
		}()
	}
	wg.Wait()
	if failures.Load() != 0 {
		return
	}
	// The times the Quartermaster itself recorded are the truth. (Reading the
	// shared fake clock in the goroutines would be wrong: another writer's sleep
	// advances it before this one looks.)
	q.mu.Lock()
	at := append([]time.Time(nil), q.state.Writes...)
	q.mu.Unlock()
	if len(at) != 12 {
		t.Fatalf("%d writes recorded for 12 writers", len(at))
	}
	checkPacing(t, at, 3*time.Second, 15, 150)
}

// checkPacing verifies the invariants over a list of write times.
func checkPacing(t *testing.T, at []time.Time, gap time.Duration, perMin, perHour int) {
	t.Helper()
	sorted := append([]time.Time(nil), at...)
	for i := 1; i < len(sorted); i++ { // insertion sort: lists are small
		for j := i; j > 0 && sorted[j].Before(sorted[j-1]); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	for i := 1; i < len(sorted); i++ {
		if d := sorted[i].Sub(sorted[i-1]); d < gap {
			t.Fatalf("writes %d and %d are only %v apart, minimum %v", i-1, i, d, gap)
		}
	}
	for i := range sorted {
		inMin, inHour := 0, 0
		for j := 0; j <= i; j++ {
			if d := sorted[i].Sub(sorted[j]); d < time.Minute {
				inMin++
			}
			if d := sorted[i].Sub(sorted[j]); d < time.Hour {
				inHour++
			}
		}
		if inMin > perMin {
			t.Fatalf("%d writes within a minute ending at write %d, cap %d", inMin, i, perMin)
		}
		if inHour > perHour {
			t.Fatalf("%d writes within an hour ending at write %d, cap %d", inHour, i, perHour)
		}
	}
}

// For random bursts and pauses over several simulated hours, the caps always hold.
func TestTheCapsHoldUnderRandomTraffic(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		rng := rand.New(rand.NewSource(seed))
		clk := &fakeClock{t: t0}
		q := pacer(t, clk, nil, func(o *Options) {
			o.WriteGap = 2 * time.Second
			o.WritesPerMinute = 10
			o.WritesPerHour = 60
			o.MaxWait = 3 * time.Hour
			o.WriteJitter = time.Second
			o.Rand = rng.Float64
		})
		var at []time.Time
		for i := 0; i < 250; i++ {
			clk.Add(time.Duration(rng.Intn(20)) * time.Second) // bursts and lulls
			if _, err := q.Reserve(context.Background()); err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			at = append(at, clk.Now())
		}
		checkPacing(t, at, 2*time.Second, 10, 60)
		if span := at[len(at)-1].Sub(at[0]); span < 2*time.Hour {
			t.Fatalf("seed %d: only %v simulated, too short to exercise the hourly cap", seed, span)
		}
	}
}

func TestAFailingSaveDoesNotBreakPacing(t *testing.T) {
	clk := &fakeClock{t: t0}
	q := pacer(t, clk, &failingStore{})
	reserve(t, q)
	if w, err := reserve(t, q); err != nil || w != 3*time.Second {
		t.Fatalf("%v %v", w, err)
	}
}

func TestWritePacingDefaults(t *testing.T) {
	q, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if q.writeGap != 3*time.Second || q.writeJitter != time.Second || q.writesPerMinute != 15 || q.writesPerHour != 150 ||
		q.maxWait != 2*time.Minute || q.rand == nil {
		t.Fatalf("%+v", q)
	}
}
