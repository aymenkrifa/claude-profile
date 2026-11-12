package main

import (
	"reflect"
	"testing"
)

func TestThrowawayArgs(t *testing.T) {
	got := throwawayArgs("/tmp/x", "https://claude.ai/oauth?a=1", false)
	want := []string{
		"--user-data-dir=/tmp/x",
		"--no-first-run",
		"--no-default-browser-check",
		"--class=" + throwawayClass,
		"--new-window",
		"https://claude.ai/oauth?a=1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("linux args = %q, want %q", got, want)
	}
	// --class is an X11 window class; macOS gets no such flag.
	for _, a := range throwawayArgs("/tmp/x", "https://u", true) {
		if a == "--class="+throwawayClass {
			t.Errorf("macOS args carry %q", a)
		}
	}
}

func TestOpenerCall(t *testing.T) {
	cases := []struct {
		marker string
		args   []string
		want   bool
	}{
		{openerOpen, []string{"https://claude.ai/x"}, true},
		{openerOpen, []string{"http://localhost:1234/cb"}, true},
		{"", []string{"https://claude.ai/x"}, false},    // no marker: an ordinary call
		{openerOpen, []string{"ls"}, false},             // a subcommand, not a URL
		{openerOpen, []string{"https://a", "b"}, false}, // a URL is the only argument
		{openerOpen, nil, false},
	}
	for _, c := range cases {
		if got := isOpenerCall(c.marker, c.args); got != c.want {
			t.Errorf("isOpenerCall(%q, %q) = %v, want %v", c.marker, c.args, got, c.want)
		}
	}
}
