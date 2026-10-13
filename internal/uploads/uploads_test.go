package uploads

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00")

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "uploads")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return s, root
}

func TestSaveStoresImageUnderTheSession(t *testing.T) {
	s, root := newStore(t)
	path, kind, err := s.Save("abc123", "shot.png", png)
	if err != nil || kind != "image" {
		t.Fatalf("Save = %q, %q, %v", path, kind, err)
	}
	if filepath.Dir(path) != filepath.Join(root, "abc123") || !strings.HasSuffix(path, "-shot.png") {
		t.Errorf("path = %q", path)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, png) {
		t.Error("content differs")
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(path)
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v", fi.Mode().Perm())
		}
	}
}

func TestSaveValidates(t *testing.T) {
	s, _ := newStore(t)
	cases := []struct {
		name string
		sess string
		data []byte
		want error
	}{
		{"empty", "s1", nil, ErrEmpty},
		{"too large", "s1", append(append([]byte{}, png...), make([]byte, MaxBytes)...), ErrTooLarge},
		{"executable", "s1", []byte("\x7fELF\x02\x01\x01\x00binary"), ErrUnsupported},
		{"script named png", "s1", []byte("\x00\x01\x02 not an image"), ErrUnsupported},
		{"bad session", "../x", png, ErrBadSession},
		{"empty session", "", png, ErrBadSession},
	}
	for _, c := range cases {
		if _, _, err := s.Save(c.sess, "f.png", c.data); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestSaveAcceptsPDFAndText(t *testing.T) {
	s, _ := newStore(t)
	for data, want := range map[string]string{"%PDF-1.4\n%": "pdf", "hello notes\n": "text"} {
		if _, kind, err := s.Save("s1", "x", []byte(data)); err != nil || kind != want {
			t.Errorf("%q: kind %q err %v", data, kind, err)
		}
	}
}

func TestNamesCannotEscapeTheSessionFolder(t *testing.T) {
	s, root := newStore(t)
	for _, name := range []string{"../../etc/passwd", `..\..\evil.png`, "/abs/x.png", "..", ".", "", "a b\n$(x).png", strings.Repeat("n", 300)} {
		path, _, err := s.Save("s1", name, png)
		if err != nil {
			t.Fatalf("%q: %v", name, err)
		}
		if filepath.Dir(path) != filepath.Join(root, "s1") {
			t.Errorf("%q escaped to %q", name, path)
		}
		if base := filepath.Base(path); strings.ContainsAny(base, " \n$()") || len(base) > 80 {
			t.Errorf("%q gave unsafe base %q", name, base)
		}
	}
}

func TestRemoveDeletesOnlyThatSession(t *testing.T) {
	s, _ := newStore(t)
	a, _, _ := s.Save("a", "x.png", png)
	b, _, _ := s.Save("b", "x.png", png)
	s.Remove("a")
	s.Remove("../b") // invalid ids are ignored
	if _, err := os.Stat(a); !os.IsNotExist(err) {
		t.Error("a's file should be gone")
	}
	if _, err := os.Stat(b); err != nil {
		t.Error("b's file should remain")
	}
}

func TestNewRemovesOnlyStaleFolders(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uploads")
	for _, n := range []string{"old", "fresh"} {
		_ = os.MkdirAll(filepath.Join(root, n), 0o700)
		_ = os.WriteFile(filepath.Join(root, n, "f"), []byte("x"), 0o600)
	}
	past := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(filepath.Join(root, "old"), past, past)
	if _, err := New(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "old")); !os.IsNotExist(err) {
		t.Error("stale upload folder should be removed")
	}
	if _, err := os.Stat(filepath.Join(root, "fresh")); err != nil {
		t.Error("a young folder belongs to another server and must stay")
	}
	if _, err := New("relative"); err == nil {
		t.Error("relative root must be refused")
	}
}
