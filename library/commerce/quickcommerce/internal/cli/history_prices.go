// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newNovelHistoryPricesCmd(flags *rootFlags) *cobra.Command {
	var item, since, dbPath string
	cmd := &cobra.Command{
		Use: "prices", Short: "Query local observations to see price, stock, rating, and availability movement over time.",
		Example:     "  quickcommerce-pp-cli history prices --item 501346 --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "history prices")
			}
			if item == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--item is required; use a platform item ID"))
			}
			dur, err := parseQCDuration(since)
			if err != nil {
				return usageErr(err)
			}
			path := qcDBPath(dbPath)
			if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
				return qcMissingMirror(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, path, "products")
			} else if statErr != nil {
				return apiErr(fmt.Errorf("checking local mirror: %w", statErr))
			}
			db, err := openQCCreatable(cmd.Context(), path)
			if err != nil {
				return err
			}
			defer db.Close()
			cutoff := time.Now().UTC().Add(-dur)
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT item_id, platform, location, query, captured_at, price, mrp, inventory, available, quantity, eta, open, store_id FROM quickcommerce_observations WHERE (item_id = ? OR id = ?) AND captured_at >= ? ORDER BY captured_at DESC`, item, item, cutoff)
			if err != nil {
				return apiErr(fmt.Errorf("querying price history: %w", err))
			}
			result := make([]map[string]any, 0)
			for rows.Next() {
				var itemID, platform, location, query, quantity, eta, storeID string
				var captured any
				var price, mrp sql.NullFloat64
				var inventory sql.NullInt64
				var available, open sql.NullBool
				if err := rows.Scan(&itemID, &platform, &location, &query, &captured, &price, &mrp, &inventory, &available, &quantity, &eta, &open, &storeID); err != nil {
					_ = rows.Close()
					return apiErr(fmt.Errorf("scan history row: %w", err))
				}
				row := map[string]any{"item_id": itemID, "platform": platform, "location": location, "query": query, "captured_at": qcDBTime(captured).Format(time.RFC3339)}
				if price.Valid {
					row["price"] = price.Float64
				}
				if mrp.Valid {
					row["mrp"] = mrp.Float64
				}
				if inventory.Valid {
					row["inventory"] = inventory.Int64
				}
				if available.Valid {
					row["available"] = available.Bool
				}
				if quantity != "" {
					row["quantity"] = quantity
				}
				if eta != "" {
					row["eta"] = eta
				}
				if open.Valid {
					row["open"] = open.Bool
				}
				if storeID != "" {
					row["store_id"] = storeID
				}
				result = append(result, row)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			if err := rows.Close(); err != nil {
				return apiErr(err)
			}
			return qcPrint(cmd.OutOrStdout(), flags, result, result)
		},
	}
	cmd.Flags().StringVar(&item, "item", "", "Platform item ID to inspect")
	cmd.Flags().StringVar(&since, "since", "30d", "Look back this long, for example 24h or 30d")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path")
	return cmd
}
