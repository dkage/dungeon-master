//go:build windows

package gatekeeper

import "io/fs"

// checkPrivate has nothing to check: Windows has no file modes, and the
// directory sits under the user's own temporary directory.
func checkPrivate(string, fs.FileInfo) error { return nil }
