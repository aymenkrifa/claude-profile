// Package rewrite plans and applies the path substitutions a profile rename
// implies.
//
// Renaming ~/.claude-work to ~/.claude-g4 moves five directories, but the old
// path is also baked into files *inside* them. Not all of those references are
// equal, so the package sorts every hit into a Kind and only rewrites what has
// to change for the account to keep working:
//
//	Config   live settings the CLI reads on start -- MCP server commands,
//	         the statusLine hook, plugin install locations. Must be rewritten.
//	Session  Claude Desktop's Code-tab session records, whose cwd/claudeDir
//	         point into the Electron user-data-dir. Must be rewritten, or the
//	         app lists sessions it can no longer open.
//	History  transcripts, history.jsonl, file-history, debug logs. These are a
//	         record of what happened; rewriting them would falsify it, and
//	         nothing reads a path out of them. Left alone by default.
//	Volatile shell snapshots, daemon locks, temp files -- regenerated on the
//	         next run, so not worth touching.
package rewrite

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"claude-profile/internal/fsx"
)

// Kind classifies why a file mentions the old path.
type Kind int

const (
	Config Kind = iota
	Session
	History
	Volatile
)

func (k Kind) String() string {
	switch k {
	case Config:
		return "config"
	case Session:
		return "session"
	case History:
		return "history"
	default:
		return "volatile"
	}
}

// Move is a directory (or file) that changes location.
type Move struct{ From, To string }

// Edit is a file containing the old path, with the number of occurrences.
type Edit struct {
	Path string
	Hits int
	Kind Kind
}

// Plan is everything a rename will do, computed before anything is touched so
// that --dry-run and the real run cannot disagree.
type Plan struct {
	Moves        []Move
	Edits        []Edit // rewritten by Apply
	Left         []Edit // found, deliberately not rewritten
	Binary       []Edit // found in a binary file: never safe to rewrite
	Replacements []Replacement
}

// Replacement is one old -> new string substitution applied to file contents.
type Replacement struct{ Old, New string }

// liveConfig lists the files, relative to a profile's config directory, that
// the CLI reads as configuration. Keeping this an explicit list means a rename
// never has to walk a multi-gigabyte profile to find three JSON keys.
var liveConfig = []string{
	".claude.json",
	"settings.json",
	"settings.local.json",
	"statusline-command.sh",
	"plugins/config.json",
	"plugins/installed_plugins.json",
	"plugins/known_marketplaces.json",
	"plugins/blocklist.json",
}

// historyDirs hold the record of past work: large, append-only, and read for
// their content rather than for paths.
var historyDirs = []string{
	"projects", ".deleted-sessions", "file-history", "history.jsonl",
	"debug", "backups", "plans", "telemetry",
}

// volatileDirs are rebuilt from scratch on the next session.
var volatileDirs = []string{
	"shell-snapshots", "session-env", "daemon", "jobs", "cache", "sessions",
	"ide", "paste-cache", "image-cache", "uploads", "downloads", "tasks",
	"daemon.lock", "daemon.log", "daemon.status.json",
}

// Builder assembles a Plan for renaming a profile.
type Builder struct {
	OldDir, NewDir         string // ~/.claude-work        -> ~/.claude-g4
	OldDesktop, NewDesktop string // ~/.config/Claude-work -> ~/.config/Claude-g4
	OldVSCode, NewVSCode   string // ~/.vscode-work        -> ~/.vscode-g4
	IncludeHistory         bool   // also rewrite transcripts and logs
	AlreadyMoved           bool   // directories are at their new path already;
	// plan the rewrite alone (finishing a rename applied without history)
	Deep bool // consider every file under the profile, not just live config
}

// configRoot and desktopRoot are where the files are *now*: a rename reads them
// at the old path, a repath after the move reads them at the new one.
func (b Builder) configRoot() string {
	if b.AlreadyMoved {
		return b.NewDir
	}
	return b.OldDir
}

func (b Builder) desktopRoot() string {
	if b.AlreadyMoved {
		return b.NewDesktop
	}
	return b.OldDesktop
}

// Build inspects the filesystem and returns the plan. It reads files but never
// writes.
func (b Builder) Build() (*Plan, error) {
	p := &Plan{
		Replacements: []Replacement{
			{Old: b.OldDir, New: b.NewDir},
			{Old: b.OldDesktop, New: b.NewDesktop},
			{Old: b.OldVSCode, New: b.NewVSCode},
		},
	}
	if !b.AlreadyMoved {
		for _, m := range []Move{
			{b.OldDir, b.NewDir},
			{b.OldDesktop, b.NewDesktop},
			{b.OldVSCode, b.NewVSCode},
		} {
			if fsx.Exists(m.From) {
				p.Moves = append(p.Moves, m)
			}
		}
	}

	// 1. The live configuration the CLI reads.
	for _, rel := range liveConfig {
		b.consider(p, filepath.Join(b.configRoot(), rel), Config)
	}

	// 2. Claude Desktop's session records, wherever the account filed them.
	//    Only this glob: the rest of the Electron directory is LevelDB and
	//    IndexedDB, where a length-changing edit would corrupt the store.
	sessions, _ := filepath.Glob(filepath.Join(b.desktopRoot(), "claude-code-sessions", "*", "*", "*.json"))
	for _, f := range sessions {
		b.consider(p, f, Session)
	}

	// 3. Everything else: the whole profile when Deep, otherwise just the top
	//    of it, so a straggler is reported rather than silently missed.
	if err := b.scan(p); err != nil {
		return nil, err
	}

	sort.Slice(p.Edits, func(i, j int) bool { return p.Edits[i].Path < p.Edits[j].Path })
	sort.Slice(p.Left, func(i, j int) bool { return p.Left[i].Path < p.Left[j].Path })
	return p, nil
}

