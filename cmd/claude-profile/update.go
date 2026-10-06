package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const (
	releasesURL  = "https://github.com/aymenkrifa/claude-profile/releases/latest"
	installerURL = "https://raw.githubusercontent.com/aymenkrifa/claude-profile/main/install.sh"
)

// A release build is stamped with its tag; anything else (v1.0.2-1-gabc,
// -dirty, dev) was built from a clone, and replacing it with a release would
// quietly throw away whatever that clone had.
var releaseVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// cmdUpdate brings this install up to a release by running the published
// installer against the prefix this binary lives in. The installer stays the
// one place that downloads, verifies and lays files out, so an update installs
// exactly what a fresh install would.
func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	check := fs.Bool("check", false, "only report whether a newer release exists")
	to := fs.String("version", "", "install this release instead of the latest (vX.Y.Z)")
	force := fs.Bool("force", false, "reinstall even when current, or replace a build from source")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: claude-profile update [--check] [--version vX.Y.Z] [--force]")
		fs.PrintDefaults()
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	target := *to
	if target == "" {
		latest, err := latestRelease()
		if err != nil {
			return err
		}
		target = latest
	}
	if !releaseVersion.MatchString(target) {
		return fmt.Errorf("%q is not a release version (vX.Y.Z)", target)
	}

	if target == version && !*force {
		fmt.Printf("claude-profile %s is up to date\n", version)
		return nil
	}
	fromSource := !releaseVersion.MatchString(version)
	if *check {
		if fromSource {
			fmt.Printf("claude-profile %s was built from source; the latest release is %s\n", version, target)
		} else {
			fmt.Printf("claude-profile %s is installed; %s is available -- run 'claude-profile update'\n", version, target)
		}
		return nil
	}
	if fromSource && !*force {
		return fmt.Errorf(`this claude-profile (%s) was built from source.
Update it from the clone with 'git pull && make install', or pass --force to
replace it with the %s release`, version, target)
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	// <prefix>/bin/claude-profile: the same layout ShellInit looks in.
	prefix := filepath.Dir(filepath.Dir(exe))

	script, err := installer()
	if err != nil {
		return fmt.Errorf("could not fetch the installer: %w", err)
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		return err
	}
	fmt.Printf("updating claude-profile %s -> %s in %s\n", version, target, prefix)
	// Exec rather than run: the installer replaces this very binary, and this
	// process should be gone by then. The script goes in as -c so that nothing
	// is left behind in a temp directory.
	return syscall.Exec(sh, []string{"sh", "-c", script, "install.sh",
		"--prefix", prefix, "--version", target, "--update"}, os.Environ())
}

// latestRelease reads the tag off GitHub's releases/latest redirect rather
// than the API, which rate-limits unauthenticated callers to 60 an hour.
func latestRelease() (string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Head(releasesURL)
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub: %w", err)
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	i := strings.LastIndex(loc, "/tag/")
	if i < 0 {
		return "", fmt.Errorf("could not work out the latest release (GitHub answered %s)", resp.Status)
	}
	return loc[i+len("/tag/"):], nil
}

// installer is the published install.sh, or CLAUDE_PROFILE_INSTALLER -- a URL
// or a local path -- so the end-to-end test can update against the checkout.
func installer() (string, error) {
	src := os.Getenv("CLAUDE_PROFILE_INSTALLER")
	if src == "" {
		return fetch(installerURL)
	}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return fetch(src)
	}
	b, err := os.ReadFile(src)
	return string(b), err
}

func fetch(url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}
