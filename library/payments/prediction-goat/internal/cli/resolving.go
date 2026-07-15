// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newResolvingCmd(flags *rootFlags) *cobra.Command {
	var week, month bool
	var days, limit int
	var dbPath string
	var vf venueFlags
	cmd := &cobra.Command{
		Use:   "resolving",
		Short: "Markets resolving in a selected window",
		Example: `  prediction-goat-pp-cli resolving --week --json
  prediction-goat-pp-cli resolving --days 14 --kalshi`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			venue, err := resolveVenue(vf)
			if err != nil {
				return err
			}
			if week && month {
				return usageErr(fmt.Errorf("resolving: --week and --month are mutually exclusive (also use --days N to override)"))
			}
			windowDays := 7
			if month {
				windowDays = 30
			}
			if days > 0 {
				windowDays = days
			}
			now := time.Now().UTC()
			items, err := runMarketScreen(cmd, "resolving", dbPath, venue, limit, 0, now.Format(time.RFC3339), now.AddDate(0, 0, windowDays).Format(time.RFC3339))
			if err != nil {
				return err
			}
			outcome := refreshMarketScreenItems(cmd.Context(), nil, items)
			meta := buildFreshnessMeta(outcome, indexSyncedAtFromPath(cmd.Context(), dbPath))
			if renderErr := renderTrending(cmd, flags, trendingResult{Items: items, Meta: meta}); renderErr != nil {
				return renderErr
			}
			if len(items) == 0 {
				if hint := emptyStoreHint(cmd, dbPath, "resolving", venue); hint != nil {
					return hint
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&week, "week", false, "Resolve within 7 days")
	cmd.Flags().BoolVar(&month, "month", false, "Resolve within 30 days")
	cmd.Flags().IntVar(&days, "days", 0, "Resolve within N days (overrides --week/--month)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	addVenueFlags(cmd, &vf)
	return cmd
}
