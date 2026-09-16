package inuse

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// find asks lsof which of this user's processes hold a file open under the
// profile. One invocation lists every open file the user has and the prefixes
// are matched here, which is far quicker than lsof's own recursive +D walk
// over a profile that can run to gigabytes.
//
// Unlike the Linux implementation this reports no weak evidence: reading
// another process's environment on macOS needs sysctl(KERN_PROCARGS2) per pid,
// and it is only the writers that block a rename anyway.
func find(roots []string) ([]User, error) {
	bin, err := exec.LookPath("lsof")
	if err != nil {
		return nil, fmt.Errorf("lsof not found on PATH: cannot tell which processes hold the profile open")
	}
	cmd := exec.Command(bin, "-u", strconv.Itoa(os.Getuid()), "-n", "-P", "-F", "pcfn")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	// lsof exits non-zero when any single file cannot be stat'd, which is
	// routine and not a failure of the search. Only empty output is.
	runErr := cmd.Run()
	if stdout.Len() == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("lsof: %w: %s", runErr, bytes.TrimSpace(stderr.Bytes()))
		}
		return nil, nil
	}
	return parseLsof(&stdout, roots, os.Getpid()), nil
}
