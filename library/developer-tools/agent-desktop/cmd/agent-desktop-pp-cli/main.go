package main

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/agent-desktop/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
