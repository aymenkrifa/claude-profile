package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"claude-profile/internal/desktop"
	"claude-profile/internal/paths"
	"claude-profile/internal/profile"
)

func cmdLs(l paths.Layout, args []string) error {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	quiet := fs.Bool("q", false, "print only profile names, one per line")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	profiles, err := profile.Discover(l)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		fmt.Println("no accounts yet -- claude-profile add <name>")
		return nil
	}
	if *quiet {
		for _, p := range profiles {
			fmt.Println(p.Name)
		}
		return nil
	}

	active := os.Getenv("CLAUDE_CONFIG_DIR")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROFILE\tCLASS\tACCOUNT\tBROWSER\tNOTES")
	for _, p := range profiles {
		acct := "(not signed in)"
		if a := p.Account(); a.SignedIn {
			acct = a.Email
			if a.Org != "" && !strings.Contains(a.Org, "@") {
				acct += "  [" + a.Org + "]"
			}
		}
		var notes []string
		if p.Dir == active {
			notes = append(notes, "active here")
		}
		if desktop.Installed(l, p.Name) {
			notes = append(notes, "desktop")
		}
		if p.Label != "" {
			notes = append(notes, p.Label)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", p.Name, p.Class, acct, browserName(p.Browser()), strings.Join(notes, ", "))
	}
	return w.Flush()
}

// browserName reduces an env.BROWSER value to what fits a column: the command's
// own name, without its directory or any arguments.
func browserName(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return "(default)"
	}
	return filepath.Base(fields[0])
}
