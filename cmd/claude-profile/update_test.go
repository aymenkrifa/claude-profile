package main

import "testing"

func TestReleaseVersion(t *testing.T) {
	for v, want := range map[string]bool{
		"v1.0.2":            true,
		"v10.20.300":        true,
		"v1.0.2-1-gfcf296c": false, // git describe past a tag: a source build
		"v1.0.2-dirty":      false,
		"dev":               false,
		"1.0.2":             false,
		"v1.0":              false,
	} {
		if got := releaseVersion.MatchString(v); got != want {
			t.Errorf("releaseVersion(%q) = %v, want %v", v, got, want)
		}
	}
}
