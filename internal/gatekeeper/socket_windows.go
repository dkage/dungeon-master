//go:build windows

package gatekeeper

import (
	"io/fs"

	"github.com/dkage/dungeon-master/internal/fsutil"
)

// checkPrivate has nothing to check: Windows has no file modes, and the
// directory sits under the user's own temporary directory.
func checkPrivate(path string, _ fs.FileInfo) error { return fsutil.MakePrivate(path, true) }
