package gatekeeper

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/dkage/dungeon-master/internal/platform"
)

// Rule is added to the instructions of every session the app starts. It
// cannot be enforced, so the shim backs it up, but a clear instruction is
// what makes an agent stop instead of retrying.
const Rule = "GitHub limits: create or change GitHub issues, comments and labels with the gh CLI one at a time, " +
	"never in a loop or a batch. Commands may be delayed on purpose to stay within GitHub's limits: wait for them " +
	"instead of retrying or working around them. If gh reports a rate limit or says writes are paused, stop and tell the user."

// Install makes dir/bin/gh a link to exe (this program), which behaves as the
// shim when started under that name. It replaces any earlier link and returns
// the directory to put first on a session's PATH.
func Install(dir, exe string) (binDir string, err error) {
	binDir = filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		src, err := os.Open(exe)
		if err != nil {
			return "", err
		}
		defer src.Close()
		tmp, err := os.CreateTemp(binDir, "gh-*.exe")
		if err != nil {
			return "", err
		}
		defer os.Remove(tmp.Name())
		_, copyErr := io.Copy(tmp, src)
		closeErr := tmp.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if err := os.Rename(tmp.Name(), filepath.Join(binDir, "gh.exe")); err != nil {
			return "", err
		}
		return binDir, nil
	}
	link := filepath.Join(binDir, "gh")
	_ = os.Remove(link)
	if err := os.Symlink(exe, link); err != nil {
		return "", fmt.Errorf("gatekeeper: link gh: %w", err)
	}
	return binDir, nil
}

// SessionEnv is what a session needs to use the shim.
type SessionEnv struct {
	BinDir string // where the shim lives
	URL    string // the app's address
	Token  string // valid for this session only
	RealGH string // the gh to run
}

// Apply returns env with the shim first on PATH and the gate settings added.
// Earlier values of the same variables are replaced.
func (s SessionEnv) Apply(env []string) []string {
	drop := map[string]bool{"PATH": true, EnvGateURL: true, EnvGateTok: true, EnvRealGH: true, EnvGateBin: true}
	out := make([]string, 0, len(env)+5)
	path := ""
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		k = platform.EnvKey(k)
		if k == "PATH" {
			path = v
		}
		if !drop[k] {
			out = append(out, kv)
		}
	}
	newPath := s.BinDir
	if path != "" {
		newPath += string(os.PathListSeparator) + path
	}
	return append(out,
		"PATH="+newPath,
		EnvGateURL+"="+s.URL,
		EnvGateTok+"="+s.Token,
		EnvRealGH+"="+s.RealGH,
		EnvGateBin+"="+s.BinDir,
	)
}

// Tokens are the per-session credentials of the shim. A token can only ask
// for write slots and report limits; it opens nothing else of the app.
type Tokens struct {
	mu sync.Mutex
	m  map[string]string // token -> session id
}

// NewTokens returns an empty registry.
func NewTokens() *Tokens { return &Tokens{m: map[string]string{}} }

// Issue creates a token for a session.
func (t *Tokens) Issue(session string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := "gate_" + hex.EncodeToString(b)
	t.mu.Lock()
	t.m[tok] = session
	t.mu.Unlock()
	return tok, nil
}

// Session returns the session a token belongs to.
func (t *Tokens) Session(tok string) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.m[tok]
	return s, ok
}

// Revoke invalidates every token of a session.
func (t *Tokens) Revoke(session string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for tok, s := range t.m {
		if s == session {
			delete(t.m, tok)
		}
	}
}
