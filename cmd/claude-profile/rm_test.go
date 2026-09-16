package main

import (
	"strings"
	"testing"
)

// The confirmation is the only thing between --purge and an irreversible
// delete, so every way of not answering it has to come out as an abort.
func TestConfirmPurge(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"the name, with a newline", "g4\n", false},
		{"the name, padded", "  g4  \n", false},
		{"the name, no trailing newline", "g4", false},
		{"a different name", "personal\n", true},
		{"empty answer", "\n", true},
		{"closed stdin", "", true},
		{"just whitespace", "   \n", true},
		{"a prefix of the name", "g\n", true},
		{"the name plus more", "g4x\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := confirmPurge(strings.NewReader(c.input), "g4", []string{"/nonexistent"})
			if (err != nil) != c.wantErr {
				t.Errorf("confirmPurge(%q) error = %v, want error: %v", c.input, err, c.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "nothing was changed") {
				t.Errorf("abort message should say nothing changed, got %q", err)
			}
		})
	}
}
