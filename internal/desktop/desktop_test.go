package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-profile/internal/paths"
	"claude-profile/internal/profile"
)

// The macOS entry is generated from the Layout rather than behind a build
// constraint, which is what lets this run on any machine.
func TestInstallBundleOnDarwinLayout(t *testing.T) {
	home := t.TempDir()
	l := paths.Layout{Home: home, OS: "darwin"}
	p := &profile.Profile{Name: "g4", Dir: l.ConfigDir("g4"), Class: "work", Label: "work seat"}

	if err := Install(l, p); err != nil {
		t.Fatal(err)
	}
	if !Installed(l, "g4") {
		t.Fatal("Installed = false after Install")
	}

	bundle := filepath.Join(home, "Applications", "Claude (g4).app")
	plist, err := os.ReadFile(filepath.Join(bundle, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<string>Claude (G4)</string>",
		"com.aymenkrifa.claude-profile.g4",
		"<string>claude-g4</string>",
		"<string>APPL</string>",
	} {
		if !strings.Contains(string(plist), want) {
			t.Errorf("Info.plist missing %q", want)
		}
	}

	// The bundle executable is what Finder runs, so it has to be executable
	// and it has to route back through this tool rather than duplicate flags.
	exe := filepath.Join(bundle, "Contents", "MacOS", "claude-g4")
	fi, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("bundle executable mode = %v, want the execute bit set", fi.Mode().Perm())
	}
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "claude-profile desktop g4") {
		t.Errorf("bundle executable does not launch through claude-profile:\n%s", body)
	}

	// No .desktop file has any business existing on a macOS layout.
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "applications")); !os.IsNotExist(err) {
		t.Error("a freedesktop applications directory was created on a darwin layout")
	}

	// Remove has to recurse: the entry is a directory here, not a file.
	if err := Remove(l, "g4"); err != nil {
		t.Fatal(err)
	}
	if Installed(l, "g4") {
		t.Error("bundle still present after Remove")
	}
	if _, err := os.Stat(l.Shim("g4")); !os.IsNotExist(err) {
		t.Error("shim still present after Remove")
	}
}

func TestInstallDesktopEntryOnLinuxLayout(t *testing.T) {
	home := t.TempDir()
	l := paths.Layout{Home: home, OS: "linux"}
	p := &profile.Profile{Name: "g4", Dir: l.ConfigDir("g4"), Class: "work"}

	if err := Install(l, p); err != nil {
		t.Fatal(err)
	}
	entry, err := os.ReadFile(l.DesktopEntry("g4"))
	if err != nil {
		t.Fatal(err)
	}
	// With no label, the comment falls back to naming the account.
	for _, want := range []string{"Name=Claude (G4)", "g4 account", "StartupWMClass=claude-desktop-g4"} {
		if !strings.Contains(string(entry), want) {
			t.Errorf(".desktop entry missing %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "Applications")); !os.IsNotExist(err) {
		t.Error("an ~/Applications directory was created on a linux layout")
	}
	if err := Remove(l, "g4"); err != nil {
		t.Fatal(err)
	}
	if Installed(l, "g4") {
		t.Error("entry still present after Remove")
	}
}
