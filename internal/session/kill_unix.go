//go:build !windows

package session

import (
	"os"
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
