// Copyright 2026 jrimmer. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/forgejo/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
