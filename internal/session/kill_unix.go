//go:build !windows

package session

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// signalGroup signals the child's whole process group (the child is a
// session leader, so its pid is the group id).
func signalGroup(p *os.Process, force bool) {
	if p == nil {
		return
	}
	sig := syscall.SIGHUP
	if force {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(-p.Pid, sig); err != nil {
		_ = p.Signal(sig)
	}
}

// signalSession kills every process of the child's session, wherever it is in
// a process group of its own: a shell with job control puts a background job
// in a new group, which the group signal misses, and such a job also keeps
// the terminal open so the session would never finish.
func signalSession(p *os.Process) {
	if p == nil {
		return
	}
	for _, pid := range sessionMembers(p.Pid) {
		if pid != os.Getpid() {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

// sessionMembers lists the processes whose session id is sid, from /proc where
// it exists and from pgrep elsewhere. An unavailable source finds nothing.
func sessionMembers(sid int) []int {
	if entries, err := os.ReadDir("/proc"); err == nil {
		var out []int
		for _, e := range entries {
			pid, err := strconv.Atoi(e.Name())
			if err != nil {
				continue
			}
			b, err := os.ReadFile("/proc/" + e.Name() + "/stat")
			if err != nil {
				continue
			}
			// "pid (comm) state ppid pgrp session ...": comm may hold spaces and parentheses.
			s := string(b)
			i := strings.LastIndex(s, ")")
			if i < 0 {
				continue
			}
			f := strings.Fields(s[i+1:])
			if len(f) > 3 {
				if got, err := strconv.Atoi(f[3]); err == nil && got == sid {
					out = append(out, pid)
				}
			}
		}
		return out
	}
	out, err := exec.Command("pgrep", "-s", strconv.Itoa(sid)).Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, l := range strings.Fields(string(out)) {
		if pid, err := strconv.Atoi(l); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}
