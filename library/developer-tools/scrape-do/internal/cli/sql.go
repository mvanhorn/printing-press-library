// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored `sql` command — read-only SELECT access to the local store so
// power users and agents can query stored SERP snapshots, the credit ledger,
// scrape jobs, and usage history directly (e.g. share-of-voice by domain, spend
// over time). SELECT/WITH only; any mutating statement is rejected. Hand file
// (no generator header) so it survives regeneration.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SELECT against the local store (SERP snapshots, ledger, jobs, usage)",
		Long: `Run an ad-hoc read-only SQL query against the local SQLite store. Useful for
share-of-voice (GROUP BY domain over serp_organic), spend analysis over
credit_ledger, and joining stored snapshots. Only SELECT and WITH...SELECT are
allowed; any mutating statement is rejected.

Key tables: serp_snapshots, serp_organic, scrape_jobs, credit_ledger,
usage_snapshots.`,
		Example: strings.Trim(`
  scrape-do-pp-cli sql "SELECT domain, COUNT(*) c FROM serp_organic GROUP BY domain ORDER BY c DESC LIMIT 10"
  scrape-do-pp-cli sql "SELECT mode, SUM(cost) FROM credit_ledger GROUP BY mode" --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "query=SELECT name FROM sqlite_master LIMIT 1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSuffix(strings.TrimSpace(strings.Join(args, " ")), ";")
			if strings.Contains(query, ";") {
				return usageErr(fmt.Errorf("multiple statements are not allowed; run one SELECT at a time"))
			}
			head := strings.ToUpper(query)
			if !strings.HasPrefix(head, "SELECT") && !strings.HasPrefix(head, "WITH") {
				return usageErr(fmt.Errorf("only read-only SELECT (or WITH...SELECT) queries are allowed"))
			}
			// Defense-in-depth: reject any mutating/side-effecting keyword as a
			// whole word, so a `WITH x AS (...) DELETE`, ATTACH, VACUUM INTO, or
			// PRAGMA cannot slip past the prefix check on the read-write handle.
			words := " " + strings.Join(strings.FieldsFunc(head, func(r rune) bool {
				return !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_')
			}), " ") + " "
			for _, kw := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "REPLACE", "ATTACH", "DETACH", "VACUUM", "PRAGMA", "REINDEX", "BEGIN", "COMMIT", "ROLLBACK"} {
				if strings.Contains(words, " "+kw+" ") {
					return usageErr(fmt.Errorf("disallowed keyword %q: sql is read-only (SELECT/WITH only)", kw))
				}
			}
			if dryRunOK(flags) {
				return nil
			}
			st, ext, err := openExtras(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			_ = ext // schema ensured

			rows, err := st.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}
			var out []map[string]any
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				row := make(map[string]any, len(cols))
				for i, c := range cols {
					row[c] = normalizeSQLVal(vals[i])
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no rows)")
				return nil
			}
			tableRows := make([][]string, 0, len(out))
			for _, r := range out {
				cells := make([]string, len(cols))
				for i, c := range cols {
					cells[i] = fmt.Sprintf("%v", r[c])
				}
				tableRows = append(tableRows, cells)
			}
			return flags.printTable(cmd, cols, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// normalizeSQLVal turns driver []byte values into strings so JSON/table output
// is readable rather than base64.
func normalizeSQLVal(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case sql.RawBytes:
		return string(x)
	default:
		return v
	}
}
