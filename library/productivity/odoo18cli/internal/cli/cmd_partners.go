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

var partnerFields = []string{
	"id", "name", "email", "phone", "is_company",
	"customer_rank", "supplier_rank", "vat",
	"street", "city", "country_id",
}

func newPartnersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "partners",
		Short: "Manage Odoo partners (customers and vendors)",
	}
	cmd.AddCommand(newPartnersListCmd(flags))
	cmd.AddCommand(newPartnersCustomersCmd(flags))
	cmd.AddCommand(newPartnersSuppliersCmd(flags))
	cmd.AddCommand(newPartnersGetCmd(flags))
	return cmd
}

func newPartnersListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var offset int
	var domain string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all partners",
		Example: `  odoo18cli-pp-cli partners list
  odoo18cli-pp-cli partners list --limit 20 --json
  odoo18cli-pp-cli partners list --domain '[["is_company","=",true]]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			d := parseOptionalDomain(domain)
			results, err := c.SearchRead("res.partner", d, partnerFields, limit, offset, "name asc")
			if err != nil {
				return fmt.Errorf("fetching partners: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			printPartnerTable(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 80, "Max records to return (0 = all)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Records to skip")
	cmd.Flags().StringVar(&domain, "domain", "", "Odoo domain filter as JSON")
	return cmd
}

func newPartnersCustomersCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "customers",
		Short: "List partners with customer_rank > 0",
		Example: `  odoo18cli-pp-cli partners customers
  odoo18cli-pp-cli partners customers --limit 50 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			domain := []interface{}{[]interface{}{"customer_rank", ">", 0}}
			results, err := c.SearchRead("res.partner", domain, partnerFields, limit, 0, "name asc")
			if err != nil {
				return fmt.Errorf("fetching customers: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			printPartnerTable(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 80, "Max records to return")
	return cmd
}

func newPartnersSuppliersCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "suppliers",
		Short: "List partners with supplier_rank > 0",
		Example: `  odoo18cli-pp-cli partners suppliers
  odoo18cli-pp-cli partners suppliers --limit 50 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := odoo.NewFromFlags(flags.odooURL, flags.odooDB, flags.odooUser)
			if err != nil {
				return err
			}
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			domain := []interface{}{[]interface{}{"supplier_rank", ">", 0}}
			results, err := c.SearchRead("res.partner", domain, partnerFields, limit, 0, "name asc")
			if err != nil {
				return fmt.Errorf("fetching suppliers: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			printPartnerTable(cmd.OutOrStdout(), results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 80, "Max records to return")
	return cmd
}

func newPartnersGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a partner by ID",
		Example: `  odoo18cli-pp-cli partners get 42
  odoo18cli-pp-cli partners get 42 --json`,
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
			results, err := c.Read("res.partner", []int{id}, partnerFields)
			if err != nil {
				return fmt.Errorf("fetching partner: %w", err)
			}
			if len(results) == 0 {
				return fmt.Errorf("partner %d not found", id)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results[0])
			}
			r := results[0]
			fmt.Fprintf(cmd.OutOrStdout(), "ID:           %v\n", r["id"])
			fmt.Fprintf(cmd.OutOrStdout(), "Name:         %s\n", odoo.StringVal(r["name"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Company:      %v\n", odoo.BoolVal(r["is_company"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Email:        %s\n", odoo.StringVal(r["email"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Phone:        %s\n", odoo.StringVal(r["phone"]))
			fmt.Fprintf(cmd.OutOrStdout(), "VAT:          %s\n", odoo.StringVal(r["vat"]))
			fmt.Fprintf(cmd.OutOrStdout(), "City:         %s\n", odoo.StringVal(r["city"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Country:      %s\n", odoo.NameFromMany2one(r["country_id"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Cust. rank:   %d\n", odoo.IntVal(r["customer_rank"]))
			fmt.Fprintf(cmd.OutOrStdout(), "Suppl. rank:  %d\n", odoo.IntVal(r["supplier_rank"]))
			return nil
		},
	}
	return cmd
}

func printPartnerTable(out io.Writer, records []map[string]interface{}) {
	if len(records) == 0 {
		fmt.Fprintln(out, "No partners found.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tCOMPANY\tEMAIL\tCUST\tVEND\tCITY")
	for _, r := range records {
		company := "person"
		if odoo.BoolVal(r["is_company"]) {
			company = "company"
		}
		cust := odoo.IntVal(r["customer_rank"])
		vend := odoo.IntVal(r["supplier_rank"])
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%d\t%s\n",
			odoo.IntVal(r["id"]),
			truncate(odoo.StringVal(r["name"]), 35),
			company,
			truncate(odoo.StringVal(r["email"]), 30),
			cust, vend,
			odoo.StringVal(r["city"]),
		)
	}
	w.Flush()
}
