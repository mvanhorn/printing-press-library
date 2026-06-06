// Copyright 2026 John Fiedler and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/foxnews/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(cli.ExitCode(err))
	}
}