// consider records path under the right bucket if it mentions any old path.
func (b Builder) consider(p *Plan, path string, kind Kind) {
	hits, binary := b.count(path, p.Replacements)
	if hits == 0 {
		return
	}
	e := Edit{Path: path, Hits: hits, Kind: kind}
	// Substituting a shorter path inside a binary shifts every byte after it,
	// which corrupts anything with an internal offset or length prefix -- the
	// .pyc caches a job leaves behind, for instance. Report, never rewrite.
	if binary {
		p.Binary = append(p.Binary, e)
		return
	}
	switch {
	case kind == History && !b.IncludeHistory:
		p.Left = append(p.Left, e)
	case kind == Volatile && !b.Deep:
		p.Left = append(p.Left, e)
	default:
		p.Edits = append(p.Edits, e)
	}
}

// count returns how many times the old paths appear in a file, and whether the
// file is binary. A NUL byte in the first 8KB is the usual heuristic and is
// what file(1) effectively uses.
func (b Builder) count(path string, reps []Replacement) (hits int, binary bool) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Size() == 0 {
		return 0, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	for _, r := range reps {
		hits += bytes.Count(data, []byte(r.Old))
	}
	head := data
	if len(head) > 8192 {
		head = head[:8192]
	}
	return hits, bytes.IndexByte(head, 0) >= 0
}

// scan walks the profile looking for files that name the old path. Deep visits
// every file; otherwise it stays near the top and skips the bulk directories,
// so planning a rename stays instant on a multi-gigabyte profile.
func (b Builder) scan(p *Plan) error {
	seen := map[string]bool{}
	for _, e := range p.Edits {
		seen[e.Path] = true
	}
	root := b.configRoot()
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable corner must not abort the plan
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}
		top := strings.Split(rel, string(os.PathSeparator))[0]
		if d.IsDir() {
			if b.Deep {
				return nil
			}
			if contains(historyDirs, top) || contains(volatileDirs, top) {
				if contains(historyDirs, top) {
					b.considerTree(p, path, History)
				}
				return fs.SkipDir
			}
			if strings.Count(rel, string(os.PathSeparator)) >= 2 {
				return fs.SkipDir
			}
			return nil
		}
		if seen[path] {
			return nil
		}
		b.consider(p, path, classify(top, path))
		return nil
	})
}

// classify sorts a file by where it sits and what it is called.
func classify(top, path string) Kind {
	switch {
	case contains(volatileDirs, top), strings.Contains(filepath.Base(path), ".tmp."):
		return Volatile
	case contains(historyDirs, top), strings.HasSuffix(path, ".jsonl"):
		return History
	default:
		return Config
	}
}

// considerTree records a whole history directory as one entry rather than
// walking it into hundreds of lines -- these hold gigabytes and nothing reads a
// path out of them. It still checks that the directory contains at least one
// reference, so the report never claims work that is not there.
func (b Builder) considerTree(p *Plan, dir string, kind Kind) {
	if b.IncludeHistory {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil //nolint:nilerr
			}
			b.consider(p, path, History) // IncludeHistory puts these in the rewrite set
			return nil
		})
		return
	}
	if !b.anyMatch(dir, p.Replacements) {
		return
	}
	p.Left = append(p.Left, Edit{Path: dir + string(os.PathSeparator) + "...", Kind: kind})
}

// anyMatch reports whether any file below dir names an old path, stopping at
// the first one it finds.
func (b Builder) anyMatch(dir string, reps []Replacement) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr
		}
		if hits, _ := b.count(path, reps); hits > 0 {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Apply performs the moves, then rewrites the planned files at their new
// locations. Moves happen first so that a rewrite never targets a path that is
// about to move out from under it.
func (p *Plan) Apply() error {
	for _, m := range p.Moves {
		if err := fsx.Move(m.From, m.To); err != nil {
			return fmt.Errorf("move %s: %w", m.From, err)
		}
	}
	for _, e := range p.Edits {
		path := p.relocate(e.Path)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		out := data
		for _, r := range p.Replacements {
			out = bytes.ReplaceAll(out, []byte(r.Old), []byte(r.New))
		}
		if bytes.Equal(out, data) {
			continue
		}
		if err := fsx.WriteAtomic(path, out); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// relocate maps a path recorded before the moves onto its post-move location.
func (p *Plan) relocate(path string) string {
	for _, m := range p.Moves {
		if path == m.From || strings.HasPrefix(path, m.From+string(os.PathSeparator)) {
			return m.To + strings.TrimPrefix(path, m.From)
		}
	}
	return path
}

// Summary counts planned edits per kind.
func (p *Plan) Summary() map[Kind]int {
	out := map[Kind]int{}
	for _, e := range p.Edits {
		out[e.Kind]++
	}
	return out
}
