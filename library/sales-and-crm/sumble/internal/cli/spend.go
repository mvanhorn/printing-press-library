package cli

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// PATCH(spend-since-date-validation): SQLite's date() returns NULL for any
// non-ISO-8601 input, so a malformed --since (e.g. "2026/05/01") would silently
// return zero rows. Validate the format at the CLI boundary so users see a
// clear usage error instead of an empty report.
var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type spendRow struct {
	Group       string `json:"group"`
	Calls       int    `json:"calls"`
	CreditsUsed int    `json:"credits_used"`
}

func newSpendCmd(flags *rootFlags) *cobra.Command {
	var since, by string

	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Break down credits spent over time by endpoint or by day",
		Long: strings.Trim(`
Report the credits this CLI has spent, aggregated from the local ledger. Group
by endpoint (default) to see what is eating credits, or by day to see spend over
time. Filter with --since (YYYY-MM-DD).

Only calls made through the credit-aware commands (balance, stack-diff,
reconcile) are recorded; raw endpoint commands are not yet ledger-tracked.
`, "\n"),
		Example: strings.Trim(`
  sumble-pp-cli spend
  sumble-pp-cli spend --by day --since 2026-05-01
  sumble-pp-cli spend --by endpoint --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch by {
			case "endpoint", "day":
			default:
				return usageErr(fmt.Errorf("--by must be 'endpoint' or 'day', got %q", by))
			}
			db, derr := openCreditStore()
			if derr != nil {
				return configErr(derr)
			}
			defer db.Close()

			groupExpr := "endpoint"
			if by == "day" {
				groupExpr = "date(ts)"
			}
			query := fmt.Sprintf(
				`SELECT %s AS grp, COUNT(*), COALESCE(SUM(credits_used),0) FROM credit_ledger`, groupExpr)
			var queryArgs []any
			if s := strings.TrimSpace(since); s != "" {
				if !isoDatePattern.MatchString(s) {
					return usageErr(fmt.Errorf("--since must be ISO-8601 YYYY-MM-DD (got %q)", s))
				}
				query += ` WHERE date(ts) >= date(?)`
				queryArgs = append(queryArgs, s)
			}
			query += fmt.Sprintf(` GROUP BY %s ORDER BY 3 DESC, 1`, groupExpr)

			rows, err := db.DB().Query(query, queryArgs...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			var out []spendRow
			total := 0
			for rows.Next() {
				var g sql.NullString
				var calls, used int
				if err := rows.Scan(&g, &calls, &used); err != nil {
					continue
				}
				out = append(out, spendRow{Group: g.String, Calls: calls, CreditsUsed: used})
				total += used
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"by":                 by,
					"since":              since,
					"rows":               out,
					"credits_used_total": total,
				})
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No spend recorded yet.")
				return nil
			}
			fmt.Fprintf(w, "%-34s %8s %8s\n", strings.ToUpper(by), "CALLS", "CREDITS")
			for _, r := range out {
				fmt.Fprintf(w, "%-34s %8d %8d\n", r.Group, r.Calls, r.CreditsUsed)
			}
			fmt.Fprintf(w, "%-34s %8s %8d\n", "TOTAL", "", total)
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only count calls since this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&by, "by", "endpoint", "Group by 'endpoint' or 'day'")
	return cmd
}
