package rewrite

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture builds a miniature profile that mirrors the real shapes: an MCP
// server command and a statusLine hook pointing back at the config dir, a
// Desktop session record pointing into the Electron data dir, a transcript
// mentioning the path in passing, and a regenerated shell snapshot.
func fixture(t *testing.T) (home string, b Builder) {
	t.Helper()
	home = t.TempDir()
	oldDir := filepath.Join(home, ".claude-work")
	oldDesktop := filepath.Join(home, ".config", "Claude-work")
	oldVSCode := filepath.Join(home, ".vscode-work")

	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(filepath.Join(oldDir, "profile.env"), "class=work\nlabel=x\n")
	write(filepath.Join(oldDir, ".claude.json"),
		`{"mcpServers":{"k8s":{"command":"`+oldDir+`/bin/k8s-mcp"}}}`)
	write(filepath.Join(oldDir, "settings.json"),
		`{"statusLine":{"command":"bash `+oldDir+`/statusline-command.sh"}}`)
	write(filepath.Join(oldDir, "plugins", "known_marketplaces.json"),
		`{"m":{"installLocation":"`+oldDir+`/plugins/marketplaces/m"}}`)
	write(filepath.Join(oldDir, "projects", "-home-x", "abc.jsonl"),
		`{"cwd":"/home/x","text":"I read `+oldDir+`/settings.json"}`)
	write(filepath.Join(oldDir, "shell-snapshots", "snap.sh"),
		`export CLAUDE_CONFIG_DIR=`+oldDir)
	write(filepath.Join(oldDesktop, "claude-code-sessions", "acct", "org", "local_1.json"),
		`{"cwd":"`+oldDesktop+`/claude-code-sessions/acct/org/local_1/outputs"}`)
	if err := os.MkdirAll(oldVSCode, 0o755); err != nil {
		t.Fatal(err)
	}

	return home, Builder{
		OldDir: oldDir, NewDir: filepath.Join(home, ".claude-g4"),
		OldDesktop: oldDesktop, NewDesktop: filepath.Join(home, ".config", "Claude-g4"),
		OldVSCode: oldVSCode, NewVSCode: filepath.Join(home, ".vscode-g4"),
	}
}

func TestBuildClassifies(t *testing.T) {
	_, b := fixture(t)
	plan, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 3 {
		t.Errorf("Moves = %d, want 3 (config, desktop, vscode)", len(plan.Moves))
	}
	edited := map[string]Kind{}
	for _, e := range plan.Edits {
		edited[filepath.Base(e.Path)] = e.Kind
	}
	for _, want := range []string{".claude.json", "settings.json", "known_marketplaces.json"} {
		if k, ok := edited[want]; !ok || k != Config {
			t.Errorf("%s: kind %v present=%v, want config", want, k, ok)
		}
	}
	if k, ok := edited["local_1.json"]; !ok || k != Session {
		t.Errorf("session record: kind %v present=%v, want session", k, ok)
	}
	// A transcript is a record of what happened; rewriting it would falsify it.
	for _, e := range plan.Edits {
		if strings.Contains(e.Path, "projects/") || strings.Contains(e.Path, "shell-snapshots") {
			t.Errorf("%s should not be rewritten by default", e.Path)
		}
	}
	if len(plan.Left) == 0 {
		t.Error("expected the transcript to be reported as left alone")
	}
}

func TestApplyMovesAndRewrites(t *testing.T) {
	home, b := fixture(t)
	plan, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}

	newDir := filepath.Join(home, ".claude-g4")
	if _, err := os.Stat(filepath.Join(home, ".claude-work")); !os.IsNotExist(err) {
		t.Error("old config dir still present")
	}
	for _, tc := range []struct{ path, want string }{
		{filepath.Join(newDir, ".claude.json"), newDir + "/bin/k8s-mcp"},
		{filepath.Join(newDir, "settings.json"), newDir + "/statusline-command.sh"},
		{filepath.Join(newDir, "plugins", "known_marketplaces.json"), newDir + "/plugins/marketplaces/m"},
		{filepath.Join(home, ".config", "Claude-g4", "claude-code-sessions", "acct", "org", "local_1.json"),
			filepath.Join(home, ".config", "Claude-g4")},
	} {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if !strings.Contains(string(data), tc.want) {
			t.Errorf("%s: missing %q after rename\ngot: %s", tc.path, tc.want, data)
		}
		if strings.Contains(string(data), ".claude-work") || strings.Contains(string(data), "Claude-work") {
			t.Errorf("%s: still references the old name: %s", tc.path, data)
		}
	}

	// The transcript moved with the directory but kept its contents.
	tr, err := os.ReadFile(filepath.Join(newDir, "projects", "-home-x", "abc.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tr), ".claude-work") {
		t.Error("transcript was rewritten; history should be left as recorded")
	}
}

