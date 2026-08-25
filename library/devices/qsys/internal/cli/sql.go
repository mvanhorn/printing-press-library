// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// sqlReadOnlyPrefixes are the only statement forms this command will execute.
// The corpus is a local cache that can be rebuilt with `harvest`, but a stray
// DROP or UPDATE from an agent-composed query would still silently corrupt
// results for every later command, so writes are refused outright.
var sqlReadOnlyPrefixes = []string{"select", "with", "explain", "pragma table_info"}

func newNovelSqlCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run arbitrary SQL across products, spec text, documentation pages, and the compatibility matrix.",
		Long: strings.Trim(`
Run a read-only SQL query against the local Q-SYS corpus.

Tables:
  qsys_products  model, title, is_product, family, slug, url, overview,
                 spec_pdf_url, manual_pdf_url, spec_text, discontinued
  qsys_pages     url, section, title, body
  qsys_compat    qds_version, release_date, added_hardware, removed_hardware
  qsys_harvest   source, attempted, succeeded, with_specs, finished_at

Note that /products-solutions/ also hosts marketing articles, so filter on
is_product = 1 when you want real equipment only.

Only SELECT, WITH, EXPLAIN, and PRAGMA table_info are permitted; anything that
could mutate the corpus is refused.
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli sql "SELECT model, family FROM qsys_products WHERE discontinued = 1"
  qsys-pp-cli sql "SELECT title FROM qsys_products WHERE is_product = 1 ORDER BY title"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sql")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a SQL query is required, e.g. \"SELECT model FROM qsys_products\""))
			}
			query := strings.TrimSpace(args[0])
			lower := strings.ToLower(query)
			allowed := false
			for _, p := range sqlReadOnlyPrefixes {
				if strings.HasPrefix(lower, p) {
					allowed = true
					break
				}
			}
			// Read-only guard invariant: this prefix check is safe ONLY
			// because modernc.org/sqlite executes just the first statement of a
			// multi-statement string via Query (a trailing "SELECT 1; DROP ..."
			// never prepares the tail). Do not switch drivers or execution
			// paths without revisiting this guard, and do not relax it to
			// accept statements that begin with a write keyword.
			if !allowed {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("only read-only queries are permitted (SELECT, WITH, EXPLAIN, PRAGMA table_info)"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]map[string]any, 0), flags)
				}
				return nil
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := st.DB().QueryContext(ctx, query)
			if err != nil {
				return fmt.Errorf("running query: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("reading columns: %w", err)
			}
			out := make([]map[string]any, 0)
			for rows.Next() {
				if limit > 0 && len(out) >= limit {
					break
				}
				// Scan every column as NullString: any column can be NULL, and a
				// bare string target turns a NULL into a scan error that would
				// silently drop the row.
				cells := make([]any, len(cols))
				holders := make([]sql.NullString, len(cols))
				for i := range holders {
					cells[i] = &holders[i]
				}
				if err := rows.Scan(cells...); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				row := make(map[string]any, len(cols))
				for i, c := range cols {
					if holders[i].Valid {
						row[c] = holders[i].String
					} else {
						row[c] = nil
					}
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No rows.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = no limit)")
	return cmd
}

// Self-registration: this command was originally wired from generated
// root.go, which a cross-spec regen re-emits from research.json novel_features
// and would drop. Registering from the preserved file keeps `sql` available
// even when it is not a headline novel feature.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelSqlCmd(flags))
	})
}
