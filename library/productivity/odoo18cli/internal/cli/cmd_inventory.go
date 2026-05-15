// Copyright 2026 andreampiovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/odoo18cli/internal/odoo"
)

func newInventoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Manage Odoo stock inventory",
	}
	cmd.AddCommand(newInventoryListCmd(flags))
	cmd.AddCommand(newInventoryBalanceCmd(flags))
	return cmd
}

func newInventoryListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var domain string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stock quantities (on-hand inventory)",
		Example: `  odoo18cli-pp-cli inventory list
  odoo18cli-pp-cli inventory list --limit 50 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			d := parseOptionalDomain(domain)
			// Default to internal locations only
			if domain == "" {
				d = []interface{}{[]interface{}{"location_id.usage", "=", "internal"}}
			}
			fields := []string{"id", "product_id", "location_id", "lot_id", "quantity", "reserved_quantity"}
			results, err := c.SearchRead("stock.quant", d, fields, limit, 0, "product_id asc")
			if err != nil {
				return fmt.Errorf("fetching inventory: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			printInventoryTable(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 200, "Max records to return")
	cmd.Flags().StringVar(&domain, "domain", "", "Odoo domain filter as JSON")
	return cmd
}

func newInventoryBalanceCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Aggregate on-hand stock by product",
		Example: `  odoo18cli-pp-cli inventory balance
  odoo18cli-pp-cli inventory balance --limit 50 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			domain := []interface{}{[]interface{}{"location_id.usage", "=", "internal"}}
			fields := []string{"product_id", "quantity", "reserved_quantity"}
			results, err := c.SearchRead("stock.quant", domain, fields, 0, 0, "product_id asc")
			if err != nil {
				return fmt.Errorf("fetching inventory: %w", err)
			}

			// Aggregate by product
			type balance struct {
				ProductName string
				OnHand      float64
				Reserved    float64
			}
			balMap := map[string]*balance{}
			for _, r := range results {
				name := odoo.NameFromMany2one(r["product_id"])
				if name == "" {
					name = fmt.Sprintf("product/%d", odoo.IntVal(r["id"]))
				}
				b := balMap[name]
				if b == nil {
					balMap[name] = &balance{ProductName: name}
					b = balMap[name]
				}
				b.OnHand += odoo.FloatVal(r["quantity"])
				b.Reserved += odoo.FloatVal(r["reserved_quantity"])
			}

			// Sort by product name and apply limit
			keys := make([]string, 0, len(balMap))
			for k := range balMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if limit > 0 && limit < len(keys) {
				keys = keys[:limit]
			}

			if flags.asJSON {
				out := make([]map[string]interface{}, 0, len(keys))
				for _, k := range keys {
					b := balMap[k]
					out = append(out, map[string]interface{}{
						"product":  b.ProductName,
						"on_hand":  b.OnHand,
						"reserved": b.Reserved,
						"available": b.OnHand - b.Reserved,
					})
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PRODUCT\tON HAND\tRESERVED\tAVAILABLE")
			for _, k := range keys {
				b := balMap[k]
				fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.2f\n",
					truncate(b.ProductName, 40),
					b.OnHand, b.Reserved, b.OnHand-b.Reserved)
			}
			w.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Max products to show (0 = all)")
	return cmd
}

func printInventoryTable(out io.Writer, records []map[string]interface{}) {
	if len(records) == 0 {
		fmt.Fprintln(out, "No stock quantities found.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPRODUCT\tLOCATION\tLOT\tON HAND\tRESERVED")
	for _, r := range records {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%.2f\t%.2f\n",
			odoo.IntVal(r["id"]),
			truncate(odoo.NameFromMany2one(r["product_id"]), 35),
			truncate(odoo.NameFromMany2one(r["location_id"]), 25),
			odoo.NameFromMany2one(r["lot_id"]),
			odoo.FloatVal(r["quantity"]),
			odoo.FloatVal(r["reserved_quantity"]),
		)
	}
	w.Flush()
}
