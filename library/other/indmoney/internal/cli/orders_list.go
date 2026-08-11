// Copyright 2026 dev-abhirup-sc and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented novel command. generate --force preserves implemented bodies.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newNovelOrdersListCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagPage int

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Every buy and sell order you have placed, with transaction type, quantity, avg price, and status.",
		Example:     "  indmoney-pp-cli orders list --limit 100 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "orders list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{
				"page":            strconv.Itoa(flagPage),
				"limit":           strconv.Itoa(flagLimit),
				"tabId":           "4", // All Orders
				"filterId":        "1",
				"response_format": "json",
			}
			data, err := c.GetWithHeaders(ctx, "/portfolio/orders/summary", params, map[string]string{"platform": "web"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var resp struct {
				Status bool `json:"status"`
				Data   struct {
					Orders []struct {
						OrderID        int     `json:"order_id"`
						Name           string  `json:"name"`
						IndKey         string  `json:"ind_key"`
						Status         string  `json:"status"`
						ProductType    string  `json:"product_type"`
						Date           string  `json:"date"`
						DateEpoch      int64   `json:"date_epoch"`
						Qty            float64 `json:"qty"`
						OrderValue     string  `json:"order_value"`
						OrderType      string  `json:"order_type"`
						AvgPrice       float64 `json:"avg_price"`
						TransactionType string `json:"transaction_type"`
					} `json:"orders"`
				} `json:"data"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parsing orders response: %w", err)
			}

			type order struct {
				OrderID         int     `json:"order_id"`
				Name            string  `json:"name"`
				Symbol          string  `json:"symbol"`
				TransactionType string  `json:"transaction_type"`
				Qty             float64 `json:"qty"`
				AvgPrice        float64 `json:"avg_price"`
				OrderValue      string  `json:"order_value"`
				Status          string  `json:"status"`
				OrderType       string  `json:"order_type"`
				Date            string  `json:"date"`
			}

			orders := make([]order, 0, len(resp.Data.Orders))
			for _, o := range resp.Data.Orders {
				orders = append(orders, order{
					OrderID: o.OrderID, Name: o.Name, Symbol: o.IndKey,
					TransactionType: o.TransactionType, Qty: o.Qty,
					AvgPrice: o.AvgPrice, OrderValue: o.OrderValue,
					Status: o.Status, OrderType: o.OrderType, Date: o.Date,
				})
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), orders, flags)
			}

			if len(orders) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No orders found.")
				return nil
			}
			items := make([]map[string]any, len(orders))
			for i, o := range orders {
				items[i] = map[string]any{
					"date": o.Date, "type": o.TransactionType,
					"name": o.Name, "qty": o.Qty,
					"avg_price": o.AvgPrice, "value": o.OrderValue,
					"status": o.Status,
				}
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Number of orders to fetch per page")
	cmd.Flags().IntVar(&flagPage, "page", 1, "Page number for pagination")
	return cmd
}
