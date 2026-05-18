package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newMarkupAuditCmd(flags *rootFlags) *cobra.Command {
	var tolerance float64
	var kind string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "markup-audit",
		Short: "Find SKUs whose markup has drifted off the materials-markup tier ladder",
		Long: "Joins every active material and equipment SKU against the materials-markup\n" +
			"tier ladder in the local store and flags the ones whose realised markup\n" +
			"(price-cost)/cost deviates from the tier's expected percent by more than\n" +
			"--tolerance points, plus zero-cost and no-tier SKUs. Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli markup-audit --tolerance 5
  servicetitan-pricebook-pp-cli markup-audit --kind equipment --json
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

			rows, err := pricebook.MarkupAudit(db, tolerance)
			if err != nil {
				return err
			}
			rows = kindFilter(rows, kind, func(r pricebook.MarkupRow) pricebook.SKUKind { return r.Kind })
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []pricebook.MarkupRow{}
			}

			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					string(r.Kind), r.Code, r.DisplayName,
					f2(r.Cost), f2(r.Price), f2(r.ActualPercent), f2(r.TierPercent),
					f2(r.DeltaPercent), f2(r.ExpectedPrice), r.Reason,
				})
			}
			return pbOutput(cmd, flags, rows,
				[]string{"KIND", "CODE", "NAME", "COST", "PRICE", "ACTUAL%", "TIER%", "DELTA%", "EXPECTED", "REASON"},
				table)
		},
	}
	cmd.Flags().Float64Var(&tolerance, "tolerance", 5.0, "Allowed absolute deviation (percentage points) from the tier markup before a SKU is flagged")
	cmd.Flags().StringVar(&kind, "kind", "", "Limit to one SKU kind: material or equipment")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}
