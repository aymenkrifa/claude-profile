package main

import (
	"flag"
	"fmt"
	"os"

	"claude-profile/internal/fsx"
	"claude-profile/internal/paths"
	"claude-profile/internal/profile"
	"claude-profile/internal/rewrite"
)

// cmdRepath finishes a rename that was applied without --rewrite-history.
//
// The directories moved with the rename, so the files are already in the right
// place; what is left is the old path still written inside them -- transcripts,
// history.jsonl, debug logs, job scratch. Nothing reads a path out of those, so
// this is cosmetic rather than corrective, but a profile that names a directory
// that no longer exists is confusing to grep through later.
func cmdRepath(l paths.Layout, args []string) error {
	fs := flag.NewFlagSet("repath", flag.ExitOnError)
	from := fs.String("from", "", "the profile name these files still refer to")
	dry := fs.Bool("dry-run", false, "show what would change, change nothing")
	force := fs.Bool("force", false, "rewrite even while processes have the profile open")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: claude-profile repath <profile> --from <old-name> [--dry-run]")
		fs.PrintDefaults()
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	p, err := profile.Load(l, fs.Arg(0))
	if err != nil {
		return err
	}
	oldName := *from
	if oldName == "" {
		fs.Usage()
		return fmt.Errorf("--from is required: which old name are we replacing?")
	}
	if oldName == p.Name {
		return fmt.Errorf("--from %q is the profile's own name", oldName)
	}
	if fsx.Exists(l.ConfigDir(oldName)) {
		return fmt.Errorf("%s still exists -- this is a rename, not a repath:\n  claude-profile rename %s %s --rewrite-history",
			l.ConfigDir(oldName), oldName, p.Name)
	}

	// A live session appends to history.jsonl and to its own transcript, and
	// this rewrites whole files: anything appended between the read and the
	// write would be lost. Sessions started before the rename still name the
	// old directory in their environment, so check both.
	if err := guardInUse(p.Name, []string{p.Dir, l.ConfigDir(oldName), l.DesktopData(p.Name)}, *dry, *force); err != nil {
		return err
	}

	b := rewrite.Builder{
		OldDir: l.ConfigDir(oldName), NewDir: p.Dir,
		OldDesktop: l.DesktopData(oldName), NewDesktop: l.DesktopData(p.Name),
		OldVSCode: l.VSCodeData(oldName), NewVSCode: l.VSCodeData(p.Name),
		AlreadyMoved:   true,
		Deep:           true,
		IncludeHistory: true,
	}
	plan, err := b.Build()
	if err != nil {
		return err
	}
	if len(plan.Edits) == 0 && len(plan.Binary) == 0 {
		fmt.Printf("%s: no references to '%s' left\n", p.Name, oldName)
		return nil
	}

	fmt.Printf("rewrite '%s' -> '%s' inside %s\n\n", oldName, p.Name, short(l, p.Dir))
	byKind := map[string]struct {
		files, hits int
	}{}
	for _, e := range plan.Edits {
		v := byKind[e.Kind.String()]
		v.files, v.hits = v.files+1, v.hits+e.Hits
		byKind[e.Kind.String()] = v
	}
	for _, kind := range []string{"config", "session", "history", "volatile"} {
		if v, ok := byKind[kind]; ok {
			fmt.Printf("  %-9s %4d file(s)  %5d reference(s)\n", kind, v.files, v.hits)
		}
	}
	if len(plan.Binary) > 0 {
		fmt.Printf("\n  skipping %d binary file(s) -- a shorter path would shift every byte\n", len(plan.Binary))
		for i, e := range plan.Binary {
			if i == 5 {
				fmt.Printf("    ... and %d more\n", len(plan.Binary)-5)
				break
			}
			fmt.Printf("    %s\n", short(l, e.Path))
		}
	}

	if *dry {
		fmt.Println("\ndry run -- nothing changed.")
		return nil
	}
	if err := plan.Apply(); err != nil {
		return err
	}
	fmt.Printf("\nrewrote %d file(s)\n", len(plan.Edits))
	return nil
}
