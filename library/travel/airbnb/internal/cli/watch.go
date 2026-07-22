package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/store"
	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "watch", Short: "Manage listing price-drop watchlist"}
	cmd.AddCommand(newWatchAddCmd(flags), newWatchListCmd(flags), newWatchRemoveCmd(flags), newWatchCheckCmd(flags))
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
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("airbnb-pp-cli"))
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

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <listing-url-or-id>",
		Short: "Remove a listing from the price watchlist",
		Example: "  airbnb-pp-cli watch remove 'https://www.airbnb.com/rooms/37124493'\n" +
			"  airbnb-pp-cli watch remove 37124493",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			target := stripURLArg(args[0])
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"target": target, "removed": false, "dry_run": true}, flags)
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("airbnb-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			// Resolve the exact stored listing_url to delete. A full URL is
			// matched verbatim (DeleteWatchlistItem keys on listing_url,
			// which is UNIQUE). A bare id (e.g. "37124493") is resolved to
			// its stored URL by scanning the watchlist for a matching
			// listing_id, so users can remove by the same id they see in
			// `watch list` output without reconstructing the URL.
			deleteURL := target
			if _, perr := parseListingURL(target); perr != nil {
				items, lerr := db.ListWatchlist(0)
				if lerr != nil {
					return lerr
				}
				matched := false
				for _, it := range items {
					if it.ListingID == target {
						deleteURL = it.ListingURL
						matched = true
						break
					}
				}
				if !matched {
					return notFoundErr(fmt.Errorf("no watchlist entry matches %q (pass a full listing URL or an id from 'watch list')", target))
				}
			}

			n, err := db.DeleteWatchlistItem(deleteURL)
			if err != nil {
				return err
			}
			if n == 0 {
				return notFoundErr(fmt.Errorf("listing not on watchlist: %s", deleteURL))
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"listing_url": deleteURL, "removed": true, "count": n}, flags)
		},
	}
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
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("airbnb-pp-cli"))
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
		Short:       "Check watched listings; exit 7 only when a real scraped price drops under threshold (a null/unavailable price is reported, never a hit)",
		Example:     "  airbnb-pp-cli watch check --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"hits": []any{}, "no_price": []any{}, "dry_run": true}, flags)
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("airbnb-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			items, err := db.ListWatchlist(0)
			if err != nil {
				return err
			}
			var hits []store.WatchlistItem
			var noPrice []map[string]any
			for _, item := range items {
				// PATCH: pass db so the scrape inside computeCheapest persists the
				// listing + a price snapshot through the already-open handle
				// — only when a real price is scraped (the snapshot write
				// is guarded on total > 0).
				ch, err := computeCheapest(cmd.Context(), item.ListingURL, cheapestParams{Checkin: item.Checkin, Checkout: item.Checkout, store: db})
				if err != nil {
					return apiErr(err)
				}
				price, _ := firstPlatformTotals(ch)
				hasPrice, hit := classifyWatchPrice(price, item.MaxPrice)
				// PATCH: a non-positive price means the SSR did not expose a
				// bookable total for these dates (unavailable/sold-out/auth-
				// gated). That is "no price data" — NOT a drop. Record the
				// check timestamp with hit=false, surface the item in the
				// no_price bucket, and never let it mint the exit-7 drop
				// sentinel.
				if !hasPrice {
					if uerr := db.UpdateWatchPrice(item.ID, 0, false); uerr != nil {
						return uerr
					}
					noPrice = append(noPrice, map[string]any{
						"listing_url": item.ListingURL,
						"listing_id":  item.ListingID,
						"platform":    item.Platform,
						"reason":      "no_price_data",
					})
					continue
				}
				if err := db.UpdateWatchPrice(item.ID, price, hit); err != nil {
					return err
				}
				if hit {
					item.LastPrice = price
					hits = append(hits, item)
				}
			}
			out := map[string]any{"hits": hits, "no_price": noPrice}
			if len(hits) > 0 {
				// Genuine price drop(s): keep the cron-friendly exit-7
				// success sentinel. no_price items never reach this branch.
				_ = printJSONFiltered(cmd.OutOrStdout(), out, flags)
				return rateLimitErr(fmt.Errorf("watch price drop hit"))
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// classifyWatchPrice is the PATCH decision for one watched listing given the
// scraped platform total and the watchlist's max-price threshold. It returns
// (hasPrice, hit):
//
//   - hasPrice is false when price <= 0 — the SSR exposed no bookable total
//     for these dates (unavailable / sold-out / auth-gated). The caller routes
//     these to the no_price bucket and they NEVER count as a drop or trigger
//     the exit-7 sentinel. firstPlatformTotals returns 0 both for genuinely-
//     free and for unscraped listings and cannot distinguish them, so the
//     positivity guard is the only safe interpretation here.
//   - hit is true only when there is a real positive price, a positive
//     threshold was set, and the price is at or below it.
func classifyWatchPrice(price, maxPrice float64) (hasPrice, hit bool) {
	if price <= 0 {
		return false, false
	}
	return true, maxPrice > 0 && price <= maxPrice
}
