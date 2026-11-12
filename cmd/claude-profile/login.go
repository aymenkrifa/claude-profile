package main

// login: sign a profile in through a throwaway Chrome, so the browser you use
// every day stays signed in to its own account.
//
// The CLI opens its sign-in page with whatever $BROWSER names, spawning it
// with the URL as the only argument and waiting for it to exit. For one run,
// 'login' points $BROWSER at this binary and sets a marker, so the call comes
// back here as the opener. The opener makes an empty Chrome profile folder,
// hands the URL to a detached supervisor and exits at once; the supervisor
// runs Chrome on that folder and deletes the folder when the window closes.
// A Chrome on its own --user-data-dir is a separate instance with its own
// cookies, so nothing crosses over to the everyday browser.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"claude-profile/internal/fsx"
	"claude-profile/internal/paths"
	"claude-profile/internal/profile"
)

const (
	openerEnv  = "CLAUDE_PROFILE_OPENER"
	openerOpen = "throwaway"      // this process is $BROWSER: open the URL
	openerWait = "throwaway-wait" // the detached supervisor: run Chrome, clean up

	// Window class of the throwaway on X11, so it can be told apart from the
	// everyday Chrome in a window list.
	throwawayClass = "claude-profile-login"
)

func cmdLogin(l paths.Layout, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: claude-profile login <name>")
	}
	p, err := profile.Load(l, args[0])
	if err != nil {
		return err
	}
	if _, err := chromeBin(l); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating claude-profile itself: %w", err)
	}
	if b := p.Browser(); b != "" {
		fmt.Printf("note: %s sets its own browser (%s) in its settings; it may win over the throwaway one\n",
			p.Name, browserName(b))
	}
	fmt.Println("Run /login. The sign-in page opens in a throwaway Chrome window: no accounts,")
	fmt.Println("no extensions, deleted when you close it. Your everyday Chrome is not touched.")
	fmt.Println()
	return execCLIWith(p.Dir, nil, "BROWSER="+self, openerEnv+"="+openerOpen)
}

// isOpenerCall reports whether this process was started as the throwaway
// $BROWSER: the marker is set and the only argument is a web URL.
func isOpenerCall(marker string, args []string) bool {
	if marker != openerOpen || len(args) != 1 {
		return false
	}
	return strings.HasPrefix(args[0], "https://") || strings.HasPrefix(args[0], "http://")
}

// openThrowaway is the $BROWSER entry point. It must return quickly: the CLI
// waits on it, so Chrome's lifetime belongs to a detached supervisor.
func openThrowaway(l paths.Layout, url string) error {
	if _, err := chromeBin(l); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "claude-login-")
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		os.RemoveAll(dir)
		return err
	}
	cmd := exec.Command(self, dir, url)
	cmd.Env = append(withoutOpener(os.Environ()), openerEnv+"="+openerWait)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return err
	}
	return cmd.Process.Release()
}

// runThrowaway is the supervisor: Chrome on the empty folder, then the folder
// goes, whatever Chrome's exit status.
func runThrowaway(l paths.Layout, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("throwaway supervisor: want <dir> <url>")
	}
	dir, url := args[0], args[1]
	// Only ever delete a folder this command made.
	if !strings.HasPrefix(filepath.Base(dir), "claude-login-") {
		return fmt.Errorf("throwaway supervisor: refusing folder %s", dir)
	}
	defer os.RemoveAll(dir)
	bin, err := chromeBin(l)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, throwawayArgs(dir, url, l.Darwin())...)
	cmd.Env = withoutOpener(os.Environ())
	return cmd.Run()
}

func throwawayArgs(dir, url string, darwin bool) []string {
	args := []string{
		"--user-data-dir=" + dir,
		"--no-first-run",
		"--no-default-browser-check",
	}
	if !darwin {
		args = append(args, "--class="+throwawayClass)
	}
	return append(args, "--new-window", url)
}

// chromeBin finds Google Chrome, the only browser the throwaway is tested with.
func chromeBin(l paths.Layout) (string, error) {
	if l.Darwin() {
		for _, c := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			filepath.Join(l.Home, "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome"),
		} {
			if fsx.Exists(c) {
				return c, nil
			}
		}
		return "", fmt.Errorf("Google Chrome not found in /Applications or ~/Applications")
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable"} {
		if bin, err := exec.LookPath(name); err == nil {
			return bin, nil
		}
	}
	return "", fmt.Errorf("google-chrome not found on PATH")
}

func withoutOpener(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, openerEnv+"=") {
			out = append(out, kv)
		}
	}
	return out
}
