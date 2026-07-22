// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newNewCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	var dbPath string
	var vf venueFlags
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Markets created in the last N days across Polymarket and Kalshi",
		Example: `  prediction-goat-pp-cli new --days 7 --json
  prediction-goat-pp-cli new --days 1 --polymarket`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			venue, err := resolveVenue(vf)
			if err != nil {
				return err
			}
			cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)
			items, err := runMarketScreen(cmd, "new", dbPath, venue, limit, 0, cutoff, "")
			if err != nil {
				return err
			}
			outcome := refreshMarketScreenItems(cmd.Context(), nil, items)
			meta := buildFreshnessMeta(outcome, indexSyncedAtFromPath(cmd.Context(), dbPath))
			if renderErr := renderTrending(cmd, flags, trendingResult{Items: items, Meta: meta}); renderErr != nil {
				return renderErr
			}
			if len(items) == 0 {
				if hint := emptyStoreHint(cmd, dbPath, "new", venue); hint != nil {
					return hint
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Created within the last N days")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	addVenueFlags(cmd, &vf)
	return cmd
}
