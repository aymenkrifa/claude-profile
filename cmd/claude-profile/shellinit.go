package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-profile/internal/paths"
)

// shellSpec is what differs between the shells: where the integration is
// sourced from, and how that shell spells a variable assignment.
type shellSpec struct {
	name   string
	rc     string
	config []string
}

var shells = []shellSpec{
	{
		name: "zsh",
		rc:   "~/.zshrc",
		config: []string{
			`# CLAUDE_PROFILE_TERM_CLASS="ghostty:personal WezTerm:personal"`,
			`# CLAUDE_PROFILE_CLASS_DEFAULT="work:acme personal:personal"`,
			`# CLAUDE_PROFILE_FALLBACK_CLASS=work`,
		},
	},
	{
		name: "bash",
		rc:   "~/.bashrc",
		config: []string{
			`# CLAUDE_PROFILE_TERM_CLASS="ghostty:personal WezTerm:personal"`,
			`# CLAUDE_PROFILE_CLASS_DEFAULT="work:acme personal:personal"`,
			`# CLAUDE_PROFILE_FALLBACK_CLASS=work`,
		},
	},
	{
		name: "fish",
		rc:   "~/.config/fish/config.fish",
		config: []string{
			`# set -g CLAUDE_PROFILE_TERM_CLASS ghostty:personal WezTerm:personal`,
			`# set -g CLAUDE_PROFILE_CLASS_DEFAULT work:acme personal:personal`,
			`# set -g CLAUDE_PROFILE_FALLBACK_CLASS work`,
		},
	},
}

func lookupShell(name string) (shellSpec, bool) {
	for _, s := range shells {
		if s.name == name {
			return s, true
		}
	}
	return shellSpec{}, false
}

// currentShell guesses from $SHELL, so that plain 'shell-init' prints
// something the caller can actually use. zsh when the guess comes to nothing:
// it is macOS's default and the one most of this was written against.
func currentShell() string {
	if _, ok := lookupShell(filepath.Base(os.Getenv("SHELL"))); ok {
		return filepath.Base(os.Getenv("SHELL"))
	}
	return "zsh"
}

func shellNames() string {
	var names []string
	for _, s := range shells {
		names = append(names, s.name)
	}
	return strings.Join(names, " | ")
}

// cmdShellInit prints what to add to the shell's startup file.
//
// It prints a source line rather than something to eval: the integration
// discovers profiles with shell builtins so that it costs nothing on every
// shell start, and running this binary from a startup file instead would put a
// process spawn on that path for no gain.
func cmdShellInit(l paths.Layout, args []string) error {
	fs := flag.NewFlagSet("shell-init", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: claude-profile shell-init [%s]\n", shellNames())
		fmt.Fprintln(os.Stderr, "  prints the line to add to that shell's startup file")
		fmt.Fprintln(os.Stderr, "  (default: whichever $SHELL names)")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	name := fs.Arg(0)
	if name == "" {
		name = currentShell()
	}
	spec, ok := lookupShell(name)
	if !ok {
		return fmt.Errorf("no %s integration: %s only (see the README)", name, shellNames())
	}

	script := l.ShellInit(spec.name)
	if script == "" {
		return fmt.Errorf(`the %s integration is not installed.

It ships with claude-profile as claude-profile.%s and belongs at
  %s

Install it with 'make install' from a clone, or with the install script:
  curl -fsSL https://raw.githubusercontent.com/aymenkrifa/claude-profile/main/install.sh | sh`,
			spec.name, spec.name, l.ShellInitDefault(spec.name))
	}

	fmt.Printf(`# claude-profile: a claude-<name> and vs<name> command per account.
# Add to %s. Every account is exposed unless you set a terminal map.

# Optional -- 'class' restricts which accounts a terminal may reach, so a
# personal terminal cannot open a work account by accident. Set these before
# the source line; with no terminal map, no filtering is applied.
#
%s

source %s
`, spec.rc, strings.Join(spec.config, "\n"), script)
	return nil
}
