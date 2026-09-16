package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
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
		if shellReady() {
			fmt.Printf("run 'exec %s': the class decides which terminals expose claude-%s\n", currentShell(), p.Name)
		} else {
			fmt.Printf("note: class only takes effect through the shell integration -- claude-profile shell-init\n")
		}
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
	targets := []string{p.Dir, l.DesktopData(p.Name), l.VSCodeData(p.Name)}

	// Ask before touching anything. Unregistering first and confirming second
	// meant a mistyped answer left the account already unregistered and its
	// Desktop entry already gone -- an abort that had changed things.
	if *purge && !*yes {
		if err := confirmPurge(os.Stdin, p.Name, targets); err != nil {
			return err
		}
	}

	if err := desktop.Remove(l, p.Name); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(p.Dir, profile.MarkerFile)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if !*purge {
		fmt.Printf("unregistered '%s' (data left in %s)\n", p.Name, p.Dir)
		return nil
	}
	for _, t := range targets {
		if err := os.RemoveAll(t); err != nil {
			return err
		}
	}
	fmt.Printf("deleted %s and its Desktop / VS Code data\n", p.Dir)
	return nil
}

// confirmPurge makes the caller type the profile name back. Anything else,
// including a closed or non-interactive stdin, is an abort: a delete that
// cannot be confirmed must not proceed by default.
func confirmPurge(in io.Reader, name string, targets []string) error {
	fmt.Println("about to permanently delete:")
	for _, t := range targets {
		if fsx.Exists(t) {
			fmt.Println("  " + t)
		}
	}
	fmt.Printf("type the profile name to confirm: ")
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && answer == "" {
		return fmt.Errorf("aborted -- nothing was changed")
	}
	if strings.TrimSpace(answer) != name {
		return fmt.Errorf("aborted -- nothing was changed")
	}
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
	bin, err := claudeDesktopBin(l)
	if err != nil {
		return err
	}
	argv := []string{bin, "--user-data-dir=" + l.DesktopData(name)}
	if !l.Darwin() {
		// An X11 window class, which is what a .desktop entry's
		// StartupWMClass matches against; it means nothing on macOS.
		argv = append(argv, "--class=claude-desktop-"+name)
	}
	argv = append(argv, args[1:]...)
	env := append(os.Environ(), "CLAUDE_CONFIG_DIR="+p.Dir)
	return syscall.Exec(bin, argv, env)
}

// claudeDesktopBin locates the Claude Desktop executable. On Linux it is a
// command on PATH; macOS ships it inside an .app bundle, where the Electron
// binary has to be exec'd directly -- going through 'open' would hand the
// flags to a process that has already read its user-data-dir.
func claudeDesktopBin(l paths.Layout) (string, error) {
	if !l.Darwin() {
		bin, err := exec.LookPath("claude-desktop")
		if err != nil {
			return "", fmt.Errorf("claude-desktop not found on PATH")
		}
		return bin, nil
	}
	candidates := []string{
		"/Applications/Claude.app/Contents/MacOS/Claude",
		filepath.Join(l.Home, "Applications", "Claude.app", "Contents", "MacOS", "Claude"),
	}
	for _, c := range candidates {
		if fsx.Exists(c) {
			return c, nil
		}
	}
	return "", fmt.Errorf("Claude Desktop not found in /Applications or ~/Applications")
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
