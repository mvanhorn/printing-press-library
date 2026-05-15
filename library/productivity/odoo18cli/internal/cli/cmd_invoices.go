// Copyright 2026 andreampiovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/odoo18cli/internal/odoo"
)

var invoiceFields = []string{
	"id", "name", "partner_id", "date", "invoice_date", "invoice_date_due",
	"amount_total", "amount_residual", "currency_id",
	"state", "payment_state", "move_type",
}

func newInvoicesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoices",
		Short: "Manage Odoo invoices and bills",
	}
	cmd.AddCommand(newInvoicesListCmd(flags))
	cmd.AddCommand(newInvoicesGetCmd(flags))
	cmd.AddCommand(newInvoicesUnpaidCmd(flags))
	cmd.AddCommand(newInvoicesOverdueCmd(flags))
	return cmd
}

func newInvoicesListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var offset int
	var moveType string
	var domain string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List invoices and bills",
		Example: `  odoo18cli-pp-cli invoices list
  odoo18cli-pp-cli invoices list --type out_invoice --limit 20
  odoo18cli-pp-cli invoices list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			d := parseOptionalDomain(domain)
			if moveType != "" {
				d = append(d, []interface{}{"move_type", "=", moveType})
			}
			results, err := c.SearchRead("account.move", d, invoiceFields, limit, offset, "invoice_date desc")
			if err != nil {
				return fmt.Errorf("fetching invoices: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			printInvoiceTable(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 80, "Max records to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Records to skip")
	cmd.Flags().StringVar(&moveType, "type", "", "Filter by type: out_invoice, in_invoice, out_refund, in_refund")
	cmd.Flags().StringVar(&domain, "domain", "", "Odoo domain filter as JSON")
	return cmd
}

func newInvoicesGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get an invoice by ID",
		Example: `  odoo18cli-pp-cli invoices get 42
  odoo18cli-pp-cli invoices get 42 --json`,
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
			results, err := c.Read("account.move", []int{id}, invoiceFields)
			if err != nil {
				return fmt.Errorf("fetching invoice: %w", err)
			}
			if len(results) == 0 {
				return fmt.Errorf("invoice %d not found", id)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results[0])
			}
			r := results[0]
			fmt.Fprintf(cmd.OutOrStdout(), "ID:            %v\n", r["id"])
			fmt.Fprintf(cmd.OutOrStdout(), "Reference:     %s\n", odoo.StringVal(r["name"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Partner:       %s\n", odoo.NameFromMany2one(r["partner_id"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Type:          %s\n", odoo.StringVal(r["move_type"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Date:          %s\n", odoo.StringVal(r["invoice_date"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Due:           %s\n", odoo.StringVal(r["invoice_date_due"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Total:         %.2f %s\n", odoo.FloatVal(r["amount_total"]), odoo.NameFromMany2one(r["currency_id"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Residual:      %.2f\n", odoo.FloatVal(r["amount_residual"]))
			fmt.Fprintf(cmd.OutOrStdout(), "State:         %s\n", odoo.StringVal(r["state"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Payment state: %s\n", odoo.StringVal(r["payment_state"]))
			return nil
		},
	}
	return cmd
}

func newInvoicesUnpaidCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var moveType string

	cmd := &cobra.Command{
		Use:   "unpaid",
		Short: "List posted invoices not yet fully paid",
		Example: `  odoo18cli-pp-cli invoices unpaid
  odoo18cli-pp-cli invoices unpaid --type in_invoice --json`,
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
				[]interface{}{"state", "=", "posted"},
				[]interface{}{"payment_state", "in", []interface{}{"not_paid", "partial"}},
			}
			if moveType != "" {
				domain = append(domain, []interface{}{"move_type", "=", moveType})
			} else {
				domain = append(domain, []interface{}{"move_type", "in", []interface{}{"out_invoice", "in_invoice"}})
			}
			results, err := c.SearchRead("account.move", domain, invoiceFields, limit, 0, "invoice_date_due asc")
			if err != nil {
				return fmt.Errorf("fetching unpaid invoices: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			printInvoiceTable(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 80, "Max records to return")
	cmd.Flags().StringVar(&moveType, "type", "", "Filter by type: out_invoice (customer), in_invoice (vendor)")
	return cmd
}

func newInvoicesOverdueCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var moveType string

	cmd := &cobra.Command{
		Use:   "overdue",
		Short: "List unpaid invoices past their due date",
		Example: `  odoo18cli-pp-cli invoices overdue
  odoo18cli-pp-cli invoices overdue --type out_invoice --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			today := time.Now().UTC().Format("2006-01-02")
			domain := []interface{}{
				[]interface{}{"state", "=", "posted"},
				[]interface{}{"payment_state", "in", []interface{}{"not_paid", "partial"}},
				[]interface{}{"invoice_date_due", "<", today},
			}
			if moveType != "" {
				domain = append(domain, []interface{}{"move_type", "=", moveType})
			} else {
				domain = append(domain, []interface{}{"move_type", "in", []interface{}{"out_invoice", "in_invoice"}})
			}
			results, err := c.SearchRead("account.move", domain, invoiceFields, limit, 0, "invoice_date_due asc")
			if err != nil {
				return fmt.Errorf("fetching overdue invoices: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			printInvoiceTable(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 80, "Max records to return")
	cmd.Flags().StringVar(&moveType, "type", "", "Filter by type: out_invoice (customer), in_invoice (vendor)")
	return cmd
}

func printInvoiceTable(out io.Writer, records []map[string]interface{}) {
	if len(records) == 0 {
		fmt.Fprintln(out, "No invoices found.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tREF\tTYPE\tPARTNER\tDATE\tDUE\tTOTAL\tRESIDUAL\tSTATUS")
	for _, r := range records {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%.2f\t%.2f\t%s\n",
			odoo.IntVal(r["id"]),
			truncate(odoo.StringVal(r["name"]), 20),
			odoo.StringVal(r["move_type"]),
			truncate(odoo.NameFromMany2one(r["partner_id"]), 25),
			odoo.StringVal(r["invoice_date"]),
			odoo.StringVal(r["invoice_date_due"]),
			odoo.FloatVal(r["amount_total"]),
			odoo.FloatVal(r["amount_residual"]),
			odoo.StringVal(r["payment_state"]),
		)
	}
	w.Flush()
}
