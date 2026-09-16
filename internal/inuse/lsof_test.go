package inuse

import (
	"strings"
	"testing"
)

// A capture of `lsof -u <uid> -n -P -F pcfn` output, trimmed to the shapes
// that matter. The darwin backend cannot be run on Linux, but the parser is
// where the decisions live, so it is tested everywhere.
const lsofSample = `p101
cclaude
fcwd
n/Users/me/.claude-g4
f3
n/Users/me/.claude-g4/history.jsonl
p202
cClaude
ftxt
n/Users/me/Library/Application Support/Claude-g4/some.dylib
f7
n/Users/me/Library/Application Support/Claude-g4/Local Storage/leveldb/LOG
p303
czsh
fcwd
n/Users/me/projects
f1
n/dev/null
p404
cnode
f12
n/Users/me/.claude-g4-other/settings.json
p505
cgrep
frtd
n/Users/me/.claude-g4
`

func TestParseLsof(t *testing.T) {
	roots := []string{
		"/Users/me/.claude-g4",
		"/Users/me/Library/Application Support/Claude-g4",
	}
	users := parseLsof(strings.NewReader(lsofSample), roots, 0)

	if len(users) != 2 {
		t.Fatalf("got %d users, want 2: %+v", len(users), users)
	}
	// cwd outranks the descriptor found later for the same process.
	if users[0].PID != 101 || users[0].How != "cwd" || !users[0].Writer {
		t.Errorf("first user = %+v, want pid 101 holding cwd", users[0])
	}
	if users[0].Name != "claude" {
		t.Errorf("command = %q, want %q", users[0].Name, "claude")
	}
	// A numbered descriptor counts; the txt mapping above it does not, so the
	// evidence has to name LOG rather than the dylib.
	if users[1].PID != 202 || !users[1].Writer || users[1].How != "open file LOG" {
		t.Errorf("second user = %+v, want pid 202 holding an open file LOG", users[1])
	}
	if Writers(users) != 2 {
		t.Errorf("Writers = %d, want 2", Writers(users))
	}
}

// pid 404 has ".claude-g4-other" open, which merely starts with a root's name.
// Treating that as a hold would block renames on a profile nobody is using.
func TestParseLsofIgnoresSiblingWithSharedPrefix(t *testing.T) {
	users := parseLsof(strings.NewReader(lsofSample), []string{"/Users/me/.claude-g4"}, 0)
	for _, u := range users {
		if u.PID == 404 {
			t.Errorf("pid 404 counted as a user of .claude-g4: %+v", u)
		}
	}
}

// The process doing the renaming always has the profile in view, so it must
// never report itself as the reason it cannot proceed.
func TestParseLsofSkipsSelf(t *testing.T) {
	users := parseLsof(strings.NewReader(lsofSample), []string{"/Users/me/.claude-g4"}, 101)
	for _, u := range users {
		if u.PID == 101 {
			t.Errorf("self reported as a user: %+v", u)
		}
	}
}

// Find must never answer "nothing holds this" when it has not looked.
func TestFindWithNoDirsIsNotAnError(t *testing.T) {
	users, err := Find()
	if err != nil || users != nil {
		t.Errorf("Find() = %v, %v; want nil, nil", users, err)
	}
}
