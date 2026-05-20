package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newOrphanSkusCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "orphan-skus",
		Short: "List SKUs assigned to inactive or non-existent categories",
		Long: "Joins materials, equipment, and services against the synced categories\n" +
			"and reports SKUs whose category assignment is broken: pointing at an\n" +
			"inactive category, a category ID not in the store, or having no\n" +
			"category at all. The join is impossible in one ServiceTitan API call.\n" +
			"Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli orphan-skus
  servicetitan-pricebook-pp-cli orphan-skus --kind service --json
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

			rows, err := pricebook.OrphanSKUs(db)
			if err != nil {
				return err
			}
			rows = kindFilter(rows, kind, func(r pricebook.OrphanSKU) pricebook.SKUKind { return r.Kind })
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []pricebook.OrphanSKU{}
			}

			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					string(r.Kind), r.Code, r.DisplayName, i64(r.CategoryID), r.Reason,
				})
			}
			return pbOutput(cmd, flags, rows,
				[]string{"KIND", "CODE", "NAME", "CATEGORY ID", "REASON"}, table)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Limit to one SKU kind: material, equipment, or service")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}
