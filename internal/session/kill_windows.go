//go:build windows

package session

import "os"

func signalGroup(p *os.Process, _ bool) {
	if p != nil {
		_ = p.Kill()
	}
}

// signalSession has nothing to add: the whole job is closed with the console.
func signalSession(*os.Process) {}
