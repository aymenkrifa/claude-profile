package inuse

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// find reads /proc. A kernel mounted with hidepid, or no /proc at all, means
// the answer is unknown rather than "nothing".
func find(roots []string) ([]User, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("cannot read /proc: %w", err)
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
	return out, nil
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

func comm(proc string) string {
	b, err := os.ReadFile(filepath.Join(proc, "comm"))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(b))
}
