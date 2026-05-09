package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/marginalrevolution/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
