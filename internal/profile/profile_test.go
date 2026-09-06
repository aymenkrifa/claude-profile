package profile

import (
	"os"
	"path/filepath"
	"testing"

	"claude-profile/internal/paths"
)

func layout(t *testing.T) paths.Layout {
	t.Helper()
	return paths.Layout{Home: t.TempDir()}
}

func TestValidateName(t *testing.T) {
	ok := []string{"work", "g4", "acme-2", "a"}
	bad := []string{"", "Work", "-lead", "with space", "under_score", "café"}
	for _, n := range ok {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", n, err)
		}
	}
	for _, n := range bad {
		if err := ValidateName(n); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", n)
		}
	}
}

func TestMarkerRoundTrip(t *testing.T) {
	l := layout(t)
	want := &Profile{Name: "g4", Dir: l.ConfigDir("g4"), Class: "work", Label: "Acme — G4 seat"}
	if err := want.WriteMarker(); err != nil {
		t.Fatal(err)
	}
	got, err := Load(l, "g4")
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != want.Class || got.Label != want.Label || got.Name != want.Name {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

// A directory only counts as a profile once it carries the marker: this is what
// keeps ~/.claude-code-best-practices from being listed as an account.
func TestDiscoverRequiresMarker(t *testing.T) {
	l := layout(t)
	if err := os.MkdirAll(filepath.Join(l.Home, ".claude-code-best-practices"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(l.Home, ".claude-halfsetup"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"work", "personal"} {
		p := &Profile{Name: name, Dir: l.ConfigDir(name), Class: name}
		if err := p.WriteMarker(); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Discover(l)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "personal" || got[1].Name != "work" {
		t.Fatalf("Discover() = %v, want [personal work] sorted", names(got))
	}
}

func TestClassDefaultsToName(t *testing.T) {
	l := layout(t)
	dir := l.ConfigDir("solo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, MarkerFile), []byte("# no class here\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(l, "solo")
	if err != nil {
		t.Fatal(err)
	}
	if p.Class != "solo" {
		t.Errorf("Class = %q, want %q", p.Class, "solo")
	}
}

func TestAccount(t *testing.T) {
	l := layout(t)
	p := &Profile{Name: "work", Dir: l.ConfigDir("work"), Class: "work"}
	if err := p.WriteMarker(); err != nil {
		t.Fatal(err)
	}
	if a := p.Account(); a.SignedIn {
		t.Error("Account() reported signed in with no .claude.json")
	}
	body := `{"oauthAccount":{"emailAddress":"dev@example.com","organizationName":"Acme",
	  "accountUuid":"acct-1","organizationUuid":"org-1"},"projects":{}}`
	if err := os.WriteFile(filepath.Join(p.Dir, ".claude.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	a := p.Account()
	if !a.SignedIn || a.Email != "dev@example.com" || a.Org != "Acme" || a.UUID != "acct-1" || a.OrgUUID != "org-1" {
		t.Errorf("Account() = %+v", a)
	}
}

func TestTitle(t *testing.T) {
	p := &Profile{Name: "acme"}
	if p.Title() != "Acme" {
		t.Errorf("Title() = %q", p.Title())
	}
}

func names(ps []Profile) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}
