package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/cli"
)

func main() {
	if err := cli.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(cli.ExitCodeFor(err))
	}
}
