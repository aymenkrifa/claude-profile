package main

import (
	"fmt"
	"os"

	"claude-profile/internal/inuse"
)

// guardInUse stops a command that rewrites or moves a profile while something
// is still holding it open. Processes that merely inherited CLAUDE_CONFIG_DIR
// are reported but not blocking -- every tool launched from a Claude session
// carries that variable, and blocking on them would turn --force into a habit.
func guardInUse(name string, dirs []string, dry, force bool) error {
	users := inuse.Find(dirs...)
	if len(users) == 0 {
		return nil
	}
	writers := inuse.Writers(users)
	fmt.Fprintf(os.Stderr, "profile '%s' is referenced by %d process(es), %d holding it open:\n", name, len(users), writers)
	for _, u := range users {
		mark := " "
		if u.Writer {
			mark = "*"
		}
		fmt.Fprintf(os.Stderr, "  %s pid %-7d %-16s (%s)\n", mark, u.PID, u.Name, u.How)
	}
	fmt.Fprintln(os.Stderr)
	if writers > 0 && !dry && !force {
		return fmt.Errorf("close the processes marked * first, or pass --force")
	}
	return nil
}
