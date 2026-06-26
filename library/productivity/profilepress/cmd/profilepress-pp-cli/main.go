package main

import (
	"os"

	"profilepress-pp-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		_, _ = os.Stderr.WriteString("error: " + err.Error() + "\n")
		os.Exit(cli.ExitCode(err))
	}
}
