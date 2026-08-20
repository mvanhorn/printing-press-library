// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: shortlist delta — favorites added/removed vs last snapshot.

package cli

// pp:data-source local

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelShortlistDeltaCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:         "delta",
		Short:       "Show favorites added, removed, or changed since the last sync snapshot.",
		Example:     "  peerspace-pp-cli shortlist delta --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
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
					"added":   make([]string, 0),
					"removed": make([]string, 0),
					"kept":    make([]string, 0),
				}, flags)
			}
			defer s.Close()

			current, err := loadFavoriteIDs(ctx, s)
			if err != nil {
				return err
			}
			listings, err := loadListings(ctx, s)
			if err != nil {
				return err
			}
			attrs := map[string]venuex.SnapshotAttrs{}
			for _, l := range venuex.JoinFavorites(current, listings) {
				if l.ID != "" {
					attrs[l.ID] = venuex.AttrsFromListing(l)
				}
			}

			prev, ok, err := venuex.LatestSnapshot(ctx, s.DB())
			if err != nil {
				return err
			}
			var delta venuex.DeltaResult
			if ok {
				delta = venuex.DeltaIDs(prev.FavIDs, current)
			} else {
				delta = venuex.DeltaResult{
					Added:   append([]string(nil), current...),
					Removed: make([]string, 0),
					Kept:    make([]string, 0),
				}
			}
			// Persist current snapshot for next comparison.
			if err := venuex.InsertSnapshot(ctx, s.DB(), current, attrs); err != nil {
				return fmt.Errorf("saving shortlist snapshot: %w", err)
			}

			result := map[string]any{
				"added":        delta.Added,
				"removed":      delta.Removed,
				"kept":         delta.Kept,
				"current":      len(current),
				"had_previous": ok,
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "shortlist delta current=%d previous=%v\n", len(current), ok)
				fmt.Fprintf(cmd.OutOrStdout(), "  added:   %v\n", delta.Added)
				fmt.Fprintf(cmd.OutOrStdout(), "  removed: %v\n", delta.Removed)
				fmt.Fprintf(cmd.OutOrStdout(), "  kept:    %d\n", len(delta.Kept))
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}
