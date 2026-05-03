// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.
// Scaffolded for CLI Printing Press. Endpoint implementations are intentionally deferred.

package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(cli.ExitCode(err))
	}
}
