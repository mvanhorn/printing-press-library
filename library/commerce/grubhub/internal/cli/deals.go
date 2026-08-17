// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
)

type dealRow struct {
	RestaurantID   string   `json:"restaurant_id"`
	RestaurantName string   `json:"restaurant_name"`
	BestOffer      string   `json:"best_offer"`
	BestValue      string   `json:"best_value"`
	BestValueCents int      `json:"best_value_cents"`
	OrderMinimum   string   `json:"order_minimum,omitempty"`
	Offers         []string `json:"offers"`
	CouponsFlag    bool     `json:"coupons_available"`
	DeliveryFee    string   `json:"delivery_fee"`
}

func newNovelDealsCmd(flags *rootFlags) *cobra.Command {
	var sortKey string
	var limit int
	var pickup bool

	cmd := &cobra.Command{
		Use:   "deals <address>",
		Short: "Rank every nearby restaurant currently running an offer, coupon, or promo in one sweep",
		Long: "Sweep every nearby restaurant for active offers, coupons, and promo codes and rank them by value in one view.\n\n" +
			"Use this for a ranked cross-restaurant view of deals. To read offers on a single restaurant, the 'near' table already shows a deal count per row.",
		Example:     "  grubhub-pp-cli deals \"350 5th Ave, New York, NY\" --sort value",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank nearby restaurants by active deals")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an address is required, e.g. deals \"350 5th Ave, New York, NY\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := grubhubClient(ctx, flags)
			if err != nil {
				return err
			}
			coord, err := geocodeAddress(ctx, c, args[0])
			if err != nil {
				return err
			}
			method := "delivery"
			if pickup {
				method = "pickup"
			}
			cards, _, err := searchCards(ctx, c, coord, searchOptions{orderMethod: method, pageSize: 50})
			if err != nil {
				return err
			}
			rows := dealRowsFromCards(cards)
			sortDeals(rows, sortKey)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{
					"address":   args[0],
					"sorted_by": dealsSortKey(sortKey),
					"count":     len(rows),
					"deals":     rows,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No active deals found near %s, %s right now.\n", coord.Locality, coord.Region)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d restaurants with active deals near %s, %s\n\n", len(rows), coord.Locality, coord.Region)
			return renderDeals(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&sortKey, "sort", "value", "Sort by: value (best offer amount) or count (number of offers)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum restaurants to return")
	cmd.Flags().BoolVar(&pickup, "pickup", false, "Use pickup instead of delivery")
	return cmd
}

func dealRowsFromCards(cards []grubhub.Card) []dealRow {
	rows := make([]dealRow, 0)
	for _, card := range cards {
		if card.DealCount() == 0 {
			continue
		}
		row := dealRow{
			RestaurantID:   card.ID,
			RestaurantName: card.Name,
			CouponsFlag:    card.CouponsAvailable,
			DeliveryFee:    grubhub.Dollars(card.DeliveryFee.Price),
			Offers:         make([]string, 0, len(card.AvailableOffers)),
		}
		bestCents := 0
		for _, o := range card.AvailableOffers {
			label := strings.TrimSpace(o.Title)
			if label == "" {
				label = strings.TrimSpace(o.Description)
			}
			if label != "" {
				row.Offers = append(row.Offers, label)
			}
			if o.ValueCents() > bestCents {
				bestCents = o.ValueCents()
				row.BestOffer = label
				if o.OrderMinimumCents() > 0 {
					row.OrderMinimum = grubhub.Dollars(o.OrderMinimumCents())
				} else {
					row.OrderMinimum = ""
				}
			}
		}
		row.BestValueCents = bestCents
		if bestCents > 0 {
			row.BestValue = grubhub.Dollars(bestCents)
		}
		if row.BestOffer == "" && card.CouponsAvailable {
			row.BestOffer = "coupons available"
		}
		rows = append(rows, row)
	}
	return rows
}

func dealsSortKey(k string) string {
	if strings.ToLower(strings.TrimSpace(k)) == "count" {
		return "count"
	}
	return "value"
}

func sortDeals(rows []dealRow, key string) {
	if dealsSortKey(key) == "count" {
		sort.SliceStable(rows, func(i, j int) bool { return len(rows[i].Offers) > len(rows[j].Offers) })
		return
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].BestValueCents > rows[j].BestValueCents })
}

func renderDeals(cmd *cobra.Command, rows []dealRow) error {
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "RESTAURANT\tBEST OFFER\tMIN\tOFFERS\tFEE\tID")
	for _, r := range rows {
		minStr := r.OrderMinimum
		if minStr == "" {
			minStr = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n",
			truncate(r.RestaurantName, 28), truncate(r.BestOffer, 28), minStr, len(r.Offers), r.DeliveryFee, r.RestaurantID)
	}
	return tw.Flush()
}
