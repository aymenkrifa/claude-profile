package main

import (
	"flag"
	"testing"
)

// Flags typed after the positional arguments must still take effect: the
// standard flag package stops at the first non-flag word, which once turned
// `rename work g4 --dry-run` into a real rename.
func TestParseFlagsAfterPositionals(t *testing.T) {
	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	dry := fs.Bool("dry-run", false, "")
	label := fs.String("label", "", "")

	if err := parseFlags(fs, []string{"work", "g4", "--dry-run", "--label", "G4 seat"}); err != nil {
		t.Fatal(err)
	}
	if !*dry {
		t.Error("--dry-run after positionals was ignored")
	}
	if *label != "G4 seat" {
		t.Errorf("label = %q, want %q", *label, "G4 seat")
	}
	if fs.Arg(0) != "work" || fs.Arg(1) != "g4" {
		t.Errorf("positionals = %v, want [work g4]", fs.Args())
	}
}

func TestParseFlagsForms(t *testing.T) {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	class := fs.String("class", "", "")
	noDesktop := fs.Bool("no-desktop", false, "")

	if err := parseFlags(fs, []string{"acme", "--class=work", "-no-desktop"}); err != nil {
		t.Fatal(err)
	}
	if *class != "work" || !*noDesktop {
		t.Errorf("class=%q no-desktop=%v", *class, *noDesktop)
	}
	if fs.Arg(0) != "acme" {
		t.Errorf("positional = %q", fs.Arg(0))
	}
}

// Everything after a bare -- is data, not flags.
func TestParseFlagsDoubleDash(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	dry := fs.Bool("dry-run", false, "")
	if err := parseFlags(fs, []string{"work", "--", "--dry-run", "-p"}); err != nil {
		t.Fatal(err)
	}
	if *dry {
		t.Error("--dry-run after -- should have stayed positional")
	}
	if got := fs.Args(); len(got) != 3 || got[1] != "--dry-run" {
		t.Errorf("args = %v", got)
	}
}
