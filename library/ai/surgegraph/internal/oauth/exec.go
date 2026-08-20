package oauth

import (
	"os/exec"
)

// runOSExec runs a short-lived command, ignoring stdout/stderr. Used by
// OpenBrowser. Split out so a future test build can stub it.
func runOSExec(cmd string, args ...string) error {
	return exec.Command(cmd, args...).Start()
}
