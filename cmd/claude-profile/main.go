// Command claude-profile manages the Claude Code accounts on this machine.
//
// Each account lives in its own config directory, ~/.claude-<name>, marked as a
// profile by a profile.env file. The shell commands (claude-<name>, vs<name>),
// the per-account Claude Desktop instances and the session-sync helpers are all
// derived from that marker, so adding, renaming or removing an account is a
// single command and never means hand-editing dotfiles.
package main

import (
	"fmt"
	"os"

	"claude-profile/internal/paths"
)

const usage = `claude-profile -- manage the Claude Code accounts on this machine

  ls                          accounts, their class, and who each is signed in as
  add <name> [flags]          create (or adopt) an account
  set <name> [flags]          change an account's class or label
  rename <old> <new> [flags]  rename everywhere: config dir, Desktop instance,
                              VS Code data, MCP paths, session records
  repath <name> --from <old>  finish a rename: rewrite an old name still written
                              inside transcripts, logs and job scratch
  rm <name> [--purge]         unregister an account; --purge deletes its data
  run <name> [-- args...]     run the CLI against one account
  desktop <name> [args...]    launch that account's Claude Desktop instance
  path <name>                 print its config directory
  doctor [name] [--deep]      report stale references to a profile path

Run 'claude-profile <command> -h' for that command's flags.

Layout, per account <name>:
  ~/.claude-<name>            CLAUDE_CONFIG_DIR: credentials, settings, projects
  ~/.config/Claude-<name>     Claude Desktop user-data-dir
  ~/.vscode-<name>            VS Code user-data-dir (vs<name> shell function)
  ~/.local/bin/claude-desktop-<name>                    launcher
  ~/.local/share/applications/claude-desktop-<name>.desktop
`

func main() {
	if len(os.Args) < 2 {
		if err := cmdLs(paths.New(), nil); err != nil {
			fail(err)
		}
		return
	}
	l := paths.New()
	args := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "ls", "list":
		err = cmdLs(l, args)
	case "add", "new":
		err = cmdAdd(l, args)
	case "set", "modify", "edit":
		err = cmdSet(l, args)
	case "rename", "mv":
		err = cmdRename(l, args)
	case "repath":
		err = cmdRepath(l, args)
	case "rm", "remove", "delete":
		err = cmdRm(l, args)
	case "run", "exec":
		err = cmdRun(l, args)
	case "desktop":
		err = cmdDesktop(l, args)
	case "path":
		err = cmdPath(l, args)
	case "doctor", "check":
		err = cmdDoctor(l, args)
	case "-h", "--help", "help":
		fmt.Print(usage)
	case "-v", "--version", "version":
		fmt.Println("claude-profile", version)
	default:
		fail(fmt.Errorf("unknown command %q (try: claude-profile --help)", os.Args[1]))
	}
	if err != nil {
		fail(err)
	}
}

var version = "1.0.0"

func fail(err error) {
	fmt.Fprintln(os.Stderr, "claude-profile:", err)
	os.Exit(1)
}
