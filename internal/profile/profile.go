// Package profile discovers and edits the Claude Code accounts on this machine.
//
// A directory ~/.claude-<name> is a profile only if it contains a profile.env
// marker. That marker is deliberate: it is what stops unrelated dotfile
// directories (~/.claude-code-best-practices, say) from being mistaken for an
// account, and it is cheap enough that ~/.zshrc reads it with shell builtins on
// every shell start.
package profile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"claude-profile/internal/fsx"
	"claude-profile/internal/paths"
)

// MarkerFile names a directory as a profile and carries its metadata.
const MarkerFile = "profile.env"

// Profile is one Claude account's local footprint.
type Profile struct {
	Name  string // "work"
	Dir   string // ~/.claude-work
	Class string // work | personal -- which terminals may reach it
	Label string // free text, shown in ls and the Desktop entry
}

// Account is the identity a profile is currently signed into, read from the
// oauthAccount block of .claude.json. Zero value means "not signed in".
type Account struct {
	Email    string
	Org      string
	OrgUUID  string
	UUID     string
	SignedIn bool
}

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidateName rejects names that would not survive being pasted into a
// directory name, a shell function name and a .desktop filename alike.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: use lowercase letters, digits and dashes", name)
	}
	return nil
}

// Discover returns every registered profile, sorted by name.
func Discover(l paths.Layout) ([]Profile, error) {
	matches, err := filepath.Glob(l.ProfileGlob())
	if err != nil {
		return nil, err
	}
	var out []Profile
	for _, dir := range matches {
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			continue
		}
		if !fsx.Exists(filepath.Join(dir, MarkerFile)) {
			continue
		}
		p, err := fromDir(l, dir)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Load returns one profile by name.
func Load(l paths.Layout, name string) (*Profile, error) {
	dir := l.ConfigDir(name)
	if !fsx.Exists(filepath.Join(dir, MarkerFile)) {
		return nil, fmt.Errorf("no such profile %q (looked for %s)", name, filepath.Join(dir, MarkerFile))
	}
	return fromDir(l, dir)
}

// Exists reports whether a registered profile with this name is present.
func Exists(l paths.Layout, name string) bool {
	return fsx.Exists(filepath.Join(l.ConfigDir(name), MarkerFile))
}

func fromDir(l paths.Layout, dir string) (*Profile, error) {
	name := strings.TrimPrefix(filepath.Base(dir), ".claude-")
	p := &Profile{Name: name, Dir: dir, Class: name}
	f, err := os.Open(filepath.Join(dir, MarkerFile))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "class":
			if v := strings.TrimSpace(val); v != "" {
				p.Class = v
			}
		case "label":
			p.Label = strings.TrimSpace(val)
		}
	}
	return p, sc.Err()
}

// WriteMarker persists the profile's metadata, creating the directory if needed.
func (p *Profile) WriteMarker() error {
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		return err
	}
	body := fmt.Sprintf(`# Marks this directory as a Claude Code profile. Read by ~/.zshrc and by
# claude-profile; edit with 'claude-profile set %s'.
class=%s
label=%s
`, p.Name, p.Class, p.Label)
	return fsx.WriteAtomic(filepath.Join(p.Dir, MarkerFile), []byte(body))
}

// Account reads which account this profile is signed into. A profile that has
// never been logged into simply reports SignedIn false.
func (p *Profile) Account() Account {
	var doc struct {
		OAuth struct {
			Email   string `json:"emailAddress"`
			Org     string `json:"organizationName"`
			OrgUUID string `json:"organizationUuid"`
			UUID    string `json:"accountUuid"`
		} `json:"oauthAccount"`
	}
	data, err := os.ReadFile(filepath.Join(p.Dir, ".claude.json"))
	if err != nil {
		return Account{}
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Account{}
	}
	if doc.OAuth.Email == "" {
		return Account{}
	}
	return Account{
		Email:    doc.OAuth.Email,
		Org:      doc.OAuth.Org,
		OrgUUID:  doc.OAuth.OrgUUID,
		UUID:     doc.OAuth.UUID,
		SignedIn: true,
	}
}

// Title is the capitalised form used in menus and window classes.
func (p *Profile) Title() string {
	if p.Name == "" {
		return ""
	}
	return strings.ToUpper(p.Name[:1]) + p.Name[1:]
}
