// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type attributedProduct struct {
	Title        string  `json:"title"`
	SKU          string  `json:"sku,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	TotalRevenue float64 `json:"total_revenue"`
	TotalOrders  int     `json:"total_orders"`
	UnitsSold    int     `json:"units_sold"`
}

type attributionResult struct {
	CampaignID     string              `json:"campaign_id"`
	StoreID        string              `json:"store_id,omitempty"`
	TotalRevenue   float64             `json:"total_revenue"`
	TotalOrders    int                 `json:"total_orders"`
	UniqueOpens    int                 `json:"unique_opens"`
	ConversionRate float64             `json:"conversion_rate"`
	TopProducts    []attributedProduct `json:"top_products"`
}

func newAttributionCmd(flags *rootFlags) *cobra.Command {
	var storeID string

	cmd := &cobra.Command{
		Use:   "attribution <campaign-id>",
		Short: "Per-campaign revenue attribution: total attributed revenue, top products, conversion rate (orders / unique opens).",
		Long: `Joins /reports/{id}/ecommerce-product-activity with /reports/{id} to compute:
  - Total attributed revenue and order count
  - Top products by attributed revenue
  - Conversion rate (orders divided by unique opens)

Mailchimp's dashboard surfaces these in the campaign reporting view but doesn't
expose the join via a single API call.`,
		Example: `  mailchimp-pp-cli attribution 7f8a9b0c1d
  mailchimp-pp-cli attribution 7f8a9b0c1d --store mystore
  mailchimp-pp-cli attribution 7f8a9b0c1d --json --select top_products`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			cid := args[0]
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_fetch": []string{
						fmt.Sprintf("/reports/%s", cid),
						fmt.Sprintf("/reports/%s/ecommerce-product-activity", cid),
					},
					"store_filter": storeID,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			report, err := c.Get(fmt.Sprintf("/reports/%s", cid), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			products, err := c.Get(fmt.Sprintf("/reports/%s/ecommerce-product-activity", cid), nil)
			if err != nil {
				// Non-ecommerce campaigns 404 here. Surface a hint rather than failing.
				return classifyAPIError(err, flags)
			}

			result := attributionResult{CampaignID: cid, StoreID: storeID}
			var rep map[string]any
			_ = json.Unmarshal(report, &rep)
			if ec, ok := rep["ecommerce"].(map[string]any); ok {
				if v, ok := ec["total_revenue"].(float64); ok {
					result.TotalRevenue = v
				}
				if v, ok := ec["total_orders"].(float64); ok {
					result.TotalOrders = int(v)
				}
			}
			if v, ok := rep["unique_opens"].(float64); ok {
				result.UniqueOpens = int(v)
			}
			if result.UniqueOpens > 0 {
				result.ConversionRate = float64(result.TotalOrders) / float64(result.UniqueOpens)
			}

			var pr map[string]any
			_ = json.Unmarshal(products, &pr)
			if items, ok := pr["products"].([]any); ok {
				for _, p := range items {
					m, _ := p.(map[string]any)
					if m == nil {
						continue
					}
					if storeID != "" {
						if sid, ok := m["store_id"].(string); !ok || sid != storeID {
							continue
						}
					}
					prod := attributedProduct{}
					if v, ok := m["title"].(string); ok {
						prod.Title = v
					}
					if v, ok := m["sku"].(string); ok {
						prod.SKU = v
					}
					if v, ok := m["currency_code"].(string); ok {
						prod.Currency = v
					}
					if v, ok := m["total_revenue"].(float64); ok {
						prod.TotalRevenue = v
					}
					if v, ok := m["total_orders"].(float64); ok {
						prod.TotalOrders = int(v)
					}
					if v, ok := m["total_purchased"].(float64); ok {
						prod.UnitsSold = int(v)
					}
					if prod.Title != "" {
						result.TopProducts = append(result.TopProducts, prod)
					}
				}
			}
			sort.Slice(result.TopProducts, func(i, j int) bool { return result.TopProducts[i].TotalRevenue > result.TopProducts[j].TotalRevenue })
			if len(result.TopProducts) > 10 {
				result.TopProducts = result.TopProducts[:10]
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&storeID, "store", "", "Filter attribution to a specific store ID")
	return cmd
}
