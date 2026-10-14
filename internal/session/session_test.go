package session

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
	"time"

	pty "github.com/aymanbagabas/go-pty"
	"github.com/dkage/dungeon-master/internal/testutil"
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
	s, err := m.Spawn(Spec{Argv: []string{testutil.Program(t, "hello", testutil.ProgramOptions{Output: "hello-dungeon", SleepMS: 200})}, Env: Environ()})
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
	s, err := m.Spawn(Spec{Argv: []string{testutil.Program(t, "read", testutil.ProgramOptions{Mode: "read", Exit: 3})}, Env: Environ()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write([]byte(testutil.TerminalInput("ping\n"))); err != nil {
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
	argv := []string{"sh", "-c", "sleep 60 & sleep 60"}
	if runtime.GOOS == "windows" {
		argv = []string{testutil.Program(t, "spawn", testutil.ProgramOptions{Mode: "spawn"})}
	}
	s, err := m.Spawn(Spec{Argv: argv, Env: Environ()})
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
	s, err := m.Spawn(Spec{Argv: []string{testutil.Program(t, "sleep", testutil.ProgramOptions{SleepMS: 1000})}, Env: Environ(), Cols: 80, Rows: 24})
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
		s, err := m.Spawn(Spec{Argv: []string{testutil.Program(t, "sleep", testutil.ProgramOptions{SleepMS: 1000})}, Env: Environ()})
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
	s, _ := m2.Spawn(Spec{Argv: []string{testutil.Program(t, "sleep", testutil.ProgramOptions{SleepMS: 1000})}, Env: Environ()})
	if seen[s.ID] {
		t.Fatalf("id %q repeated across managers", s.ID)
	}
}

func TestParentReleasesSlaveEnd(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()
	s, err := m.Spawn(Spec{Argv: []string{testutil.Program(t, "sleep", testutil.ProgramOptions{SleepMS: 1000})}, Env: Environ()})
	if err != nil {
		t.Fatal(err)
	}
	up, ok := s.p.(pty.UnixPty)
	if !ok {
		t.Skip("not a unix pty")
	}
	// If the parent kept its copy of the slave open, reading the master would
	// not see end-of-file on Linux after the child exits.
	if _, err := up.Slave().Stat(); err == nil {
		t.Fatal("parent still holds the slave end open")
	}
}

func TestSessionFinishesWhenProcessExits(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()
	s, err := m.Spawn(Spec{Argv: []string{testutil.Program(t, "exit", testutil.ProgramOptions{})}, Env: Environ()})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("session never finished after its process exited")
	}
}

// A viewer that does not keep up with a burst is dropped, and attaching again
// replays the scrollback so it ends on the last line.
func TestSlowViewerDroppedCanReattachAndReplay(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs seq")
	}
	m := NewManager()
	defer m.Shutdown()
	s, err := m.Spawn(Spec{Argv: []string{"sh", "-c", "sleep 0.3; seq 1 400000"}, Env: Environ()})
	if err != nil {
		t.Fatal(err)
	}
	_, slow := s.Attach() // never read while the burst runs
	<-s.Done()
	n := 0
	for range slow.C {
		n++
	}
	if n > subBuffer {
		t.Fatalf("viewer received %d chunks; it should have been dropped at %d", n, subBuffer)
	}
	replay, again := s.Attach()
	defer s.Detach(again)
	if !strings.Contains(string(replay), "400000") {
		t.Fatalf("replay does not end on the last line: %q", replay[max(0, len(replay)-40):])
	}
}

func TestRingSince(t *testing.T) {
	r := newRing(5)
	r.write([]byte("abc"))
	if d, ok := r.since(1); !ok || string(d) != "bc" {
		t.Fatalf("got %q %v", d, ok)
	}
	if d, ok := r.since(3); !ok || len(d) != 0 {
		t.Fatalf("at the end: %q %v", d, ok)
	}
	r.write([]byte("defgh")) // total 8, retained "defgh"
	if d, ok := r.since(3); !ok || string(d) != "defgh" {
		t.Fatalf("got %q %v", d, ok)
	}
	if _, ok := r.since(2); ok {
		t.Fatal("a position that was overwritten must not resume")
	}
	if _, ok := r.since(9); ok {
		t.Fatal("a position beyond the stream must not resume")
	}
	if r.start() != 3 {
		t.Fatalf("start %d", r.start())
	}
}

func TestAttachFromReplaysOnlyWhatWasMissed(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()
	s, err := m.Spawn(Spec{Argv: []string{testutil.Program(t, "hello", testutil.ProgramOptions{Output: "hello-dungeon", SleepMS: 200})}, Env: Environ()})
	if err != nil {
		t.Fatal(err)
	}
	<-s.Done()
	full, start, resumed, _ := s.AttachFrom(1 << 40) // unknown position
	if resumed || start != 0 || !strings.Contains(string(full), "hello-dungeon") {
		t.Fatalf("beyond the stream: resumed=%v start=%d %q", resumed, start, full)
	}
	cut := uint64(len(full) - 3)
	tail, start, resumed, _ := s.AttachFrom(cut)
	if !resumed || start != cut || string(tail) != string(full[len(full)-3:]) {
		t.Fatalf("resume: resumed=%v start=%d %q", resumed, start, tail)
	}
}
