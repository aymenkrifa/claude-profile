// Command claude-profile manages the Claude Code accounts on this machine.
//
// Each account lives in its own config directory, ~/.claude-<name>, marked as a
// profile by a profile.env file. The shell commands (claude-<name>, vs<name>),
// the per-account Claude Desktop instances and the session-sync helpers are all
// derived from that marker, so adding, renaming or removing an account is a
// single command and never means hand-editing dotfiles.
//
// Linux and macOS. The layout it manages -- a ~/.local/bin shim, an Electron
// user-data-dir, an application-menu entry -- has no counterpart elsewhere.
package main

import (
	"fmt"
	"os"

	"claude-profile/internal/paths"
)

const usageHead = `claude-profile -- manage the Claude Code accounts on this machine

  ls                          accounts, their class, who each is signed in as
                              and which browser it opens URLs with
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
  shell-init [shell]          print the line to add to the shell's startup file
  update [--check]            install the latest release over this one

Run 'claude-profile <command> -h' for that command's flags.

Layout, per account <name>:
`

func main() {
	l := paths.New()
	if len(os.Args) < 2 {
		if err := cmdLs(l, nil); err != nil {
			fail(err)
		}
		return
	}
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
	case "shell-init", "init":
		err = cmdShellInit(l, args)
	case "update", "upgrade":
		err = cmdUpdate(args)
	case "-h", "--help", "help":
		fmt.Print(usageHead, l.Summary())
	case "-v", "--version", "version":
		fmt.Println("claude-profile", version)
	default:
		fail(fmt.Errorf("unknown command %q (try: claude-profile --help)", os.Args[1]))
	}
	if err != nil {
		fail(err)
	}
}

// Stamped by the Makefile with the git tag; a bare go build is "dev".
var version = "dev"

func fail(err error) {
	fmt.Fprintln(os.Stderr, "claude-profile:", err)
	os.Exit(1)
}

// shellReady reports whether the zsh integration is sourced in this shell.
// The commands that would otherwise tell someone to run 'exec zsh' and use a
// claude-<name> function check this first: without the integration those
// functions do not exist, and printing the instruction anyway is how a tool
// ends up looking broken on a machine that is set up perfectly well.
func shellReady() bool { return os.Getenv("CLAUDE_PROFILE_SHELL") != "" }
