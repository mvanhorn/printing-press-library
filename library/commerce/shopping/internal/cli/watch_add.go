// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// watchAddResult confirms a pin written to the local watches table.
type watchAddResult struct {
	RetailerID  string   `json:"retailer_id"`
	ProductID   string   `json:"product_id"`
	TargetPrice *float64 `json:"target_price"`
	AddedAt     string   `json:"added_at"`
}

func newNovelWatchAddCmd(flags *rootFlags) *cobra.Command {
	var flagTargetPrice float64
	var flagDB string

	cmd := &cobra.Command{
		Use:     "add <retailer> <product_id>",
		Short:   "Pin a product you already indexed so 'watch status' tracks its price",
		Example: "  shopping-pp-cli watch add walmart 100847732 --target-price 79.99",
		// No mcp:read-only: watch add writes the local watches table, so the
		// MCP tool defaults to could-write and the host prompts before use.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would add a watch for the given retailer and product to the local store")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("retailer and product_id are required"))
			}
			retailer := args[0]
			productID := args[1]

			ctx := cmd.Context()
			db, err := openShopStore(ctx, resolveShopDBPath(flagDB))
			if err != nil {
				return err
			}
			defer db.Close()

			addedAt := time.Now().UTC().Format(time.RFC3339)

			var target any
			result := watchAddResult{
				RetailerID: retailer,
				ProductID:  productID,
				AddedAt:    addedAt,
			}
			if cmd.Flags().Changed("target-price") {
				tp := flagTargetPrice
				target = tp
				result.TargetPrice = &tp
			}

			if _, err := db.DB().ExecContext(ctx,
				`INSERT INTO watches (retailers_id, product_id, target_price, added_at)
				 VALUES (?, ?, ?, ?)
				 ON CONFLICT(retailers_id, product_id) DO UPDATE SET
				   target_price = excluded.target_price,
				   added_at     = excluded.added_at`,
				retailer, productID, target, addedAt,
			); err != nil {
				return fmt.Errorf("insert watch: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().Float64Var(&flagTargetPrice, "target-price", 0, "Target price that flips hit_target true in 'watch status'")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local store (defaults to SHOPPING_DB or the share path)")
	return cmd
}
