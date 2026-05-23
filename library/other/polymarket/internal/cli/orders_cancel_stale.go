// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: orders cancel-stale (sibling of orders cancel-all).

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newOrdersCancelStaleCmd(flags *rootFlags) *cobra.Command {
	var olderThan time.Duration
	var broadcast bool

	cmd := &cobra.Command{
		Use:   "cancel-stale",
		Short: "List (and optionally cancel) every open order older than a threshold. Helps makers clear dust orders that count against per-IP quotas.",
		Example: `  polymarket-pp-cli orders cancel-stale --older-than 24h --dry-run
  polymarket-pp-cli orders cancel-stale --older-than 168h --broadcast`,
		Annotations: map[string]string{"pp:novel": "orders.cancel_stale"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Pull open orders.
			rawOrders, oerr := c.GetWithHeaders(cmd.Context(), "https://clob.polymarket.com/orders",
				map[string]string{}, nil)
			if oerr != nil {
				return classifyAPIError(oerr, flags)
			}
			var orders []map[string]any
			if err := json.Unmarshal(rawOrders, &orders); err != nil {
				// Try wrapped {data:[...]} shape
				var wrapped struct {
					Data []map[string]any `json:"data"`
				}
				if err2 := json.Unmarshal(rawOrders, &wrapped); err2 != nil {
					return apiErr(fmt.Errorf("parsing orders: %w", err))
				}
				orders = wrapped.Data
			}

			cutoff := time.Now().Add(-olderThan)
			type staleRow struct {
				OrderID    string  `json:"order_id"`
				Market     string  `json:"market"`
				Side       string  `json:"side"`
				Price      float64 `json:"price"`
				Size       float64 `json:"size"`
				CreatedAt  string  `json:"created_at"`
				AgeSeconds float64 `json:"age_seconds"`
			}
			var stale []staleRow
			for _, o := range orders {
				var createdStr string
				if v, ok := o["created_at"].(string); ok {
					createdStr = v
				} else if v, ok := o["createdAt"].(string); ok {
					createdStr = v
				}
				if createdStr == "" {
					continue
				}
				createdAt, perr := time.Parse(time.RFC3339, createdStr)
				if perr != nil {
					createdAt, perr = time.Parse("2006-01-02T15:04:05Z", createdStr)
					if perr != nil {
						continue
					}
				}
				if createdAt.After(cutoff) {
					continue
				}
				row := staleRow{
					CreatedAt:  createdStr,
					AgeSeconds: time.Since(createdAt).Seconds(),
				}
				if v, ok := o["id"].(string); ok {
					row.OrderID = v
				} else if v, ok := o["order_id"].(string); ok {
					row.OrderID = v
				}
				if v, ok := o["market"].(string); ok {
					row.Market = v
				}
				if v, ok := o["side"].(string); ok {
					row.Side = v
				}
				if v, ok := o["price"].(float64); ok {
					row.Price = v
				}
				if v, ok := o["size_original"].(float64); ok {
					row.Size = v
				} else if v, ok := o["size"].(float64); ok {
					row.Size = v
				}
				stale = append(stale, row)
			}

			out := map[string]any{
				"older_than":     olderThan.String(),
				"cutoff":         cutoff.Format(time.RFC3339),
				"orders_scanned": len(orders),
				"stale_count":    len(stale),
				"stale_orders":   stale,
				"broadcast":      broadcast,
			}
			if broadcast {
				out["broadcast_status"] = "NOT_IMPLEMENTED"
				out["broadcast_note"] = "Live cancellation requires the absorbed `orders cancel-batch` primitive, which itself needs EIP-712 signing. The stale_orders list above is the exact set you would pass to `polymarket-pp-cli orders cancel-batch --order-ids ID,ID,...` once signing lands in v0.2. Until then, the Polymarket web UI or the official Rust CLI can broadcast this batch."
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().DurationVar(&olderThan, "older-than", 24*time.Hour, "Cancel orders older than this duration (default 24h)")
	cmd.Flags().BoolVar(&broadcast, "broadcast", false, "Actually send cancellation transactions (not implemented — see broadcast_note)")
	return cmd
}
