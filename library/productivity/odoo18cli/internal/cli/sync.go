// Copyright 2026 andreampiovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/odoo18cli/internal/odoo"
)

// newSyncCmd verifies Odoo connectivity and reports live record counts from
// the main business models. odoo18cli is a live-query CLI with no local cache,
// so "sync" here means "authenticate and sample data from Odoo".
func newSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Verify Odoo connectivity and report live record counts",
		Long: `Authenticates with Odoo and performs live queries to report current
record counts from the main business models. No data is cached locally.

Use to confirm credentials and connectivity:
  odoo18cli-pp-cli sync --url https://myodoo.example.com --db mydb --user admin`,
		Example: `  odoo18cli-pp-cli sync
  odoo18cli-pp-cli sync --json
  odoo18cli-pp-cli sync --url https://myodoo.example.com --db mydb`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			progress := cmd.ErrOrStderr()
			if flags.asJSON {
				progress = cmd.ErrOrStderr()
			}
			fmt.Fprintf(progress, "Authenticating with %s...\n", c.URL)
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			fmt.Fprintf(progress, "Authenticated as UID %d\n", c.UID)

			counts := map[string]interface{}{}

			// Active partners (all types)
			domain := []interface{}{}
			partners, err := c.SearchRead("res.partner", domain,
				[]string{"id", "customer_rank", "supplier_rank"}, 0, 0, "id asc")
			if err == nil {
				customers, suppliers := 0, 0
				for _, p := range partners {
					if odoo.IntVal(p["customer_rank"]) > 0 {
						customers++
					}
					if odoo.IntVal(p["supplier_rank"]) > 0 {
						suppliers++
					}
				}
				counts["partners"] = len(partners)
				counts["customers"] = customers
				counts["suppliers"] = suppliers
			}

			// Open sales orders
			domain = []interface{}{[]interface{}{"state", "in", []interface{}{"draft", "sent", "sale"}}}
			salesOrders, err := c.SearchRead("sale.order", domain,
				[]string{"id", "amount_total"}, 0, 0, "id asc")
			if err == nil {
				total := 0.0
				for _, so := range salesOrders {
					total += odoo.FloatVal(so["amount_total"])
				}
				counts["open_sales_orders"] = len(salesOrders)
				counts["open_sales_total"] = total
			}

			// WIP manufacturing orders
			domain = []interface{}{[]interface{}{"state", "in", []interface{}{"confirmed", "progress", "to_close"}}}
			wip, err := c.SearchRead("mrp.production", domain,
				[]string{"id"}, 0, 0, "id asc")
			if err == nil {
				counts["wip_manufacturing"] = len(wip)
			}

			// Unpaid customer and vendor invoices
			domain = []interface{}{
				[]interface{}{"state", "=", "posted"},
				[]interface{}{"payment_state", "in", []interface{}{"not_paid", "partial"}},
				[]interface{}{"move_type", "in", []interface{}{"out_invoice", "in_invoice"}},
			}
			unpaid, err := c.SearchRead("account.move", domain,
				[]string{"id", "amount_residual"}, 0, 0, "id asc")
			if err == nil {
				residual := 0.0
				for _, inv := range unpaid {
					residual += odoo.FloatVal(inv["amount_residual"])
				}
				counts["unpaid_invoices"] = len(unpaid)
				counts["unpaid_residual"] = residual
			}

			result := map[string]interface{}{
				"status":     "ok",
				"url":        c.URL,
				"database":   c.DB,
				"counts":     counts,
				"queried_at": time.Now().UTC().Format(time.RFC3339),
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\nOdoo instance: %s (db: %s)\n\n", c.URL, c.DB)
			if v, ok := counts["partners"]; ok {
				fmt.Fprintf(w, "  Partners:           %v (customers: %v, suppliers: %v)\n",
					v, counts["customers"], counts["suppliers"])
			}
			if v, ok := counts["open_sales_orders"]; ok {
				fmt.Fprintf(w, "  Open sales orders:  %v (total: %.2f)\n",
					v, counts["open_sales_total"])
			}
			if v, ok := counts["wip_manufacturing"]; ok {
				fmt.Fprintf(w, "  WIP manufacturing:  %v\n", v)
			}
			if v, ok := counts["unpaid_invoices"]; ok {
				fmt.Fprintf(w, "  Unpaid invoices:    %v (residual: %.2f)\n",
					v, counts["unpaid_residual"])
			}
			return nil
		},
	}
	return cmd
}
