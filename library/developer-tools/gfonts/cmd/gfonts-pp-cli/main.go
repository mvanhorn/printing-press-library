package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/gfonts/internal/cli"
)

// The source-build default stays at the public CLI's existing value.
// Release builds set this through -ldflags -X main.version.
var version = "1.0.0"

func main() {
	// The original CLI accepted --version anywhere, including after a command.
	for _, arg := range os.Args[1:] {
		if arg == "--version" {
			fmt.Printf("gfonts %s\n", version)
			return
		}
	}
	os.Exit(cli.Execute(version))
}
