package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cliutil"

	"github.com/spf13/cobra"
)

// priceCompareRow is one ranked store-price line emitted by `compare-prices`.
type priceCompareRow struct {
	StoreID          string  `json:"store_id"`
	StoreAddress     string  `json:"store_address"`
	TotalCents       int64   `json:"total_cents"`
	DeliveryFeeCents int64   `json:"delivery_fee_cents"`
	WaitMin          float64 `json:"wait_min"`
}

// newComparePricesCmd builds an identical mini-cart for every store in
// range and ranks by total. This composes store-locator + price-order
// across N stores; only feasible with our local fan-out helper.
func newComparePricesCmd(flags *rootFlags) *cobra.Command {
	var address, city, items, svc string
	cmd := &cobra.Command{
		Use:   "compare-prices",
		Short: "Price the same cart at every nearby store and rank by total",
		Long: `Price the same cart at every nearby store and rank by total.

Builds a one-of-each cart from --items (a comma-separated list of product
codes) and calls /power/price-order against each store from the locator,
then sorts by total including delivery fee.`,
		Example:     "  dominos-pp-cli compare-prices --address \"421 N 63rd St\" --city \"Seattle, WA\" --items 14SCREEN,20BCOKE",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if svc == "" {
				svc = "Delivery"
			}
			codes := splitCSV(items)
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "compare_prices",
					"dry_run": true,
					"address": address,
					"city":    city,
					"items":   codes,
				}, flags)
			}
			// Surface all missing required flags in one error so the
			// verifier's single-probe inferRequiredFlags can supply
			// synthetic values for every one in one pass.
			var missing []string
			if address == "" {
				missing = append(missing, `"address"`)
			}
			if city == "" {
				missing = append(missing, `"city"`)
			}
			if items == "" {
				missing = append(missing, `"items"`)
			}
			if len(missing) > 0 {
				return usageErr(fmt.Errorf("required flag(s) %s not set", strings.Join(missing, ", ")))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			locatorData, err := c.Get("/power/store-locator", map[string]string{
				"s": address, "c": city, "type": svc,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			storeIDs := extractStoreIDs(locatorData)
			if len(storeIDs) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"results": []priceCompareRow{},
					"hint":    "no stores returned by locator; try a broader address",
				}, flags)
			}
			ctx := context.Background()
			results, errs := cliutil.FanoutRun(ctx, storeIDs,
				func(s string) string { return s },
				func(_ context.Context, storeID string) (priceCompareRow, error) {
					return priceCartAtStore(c, storeID, address, codes, svc)
				},
			)
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
			rows := make([]priceCompareRow, 0, len(results))
			for _, r := range results {
				rows = append(rows, r.Value)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].TotalCents < rows[j].TotalCents })
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "Street address (required)")
	cmd.Flags().StringVar(&city, "city", "", "City, state, zip (required)")
	cmd.Flags().StringVar(&items, "items", "", "Comma-separated product codes (required)")
	cmd.Flags().StringVar(&svc, "service", "Delivery", "Service method: Delivery or Carryout")
	return cmd
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// priceCartAtStore builds the minimal Order body the /power/price-order
// endpoint accepts and pulls the totals out. Field names follow the
// Domino's power-API casing (StoreID, OrderChannel, Products, etc.).
func priceCartAtStore(c *client.Client, storeID, address string, codes []string, svc string) (priceCompareRow, error) {
	products := make([]map[string]any, 0, len(codes))
	for _, code := range codes {
		products = append(products, map[string]any{"Code": code, "Qty": 1})
	}
	body := map[string]any{
		"Order": map[string]any{
			"StoreID":       storeID,
			"OrderChannel":  "OLO",
			"OrderMethod":   "Web",
			"ServiceMethod": svc,
			"Products":      products,
			"Address":       map[string]any{"Street": address},
		},
	}
	data, _, err := c.Post("/power/price-order", body)
	if err != nil {
		return priceCompareRow{StoreID: storeID}, err
	}
	row := priceCompareRow{StoreID: storeID}
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		return row, fmt.Errorf("parsing price-order response for %s: %w", storeID, err)
	}
	if order, ok := resp["Order"].(map[string]any); ok {
		if amounts, ok := order["Amounts"].(map[string]any); ok {
			row.TotalCents = dollarsToCents(amounts["Customer"])
			if d, ok := amounts["Delivery"]; ok {
				row.DeliveryFeeCents = dollarsToCents(d)
			}
		}
		if v, ok := order["EstimatedWaitMinutes"]; ok {
			row.WaitMin = numberOf(v)
		}
		if a, ok := order["Address"].(map[string]any); ok {
			if s, ok := a["Street"].(string); ok {
				row.StoreAddress = s
			}
		}
	}
	return row, nil
}

// dollarsToCents converts a JSON number that may be a float-dollars value
// (the power API surface) into an integer cents value the row carries.
func dollarsToCents(v any) int64 {
	f := numberOf(v)
	return int64(f*100 + 0.5)
}
