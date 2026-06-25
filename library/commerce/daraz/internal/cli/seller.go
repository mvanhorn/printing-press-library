// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Safe to edit.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelSellerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "seller",
		Short:       "seller subcommands: stats, listings",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelSellerStatsCmd(flags))
	cmd.AddCommand(newSellerProductsCmd(flags))
	return cmd
}

type sellerProductRow struct {
	ItemID  string  `json:"itemId"`
	Name    string  `json:"name"`
	Price   float64 `json:"price"`
	Rating  float64 `json:"rating"`
	Reviews int     `json:"reviews"`
	Sold    int     `json:"sold"`
	URL     string  `json:"url"`
}

func newSellerProductsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "listings <sellerId>",
		Short:       "List a seller's listings captured in your local store.",
		Long:        "List the products this CLI has seen from a seller, ordered by price.\n\nThe local store is populated by 'deals', 'value', 'compare', 'watch', and 'since' runs. Use 'seller stats' for an aggregate scorecard instead of the raw listings.",
		Example:     "  daraz-pp-cli seller listings 1066739 --agent",
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
				return usageErr(fmt.Errorf("a seller ID is required, e.g. seller products 1066739"))
			}
			sellerID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := openDarazStore(ctx, flags)
			if err != nil {
				return err
			}
			defer s.Close()

			q := `SELECT item_id, name, price, rating, review_count, sold, item_url
			      FROM daraz_products_seen WHERE seller_id=? ORDER BY price`
			rowsArgs := []any{sellerID}
			if limit > 0 {
				q += ` LIMIT ?`
				rowsArgs = append(rowsArgs, limit)
			}
			rows, err := s.DB().QueryContext(ctx, q, rowsArgs...)
			if err != nil {
				return fmt.Errorf("reading seller listings: %w", err)
			}
			defer rows.Close()
			out := make([]sellerProductRow, 0)
			for rows.Next() {
				var id string
				var name, url sql.NullString
				var price, rating sql.NullFloat64
				var reviews, sold sql.NullInt64
				if err := rows.Scan(&id, &name, &price, &rating, &reviews, &sold, &url); err != nil {
					continue
				}
				out = append(out, sellerProductRow{
					ItemID: id, Name: name.String, Price: price.Float64, Rating: rating.Float64,
					Reviews: int(reviews.Int64), Sold: int(sold.Int64), URL: url.String,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading seller listings: %w", err)
			}
			if len(out) == 0 {
				return emptyMirrorHint(cmd, flags, fmt.Sprintf("no local listings for seller %s yet. Run deals/value/compare/watch on relevant queries first, then retry.", sellerID))
			}
			return emitDaraz(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum listings to return")
	return cmd
}
