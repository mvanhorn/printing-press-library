// PATCH(oauth-login): cross-platform browser open helper for the auth login
// flow. WSL2 is the realistic primary target for this CLI (per Matt's setup)
// and `xdg-open` is unreliable there — try `wslview` first when WSL is
// detected, then fall back to standard openers. On total failure callers
// must print the URL plainly and continue waiting on the loopback.

package cli

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// openBrowser attempts to launch the user's default browser at url. Returns
// nil on success, an error otherwise. Callers must handle the error path by
// printing the URL for manual copy/paste — the loopback listener is still
// armed and a hand-pasted browser hit will work the same.
func openBrowser(url string) error {
	for _, cmd := range browserCommands(url) {
		if err := exec.Command(cmd[0], cmd[1:]...).Start(); err == nil {
			return nil
		}
	}
	return errBrowserUnavailable
}

// errBrowserUnavailable is returned when no browser launcher succeeded. It is
// a sentinel so the login command can downgrade to "print URL" mode without
// any extra type assertions.
var errBrowserUnavailable = &browserErr{}

type browserErr struct{}

func (*browserErr) Error() string {
	return "no usable browser launcher found"
}

// browserCommands returns ordered fallbacks for the current platform. We try
// each in order until one succeeds. The ordering puts WSL-specific launchers
// first when WSL is detected because `xdg-open` on WSL often "succeeds" with
// nothing visible to the user.
func browserCommands(url string) [][]string {
	switch runtime.GOOS {
	case "darwin":
		return [][]string{{"open", url}}
	case "windows":
		return [][]string{{"cmd", "/c", "start", url}}
	}
	// Linux / WSL — order matters.
	cmds := [][]string{}
	if isWSL() {
		cmds = append(cmds, []string{"wslview", url})
	}
	cmds = append(cmds, []string{"xdg-open", url})
	cmds = append(cmds, []string{"sensible-browser", url})
	cmds = append(cmds, []string{"x-www-browser", url})
	cmds = append(cmds, []string{"www-browser", url})
	return cmds
}

// isWSL returns true when running under Windows Subsystem for Linux. The
// canonical detection reads /proc/version for the literal "microsoft" string
// (case-insensitive); Microsoft also exports WSL_DISTRO_NAME which is a more
// reliable signal when present.
func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}
