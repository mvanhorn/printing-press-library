package cli

import (
	"encoding/json"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/store"
	"github.com/spf13/cobra"
)

func newWishlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "wishlist", Short: "Sync and diff Airbnb wishlists"}
	cmd.AddCommand(newWishlistSyncCmd(flags), newWishlistDiffCmd(flags))
	return cmd
}

func newWishlistSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Airbnb wishlists into the local store",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"wishlists": 0, "items": 0, "dry_run": true}, flags)
			}
			wishlists, err := airbnb.WishlistList(cmd.Context())
			if err != nil {
				return authErr(err)
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("airbnb-vrbo-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			for _, w := range wishlists {
				b, _ := json.Marshal(w)
				_ = db.UpsertAirbnbWishlist(b)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"wishlists": len(wishlists), "items": 0, "synced_at": time.Now().Unix()}, flags)
		},
	}
	return cmd
}

func newWishlistDiffCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Show wishlist price changes since a date",
		Example:     "  airbnb-vrbo-pp-cli wishlist diff --since 2026-04-01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("airbnb-vrbo-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			snaps, err := db.ListPriceSnapshotsSince(parseSinceDate(since))
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), priceDiffs(snaps), flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start date YYYY-MM-DD")
	return cmd
}

func priceDiffs(snaps []store.PriceSnapshot) []map[string]any {
	byID := map[string][]store.PriceSnapshot{}
	for _, s := range snaps {
		key := s.Platform + ":" + s.ListingID + ":" + s.Checkin + ":" + s.Checkout
		byID[key] = append(byID[key], s)
	}
	var out []map[string]any
	for _, list := range byID {
		if len(list) < 2 {
			continue
		}
		first, last := list[0], list[len(list)-1]
		out = append(out, map[string]any{
			"listing_id": first.ListingID,
			"platform":   first.Platform,
			"from":       first.TotalPrice,
			"to":         last.TotalPrice,
			"change":     last.TotalPrice - first.TotalPrice,
		})
	}
	return out
}
