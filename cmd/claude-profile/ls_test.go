package main

import "testing"

func TestBrowserName(t *testing.T) {
	cases := map[string]string{
		"":                                  "(default)",
		"   ":                               "(default)",
		"/home/me/.local/bin/open-chromium": "open-chromium",
		"firefox":                           "firefox",
		"/usr/bin/chromium --new-window":    "chromium",
	}
	for in, want := range cases {
		if got := browserName(in); got != want {
			t.Errorf("browserName(%q) = %q, want %q", in, got, want)
		}
	}
}
