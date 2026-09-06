// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// Runtime glue for the transcendence commands: snapshot-store access and a
// shared output helper. Kept separate from bing_intel.go so the pure logic
// stays unit-testable without a database.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster/internal/snapshots"
)

// errRequiredFlag builds the same "required flag" error shape the generated
// endpoint commands use, so messaging is consistent across the CLI.
func errRequiredFlag(name string) error {
	return fmt.Errorf("required flag %q not set", name)
}

// openSnapshots opens the date-stamped snapshot store on the shared SQLite file.
func openSnapshots() (*snapshots.DB, error) {
	return snapshots.Open(defaultDBPath("bing-webmaster-pp-cli"))
}

// emitIntel writes result as JSON for agents/pipes (honoring --select/--compact)
// or invokes human() for an interactive terminal summary.
func emitIntel(cmd *cobra.Command, flags *rootFlags, result any, human func()) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return flags.printJSON(cmd, result)
	}
	human()
	return nil
}

// siteOrDryRun validates the --site flag. It returns (proceed, err): when site
// is empty under --dry-run it prints the dry-run note and returns (false, nil)
// so the caller returns cleanly; when empty without dry-run it returns an
// actionable error; otherwise (true, nil).
func siteOrDryRun(cmd *cobra.Command, flags *rootFlags, site, dryNote string) (bool, error) {
	if site == "" {
		if dryRunOK(flags) {
			cmd.Println(dryNote)
			return false, nil
		}
		return false, errRequiredFlag("site")
	}
	if dryRunOK(flags) {
		cmd.Println(dryNote)
		return false, nil
	}
	return true, nil
}
