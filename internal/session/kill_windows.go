//go:build windows

package session

import "os"

func signalGroup(p *os.Process, _ bool) {
	if p != nil {
		_ = p.Kill()
	}
}
