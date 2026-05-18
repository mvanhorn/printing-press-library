package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newCopyAuditCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "copy-audit",
		Short: "Flag SKUs whose name or description is not customer-facing sales copy",
		Long: "Scans active SKUs in the local store for customer-facing text that\n" +
			"reads like internal shorthand: a blank or very short description, a\n" +
			"missing display name, an ALL-CAPS display name, or a name/description\n" +
			"that is just the part code. A sales-copy agent rewrites the flagged\n" +
			"entries; the writeback path is 'materials update' / 'bulk-plan'.\n" +
			"Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli copy-audit
  servicetitan-pricebook-pp-cli copy-audit --kind equipment --json
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

			rows, err := pricebook.CopyAudit(db, pricebook.SKUKind(kind))
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []pricebook.CopyIssue{}
			}

			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					string(r.Kind), r.Code, r.DisplayName, strings.Join(r.Problems, "; "),
				})
			}
			return pbOutput(cmd, flags, rows,
				[]string{"KIND", "CODE", "NAME", "PROBLEMS"}, table)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Limit to one SKU kind: material, equipment, or service")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}
