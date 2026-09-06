package main

import "flag"

// parseFlags parses args allowing flags and positional arguments to be
// interleaved.
//
// The standard flag package stops at the first non-flag word, which silently
// turns `rename work g4 --dry-run` into a rename with three positionals and no
// dry run -- exactly the shape people type. This permutes the flags to the
// front first. Everything after a bare "--" is left positional.
func parseFlags(fs *flag.FlagSet, args []string) error {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		if takesValue(fs, a) && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return fs.Parse(append(flags, positional...))
}

// takesValue reports whether a flag consumes the following word: true for
// everything except booleans and the already-joined --name=value form.
func takesValue(fs *flag.FlagSet, arg string) bool {
	name := arg[1:]
	if len(name) > 0 && name[0] == '-' {
		name = name[1:]
	}
	for i := 0; i < len(name); i++ {
		if name[i] == '=' {
			return false
		}
	}
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !ok || !b.IsBoolFlag()
}
