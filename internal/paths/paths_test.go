package paths

import (
	"path/filepath"
	"strings"
	"testing"
)

// The point of Layout.OS being a field is that both platforms' layouts can be
// checked from one machine, so these tests assert the macOS spelling while
// running on whatever the developer happens to be using.
func TestLayoutPerPlatform(t *testing.T) {
	linux := Layout{Home: "/home/me", OS: "linux"}
	mac := Layout{Home: "/Users/me", OS: "darwin"}

	cases := []struct {
		what string
		got  string
		want string
	}{
		{"linux config", linux.ConfigDir("g4"), "/home/me/.claude-g4"},
		{"mac config", mac.ConfigDir("g4"), "/Users/me/.claude-g4"},
		{"linux desktop data", linux.DesktopData("g4"), "/home/me/.config/Claude-g4"},
		{"mac desktop data", mac.DesktopData("g4"), "/Users/me/Library/Application Support/Claude-g4"},
		{"linux vscode", linux.VSCodeData("g4"), "/home/me/.vscode-g4"},
		{"mac vscode", mac.VSCodeData("g4"), "/Users/me/.vscode-g4"},
		{"linux shim", linux.Shim("g4"), "/home/me/.local/bin/claude-desktop-g4"},
		{"mac shim", mac.Shim("g4"), "/Users/me/.local/bin/claude-desktop-g4"},
		{"linux entry", linux.DesktopEntry("g4"), "/home/me/.local/share/applications/claude-desktop-g4.desktop"},
		{"mac entry", mac.DesktopEntry("g4"), "/Users/me/Applications/Claude (g4).app"},
		{"linux apps dir", linux.AppsDir(), "/home/me/.local/share/applications"},
		{"mac apps dir", mac.AppsDir(), "/Users/me/Applications"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.what, c.got, c.want)
		}
	}
	if linux.Darwin() || !mac.Darwin() {
		t.Errorf("Darwin() = %v/%v, want false/true", linux.Darwin(), mac.Darwin())
	}
}

// SessionStore has to follow DesktopData onto the platform's own path, or a
// rename would rewrite session records the Desktop app never reads.
func TestSessionStoreFollowsDesktopData(t *testing.T) {
	mac := Layout{Home: "/Users/me", OS: "darwin"}
	got := mac.SessionStore("g4", "acct", "org")
	want := "/Users/me/Library/Application Support/Claude-g4/claude-code-sessions/acct/org"
	if got != want {
		t.Errorf("SessionStore = %q, want %q", got, want)
	}
}

// doctor builds its search pattern from these, so each has to be a prefix that
// a profile name gets appended to directly -- not a directory path.
func TestOwnedPrefixes(t *testing.T) {
	for _, l := range []Layout{{Home: "/home/me", OS: "linux"}, {Home: "/Users/me", OS: "darwin"}} {
		prefixes := l.OwnedPrefixes()
		if len(prefixes) != 3 {
			t.Fatalf("%s: got %d prefixes, want 3", l.OS, len(prefixes))
		}
		for _, p := range prefixes {
			if !strings.HasPrefix(p, l.Home) {
				t.Errorf("%s: prefix %q is not under the home directory", l.OS, p)
			}
			if strings.HasSuffix(p, string(filepath.Separator)) {
				t.Errorf("%s: prefix %q ends in a separator, so a name cannot follow it", l.OS, p)
			}
		}
		// Appending a name must reproduce the real directory.
		if got, want := prefixes[0]+"g4", l.ConfigDir("g4"); got != want {
			t.Errorf("%s: prefix+name = %q, want %q", l.OS, got, want)
		}
		if got, want := prefixes[2]+"g4", l.DesktopData("g4"); got != want {
			t.Errorf("%s: desktop prefix+name = %q, want %q", l.OS, got, want)
		}
	}
}

// An empty OS means "this machine", which is what the tests in other packages
// rely on when they build a Layout with only a Home.
func TestZeroOSIsThisMachine(t *testing.T) {
	if (Layout{Home: "/home/me"}).goos() != New().goos() {
		t.Error("zero OS should resolve to the running platform")
	}
}
