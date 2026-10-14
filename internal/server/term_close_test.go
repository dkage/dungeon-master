//go:build !windows

package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// processAlive is false for a killed process nobody has reaped yet (a zombie).
func processAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	f := strings.Fields(string(b)[strings.LastIndex(string(b), ")")+1:])
	return len(f) == 0 || f[0] != "Z"
}

// Closing a running session stops the agent and everything it started, and the
// record keeps a "stopped by the user" outcome instead of vanishing.
func TestClosingARunningSessionStopsItsProcessGroup(t *testing.T) {
	e, _ := connServer(t, time.Hour, time.Hour)
	sj := e.create(map[string]any{"shell": true})
	c := e.dialFrom(t, sj.ID, -1)
	defer c.CloseNow()
	readAttach(t, c)
	// A background child that keeps producing output, as an agent's tool would.
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	send(t, c, "(while :; do echo tick-output; sleep 0.05; done) & echo $! > '"+pidFile+"'\n")
	var pid int
	for deadline := time.Now().Add(5 * time.Second); pid == 0 && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
	}
	if pid <= 0 || !processAlive(pid) {
		t.Fatalf("no running child to stop (pid %d)", pid)
	}

	resp, body := e.req("DELETE", "/api/sessions/"+sj.ID, nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("close: %d %s", resp.StatusCode, body)
	}
	deadline := time.Now().Add(6 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Fatalf("the child %d is still running after the session was closed", pid)
	}

	// No longer listed, but the record says it was stopped, not completed or lost.
	_, list := e.req("GET", "/api/sessions", nil, nil)
	if strings.Contains(string(list), sj.ID) {
		t.Errorf("a closed session is still listed: %s", list)
	}
	rec, ok := e.cfg.History.Get(sj.ID)
	if !ok || !rec.Stopped || rec.Lost || rec.Ended == nil {
		t.Errorf("record %+v, want stopped by the user", rec)
	}
	if resp, _ := e.req("DELETE", "/api/sessions/"+sj.ID, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("closing twice: %d", resp.StatusCode)
	}
}
