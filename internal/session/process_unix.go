//go:build !windows

package session

import "os"

func containProcess(*os.Process) (func(), error) { return func() {}, nil }
