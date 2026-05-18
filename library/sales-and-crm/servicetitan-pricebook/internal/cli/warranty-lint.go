package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newWarrantyLintCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "warranty-lint",
		Short: "Flag equipment whose warranty text breaks JKA attribution rules",
		Long: "Lints active equipment in the local store: a manufacturer warranty\n" +
			"description must lead with \"Manufacturer's\" so it is clearly not JKA's\n" +
			"own warranty, must not be blank when a duration is set, and a JKA\n" +
			"service-provider warranty (the standard 1-year parts & labor offering)\n" +
			"should be recorded. Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli warranty-lint
  servicetitan-pricebook-pp-cli warranty-lint --json
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

			rows, err := pricebook.WarrantyLint(db)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []pricebook.WarrantyIssue{}
			}

			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					i64(r.ID), r.Code, r.DisplayName, strings.Join(r.Problems, "; "),
				})
			}
			return pbOutput(cmd, flags, rows,
				[]string{"ID", "CODE", "NAME", "PROBLEMS"}, table)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}
