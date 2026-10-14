//go:build !windows

package session

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A background job of a shell with job control sits in a process group of its
// own; closing the session must still end it, and must not hang on the
// terminal it holds open.
func TestKillEndsBackgroundJobsOutsideTheProcessGroup(t *testing.T) {
	m := NewManager()
	pidFile := filepath.Join(t.TempDir(), "job.pid")
	s, err := m.Spawn(Spec{Argv: []string{"sh", "-i"}, Env: Environ(), Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write([]byte("sleep 300 & echo $! > '" + pidFile + "'\n")); err != nil {
		t.Fatal(err)
	}
	var pid int
	for deadline := time.Now().Add(5 * time.Second); pid == 0 && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
	}
	if pid == 0 || !alive(pid) {
		t.Fatalf("the background job did not start (pid %d)", pid)
	}
	done := make(chan struct{})
	go func() { m.Remove(s.ID); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("closing the session hung on a background job")
	}
	deadline := time.Now().Add(3 * time.Second)
	for alive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if alive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("the background job %d survived the close", pid)
	}
}
