package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newExpenseTagCmd(flags *rootFlags) *cobra.Command {
	var category, project, customer, tax, description, refNumber string
	var billable, gstInclusive bool
	cmd := &cobra.Command{
		Use:   "expense-tag <expense_id>",
		Short: "Ergonomic tag wrapper: resolve category/project/customer/tax by name or id and PUT /expenses/{id}",
		Example: strings.Trim(`
  zoho-expense-pp-cli expense-tag 1234567890 --category Travel --project Acme --billable
  zoho-expense-pp-cli expense-tag 1234567890 --tax "GST 18%" --gst-inclusive
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			expenseID := strings.TrimSpace(args[0])
			if expenseID == "" {
				return usageErr(fmt.Errorf("expense_id required"))
			}

			s, err := openZohoStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			body := map[string]any{}
			if category != "" {
				id, _ := resolveExpenseCategory(s.DB(), category)
				body["category_id"] = id
			}
			if project != "" {
				id, _ := resolveProject(s.DB(), project)
				body["project_id"] = id
			}
			if customer != "" {
				id, _ := resolveCustomer(s.DB(), customer)
				body["customer_id"] = id
			}
			if tax != "" {
				id, _, _ := resolveTax(s.DB(), tax)
				body["tax_id"] = id
			}
			if cmd.Flags().Changed("billable") {
				body["is_billable"] = billable
			}
			if cmd.Flags().Changed("gst-inclusive") {
				body["is_inclusive_tax"] = gstInclusive
			}
			if description != "" {
				body["description"] = description
			}
			if refNumber != "" {
				body["reference_number"] = refNumber
			}
			if len(body) == 0 {
				return usageErr(fmt.Errorf("no fields to update; pass at least one of --category, --project, --customer, --billable, --gst-inclusive, --tax, --description, --ref"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, _, err := c.Put(cmd.Context(), "/expenses/"+expenseID, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Category id or name (resolved against local categories store)")
	cmd.Flags().StringVar(&project, "project", "", "Project id or name")
	cmd.Flags().StringVar(&customer, "customer", "", "Customer id or name")
	cmd.Flags().StringVar(&tax, "tax", "", "Tax id or name")
	cmd.Flags().BoolVar(&billable, "billable", false, "Mark expense as billable")
	cmd.Flags().BoolVar(&gstInclusive, "gst-inclusive", false, "Treat amount as inclusive of GST")
	cmd.Flags().StringVar(&description, "description", "", "Free-form description")
	cmd.Flags().StringVar(&refNumber, "ref", "", "Reference number")
	return cmd
}
