// Licensed under Apache-2.0. See LICENSE.

package main

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
