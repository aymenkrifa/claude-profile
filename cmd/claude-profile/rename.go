package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-profile/internal/desktop"
	"claude-profile/internal/fsx"
	"claude-profile/internal/paths"
	"claude-profile/internal/profile"
	"claude-profile/internal/rewrite"
)

func cmdRename(l paths.Layout, args []string) error {
	fs := flag.NewFlagSet("rename", flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "show every move and edit, change nothing")
	history := fs.Bool("rewrite-history", false, "also rewrite transcripts and logs (they are a record of what happened; off by default)")
	force := fs.Bool("force", false, "rename even while processes have the profile open")
	label := fs.String("label", "", "set a new label at the same time")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: claude-profile rename <old> <new> [flags]")
		fs.PrintDefaults()
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	oldName, newName := fs.Arg(0), fs.Arg(1)
	if oldName == "" || newName == "" {
		fs.Usage()
		return fmt.Errorf("need both an old and a new name")
	}
	if err := profile.ValidateName(newName); err != nil {
		return err
	}
	p, err := profile.Load(l, oldName)
	if err != nil {
		return err
	}
	if oldName == newName {
		return fmt.Errorf("%q is already the name", newName)
	}
	for _, taken := range []string{
		l.ConfigDir(newName), l.DesktopData(newName), l.VSCodeData(newName),
		l.Shim(newName), l.DesktopEntry(newName),
	} {
		if fsx.Exists(taken) {
			return fmt.Errorf("%s already exists -- pick another name or move it aside", taken)
		}
	}

	// A running session would keep writing into a directory that is about to
	// move, so stop before that happens rather than after. A dry run changes
	// nothing, so it reports and carries on to the plan.
	if err := guardInUse(oldName, []string{p.Dir, l.DesktopData(oldName)}, *dry, *force); err != nil {
		return err
	}

	b := rewrite.Builder{
		OldDir:         p.Dir,
		NewDir:         l.ConfigDir(newName),
		OldDesktop:     l.DesktopData(oldName),
		NewDesktop:     l.DesktopData(newName),
		OldVSCode:      l.VSCodeData(oldName),
		NewVSCode:      l.VSCodeData(newName),
		IncludeHistory: *history,
	}
	plan, err := b.Build()
	if err != nil {
		return err
	}
	printPlan(l, oldName, newName, plan)

	if *dry {
		fmt.Println("\ndry run -- nothing changed. Re-run without --dry-run to apply.")
		return nil
	}
	if err := plan.Apply(); err != nil {
		return fmt.Errorf("rename half-applied: %w", err)
	}

	// The marker, the launcher and the menu entry all carry the name.
	renamed := &profile.Profile{Name: newName, Dir: l.ConfigDir(newName), Class: p.Class, Label: p.Label}
	if *label != "" {
		renamed.Label = *label
	}
	if err := renamed.WriteMarker(); err != nil {
		return err
	}
	hadDesktop := desktop.Installed(l, oldName)
	if err := desktop.Remove(l, oldName); err != nil {
		return err
	}
	if hadDesktop {
		if err := desktop.Install(l, renamed); err != nil {
			return err
		}
	}
	if err := retargetSystemdEnv(l, p.Dir, renamed.Dir); err != nil {
		return err
	}

	fmt.Printf("\nrenamed %s -> %s\n", oldName, newName)
	fmt.Printf("  exec zsh              # claude-%s and vs%s replace the old commands\n", newName, newName)
	if os.Getenv("CLAUDE_CONFIG_DIR") == p.Dir {
		fmt.Printf("  note: this shell still points CLAUDE_CONFIG_DIR at the old path\n")
	}
	return nil
}

func printPlan(l paths.Layout, oldName, newName string, plan *rewrite.Plan) {
	fmt.Printf("rename %s -> %s\n\nmove:\n", oldName, newName)
	for _, m := range plan.Moves {
		fmt.Printf("  %s\n    -> %s\n", short(l, m.From), short(l, m.To))
	}
	fmt.Printf("\nrewrite paths inside %d file(s):\n", len(plan.Edits))
	byKind := map[string][]rewrite.Edit{}
	for _, e := range plan.Edits {
		byKind[e.Kind.String()] = append(byKind[e.Kind.String()], e)
	}
	for _, kind := range []string{"config", "session"} {
		edits := byKind[kind]
		if len(edits) == 0 {
			continue
		}
		fmt.Printf("  [%s]\n", kind)
		// Session records are many, uniformly named and all in one directory;
		// listing 38 UUIDs buries the config edits that deserve a read.
		if kind == "session" && len(edits) > 3 {
			fmt.Printf("  %-58s %d file(s)\n",
				short(l, filepath.Dir(edits[0].Path))+"/local_*.json", len(edits))
			continue
		}
		for _, e := range edits {
			fmt.Printf("  %-58s %d ref(s)\n", short(l, e.Path), e.Hits)
		}
	}
	if len(plan.Left) > 0 {
		fmt.Printf("\nleft as-is (a record of past work; nothing reads a path out of them):\n")
		for _, e := range plan.Left {
			fmt.Printf("  %-58s %s\n", short(l, e.Path), e.Kind)
		}
		fmt.Println("  pass --rewrite-history to include them")
	}
}

func short(l paths.Layout, p string) string {
	if strings.HasPrefix(p, l.Home) {
		return "~" + strings.TrimPrefix(p, l.Home)
	}
	return p
}

// retargetSystemdEnv keeps the single-instance Claude Desktop launcher (which
// reads CLAUDE_CONFIG_DIR from the systemd user environment) pointing at this
// profile if that is where it was already pointed.
func retargetSystemdEnv(l paths.Layout, oldDir, newDir string) error {
	conf := filepath.Join(l.Home, ".config", "environment.d", "claude.conf")
	data, err := os.ReadFile(conf)
	if err != nil {
		return nil //nolint:nilerr // absent is the normal case
	}
	if !strings.Contains(string(data), oldDir) {
		return nil
	}
	out := strings.ReplaceAll(string(data), oldDir, newDir)
	if err := fsx.WriteAtomic(conf, []byte(out)); err != nil {
		return err
	}
	fmt.Printf("updated %s\n", short(l, conf))
	return nil
}
