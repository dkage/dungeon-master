//go:build !windows

package testutil

import "os"

func PrivateMode(_ string, info os.FileInfo, mode os.FileMode) bool {
	return info != nil && info.Mode().Perm() == mode
}
