package main

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		_, _ = os.Stderr.WriteString("error: " + err.Error() + "\n")
		os.Exit(cli.ExitCode(err))
	}
}
