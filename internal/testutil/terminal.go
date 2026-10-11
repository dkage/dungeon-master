package testutil

import (
	"runtime"
	"strings"
)

func TerminalInput(s string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(s, "\n", "\r")
	}
	return s
}
