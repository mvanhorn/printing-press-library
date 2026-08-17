// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: shortlist similar — mechanical neighbors of a listing.

package cli

// pp:data-source local

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelShortlistSimilarCmd(flags *rootFlags) *cobra.Command {
	var flagID string
	var flagWithinPct float64
	var flagDB string

	cmd := &cobra.Command{
		Use:         "similar",
		Short:       "Find local listings near a favorite on capacity, price, and neighborhood.",
		Example:     "  peerspace-pp-cli shortlist similar --id 68d468bb44492187e415d4a6 --within-pct 20 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagID == "" && len(args) > 0 {
				flagID = args[0]
			}
			if flagID == "" && !flags.dryRun && cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if flagWithinPct <= 0 {
				flagWithinPct = 20
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, err := openNovelStoreRO(ctx, flagDB)
			if err != nil {
				return err
			}
			if s == nil {
				missingDBHint(flagDB)
				return printJSONFiltered(cmd.OutOrStdout(), make([]any, 0), flags)
			}
			defer s.Close()

			listings, err := loadListings(ctx, s)
			if err != nil {
				return err
			}
			// Default seed: first favorite, else first listing (agent-friendly probe).
			if flagID == "" {
				if favs, err := loadFavoriteIDs(ctx, s); err == nil && len(favs) > 0 {
					flagID = favs[0]
				} else if len(listings) > 0 {
					flagID = listings[0].ID
				}
			}
			if flagID == "" {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"id":      "",
					"found":   false,
					"note":    "no seed listing; pass --id or sync listings/favorites first",
					"similar": make([]any, 0),
				}, flags)
			}
			var seed venuex.Listing
			found := false
			for _, l := range listings {
				if l.ID == flagID {
					seed = l
					found = true
					break
				}
			}
			if !found {
				// Try favorites stubs / direct lookup
				if l, _, ok, err := findListingByID(ctx, s, flagID); err == nil && ok {
					seed = l
					found = true
				}
			}
			if !found {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"id":      flagID,
					"found":   false,
					"similar": make([]any, 0),
				}, flags)
			}
			sim := venuex.Similar(seed, listings, flagWithinPct)
			rows := make([]map[string]any, 0, len(sim))
			for _, l := range sim {
				rows = append(rows, listingToRow(l))
			}
			result := map[string]any{
				"id":         flagID,
				"seed":       listingToRow(seed),
				"within_pct": flagWithinPct,
				"similar":    rows,
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "similar to %s within %.0f%% → %d\n", flagID, flagWithinPct, len(rows))
				for _, r := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "  %v  price=%v guests=%v  %v\n", r["id"], r["price_hourly"], r["guests"], r["title"])
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagID, "id", "", "Seed listing id")
	cmd.Flags().Float64Var(&flagWithinPct, "within-pct", 20, "Max percent difference on price and capacity")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}
