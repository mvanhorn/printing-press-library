// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: `usage`. Local free-plan meter — site count and
// publish cadence from the local store, drive count and bytes from the live API
// when auth is available. Header is a plain copyright line so regen-merge
// preserves this file.

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelUsageCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Local rollup of site/drive/publish usage against free-plan ceilings",
		Long: strings.Trim(`
Local free-plan meter: site count (local mirror), recent publish cadence (local
publish log), and drive count + total bytes (live API; needs auth). Each is
shown as used / limit with a percentage and an over-limit flag. Free-plan
ceilings: 500 sites, 10 GiB drive bytes, 1 drive, 60 publishes/hour.
`, "\n"),
		Example: strings.Trim(`
  here-now-pp-cli usage
  here-now-pp-cli usage --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openHereNowStore(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			// A client is best-effort: if config/auth is unavailable, the
			// report still shows local site/publish stats and notes that
			// drive stats need auth.
			c, _ := flags.newClient()

			report, err := buildUsageReport(cmd.Context(), c, db, time.Now())
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/here-now-pp-cli/data.db)")
	return cmd
}
