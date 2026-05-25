package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitCode(err))
	}
}
