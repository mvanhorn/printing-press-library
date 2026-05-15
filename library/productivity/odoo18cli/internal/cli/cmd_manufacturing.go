// Copyright 2026 andreampiovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/odoo18cli/internal/odoo"
)

var moFields = []string{
	"id", "name", "product_id", "product_qty", "product_uom_id",
	"bom_id", "date_start", "date_finished", "state", "user_id",
}

func newManufacturingNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manufacturing",
		Short: "Manage manufacturing orders",
	}
	cmd.AddCommand(newManufacturingListCmd(flags))
	cmd.AddCommand(newManufacturingGetCmd(flags))
	cmd.AddCommand(newManufacturingWipCmd(flags))
	return cmd
}

func newManufacturingListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var state string
	var domain string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List manufacturing orders",
		Example: `  odoo18cli-pp-cli manufacturing list
  odoo18cli-pp-cli manufacturing list --state confirmed --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			d := parseOptionalDomain(domain)
			if state != "" {
				d = append(d, []interface{}{"state", "=", state})
			}
			results, err := c.SearchRead("mrp.production", d, moFields, limit, 0, "date_start desc")
			if err != nil {
				return fmt.Errorf("fetching manufacturing orders: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			printMOTable(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 80, "Max records to return")
	cmd.Flags().StringVar(&state, "state", "", "Filter by state: draft, confirmed, progress, to_close, done, cancel")
	cmd.Flags().StringVar(&domain, "domain", "", "Odoo domain filter as JSON")
	return cmd
}

func newManufacturingGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a manufacturing order by ID",
		Example: `  odoo18cli-pp-cli manufacturing get 42
  odoo18cli-pp-cli manufacturing get 42 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt(args[0])
			if err != nil {
				return fmt.Errorf("invalid ID %q", args[0])
			}
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			results, err := c.Read("mrp.production", []int{id}, moFields)
			if err != nil {
				return fmt.Errorf("fetching MO: %w", err)
			}
			if len(results) == 0 {
				return fmt.Errorf("manufacturing order %d not found", id)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results[0])
			}
			r := results[0]
			fmt.Fprintf(cmd.OutOrStdout(), "ID:        %v\n", r["id"])
			fmt.Fprintf(cmd.OutOrStdout(), "Reference: %s\n", odoo.StringVal(r["name"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Product:   %s\n", odoo.NameFromMany2one(r["product_id"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Qty:       %.2f %s\n", odoo.FloatVal(r["product_qty"]), odoo.NameFromMany2one(r["product_uom_id"]))
			fmt.Fprintf(cmd.OutOrStdout(), "BOM:       %s\n", odoo.NameFromMany2one(r["bom_id"]))
			fmt.Fprintf(cmd.OutOrStdout(), "State:     %s\n", odoo.StringVal(r["state"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Start:     %s\n", odoo.StringVal(r["date_start"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Finished:  %s\n", odoo.StringVal(r["date_finished"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Resp.:     %s\n", odoo.NameFromMany2one(r["user_id"]))
			return nil
		},
	}
	return cmd
}

func newManufacturingWipCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "wip",
		Short: "List work-in-progress manufacturing orders (confirmed or in progress)",
		Example: `  odoo18cli-pp-cli manufacturing wip
  odoo18cli-pp-cli manufacturing wip --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			domain := []interface{}{
				[]interface{}{"state", "in", []interface{}{"confirmed", "progress", "to_close"}},
			}
			results, err := c.SearchRead("mrp.production", domain, moFields, limit, 0, "date_start asc")
			if err != nil {
				return fmt.Errorf("fetching WIP: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			printMOTable(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 200, "Max records to return")
	return cmd
}

func printMOTable(out io.Writer, records []map[string]interface{}) {
	if len(records) == 0 {
		fmt.Fprintln(out, "No manufacturing orders found.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tREF\tSTATE\tPRODUCT\tQTY\tSTART")
	for _, r := range records {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%.2f\t%s\n",
			odoo.IntVal(r["id"]),
			odoo.StringVal(r["name"]),
			odoo.StringVal(r["state"]),
			truncate(odoo.NameFromMany2one(r["product_id"]), 35),
			odoo.FloatVal(r["product_qty"]),
			odoo.StringVal(r["date_start"]),
		)
	}
	w.Flush()
}
