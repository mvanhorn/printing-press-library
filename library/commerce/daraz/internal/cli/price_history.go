// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Safe to edit.
//
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type pricePoint struct {
	Ts    int64   `json:"ts"`
	Date  string  `json:"date"`
	Price float64 `json:"price"`
}

type priceHistoryOut struct {
	ItemID    string       `json:"itemId"`
	Name      string       `json:"name"`
	Count     int          `json:"count"`
	Current   float64      `json:"current"`
	Lowest    float64      `json:"lowest"`
	Highest   float64      `json:"highest"`
	FirstSeen string       `json:"firstSeen"`
	LastSeen  string       `json:"lastSeen"`
	Points    []pricePoint `json:"points"`
}

func newNovelPriceHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "price-history <itemId>",
		Short:       "See a product's recorded price trend and its lowest-ever price from your local store.",
		Long:        "See a product's recorded price trend and its lowest-ever price from your local store.\n\nUse this command to judge whether a current price is actually low for that item. The history is populated by 'watch', 'deals', 'value', 'compare', and 'since' runs that included the item.",
		Example:     "  daraz-pp-cli price-history 599201597 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an item ID is required, e.g. price-history 599201597"))
			}
			itemID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := openDarazStore(ctx, flags)
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := s.DB().QueryContext(ctx, `SELECT name, price, ts FROM daraz_price_snapshots WHERE item_id=? ORDER BY ts`, itemID)
			if err != nil {
				return fmt.Errorf("reading price history: %w", err)
			}
			defer rows.Close()
			out := priceHistoryOut{ItemID: itemID, Points: []pricePoint{}}
			for rows.Next() {
				var name sql.NullString
				var price sql.NullFloat64
				var ts sql.NullInt64
				if err := rows.Scan(&name, &price, &ts); err != nil {
					continue
				}
				if name.String != "" {
					out.Name = name.String
				}
				out.Points = append(out.Points, pricePoint{
					Ts:    ts.Int64,
					Date:  time.Unix(ts.Int64, 0).Format("2006-01-02 15:04"),
					Price: price.Float64,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading price history: %w", err)
			}
			if len(out.Points) == 0 {
				return emptyMirrorHint(cmd, flags, fmt.Sprintf("no price history for item %s yet. Capture it with: daraz-pp-cli watch \"<a search that returns this item>\" (or run deals/value/compare on a matching query), then retry.", itemID))
			}
			out.Count = len(out.Points)
			out.Lowest = out.Points[0].Price
			out.Highest = out.Points[0].Price
			for _, p := range out.Points {
				if p.Price < out.Lowest {
					out.Lowest = p.Price
				}
				if p.Price > out.Highest {
					out.Highest = p.Price
				}
			}
			out.Current = out.Points[len(out.Points)-1].Price
			out.FirstSeen = out.Points[0].Date
			out.LastSeen = out.Points[len(out.Points)-1].Date
			return emitDaraz(cmd, flags, out)
		},
	}
	return cmd
}
