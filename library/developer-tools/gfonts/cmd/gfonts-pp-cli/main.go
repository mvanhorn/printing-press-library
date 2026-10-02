package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/gfonts"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/gfonts/internal/cli"
)

// Release builds may set this through -ldflags -X main.version. Source builds
// read the version from the embedded release ledger.
var version string

func cliVersion() string {
	if version != "" {
		return version
	}
	return gfonts.ReleaseVersion()
}

func main() {
	currentVersion := cliVersion()
	// The original CLI accepted --version anywhere, including after a command.
	for _, arg := range os.Args[1:] {
		if arg == "--version" {
			fmt.Printf("gfonts %s\n", currentVersion)
			return
		}
	}
	os.Exit(cli.Execute(currentVersion))
}
