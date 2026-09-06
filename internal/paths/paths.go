// Package paths knows where every artefact belonging to a profile lives.
//
// A profile called "work" spreads across five places on disk, and a rename has
// to keep all of them in step -- which is exactly why they are named in one
// file rather than spelled out at each call site.
package paths

import (
	"os"
	"path/filepath"
)

// Layout resolves profile-owned paths beneath a home directory.
type Layout struct{ Home string }

// New returns the layout for the current user's home directory.
func New() Layout {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return Layout{Home: home}
}

// ConfigDir is CLAUDE_CONFIG_DIR: credentials, settings, history, projects.
func (l Layout) ConfigDir(name string) string {
	return filepath.Join(l.Home, ".claude-"+name)
}

// DesktopData is the Electron user-data-dir for this account's Desktop app.
func (l Layout) DesktopData(name string) string {
	return filepath.Join(l.Home, ".config", "Claude-"+name)
}

// VSCodeData is the VS Code user-data-dir the vs<name> shell function opens.
func (l Layout) VSCodeData(name string) string {
	return filepath.Join(l.Home, ".vscode-"+name)
}

// Shim is the per-account Claude Desktop launcher script.
func (l Layout) Shim(name string) string {
	return filepath.Join(l.Home, ".local", "bin", "claude-desktop-"+name)
}

// DesktopEntry is the .desktop file that puts the account in the app menu.
func (l Layout) DesktopEntry(name string) string {
	return filepath.Join(l.Home, ".local", "share", "applications", "claude-desktop-"+name+".desktop")
}

// AppsDir holds the .desktop entries.
func (l Layout) AppsDir() string {
	return filepath.Join(l.Home, ".local", "share", "applications")
}

// SessionStore is where the Desktop app files this account's Code-tab
// sessions; the two UUIDs come from the account the profile is signed into.
func (l Layout) SessionStore(name, accountUUID, orgUUID string) string {
	return filepath.Join(l.DesktopData(name), "claude-code-sessions", accountUUID, orgUUID)
}

// ProfileGlob matches every candidate profile directory.
func (l Layout) ProfileGlob() string { return filepath.Join(l.Home, ".claude-*") }