func TestIncludeHistoryRewritesTranscripts(t *testing.T) {
	home, b := fixture(t)
	b.IncludeHistory = true
	plan, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	tr, err := os.ReadFile(filepath.Join(home, ".claude-g4", "projects", "-home-x", "abc.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tr), ".claude-work") {
		t.Errorf("--rewrite-history left the old path in place: %s", tr)
	}
}

// Preserving file modes matters: .claude.json sits next to credentials at 0600.
func TestApplyPreservesMode(t *testing.T) {
	home, b := fixture(t)
	plan, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(home, ".claude-g4", ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}
}

func TestBuildIsReadOnly(t *testing.T) {
	home, b := fixture(t)
	if _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude-work", ".claude.json")); err != nil {
		t.Errorf("Build touched the filesystem: %v", err)
	}
}

// A binary file that happens to contain the old path must never be rewritten:
// the replacement is shorter, so every following byte shifts and anything with
// an internal offset or length prefix (a .pyc, say) is corrupted.
func TestBinaryFilesAreSkipped(t *testing.T) {
	home, b := fixture(t)
	pyc := filepath.Join(home, ".claude-work", "jobs", "x", "__pycache__", "m.pyc")
	if err := os.MkdirAll(filepath.Dir(pyc), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := append([]byte{0x0b, 0x0d, 0x0d, 0x0a, 0x00, 0x00}, []byte(filepath.Join(home, ".claude-work")+"/x.py")...)
	if err := os.WriteFile(pyc, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	b.Deep, b.IncludeHistory, b.AlreadyMoved = true, true, false
	plan, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Binary) != 1 {
		t.Fatalf("Binary = %v, want the .pyc", plan.Binary)
	}
	for _, e := range plan.Edits {
		if e.Path == pyc {
			t.Fatal(".pyc landed in the rewrite set")
		}
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(home, ".claude-g4", "jobs", "x", "__pycache__", "m.pyc"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("binary file was modified")
	}
}

// repath is the rename's second half: directories already moved, only the
// strings inside them are stale.
func TestAlreadyMovedRewritesInPlace(t *testing.T) {
	home, b := fixture(t)
	plan, err := b.Build() // a plain rename, history left alone
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}

	b.AlreadyMoved, b.Deep, b.IncludeHistory = true, true, true
	plan2, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan2.Moves) != 0 {
		t.Errorf("Moves = %v, want none: the directories are already in place", plan2.Moves)
	}
	if len(plan2.Edits) == 0 {
		t.Fatal("nothing found to rewrite, expected the transcript")
	}
	if err := plan2.Apply(); err != nil {
		t.Fatal(err)
	}
	tr, err := os.ReadFile(filepath.Join(home, ".claude-g4", "projects", "-home-x", "abc.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(tr, []byte(".claude-work")) {
		t.Errorf("transcript still names the old profile: %s", tr)
	}
}

// A history directory is only reported as "left as-is" when it actually holds a
// reference -- otherwise the plan claims work that does not exist.
func TestEmptyHistoryDirNotReported(t *testing.T) {
	home, b := fixture(t)
	empty := filepath.Join(home, ".claude-work", "telemetry")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(empty, "events.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range plan.Left {
		if strings.Contains(e.Path, "telemetry") {
			t.Errorf("telemetry has no references but was reported: %v", e.Path)
		}
	}
	var sawProjects bool
	for _, e := range plan.Left {
		if strings.Contains(e.Path, "projects") {
			sawProjects = true
		}
	}
	if !sawProjects {
		t.Error("projects does hold a reference and should still be reported")
	}
}
