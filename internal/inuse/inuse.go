// Package inuse reports whether a profile is currently open somewhere.
//
// Moving or rewriting a profile out from under a running session loses whatever
// that session appends between the read and the write, so the commands that do
// either check here first.
//
// The evidence matters. A process that merely inherited CLAUDE_CONFIG_DIR --
// every tool launched from a Claude session does -- is not a writer, and
// blocking on those would make the check something to routinely override. A
// process holding an open file under the profile, or sitting in it, is.
package inuse

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// User is one process with some tie to a profile.
type User struct {
	PID    int
	Name   string
	How    string // the evidence
	Writer bool   // holds it open, rather than just naming it
}

// Find returns processes tied to any of the given directories. Only this
// user's processes are visible, which is exactly the set that matters.
func Find(dirs ...string) []User {
	var roots []string
	for _, d := range dirs {
		if d != "" {
			roots = append(roots, d)
		}
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var out []User
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		if u, ok := inspect(e.Name(), pid, roots); ok {
			out = append(out, u)
		}
	}
	return out
}

// Writers reports how many of these processes actually hold the profile open.
func Writers(users []User) int {
	n := 0
	for _, u := range users {
		if u.Writer {
			n++
		}
	}
	return n
}

func inspect(dir string, pid int, roots []string) (User, bool) {
	proc := filepath.Join("/proc", dir)

	// Strongest evidence first: an open file, or the working directory.
	if cwd, err := os.Readlink(filepath.Join(proc, "cwd")); err == nil && under(cwd, roots) {
		return User{PID: pid, Name: comm(proc), How: "cwd", Writer: true}, true
	}
	if fds, err := os.ReadDir(filepath.Join(proc, "fd")); err == nil {
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(proc, "fd", fd.Name()))
			if err != nil || !under(target, roots) {
				continue
			}
			return User{PID: pid, Name: comm(proc), How: "open file " + filepath.Base(target), Writer: true}, true
		}
	}

	// Weaker: the process names the profile but may only have inherited it.
	if env, err := os.ReadFile(filepath.Join(proc, "environ")); err == nil {
		for _, kv := range bytes.Split(env, []byte{0}) {
			key, val, ok := bytes.Cut(kv, []byte{'='})
			if ok && string(key) == "CLAUDE_CONFIG_DIR" && contains(roots, string(val)) {
				return User{PID: pid, Name: comm(proc), How: "CLAUDE_CONFIG_DIR"}, true
			}
		}
	}
	if cmd, err := os.ReadFile(filepath.Join(proc, "cmdline")); err == nil {
		for _, root := range roots {
			if bytes.Contains(cmd, []byte("--user-data-dir="+root)) {
				return User{PID: pid, Name: comm(proc), How: "--user-data-dir"}, true
			}
		}
	}
	return User{}, false
}

func under(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func comm(proc string) string {
	b, err := os.ReadFile(filepath.Join(proc, "comm"))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(b))
}
