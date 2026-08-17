// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/us-data/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitCode(err))
	}
}
