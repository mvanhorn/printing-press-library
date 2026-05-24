// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
	"github.com/spf13/cobra"
)

type wishlistDrop struct {
	PropertyName  string  `json:"property_name"`
	PropertySlug  string  `json:"property_slug"`
	Country       string  `json:"country"`
	LatestPrice   float64 `json:"latest_price"`
	PreviousPrice float64 `json:"previous_price"`
	DropPct       float64 `json:"drop_pct"`
	Currency      string  `json:"currency"`
	ObservedAt    string  `json:"observed_at"`
}

func newWishlistDropsCmd(flags *rootFlags) *cobra.Command {
	var since time.Duration
	var minPct float64
	cmd := &cobra.Command{
		Use:         "drops",
		Short:       "Show wishlist properties whose latest observed price dropped",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]wishlistDrop, 0))
			}
			st, err := openBookingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("wishlist drops: %w", err)
			}
			defer st.Close()
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("wishlist drops: %w", err)
			}
			data, err := c.Get("/mywishlist.html", nil)
			if err != nil {
				return fmt.Errorf("wishlist drops: %w", err)
			}
			parsed, err := booking.ParseWishlist(data)
			if err != nil {
				return fmt.Errorf("wishlist drops: %w", err)
			}
			items := make([]booking.WishlistItem, 0)
			if err := json.Unmarshal(parsed, &items); err != nil {
				return fmt.Errorf("wishlist drops: %w", err)
			}
			names := map[string]booking.WishlistItem{}
			for _, item := range items {
				if item.PropertySlug == "" || item.LastSeenPrice <= 0 {
					continue
				}
				names[item.PropertySlug] = item
				_ = insertPrice(cmd.Context(), st.DB(), item.PropertySlug, item.Country, "", "", 0, item.Currency, item.LastSeenPrice)
			}
			cutoff := time.Now().Add(-since)
			out := make([]wishlistDrop, 0)
			for slug, item := range names {
				rows, err := st.DB().QueryContext(cmd.Context(), `SELECT price,currency,observed_at FROM price_history WHERE slug=? AND checkin='' AND checkout='' AND group_adults=0 ORDER BY observed_at DESC LIMIT 2`, slug)
				if err != nil {
					return fmt.Errorf("wishlist drops: %w", err)
				}
				var prices []float64
				var latestCurrency, latestObserved string
				for rows.Next() {
					var price float64
					var currency, observed string
					if err := rows.Scan(&price, &currency, &observed); err != nil {
						rows.Close()
						return fmt.Errorf("wishlist drops: %w", err)
					}
					if len(prices) == 0 {
						latestCurrency = currency
						latestObserved = observed
					}
					prices = append(prices, price)
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return fmt.Errorf("wishlist drops: iterating price_history for %s: %w", slug, err)
				}
				rows.Close()
				if len(prices) < 2 {
					continue
				}
				seenAt, _ := time.Parse(time.RFC3339, latestObserved)
				drop := (prices[1] - prices[0]) / prices[1] * 100
				if !seenAt.Before(cutoff) && drop >= minPct {
					out = append(out, wishlistDrop{PropertyName: item.PropertyName, PropertySlug: slug, Country: item.Country, LatestPrice: prices[0], PreviousPrice: prices[1], DropPct: drop, Currency: latestCurrency, ObservedAt: latestObserved})
				}
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().DurationVar(&since, "since", 30*24*time.Hour, "Observation window duration")
	cmd.Flags().Float64Var(&minPct, "min-pct", 10, "Minimum drop percentage")
	return cmd
}
