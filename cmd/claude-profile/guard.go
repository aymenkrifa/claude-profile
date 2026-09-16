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
//
// A profile that cannot be inspected at all blocks too. The check is the only
// thing standing between a rename and a live session's lost writes, so on a
// platform or a kernel where the answer is unavailable it has to fail closed:
// passing silently would leave the guard looking present while doing nothing.
func guardInUse(name string, dirs []string, dry, force bool) error {
	users, err := inuse.Find(dirs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot tell whether '%s' is in use: %v\n\n", name, err)
		if dry || force {
			return nil
		}
		return fmt.Errorf("close any session using this profile, then pass --force")
	}
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
