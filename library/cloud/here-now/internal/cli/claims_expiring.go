// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: `claims expiring`. Lists unclaimed anonymous
// sites expiring within a window so you can claim them before they vanish.
// Header is a plain copyright line so regen-merge preserves this file.

package cli

import (
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/here-now/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelClaimsExpiringCmd(flags *rootFlags) *cobra.Command {
	var (
		flagWithin string
		flagDB     string
	)

	cmd := &cobra.Command{
		Use:   "expiring",
		Short: "List unclaimed anonymous sites expiring within a time window",
		Example: strings.Trim(`
  here-now-pp-cli claims expiring
  here-now-pp-cli claims expiring --within 2h
  here-now-pp-cli claims expiring --within 1d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			within := 6 * time.Hour
			if strings.TrimSpace(flagWithin) != "" {
				d, err := cliutil.ParseDurationLoose(flagWithin)
				if err != nil {
					return usageErr(err)
				}
				within = d
			}

			db, err := openHereNowStore(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			views, err := listExpiringClaims(db, within, time.Now())
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), views, flags)
		},
	}

	cmd.Flags().StringVar(&flagWithin, "within", "6h", "Window (e.g. 6h, 2h, 1d) for sites about to expire")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/here-now-pp-cli/data.db)")
	return cmd
}
