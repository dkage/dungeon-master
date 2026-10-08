package gatekeeper

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// The shim reaches the app over a Unix domain socket, not over the port the
// browser uses. Nothing a web page can do reaches it, and the operating
// system, not a password, keeps other users out.
//
// Unix domain sockets work on macOS, Linux and Windows 10 (version 1803) or
// later, which is the minimum Windows this project supports. Windows ignores
// file modes, so there privacy comes from the directory: it is created under
// the user's own temporary directory, which only that user (and
// administrators) can open.

// URLPrefix marks a gate address that is a socket path.
const URLPrefix = "unix:"

// maxSocketPath is below the shortest sun_path limit (104 bytes on macOS).
const maxSocketPath = 100

// SocketDir is the private per-user directory that holds the sockets.
func SocketDir() string {
	id := "user"
	if u, err := user.Current(); err == nil && u.Uid != "" {
		id = u.Uid
	}
	return filepath.Join(os.TempDir(), "dungeon-"+id)
}

// ListenSocket creates the private directory when needed, refuses one that
// someone else owns or others can use, and listens on a new socket in it that
// only the user can open. The socket file is removed when the listener closes.
func ListenSocket() (net.Listener, string, error) {
	dir := SocketDir()
	if len(filepath.Join(dir, "g-0000000000.sock")) > maxSocketPath && runtime.GOOS != "windows" {
		dir = filepath.Join("/tmp", filepath.Base(dir)) // a very long TMPDIR
	}
	if err := ensurePrivateDir(dir); err != nil {
		return nil, "", err
	}
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "g-"+hex.EncodeToString(b[:])+".sock")
	if len(path) > maxSocketPath && runtime.GOOS != "windows" {
		return nil, "", fmt.Errorf("gatekeeper: socket path too long (%d bytes)", len(path))
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, "", err
	}
	if err := os.Chmod(path, 0o600); err != nil && runtime.GOOS != "windows" {
		ln.Close()
		return nil, "", err
	}
	return ln, path, nil
}

func ensurePrivateDir(dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	fi, err := os.Lstat(dir) // Lstat: a symlink planted there is refused
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("gatekeeper: %s is not a directory", dir)
	}
	return checkPrivate(dir, fi)
}

// Dialer returns the HTTP client and base URL for a gate address. A "unix:"
// address connects to that socket; anything else is used as an http URL.
func Dialer(gate string, fallback *http.Client) (*http.Client, string) {
	path, ok := strings.CutPrefix(gate, URLPrefix)
	if !ok {
		if fallback == nil {
			fallback = &http.Client{}
		}
		return fallback, strings.TrimRight(gate, "/")
	}
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		},
		DisableKeepAlives: true,
	}}, "http://dungeon-gate"
}
