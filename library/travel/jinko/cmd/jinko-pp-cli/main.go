package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cmd.ExitCode(err))
	}
}
