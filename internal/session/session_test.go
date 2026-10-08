package session

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRing(t *testing.T) {
	r := newRing(5)
	r.write([]byte("abc"))
	if got := string(r.snapshot()); got != "abc" {
		t.Fatalf("got %q", got)
	}
	r.write([]byte("defg"))
	if got := string(r.snapshot()); got != "cdefg" {
		t.Fatalf("got %q", got)
	}
	r.write([]byte("0123456789"))
	if got := string(r.snapshot()); got != "56789" {
		t.Fatalf("got %q", got)
	}
}

func collect(t *testing.T, s *Session, want string) string {
	t.Helper()
	replay, sub := s.Attach()
	defer s.Detach(sub)
	var out bytes.Buffer
	out.Write(replay)
	timeout := time.After(10 * time.Second)
	for !strings.Contains(out.String(), want) {
		select {
		case b, ok := <-sub.C:
			if !ok {
				return out.String()
			}
			out.Write(b)
		case <-timeout:
			t.Fatalf("timed out waiting for %q; got %q", want, out.String())
		}
	}
	return out.String()
}

func TestSpawnEchoAndReplay(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()
	s, err := m.Spawn(Spec{Argv: []string{"sh", "-c", "echo hello-dungeon; sleep 0.2"}, Env: Environ()})
	if err != nil {
		t.Fatal(err)
	}
	collect(t, s, "hello-dungeon")
	<-s.Done()
	if exited, code := s.Exited(); !exited || code != 0 {
		t.Fatalf("exited=%v code=%d", exited, code)
	}
	// A late viewer still sees the output via replay, and its feed is closed.
	replay, sub := s.Attach()
	if !strings.Contains(string(replay), "hello-dungeon") {
		t.Fatalf("replay %q", replay)
	}
	if _, ok := <-sub.C; ok {
		t.Fatal("subscription on an exited session should be closed")
	}
}

func TestWriteAndExitCode(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()
	s, err := m.Spawn(Spec{Argv: []string{"sh", "-c", "read x; echo got:$x; exit 3"}, Env: Environ()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write([]byte("ping\n")); err != nil {
		t.Fatal(err)
	}
	collect(t, s, "got:ping")
	<-s.Done()
	if _, code := s.Exited(); code != 3 {
		t.Fatalf("code %d", code)
	}
}

func TestKillStopsProcessGroup(t *testing.T) {
	m := NewManager()
	s, err := m.Spawn(Spec{Argv: []string{"sh", "-c", "sleep 60 & sleep 60"}, Env: Environ()})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	m.Remove(s.ID)
	if time.Since(start) > 5*time.Second {
		t.Fatal("kill took too long")
	}
	select {
	case <-s.Done():
	default:
		t.Fatal("session not done after kill")
	}
	if _, ok := m.Get(s.ID); ok {
		t.Fatal("session still registered")
	}
}

func TestResizeAndBadSpec(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()
	if _, err := m.Spawn(Spec{}); err == nil {
		t.Fatal("empty argv accepted")
	}
	if _, err := m.Spawn(Spec{Argv: []string{"definitely-not-a-real-binary-xyz"}}); err == nil {
		t.Fatal("missing binary accepted")
	}
	s, err := m.Spawn(Spec{Argv: []string{"sh", "-c", "sleep 1"}, Env: Environ(), Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if err := s.Resize(0, 0); err == nil {
		t.Fatal("bad size accepted")
	}
}

func TestIDsAreUniqueAndRandom(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()
	seen := map[string]bool{}
	for range 5 {
		s, err := m.Spawn(Spec{Argv: []string{"sh", "-c", "sleep 1"}, Env: Environ()})
		if err != nil {
			t.Fatal(err)
		}
		if len(s.ID) != 9 || s.ID[0] != 's' || seen[s.ID] {
			t.Fatalf("bad or duplicate id %q", s.ID)
		}
		seen[s.ID] = true
	}
	// A fresh manager does not restart from the same ids.
	m2 := NewManager()
	defer m2.Shutdown()
	s, _ := m2.Spawn(Spec{Argv: []string{"sh", "-c", "sleep 1"}, Env: Environ()})
	if seen[s.ID] {
		t.Fatalf("id %q repeated across managers", s.ID)
	}
}
