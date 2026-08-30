// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: shortlist drift — price/capacity/instant_book changes on favorites.

package cli

// pp:data-source local

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelShortlistDriftCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "drift",
		Short:       "Track price and availability-field changes on favorites over time (Luma-style watch).",
		Example:     "  peerspace-pp-cli shortlist drift --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagSince == "" {
				flagSince = "7d"
			}
			since, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			if since < 0 {
				since = -since
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, err := openNovelStoreRW(ctx, flagDB)
			if err != nil {
				return err
			}
			if s == nil {
				missingDBHint(flagDB)
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"since":   flagSince,
					"changes": make([]any, 0),
				}, flags)
			}
			defer s.Close()

			currentIDs, err := loadFavoriteIDs(ctx, s)
			if err != nil {
				return err
			}
			listings, err := loadListings(ctx, s)
			if err != nil {
				return err
			}
			currentAttrs := map[string]venuex.SnapshotAttrs{}
			for _, l := range venuex.JoinFavorites(currentIDs, listings) {
				if l.ID != "" {
					currentAttrs[l.ID] = venuex.AttrsFromListing(l)
				}
			}

			prior, ok, err := venuex.SnapshotSince(ctx, s.DB(), since)
			if err != nil {
				return err
			}
			changes := make([]venuex.DriftChange, 0)
			if ok {
				changes = venuex.DiffAttrs(prior.Attrs, currentAttrs)
			}
			// Always refresh snapshot so subsequent drift/delta calls have a baseline.
			_ = venuex.InsertSnapshot(ctx, s.DB(), currentIDs, currentAttrs)

			result := map[string]any{
				"since":        flagSince,
				"since_cutoff": time.Now().UTC().Add(-since).Format(time.RFC3339),
				"had_baseline": ok,
				"changes":      changes,
				"watched":      len(currentAttrs),
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "shortlist drift since=%s changes=%d watched=%d\n", flagSince, len(changes), len(currentAttrs))
				for _, c := range changes {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %s: %v -> %v  %s\n", c.ID, c.Field, c.Before, c.After, c.Title)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Look-back window (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}
