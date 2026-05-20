package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newQuoteReconcileCmd(flags *rootFlags) *cobra.Command {
	var format string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "quote-reconcile <file>",
		Short: "Diff a vendor cost file against current pricebook costs",
		Long: "Reads a vendor cost file — a CSV, or the JSON Claude extracts from a\n" +
			"quote / order confirmation / invoice PDF — and matches each line\n" +
			"against the synced pricebook by vendor part number, checking the\n" +
			"primary vendor first then other vendors. Prints a no-write cost diff;\n" +
			"unmatched lines are reported, never dropped. Run 'sync' first.\n\n" +
			"CSV header: a vendor-part column (vendor_part / part / sku) and a cost\n" +
			"column (cost / price / unit_cost), with optional description / line_ref.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli quote-reconcile ./2m-quote-2026-05.csv
  servicetitan-pricebook-pp-cli quote-reconcile ./invoice.json --format json --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			lines, err := pricebook.ParseQuoteFile(args[0], format)
			if err != nil {
				return err
			}
			db, err := openPricebookStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := pricebook.Reconcile(db, lines)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []pricebook.ReconcileRow{}
			}

			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				reason := r.Reason
				if r.Matched && reason == "" {
					reason = "matched"
				}
				table = append(table, []string{
					r.VendorPart, strconv.FormatBool(r.Matched), string(r.Kind),
					r.Code, r.DisplayName, f2(r.CurrentCost), f2(r.QuoteCost),
					f2(r.CostDelta), reason,
				})
			}
			return pbOutput(cmd, flags, rows,
				[]string{"VENDOR PART", "MATCHED", "KIND", "CODE", "NAME", "CURRENT COST", "QUOTE COST", "COST D", "REASON"},
				table)
		},
	}
	cmd.Flags().StringVar(&format, "format", "auto", "Quote file format: auto, csv, or json")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}
