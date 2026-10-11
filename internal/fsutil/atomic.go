// Package fsutil holds small filesystem helpers.
package fsutil

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path through a temporary file and a rename,
// so a crash never leaves a half-written file. The directory is created with
// 0700 and the file with 0600: Dungeon Master's state is private to the user.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := MakePrivate(dir, true); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := MakePrivate(tmp.Name(), false); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// CreateFileAtomic writes data to path only if path does not already exist.
// A temporary file is linked into place so readers see either no file or the
// complete file, and an existing user config is never replaced.
func CreateFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := MakePrivate(dir, true); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := MakePrivate(tmp.Name(), false); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Link(tmpPath, path)
}
