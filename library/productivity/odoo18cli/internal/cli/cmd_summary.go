// Copyright 2026 andreampiovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/odoo18cli/internal/odoo"
)

func newSummaryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Structured KPI snapshot for Claude pipelines",
		Long: `Outputs a structured JSON summary of Odoo KPIs from a live query:
  - Open sales orders count and total value
  - WIP manufacturing orders count
  - Unpaid customer invoices count and total residual
  - Overdue invoices count
  - Low-stock products (on_hand <= 0)

Designed for piping to Claude for analysis:
  odoo18cli-pp-cli summary | claude -p "What is the state of our business?"`,
		Example: `  odoo18cli-pp-cli summary
  odoo18cli-pp-cli summary --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

			kpis := map[string]interface{}{}

			// Open sales orders
			soCount, err := c.SearchCount("sale.order", []interface{}{
				[]interface{}{"state", "in", []interface{}{"draft", "sent", "sale"}},
			})
			if err == nil {
				kpis["open_sales_orders"] = soCount
			}

			// WIP manufacturing orders
			wipCount, err := c.SearchCount("mrp.production", []interface{}{
				[]interface{}{"state", "in", []interface{}{"confirmed", "progress", "to_close"}},
			})
			if err == nil {
				kpis["wip_manufacturing_orders"] = wipCount
			}

			// Unpaid customer invoices
			unpaidInvoices, err := c.SearchRead("account.move", []interface{}{
				[]interface{}{"state", "=", "posted"},
				[]interface{}{"payment_state", "in", []interface{}{"not_paid", "partial"}},
				[]interface{}{"move_type", "=", "out_invoice"},
			}, []string{"id", "amount_residual"}, 0, 0, "id asc")
			if err == nil {
				totalResidual := 0.0
				for _, inv := range unpaidInvoices {
					totalResidual += odoo.FloatVal(inv["amount_residual"])
				}
				kpis["unpaid_customer_invoices"] = len(unpaidInvoices)
				kpis["unpaid_customer_invoices_total"] = totalResidual
			}

			// Overdue invoices
			today := time.Now().UTC().Format("2006-01-02")
			overdueCount, err := c.SearchCount("account.move", []interface{}{
				[]interface{}{"state", "=", "posted"},
				[]interface{}{"payment_state", "in", []interface{}{"not_paid", "partial"}},
				[]interface{}{"invoice_date_due", "<", today},
				[]interface{}{"move_type", "in", []interface{}{"out_invoice", "in_invoice"}},
			})
			if err == nil {
				kpis["overdue_invoices"] = overdueCount
			}

			// Low/out-of-stock products (on_hand <= 0 in internal locations)
			lowStockCount, err := c.SearchCount("stock.quant", []interface{}{
				[]interface{}{"location_id.usage", "=", "internal"},
				[]interface{}{"quantity", "<=", 0},
			})
			if err == nil {
				kpis["low_stock_products"] = lowStockCount
			}

			kpis["as_of"] = time.Now().UTC().Format(time.RFC3339)
			kpis["odoo_url"] = c.URL
			kpis["database"] = c.DB

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(kpis)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Odoo KPI Summary — %s (%s)\n\n", c.DB, c.URL)
			if v, ok := kpis["open_sales_orders"]; ok {
				fmt.Fprintf(w, "  Open sales orders:              %v\n", v)
			}
			if v, ok := kpis["wip_manufacturing_orders"]; ok {
				fmt.Fprintf(w, "  WIP manufacturing orders:       %v\n", v)
			}
			if v, ok := kpis["unpaid_customer_invoices"]; ok {
				fmt.Fprintf(w, "  Unpaid customer invoices:       %v (total residual: %.2f)\n", v, kpis["unpaid_customer_invoices_total"])
			}
			if v, ok := kpis["overdue_invoices"]; ok {
				fmt.Fprintf(w, "  Overdue invoices:               %v\n", v)
			}
			if v, ok := kpis["low_stock_products"]; ok {
				fmt.Fprintf(w, "  Products with zero/neg stock:   %v\n", v)
			}
			return nil
		},
	}
	return cmd
}
