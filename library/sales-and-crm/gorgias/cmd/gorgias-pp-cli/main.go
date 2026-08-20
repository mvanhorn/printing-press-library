// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/client"
)

func main() {
	// Tag outbound API calls with the actual build version so Gorgias's logs
	// can attribute traffic to a specific release. cli.Version() is the value
	// goreleaser's -X ldflag wrote into the cli package at build time.
	client.SetVersion(cli.Version())
	// cli.Execute() is the single emission point for errors. We don't print
	// here — that would produce the duplicate-error-line class of bug. The
	// returned error only drives the exit code.
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
