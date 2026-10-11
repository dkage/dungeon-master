package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dkage/dungeon-master/internal/testutil"
)

func TestWriteFileAtomic(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a", "b", "f.json")
	if err := WriteFileAtomic(p, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("two")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "two" {
		t.Fatalf("got %q", got)
	}
	if fi, _ := os.Stat(p); !testutil.PrivateMode(p, fi, 0o600) {
		t.Fatalf("file mode %v", fi.Mode().Perm())
	}
	if di, _ := os.Stat(filepath.Dir(p)); !testutil.PrivateMode(filepath.Dir(p), di, 0o700) {
		t.Fatalf("dir mode %v", di.Mode().Perm())
	}
	// No temp files are left behind.
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Fatalf("leftovers: %v", entries)
	}
}

func TestCreateFileAtomicDoesNotReplaceExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.toml")
	if err := CreateFileAtomic(path, []byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := CreateFileAtomic(path, []byte("second\n")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second create error = %v, want os.ErrExist", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "first\n" {
		t.Fatalf("contents %q, error %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !testutil.PrivateMode(path, info, 0o600) {
		t.Fatalf("mode %o, want 600", info.Mode().Perm())
	}
}

func TestWriteFileAtomicFailsCleanly(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	_ = os.WriteFile(blocker, nil, 0o600)
	// The parent "directory" is a file, so this must fail without panicking.
	if err := WriteFileAtomic(filepath.Join(blocker, "x.json"), []byte("x")); err == nil {
		t.Fatal("expected an error")
	}
}
