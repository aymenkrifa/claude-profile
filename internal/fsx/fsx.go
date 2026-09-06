// Package fsx holds the small filesystem helpers the rest of the tool needs.
package fsx

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic replaces path's contents without ever leaving a half-written
// file behind, keeping the original permissions. Config files here hold
// credentials-adjacent data at 0600, so inheriting the mode matters.
func WriteAtomic(path string, data []byte) error {
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Exists reports whether path is present, of any type.
func Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// Move renames src to dst, refusing to clobber an existing dst.
func Move(src, dst string) error {
	if !Exists(src) {
		return nil
	}
	if Exists(dst) {
		return fmt.Errorf("%s already exists", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}
