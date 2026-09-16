//go:build !linux && !darwin

package inuse

import (
	"fmt"
	"runtime"
)

// find has no implementation on this platform, so the answer is unknown and
// the callers refuse rather than proceed. claude-profile targets Linux and
// macOS; the layout it manages (a ~/.local/bin shim, an Electron
// user-data-dir) does not describe anything else.
func find(roots []string) ([]User, error) {
	return nil, fmt.Errorf("unsupported platform %s: cannot tell which processes hold a profile open", runtime.GOOS)
}
