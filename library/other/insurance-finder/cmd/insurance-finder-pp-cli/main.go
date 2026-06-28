// Command insurance-finder-pp-cli is a guided assistant that helps a US small
// business find and apply for commercial insurance (General Liability first).
// It interviews the business, saves a reusable profile, and produces a ranked
// provider shortlist with paste-ready answer sheets and manual-action
// checklists. It guides the user through their own browser and never fills,
// submits, or pays on their behalf.
package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/insurance-finder/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(cli.ExitCode(err))
	}
}
