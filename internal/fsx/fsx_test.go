package fsx

import (
	"os"
	"path/filepath"
	"testing"
)

// The files this rewrites sit next to credentials at 0600, so inheriting the
// mode is the whole point of writing through here rather than with WriteFile.
func TestWriteAtomicKeepsMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("new contents")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want 0600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new contents" {
		t.Errorf("contents = %q", data)
	}
}

func TestWriteAtomicKeepsAnExecutableMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline-command.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("#!/bin/sh\necho hi\n")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o755 {
		t.Errorf("mode = %v, want 0755 -- a hook that loses its execute bit stops running", got)
	}
}

// A file that does not exist yet has no mode to inherit, and these hold
// credentials-adjacent data, so the default has to be the tight one.
func TestWriteAtomicNewFileIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.json")
	if err := WriteAtomic(path, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want 0600", got)
	}
}

// The temporary file is what makes the write atomic; leaving one behind would
// put a stray .tmp beside the config on every rename.
func TestWriteAtomicLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := WriteAtomic(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "settings.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want just settings.json", names)
	}
}

func TestWriteAtomicFailsWithoutADirectory(t *testing.T) {
	if err := WriteAtomic(filepath.Join(t.TempDir(), "nope", "f.json"), []byte("x")); err == nil {
		t.Error("expected an error writing into a directory that does not exist")
	}
}

// Exists uses Lstat, so a symlink counts even when its target is gone. That is
// what the callers want: a name that is taken is taken.
func TestExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(dir, "gone"), dangling); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		file:                          true,
		dir:                           true,
		dangling:                      true,
		filepath.Join(dir, "missing"): false,
	} {
		if got := Exists(path); got != want {
			t.Errorf("Exists(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestMove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "inner"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Moves into a parent that does not exist yet.
	dst := filepath.Join(dir, "new", "dst")
	if err := Move(src, dst); err != nil {
		t.Fatal(err)
	}
	if !Exists(filepath.Join(dst, "inner")) {
		t.Error("the tree did not come along")
	}
	if Exists(src) {
		t.Error("source still present after Move")
	}

	// A missing source is not an error: a profile need not own every artefact.
	if err := Move(filepath.Join(dir, "absent"), filepath.Join(dir, "elsewhere")); err != nil {
		t.Errorf("Move of a missing source = %v, want nil", err)
	}

	// Clobbering is refused -- that would be the rename eating a directory
	// belonging to something else.
	other := filepath.Join(dir, "other")
	if err := os.MkdirAll(other, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Move(other, dst); err == nil {
		t.Error("Move over an existing destination should fail")
	}
	if !Exists(other) {
		t.Error("a refused Move must leave the source alone")
	}
}
