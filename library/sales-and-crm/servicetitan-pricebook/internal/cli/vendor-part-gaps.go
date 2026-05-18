package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newVendorPartGapsCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "vendor-part-gaps",
		Short: "List materials and equipment missing a primary vendor part number",
		Long: "Scans active materials and equipment in the local store for SKUs whose\n" +
			"primary vendor has no part number, or that have no primary vendor at\n" +
			"all — the 'missing 2M Part #' sweep. The ServiceTitan API has no query\n" +
			"for absent vendor parts. Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli vendor-part-gaps
  servicetitan-pricebook-pp-cli vendor-part-gaps --kind material --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openPricebookStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := pricebook.VendorPartGaps(db, pricebook.SKUKind(kind))
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []pricebook.VendorPartGap{}
			}

			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					string(r.Kind), r.Code, r.DisplayName, r.PrimaryVendorName, r.Reason,
				})
			}
			return pbOutput(cmd, flags, rows,
				[]string{"KIND", "CODE", "NAME", "PRIMARY VENDOR", "REASON"}, table)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Limit to one SKU kind: material or equipment")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}
