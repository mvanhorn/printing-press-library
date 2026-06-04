// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: `sites stale`. Lists sites not updated in N
// days from the local mirror so you can reclaim free-plan slots by deleting
// dead ones. Read-only: it only lists; deletion is a separate manual step.
// Header is a plain copyright line so regen-merge preserves this file.

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelSitesStaleCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDays int
		flagDB   string
	)

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List sites not updated in N days from the local mirror",
		Long: strings.Trim(`
Lists sites whose last update (from the local mirror; run 'sync' first) is older
than --days, oldest first, so you can reclaim free-plan slots by deleting the
dead ones. This command only lists — deletion is a separate manual step via
'publish delete-site'.
`, "\n"),
		Example: strings.Trim(`
  here-now-pp-cli sites stale
  here-now-pp-cli sites stale --days 30 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			days := flagDays
			if days <= 0 {
				days = 30
			}

			db, err := openHereNowStore(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			if hintIfUnsynced(cmd, db, "publishes") {
				// fall through: still print the (empty) list
			} else {
				hintIfStale(cmd, db, "publishes", flags.maxAge)
			}

			sites, err := listStaleSites(db, days, time.Now())
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), sites, flags)
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 30, "Age threshold in days; sites older than this are listed")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/here-now-pp-cli/data.db)")
	return cmd
}
