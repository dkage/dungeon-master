//go:build !windows

package session

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// alive reports whether a process still runs. A killed process that nobody has
// reaped yet (a zombie, as when the container's init does not wait) is gone.
func alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	s := string(b)
	f := strings.Fields(s[strings.LastIndex(s, ")")+1:])
	return len(f) == 0 || f[0] != "Z"
}
