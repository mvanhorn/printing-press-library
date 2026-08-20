package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/zohotools"
)

func newGSTSplitCmd(flags *rootFlags) *cobra.Command {
	var emitCSV bool
	var interState bool
	cmd := &cobra.Command{
		Use:         "gst-split <expense_id>",
		Short:       "India: compute CGST+SGST (intra-state) or IGST (inter-state) for an expense",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  zoho-expense-pp-cli gst-split 1234567890
  zoho-expense-pp-cli gst-split 1234567890 --emit-csv
  zoho-expense-pp-cli gst-split 1234567890 --inter-state
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

			expense := expenseFromLocal(s.DB(), expenseID)
			if expense == nil {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				expense, err = fetchExpenseObject(cmd.Context(), c, expenseID)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}

			total := asFloat(expense["total"])
			if total == 0 {
				total = asFloat(expense["amount"])
			}
			taxPct := asFloat(expense["tax_percentage"])
			if taxPct == 0 {
				if taxID, _ := expense["tax_id"].(string); taxID != "" {
					_, _, pct := resolveTax(s.DB(), taxID)
					taxPct = pct
				}
			}

			split := zohotools.ComputeSplit(total, taxPct, !interState)
			split.ExpenseID = expenseID
			split.MerchantName = asStringOpt(expense, "merchant_name")
			split.ExpenseDate = asStringOpt(expense, "expense_date")

			if emitCSV {
				fmt.Fprintf(cmd.OutOrStdout(), "expense_date,expense_id,merchant_name,base_amount,cgst,sgst,igst,total\n")
				fmt.Fprintf(cmd.OutOrStdout(), "%s,%s,%s,%.2f,%.2f,%.2f,%.2f,%.2f\n",
					split.ExpenseDate, split.ExpenseID, csvField(split.MerchantName),
					split.Base, split.CGST, split.SGST, split.IGST, split.Total)
				return nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), split, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"expense %s (%s, %s)\n  base=%.2f cgst=%.2f sgst=%.2f igst=%.2f total=%.2f (tax=%.2f%%, intra_state=%t)\n",
				split.ExpenseID, split.MerchantName, split.ExpenseDate,
				split.Base, split.CGST, split.SGST, split.IGST, split.Total, split.TaxPct, split.IntraState)
			return nil
		},
	}
	cmd.Flags().BoolVar(&emitCSV, "emit-csv", false, "Emit one CSV row to stdout")
	cmd.Flags().BoolVar(&interState, "inter-state", false, "Treat as inter-state (IGST) instead of intra-state (CGST+SGST)")
	return cmd
}

func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
