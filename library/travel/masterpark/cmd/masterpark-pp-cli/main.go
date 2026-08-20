// Command masterpark-pp-cli is an unofficial CLI over the MasterPark
// (netParkV2) reservation API.
package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
