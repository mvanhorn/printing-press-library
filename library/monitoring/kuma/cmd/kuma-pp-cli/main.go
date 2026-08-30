// Command kuma-pp-cli is a Uptime Kuma v2 operator CLI for the Printing Press catalog.
package main

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/monitoring/kuma/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}
