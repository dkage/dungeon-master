//go:build !windows

package gatekeeper

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// checkPrivate refuses a directory that belongs to someone else or that other
// users can enter, which would let them replace or reach the socket.
func checkPrivate(dir string, fi fs.FileInfo) error {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("gatekeeper: %s is owned by another user", dir)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("gatekeeper: %s is accessible to other users: %w", dir, err)
		}
	}
	return nil
}
