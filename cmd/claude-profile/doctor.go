package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"claude-profile/internal/fsx"
	"claude-profile/internal/paths"
	"claude-profile/internal/profile"
)

// stalePathRE matches an absolute reference to a profile-owned directory, so a
// reference left behind by a partial rename can be told from a live one. It is
// anchored on the real home directory: without that it also matches relative
// fragments like ../../.claude-work quoted inside a transcript.
func stalePathRE(home string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(home) + `/(?:\.claude-|\.vscode-|\.config/Claude-)[a-zA-Z0-9][a-zA-Z0-9-]*`)
}

// doctorFiles are the live-config files worth checking on every run; --deep
// widens the search to the whole profile.
var doctorFiles = []string{
	".claude.json", "settings.json", "settings.local.json", "statusline-command.sh",
	"plugins/config.json", "plugins/installed_plugins.json", "plugins/known_marketplaces.json",
}

func cmdDoctor(l paths.Layout, args []string) error {
	fsFlags := flag.NewFlagSet("doctor", flag.ExitOnError)
	deep := fsFlags.Bool("deep", false, "search the whole profile, not just live config (slow: gigabytes)")
	if err := parseFlags(fsFlags, args); err != nil {
		return err
	}

	var profiles []profile.Profile
	if name := fsFlags.Arg(0); name != "" {
		p, err := profile.Load(l, name)
		if err != nil {
			return err
		}
		profiles = []profile.Profile{*p}
	} else {
		var err error
		if profiles, err = profile.Discover(l); err != nil {
			return err
		}
	}

	re := stalePathRE(l.Home)
	problems := 0
	for _, p := range profiles {
		// Live config is actionable: something the CLI reads points at nothing.
		// A hit anywhere else is a transcript or a cache recording a path that
		// used to exist, which is a fact about the past, not a fault.
		live := map[string][]string{}
		other := map[string]int{}
		scan := func(path string, actionable bool) {
			data, err := os.ReadFile(path)
			if err != nil {
				return
			}
			for _, m := range re.FindAllString(string(data), -1) {
				if fsx.Exists(m) {
					continue // points at something real
				}
				if !actionable {
					other[m]++
					continue
				}
				if !contains(live[m], short(l, path)) {
					live[m] = append(live[m], short(l, path))
				}
			}
		}
		isLive := func(rel string) bool {
			return contains(doctorFiles, rel)
		}

		if *deep {
			_ = filepath.WalkDir(p.Dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil //nolint:nilerr
				}
				rel, _ := filepath.Rel(p.Dir, path)
				scan(path, isLive(filepath.ToSlash(rel)))
				return nil
			})
		} else {
			for _, rel := range doctorFiles {
				scan(filepath.Join(p.Dir, rel), true)
			}
		}
		sessions, _ := filepath.Glob(filepath.Join(l.DesktopData(p.Name), "claude-code-sessions", "*", "*", "*.json"))
		for _, f := range sessions {
			scan(f, true)
		}

		if len(live) == 0 {
			fmt.Printf("%-10s ok", p.Name)
			if n := len(other); n > 0 {
				fmt.Printf("  (%d old path(s) named in history or caches -- a record, not a fault;\n"+
					"            'claude-profile repath %s --from <old>' rewrites them)", n, p.Name)
			}
			fmt.Println()
			continue
		}
		problems += len(live)
		fmt.Printf("%-10s %d stale path(s) in live config:\n", p.Name, len(live))
		keys := make([]string, 0, len(live))
		for k := range live {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s\n    named by: %s\n", k, strings.Join(live[k], ", "))
		}
	}
	if problems > 0 {
		fmt.Println("\na stale path points at a directory that no longer exists -- usually a")
		fmt.Println("rename done by hand. Fix the file, or re-point it with claude-profile rename.")
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
