package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"claude-profile/internal/desktop"
	"claude-profile/internal/fsx"
	"claude-profile/internal/paths"
	"claude-profile/internal/profile"
)

// seedFiles are the parts of a profile that are configuration rather than
// identity. Credentials, history, projects and sessions are never copied: the
// new account signs in fresh.
var seedFiles = []string{"settings.json", "settings.local.json", "statusline-command.sh", "CLAUDE.md"}

func cmdAdd(l paths.Layout, args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	class := fs.String("class", "", "which terminals may use it: work | personal (default: the profile name)")
	label := fs.String("label", "", "free-text note, shown in ls and the Desktop entry")
	from := fs.String("from", "", "copy settings from an existing profile")
	noDesktop := fs.Bool("no-desktop", false, "skip the Claude Desktop instance")
	login := fs.Bool("login", false, "drop straight into the CLI to run /login")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: claude-profile add <name> [flags]")
		fs.PrintDefaults()
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	name := fs.Arg(0)
	if name == "" {
		fs.Usage()
		return fmt.Errorf("missing profile name")
	}
	if err := profile.ValidateName(name); err != nil {
		return err
	}
	if profile.Exists(l, name) {
		return fmt.Errorf("profile %q already exists (%s)", name, l.ConfigDir(name))
	}

	p := &profile.Profile{Name: name, Dir: l.ConfigDir(name), Class: *class, Label: *label}
	if p.Class == "" {
		p.Class = name
	}

	adopted := fsx.Exists(p.Dir)
	if err := p.WriteMarker(); err != nil {
		return err
	}
	if adopted {
		fmt.Printf("registered existing %s\n", p.Dir)
	} else {
		fmt.Printf("created %s\n", p.Dir)
		if *from != "" {
			if err := seed(l, *from, p); err != nil {
				return err
			}
		}
	}

	if !*noDesktop {
		if err := desktop.Install(l, p); err != nil {
			return err
		}
		fmt.Printf("created Claude Desktop instance 'Claude (%s)'\n", p.Title())
	}

	if a := p.Account(); a.SignedIn {
		fmt.Printf("\nalready signed in as %s -- run it with: claude-%s\n", a.Email, name)
	} else {
		fmt.Printf(`
Next: sign the account in -- its credentials stay inside %s
  exec zsh              # pick up the new commands
  claude-%s%s   # available in any '%s' terminal, then run /login
`, p.Dir, name, padding(name), p.Class)
	}
	if *login {
		return execCLI(p.Dir, nil)
	}
	return nil
}

func padding(name string) string {
	const width = 12
	if len(name) >= width {
		return ""
	}
	return string(bytes.Repeat([]byte{' '}, width-len(name)))
}

// seed copies configuration from an existing profile, rewriting the absolute
// self-references inside it (the statusLine command, hook paths) so the copy
// points at its own directory rather than back at the source.
func seed(l paths.Layout, fromName string, to *profile.Profile) error {
	src, err := profile.Load(l, fromName)
	if err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	copied := 0
	for _, rel := range seedFiles {
		data, err := os.ReadFile(filepath.Join(src.Dir, rel))
		if err != nil {
			continue
		}
		data = bytes.ReplaceAll(data, []byte(src.Dir), []byte(to.Dir))
		dst := filepath.Join(to.Dir, rel)
		mode := os.FileMode(0o600)
		if fi, err := os.Stat(filepath.Join(src.Dir, rel)); err == nil {
			mode = fi.Mode().Perm()
		}
		if err := os.WriteFile(dst, data, mode); err != nil {
			return err
		}
		copied++
	}
	fmt.Printf("seeded %d config file(s) from '%s' (credentials, history and projects not copied)\n", copied, fromName)
	return nil
}
