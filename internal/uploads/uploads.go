// Package uploads keeps files the user pastes or drops onto a terminal in a
// private per-session folder, so the agent can be handed a real path (a
// browser never exposes the path of a dropped file).
package uploads

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dkage/dungeon-master/internal/fsutil"
)

// MaxBytes is the largest accepted file.
const MaxBytes = 10 << 20

// Errors a caller can map to a message for the user.
var (
	ErrTooLarge    = errors.New("file is larger than 10 MB")
	ErrEmpty       = errors.New("file is empty")
	ErrUnsupported = errors.New("only images, PDF and plain text files are accepted")
	ErrBadSession  = errors.New("invalid session id")
)

var sessionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Store is a folder of per-session upload folders.
type Store struct{ root string }

// staleAfter is how old a session folder must be before a new start removes it.
// Uploads normally go with their session; this only catches a crash. Another
// server on the same state directory keeps its younger folders.
const staleAfter = 24 * time.Hour

// New returns a store rooted at root and removes session folders an earlier
// run left behind.
func New(root string) (*Store, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, errors.New("uploads root must be an absolute path")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	if err := fsutil.MakePrivate(root, true); err != nil {
		return nil, err
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > staleAfter {
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
	return &Store{root: root}, nil
}

// Kind classifies content by its bytes, never by the name or the claimed
// type, and returns "" for anything not accepted.
func Kind(data []byte) string {
	switch ct := http.DetectContentType(data); {
	case ct == "image/png", ct == "image/jpeg", ct == "image/gif", ct == "image/webp":
		return "image"
	case ct == "application/pdf":
		return "pdf"
	case strings.HasPrefix(ct, "text/plain"):
		return "text"
	}
	return ""
}

// cleanName keeps a short, harmless base name: no directories, no spaces or
// control characters, never starting with a dot.
func cleanName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = name[strings.LastIndex(name, "/")+1:]
	name = strings.Trim(unsafeName.ReplaceAllString(name, "_"), "._")
	if len(name) > 60 {
		name = name[len(name)-60:]
	}
	if name == "" {
		return "file"
	}
	return name
}

// Save stores data for a session and returns the absolute path and the kind.
func (s *Store) Save(session, name string, data []byte) (path, kind string, err error) {
	if !sessionID.MatchString(session) {
		return "", "", ErrBadSession
	}
	if len(data) == 0 {
		return "", "", ErrEmpty
	}
	if len(data) > MaxBytes {
		return "", "", ErrTooLarge
	}
	if kind = Kind(data); kind == "" {
		return "", "", ErrUnsupported
	}
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	dir := filepath.Join(s.root, session)
	path = filepath.Join(dir, hex.EncodeToString(b[:])+"-"+cleanName(name))
	if rel, rerr := filepath.Rel(s.root, path); rerr != nil || strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("path escapes the uploads folder")
	}
	if err := fsutil.WriteFileAtomic(path, data); err != nil {
		return "", "", err
	}
	return path, kind, nil
}

// Remove deletes everything stored for a session.
func (s *Store) Remove(session string) {
	if sessionID.MatchString(session) {
		_ = os.RemoveAll(filepath.Join(s.root, session))
	}
}
