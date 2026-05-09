package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/commands"
)

var version = "0.1.0"

func main() {
	commands.RootCmd.Version = version
	if err := commands.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
