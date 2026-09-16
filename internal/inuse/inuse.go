// Package inuse reports whether a profile is currently open somewhere.
//
// Moving or rewriting a profile out from under a running session loses
// whatever that session appends between the read and the write, so the
// commands that do either check here first.
//
// The evidence matters. A process that merely inherited CLAUDE_CONFIG_DIR --
// every tool launched from a Claude session does -- is not a writer, and
// blocking on those would make the check something to routinely override. A
// process holding an open file under the profile, or sitting in it, is.
//
// How that evidence is gathered is per-platform (/proc on Linux, lsof on
// macOS), so Find reports an error when it cannot look rather than an empty
// result: "nothing holds this profile" and "I could not tell" must not arrive
// at a caller looking the same, because one is safe to proceed on and the
// other is not.
package inuse

import (
	"os"
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
//
// An error means the platform could not be inspected at all. Callers must
// treat that as unknown, never as clear.
func Find(dirs ...string) ([]User, error) {
	var roots []string
	for _, d := range dirs {
		if d != "" {
			roots = append(roots, d)
		}
	}
	if len(roots) == 0 {
		return nil, nil
	}
	return find(roots)
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
