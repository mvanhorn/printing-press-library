package cli

import (
	"os/exec"
	"runtime"
)

// runOpen invokes the platform's URI opener.
func runOpen(uri string) error {
	var bin string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		bin = "open"
		args = []string{uri}
	case "windows":
		bin = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", uri}
	default:
		bin = "xdg-open"
		args = []string{uri}
	}
	return exec.Command(bin, args...).Start()
}
