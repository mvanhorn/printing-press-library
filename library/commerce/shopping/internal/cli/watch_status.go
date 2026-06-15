// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source local

// watchStatusRow reports one watched product's latest movement.
type watchStatusRow struct {
	RetailerID    string   `json:"retailer_id"`
	ProductID     string   `json:"product_id"`
	CurrentPrice  *float64 `json:"current_price"`
	PreviousPrice *float64 `json:"previous_price"`
	Delta         *float64 `json:"delta"`
	TargetPrice   *float64 `json:"target_price"`
	HitTarget     *bool    `json:"hit_target"`
}

func newNovelWatchStatusCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Pin items you care about and, after each index refresh, see which ones moved, by how much",
		Example:     "  shopping-pp-cli watch status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report price movement for watched products from the local store")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}

			ctx := cmd.Context()
			db, err := openShopStore(ctx, resolveShopDBPath(flagDB))
			if err != nil {
				return err
			}
			defer db.Close()

			// Current price prefers the live products row; the two trailing
			// subqueries supply the most-recent (latest_pp) and second-most-recent
			// (prev_pp) price_points observations so the delta below can span
			// exactly one change. The correlated subqueries keep this a single
			// pass over the (small) watches table.
			query := `SELECT w.retailers_id, w.product_id, w.target_price,
				(SELECT json_extract(p.data,'$.current_price')
				   FROM products p
				  WHERE p.retailers_id = w.retailers_id AND p.id = w.product_id) AS current_price,
				(SELECT pp.price FROM price_points pp
				  WHERE pp.retailers_id = w.retailers_id AND pp.product_id = w.product_id AND pp.price IS NOT NULL
				  ORDER BY pp.ts DESC LIMIT 1) AS latest_pp,
				(SELECT pp.price FROM price_points pp
				  WHERE pp.retailers_id = w.retailers_id AND pp.product_id = w.product_id AND pp.price IS NOT NULL
				  ORDER BY pp.ts DESC LIMIT 1 OFFSET 1) AS prev_pp
			FROM watches w
			ORDER BY w.retailers_id, w.product_id`

			rows, err := db.Query(query)
			if err != nil {
				return fmt.Errorf("query watch status: %w", err)
			}
			defer rows.Close()

			results := []watchStatusRow{}
			for rows.Next() {
				var (
					rid      string
					pid      string
					target   sql.NullFloat64
					curProd  sql.NullFloat64
					latestPP sql.NullFloat64
					prevPP   sql.NullFloat64
				)
				if err := rows.Scan(&rid, &pid, &target, &curProd, &latestPP, &prevPP); err != nil {
					return fmt.Errorf("scan watch row: %w", err)
				}

				row := watchStatusRow{
					RetailerID:  rid,
					ProductID:   pid,
					TargetPrice: nullableFloat(target),
				}
				// Current: the products row wins (freshest, set by the last
				// index); otherwise the most recent price point. The prior
				// observation is the most recent price point when products
				// supplied current, else the second-most-recent — so the delta
				// always spans exactly one change instead of two.
				var current, previous *float64
				switch {
				case curProd.Valid:
					current = nullableFloat(curProd)
					// The prior observation is the most recent price point —
					// unless it duplicates the current products price (the same
					// observation, e.g. an `index --price-history` run), in which
					// case use the one before it so the delta reflects a real move
					// rather than spanning zero or two changes.
					switch {
					case latestPP.Valid && latestPP.Float64 != curProd.Float64:
						previous = nullableFloat(latestPP)
					case prevPP.Valid:
						previous = nullableFloat(prevPP)
					case latestPP.Valid:
						previous = nullableFloat(latestPP)
					}
				case latestPP.Valid:
					current = nullableFloat(latestPP)
					previous = nullableFloat(prevPP)
				}
				row.CurrentPrice = current
				row.PreviousPrice = previous

				if current != nil && previous != nil {
					d := *current - *previous
					row.Delta = &d
				}
				if current != nil && target.Valid {
					hit := *current <= target.Float64
					row.HitTarget = &hit
				}
				results = append(results, row)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating watch rows: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local store (defaults to SHOPPING_DB or the share path)")
	return cmd
}
