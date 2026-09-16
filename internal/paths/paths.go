// Package paths knows where every artefact belonging to a profile lives.
//
// A profile spreads across five places on disk, and a rename has to keep all
// of them in step -- which is exactly why they are named in one file rather
// than spelled out at each call site.
//
// Two of those places differ by platform: Electron puts its user-data-dir
// under ~/.config on Linux and under ~/Library/Application Support on macOS,
// and an application-menu entry is a .desktop file on one and an .app bundle
// on the other.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// Layout resolves profile-owned paths beneath a home directory.
//
// OS names the platform whose conventions apply. It is a field rather than a
// runtime.GOOS lookup at each use so that the macOS layout can be exercised
// from a Linux test and back; the zero value means "this machine".
type Layout struct {
	Home string
	OS   string
}

// New returns the layout for the current user's home directory.
func New() Layout {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return Layout{Home: home, OS: runtime.GOOS}
}

// goos resolves the platform this layout describes.
func (l Layout) goos() string {
	if l.OS == "" {
		return runtime.GOOS
	}
	return l.OS
}

// Darwin reports whether macOS conventions apply.
func (l Layout) Darwin() bool { return l.goos() == "darwin" }

// ConfigDir is CLAUDE_CONFIG_DIR: credentials, settings, history, projects.
func (l Layout) ConfigDir(name string) string {
	return filepath.Join(l.Home, ".claude-"+name)
}

// DesktopData is the Electron user-data-dir for this account's Desktop app.
// The per-account directories sit beside the default one, which is where
// Electron itself puts an app's data on each platform.
func (l Layout) DesktopData(name string) string {
	if l.Darwin() {
		return filepath.Join(l.Home, "Library", "Application Support", "Claude-"+name)
	}
	return filepath.Join(l.Home, ".config", "Claude-"+name)
}

// VSCodeData is the VS Code user-data-dir the vs<name> shell function opens.
// VS Code takes it as a flag, so one path serves both platforms.
func (l Layout) VSCodeData(name string) string {
	return filepath.Join(l.Home, ".vscode-"+name)
}

// Shim is the per-account Claude Desktop launcher script.
func (l Layout) Shim(name string) string {
	return filepath.Join(l.Home, ".local", "bin", "claude-desktop-"+name)
}

// DesktopEntry is what puts the account in the application menu: a .desktop
// file on Linux, an .app bundle -- a directory -- on macOS.
func (l Layout) DesktopEntry(name string) string {
	if l.Darwin() {
		return filepath.Join(l.AppsDir(), "Claude ("+name+").app")
	}
	return filepath.Join(l.AppsDir(), "claude-desktop-"+name+".desktop")
}

// AppsDir holds the application-menu entries.
func (l Layout) AppsDir() string {
	if l.Darwin() {
		return filepath.Join(l.Home, "Applications")
	}
	return filepath.Join(l.Home, ".local", "share", "applications")
}

// ShellInit is the installed integration for one shell, found relative to the
// running binary so that a --prefix install reports its own copy rather than
// the default one. Empty if it is not where it should be.
func (l Layout) ShellInit(shell string) string {
	rel := "claude-profile." + shell
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		if exe, err := filepath.EvalSymlinks(exe); err == nil {
			// <prefix>/bin/claude-profile -> <prefix>/share/claude-profile/
			prefix := filepath.Dir(filepath.Dir(exe))
			candidates = append(candidates, filepath.Join(prefix, "share", "claude-profile", rel))
		}
	}
	candidates = append(candidates, l.ShellInitDefault(shell))
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// ShellInitDefault is where a default install puts a shell's integration,
// which is what the "not installed" message has to name.
func (l Layout) ShellInitDefault(shell string) string {
	return filepath.Join(l.Home, ".local", "share", "claude-profile", "claude-profile."+shell)
}

// SessionStore is where the Desktop app files this account's Code-tab
// sessions; the two UUIDs come from the account the profile is signed into.
func (l Layout) SessionStore(name, accountUUID, orgUUID string) string {
	return filepath.Join(l.DesktopData(name), "claude-code-sessions", accountUUID, orgUUID)
}

// ProfileGlob matches every candidate profile directory.
func (l Layout) ProfileGlob() string { return filepath.Join(l.Home, ".claude-*") }

// OwnedPrefixes lists the path prefixes a profile owns, each of which is
// immediately followed by a profile name. doctor recognises a reference to a
// profile directory by matching against these rather than by hardcoding a
// platform's spelling of them.
func (l Layout) OwnedPrefixes() []string {
	return []string{
		l.ConfigDir(""),
		l.VSCodeData(""),
		l.DesktopData(""),
	}
}

// Summary describes the layout for --help, in the spelling of this platform.
func (l Layout) Summary() string {
	out := "  ~/.claude-<name>            CLAUDE_CONFIG_DIR: credentials, settings, projects\n"
	if l.Darwin() {
		out += "  ~/Library/Application Support/Claude-<name>           Claude Desktop data\n"
	} else {
		out += "  ~/.config/Claude-<name>     Claude Desktop user-data-dir\n"
	}
	out += "  ~/.vscode-<name>            VS Code user-data-dir (vs<name> shell function)\n"
	out += "  ~/.local/bin/claude-desktop-<name>                    launcher\n"
	if l.Darwin() {
		out += "  ~/Applications/Claude (<name>).app                    app entry\n"
	} else {
		out += "  ~/.local/share/applications/claude-desktop-<name>.desktop\n"
	}
	return out
}
