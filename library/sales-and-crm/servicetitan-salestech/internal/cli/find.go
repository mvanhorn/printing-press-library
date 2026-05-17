package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/salestech"
)

func newFindCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		status   string
		minTotal float64
		minScore float64
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Forgiving full-text search across estimates, summaries, job numbers, and line-item SKUs",
		Long: "Ranks every synced estimate against a natural-language query — name,\n" +
			"summary, job number, proposal tag, business unit, and every line-item\n" +
			"SKU name/description are scored and the best field wins. Results below\n" +
			"--min-score are dropped; a query that matches nothing exits non-zero\n" +
			"(grep-style). Combine with --status and --min-total to narrow the pool.\n" +
			"Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli find "well pump"
  servicetitan-salestech-pp-cli find "submersible" --status Open --min-total 5000 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			results, err := salestech.Find(db, query, status, minTotal, minScore, limit)
			if err != nil {
				return err
			}
			if results == nil {
				results = []salestech.FindResult{}
			}
			if len(results) == 0 {
				return fmt.Errorf("no estimates matched %q at or above --min-score %.2f; try different terms, lower --min-score, or widen --status / --min-total filters", query, minScore)
			}
			table := make([][]string, 0, len(results))
			for _, r := range results {
				table = append(table, []string{
					f2(r.Score), i64(r.ID), r.JobNumber, r.Status, f2(r.Total), r.MatchedOn, r.Name,
				})
			}
			return stOutput(cmd, flags, results,
				[]string{"SCORE", "ID", "JOB", "STATUS", "TOTAL", "MATCHED ON", "NAME"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-salestech-pp-cli/data.db)")
	cmd.Flags().StringVar(&status, "status", "", "Limit to estimates whose status matches (Open|Sold|Dismissed; case-insensitive)")
	cmd.Flags().Float64Var(&minTotal, "min-total", 0, "Limit to estimates whose subtotal+tax >= this amount (0 = no filter)")
	cmd.Flags().Float64Var(&minScore, "min-score", 0.4, "Minimum relevance score (0-1); results below this are dropped, and a query that matches nothing exits non-zero")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum results to return")
	return cmd
}
