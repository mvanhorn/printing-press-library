package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/store"
	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "watch", Short: "Manage listing price-drop watchlist"}
	cmd.AddCommand(newWatchAddCmd(flags), newWatchListCmd(flags), newWatchCheckCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var maxPrice float64
	var checkin, checkout string
	cmd := &cobra.Command{
		Use:   "add <listing-url>",
		Short: "Add a listing to the price watchlist",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			target := stripURLArg(args[0])
			ref, err := parseListingURL(target)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"listing_url": target, "platform": ref.Platform, "dry_run": true}, flags)
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("airbnb-vrbo-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			item := store.WatchlistItem{ListingURL: target, ListingID: ref.ID, Platform: ref.Platform, MaxPrice: maxPrice, Checkin: checkin, Checkout: checkout, AddedAt: time.Now().Unix()}
			if err := db.UpsertWatchlistItem(item); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), item, flags)
		},
	}
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Notify when total price is at or below this value")
	cmd.Flags().StringVar(&checkin, "checkin", "", "Arrival date YYYY-MM-DD")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Departure date YYYY-MM-DD")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List watched listings",
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
			items, err := db.ListWatchlist(parseSinceDate(since))
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only show items changed since DATE or duration")
	return cmd
}

func newWatchCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "check",
		Short:       "Check watched listings and exit 7 when any price drops under threshold",
		Example:     "  airbnb-vrbo-pp-cli watch check --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"hits": []any{}, "dry_run": true}, flags)
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("airbnb-vrbo-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			items, err := db.ListWatchlist(0)
			if err != nil {
				return err
			}
			var hits []store.WatchlistItem
			for _, item := range items {
				ch, err := computeCheapest(cmd.Context(), item.ListingURL, cheapestParams{Checkin: item.Checkin, Checkout: item.Checkout})
				if err != nil {
					return apiErr(err)
				}
				price, _ := firstPlatformTotals(ch)
				if err := db.UpdateWatchPrice(item.ID, price, item.MaxPrice > 0 && price <= item.MaxPrice); err != nil {
					return err
				}
				if item.MaxPrice > 0 && price <= item.MaxPrice {
					item.LastPrice = price
					hits = append(hits, item)
				}
			}
			if len(hits) > 0 {
				_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{"hits": hits}, flags)
				return rateLimitErr(fmt.Errorf("watch price drop hit"))
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"hits": hits}, flags)
		},
	}
	return cmd
}
