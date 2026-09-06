package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"claude-profile/internal/desktop"
	"claude-profile/internal/fsx"
	"claude-profile/internal/paths"
	"claude-profile/internal/profile"
)

func cmdSet(l paths.Layout, args []string) error {
	fs := flag.NewFlagSet("set", flag.ExitOnError)
	class := fs.String("class", "", "new class: work | personal")
	label := fs.String("label", "", "new label (use \"\" to clear)")
	desk := fs.Bool("desktop", true, "give this account its own Claude Desktop instance (--desktop=false removes it)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: claude-profile set <name> [--class ...] [--label ...] [--desktop=false]")
		fs.PrintDefaults()
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	p, err := profile.Load(l, fs.Arg(0))
	if err != nil {
		return err
	}
	changed, reinstall, drop, classChanged := false, false, false, false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "class":
			p.Class, changed, classChanged = *class, true, true
		case "label":
			p.Label, changed = *label, true
		case "desktop":
			reinstall, drop, changed = *desk, !*desk, true
		}
	})
	if !changed {
		return fmt.Errorf("nothing to change: pass --class, --label and/or --desktop")
	}
	if err := p.WriteMarker(); err != nil {
		return err
	}
	switch {
	case drop:
		if err := desktop.Remove(l, p.Name); err != nil {
			return err
		}
		fmt.Printf("removed the Claude Desktop instance for '%s'\n", p.Name)
	// The label is baked into the .desktop entry, so keep it in step.
	case reinstall || desktop.Installed(l, p.Name):
		if err := desktop.Install(l, p); err != nil {
			return err
		}
	}
	fmt.Printf("%s: class=%s label=%s\n", p.Name, p.Class, p.Label)
	if classChanged {
		fmt.Printf("run 'exec zsh': the class decides which terminals expose claude-%s\n", p.Name)
	}
	return nil
}

func cmdRm(l paths.Layout, args []string) error {
	fs := flag.NewFlagSet("rm", flag.ExitOnError)
	purge := fs.Bool("purge", false, "also delete the config, Desktop and VS Code data")
	yes := fs.Bool("y", false, "skip the confirmation prompt for --purge")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	p, err := profile.Load(l, fs.Arg(0))
	if err != nil {
		return err
	}
	if err := desktop.Remove(l, p.Name); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(p.Dir, profile.MarkerFile)); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Printf("unregistered '%s' (data left in %s)\n", p.Name, p.Dir)

	if !*purge {
		return nil
	}
	targets := []string{p.Dir, l.DesktopData(p.Name), l.VSCodeData(p.Name)}
	if !*yes {
		fmt.Println("\nabout to permanently delete:")
		for _, t := range targets {
			if fsx.Exists(t) {
				fmt.Println("  " + t)
			}
		}
		fmt.Printf("type the profile name to confirm: ")
		var answer string
		fmt.Scanln(&answer)
		if strings.TrimSpace(answer) != p.Name {
			return fmt.Errorf("aborted")
		}
	}
	for _, t := range targets {
		if err := os.RemoveAll(t); err != nil {
			return err
		}
	}
	fmt.Printf("deleted %s and its Desktop / VS Code data\n", p.Dir)
	return nil
}

func cmdRun(l paths.Layout, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: claude-profile run <name> [-- claude args...]")
	}
	p, err := profile.Load(l, args[0])
	if err != nil {
		return err
	}
	rest := args[1:]
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	return execCLI(p.Dir, rest)
}

func cmdPath(l paths.Layout, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: claude-profile path <name>")
	}
	p, err := profile.Load(l, args[0])
	if err != nil {
		return err
	}
	fmt.Println(p.Dir)
	return nil
}

// cmdDesktop launches one account's Claude Desktop instance. The generated
// ~/.local/bin/claude-desktop-<name> shim and the .desktop entry both come
// through here, so the flags live in exactly one place.
func cmdDesktop(l paths.Layout, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: claude-profile desktop <name> [claude-desktop args...]")
	}
	name := args[0]
	p, err := profile.Load(l, name)
	if err != nil {
		return err
	}
	bin, err := exec.LookPath("claude-desktop")
	if err != nil {
		return fmt.Errorf("claude-desktop not found on PATH")
	}
	argv := append([]string{
		bin,
		"--user-data-dir=" + l.DesktopData(name),
		"--class=claude-desktop-" + name,
	}, args[1:]...)
	env := append(os.Environ(), "CLAUDE_CONFIG_DIR="+p.Dir)
	return syscall.Exec(bin, argv, env)
}

// execCLI replaces this process with the Claude CLI bound to one profile.
func execCLI(configDir string, args []string) error {
	bin, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH")
	}
	env := append(os.Environ(), "CLAUDE_CONFIG_DIR="+configDir)
	return syscall.Exec(bin, append([]string{bin}, args...), env)
}
