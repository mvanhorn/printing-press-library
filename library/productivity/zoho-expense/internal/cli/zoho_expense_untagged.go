package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/zohotools"
)

type untaggedRow struct {
	ExpenseID    string  `json:"expense_id"`
	MerchantName string  `json:"merchant_name"`
	Amount       float64 `json:"amount"`
	ExpenseDate  string  `json:"expense_date"`
	Status       string  `json:"status"`
}

func newExpenseUntaggedCmd(flags *rootFlags) *cobra.Command {
	var autoFix bool
	cmd := &cobra.Command{
		Use:         "expense-untagged",
		Short: "List expenses missing category_id; with --auto-fix, apply merchant memory to each",
		// No mcp:read-only annotation: --auto-fix PUTs updated category_ids back to Zoho.
		Example: strings.Trim(`
  zoho-expense-pp-cli expense-untagged
  zoho-expense-pp-cli expense-untagged --auto-fix
  zoho-expense-pp-cli expense-untagged --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			// PATCH(2026-05-23): when --auto-fix is set, refresh the
			// expenses table first. Without this, the local store can be
			// up to cache.stale_after hours behind Zoho — any expense the
			// user manually tagged in the Zoho UI since the last sync
			// still appears as category_id='' here, and --auto-fix would
			// silently overwrite the user's manual choice with the
			// merchant-memory category. Read-only listing (no --auto-fix)
			// skips the refresh: stale data is acceptable for review
			// because it doesn't mutate anything, and a 1-hop hint via
			// the normal freshness machinery covers users who care.
			// Filed per Greptile P1.
			if autoFix {
				if err := forceRefreshExpenses(cmd.Context(), flags); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: pre-auto-fix refresh failed (%v); falling back to local store may overwrite categories set in Zoho since last sync\n", err)
				}
			} else {
				meta := ensureFreshForResources(cmd.Context(), flags, "expenses")
				if meta.Decision != "fresh" && meta.Error != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: expenses table refresh hit %s — local data may be stale\n", meta.Error)
				}
			}

			s, err := openZohoStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := s.DB().Query(
				`SELECT COALESCE(expense_id,id), COALESCE(merchant_name,''), COALESCE(amount,total,0),
				        COALESCE(expense_date,''), COALESCE(status,'')
				 FROM expenses
				 WHERE category_id IS NULL OR category_id = ''
				 ORDER BY expense_date DESC
				 LIMIT 500`,
			)
			if err != nil {
				return fmt.Errorf("query untagged: %w", err)
			}
			defer rows.Close()
			out := make([]untaggedRow, 0)
			for rows.Next() {
				var r untaggedRow
				if err := rows.Scan(&r.ExpenseID, &r.MerchantName, &r.Amount, &r.ExpenseDate, &r.Status); err != nil {
					return err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if autoFix {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				fixed := 0
				skipped := 0
				for _, r := range out {
					if r.MerchantName == "" {
						skipped++
						continue
					}
					m, _ := zohotools.GetMerchant(s.DB(), r.MerchantName)
					if m == nil || m.CategoryID == "" {
						skipped++
						continue
					}
					body := map[string]any{"category_id": m.CategoryID}
					if m.ProjectID != "" {
						body["project_id"] = m.ProjectID
					}
					if _, _, perr := c.Put(cmd.Context(), "/expenses/"+r.ExpenseID, body); perr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: tag %s: %v\n", r.ExpenseID, perr)
						continue
					}
					fixed++
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"untagged_total": len(out),
						"fixed":          fixed,
						"skipped":        skipped,
						"items":          out,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "fixed %d of %d untagged expenses (%d skipped — no merchant memory)\n", fixed, len(out), skipped)
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no untagged expenses in local store")
				return nil
			}
			tableRows := make([][]string, 0, len(out))
			for _, r := range out {
				tableRows = append(tableRows, []string{
					r.ExpenseID, r.MerchantName,
					fmt.Sprintf("%.2f", r.Amount),
					r.ExpenseDate, r.Status,
				})
			}
			return flags.printTable(cmd, []string{"expense_id", "merchant", "amount", "date", "status"}, tableRows)
		},
	}
	cmd.Flags().BoolVar(&autoFix, "auto-fix", false, "For each row with a merchant memory entry, PUT /expenses/{id} with category_id/project_id from the memory")
	return cmd
}
