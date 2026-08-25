// grants-pp-cli — open research grants, keyless.
// Sources: Grants.gov (open opportunities), NIH RePORTER + NSF (awarded grants).
package main

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/health/grants/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
