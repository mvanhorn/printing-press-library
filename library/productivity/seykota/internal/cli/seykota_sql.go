// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var max int
	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run a read-only SELECT against the local archive (SQLite). The main table is corpus.",
		Long: `Run an arbitrary SELECT (or WITH ... SELECT) against the local archive's
SQLite database. The seykota content lives in one table:

  corpus(id, source, url, title, year, month, month_n, range, slug,
         updated, section, ord, contributors, body, fetched_at)

source is 'faq', 'tsp', or 'risk'. Only a single SELECT/WITH statement is
allowed -- no writes, no PRAGMA, no ATTACH. There is also a corpus_fts
FTS5 index over (title, body) joined on rowid (rowid matches corpus.rowid).`,
		Example: strings.Trim(`
  seykota-pp-cli sql "SELECT year, COUNT(*) AS months FROM corpus WHERE source='faq' GROUP BY year ORDER BY year"
  seykota-pp-cli sql "SELECT slug, updated FROM corpus WHERE source='tsp' ORDER BY updated DESC"
  seykota-pp-cli sql "SELECT COUNT(*) FROM corpus WHERE body LIKE '%Lake Ratio%'" --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				return nil
			}
			s, err := openCorpus(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			cols, rows, err := s.ReadOnlyQuery(cmd.Context(), query, max)
			if err != nil {
				return usageErr(err)
			}
			if wantsJSON(cmd, flags) {
				out := make([]map[string]string, 0, len(rows))
				for _, r := range rows {
					m := map[string]string{}
					for i, c := range cols {
						if i < len(r) {
							m[c] = r[i]
						}
					}
					out = append(out, m)
				}
				return emitJSON(cmd, flags, map[string]any{"columns": cols, "row_count": len(out), "rows": out})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(0 rows)")
				return nil
			}
			if err := printRows(cmd, cols, rows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n(%d row(s))\n", len(rows))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().IntVar(&max, "max-rows", 200, "Maximum rows to return")
	return cmd
}
