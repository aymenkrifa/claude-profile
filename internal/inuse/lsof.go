package inuse

import (
	"bufio"
	"io"
	"path/filepath"
	"strings"
)

// parseLsof reads `lsof -F pcfn` output and returns the processes holding a
// file open under one of roots.
//
// The format is one field per line, each prefixed by its identifier: p<pid>
// and c<command> open a process, then f<fd> / n<name> repeat per open file.
// The fd tells us what kind of hold it is -- "cwd" for the working directory,
// a number for a real descriptor. Mapped text and memory (txt, mem, rtd) are
// skipped: /proc/<pid>/fd does not list those either, so counting them would
// make the two platforms disagree about what "in use" means.
//
// This file carries no build constraint on purpose. Only the code that shells
// out to lsof is darwin-only; keeping the parser portable is what lets it be
// tested on any machine.
func parseLsof(r io.Reader, roots []string, self int) []User {
	byPID := map[int]User{}
	var order []int

	var pid int
	var command, fd string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		field, val := line[0], line[1:]
		switch field {
		case 'p':
			pid, command, fd = atoi(val), "", ""
		case 'c':
			command = val
		case 'f':
			fd = val
		case 'n':
			if pid == 0 || pid == self || !under(val, roots) {
				continue
			}
			u, ok := holder(pid, command, fd, val)
			if !ok {
				continue
			}
			// cwd is the clearest evidence, so it outranks a descriptor
			// found earlier for the same process.
			if prev, seen := byPID[pid]; seen && (prev.How == "cwd" || u.How != "cwd") {
				continue
			} else if !seen {
				order = append(order, pid)
			}
			byPID[pid] = u
		}
	}

	out := make([]User, 0, len(order))
	for _, p := range order {
		out = append(out, byPID[p])
	}
	return out
}

// holder turns one open file into evidence, or reports that this kind of hold
// does not count.
func holder(pid int, command, fd, name string) (User, bool) {
	if command == "" {
		command = "?"
	}
	switch {
	case fd == "cwd":
		return User{PID: pid, Name: command, How: "cwd", Writer: true}, true
	case fd != "" && fd[0] >= '0' && fd[0] <= '9':
		return User{PID: pid, Name: command, How: "open file " + filepath.Base(name), Writer: true}, true
	default:
		return User{}, false
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range strings.TrimSpace(s) {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
