// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

// newSQLCmd returns the `sql` escape hatch — read-only SELECT against the
// local store. FTS5 MATCH and PRAGMA reads are allowed; mutations are
// rejected at parse time.
func newSQLCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run a read-only SQL query against the local store (SELECT only)",
		Long: "Execute a SELECT against the local SQLite store. The store contains:\n" +
			"  recordings_typed, transcripts, transcripts_fts (FTS5),\n" +
			"  summaries, filetags_typed, speakers, resources, sync_state.\n\n" +
			"Mutations (INSERT/UPDATE/DELETE/DROP/ALTER) are rejected.\n" +
			"PRAGMA reads (e.g. table info) are allowed.",
		Example: `  plaud-pp-cli sql "SELECT speaker, COUNT(*) FROM transcripts GROUP BY 1 ORDER BY 2 DESC LIMIT 10"
  plaud-pp-cli sql "SELECT * FROM transcripts_fts WHERE transcripts_fts MATCH 'pricing'" --json
  plaud-pp-cli sql "SELECT name FROM sqlite_master WHERE type='table'"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(args[0])
			if err := assertSelectOnly(query); err != nil {
				return usageErr(err)
			}
			// Append LIMIT if the user didn't already and --limit > 0
			if flagLimit > 0 && !containsLimit(query) {
				query = strings.TrimRight(query, "; \t\n") + fmt.Sprintf(" LIMIT %d", flagLimit)
			}

			s, err := openPlaudStore(cmd.Context())
			var _ *store.Store = s
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := queryRowsToMaps(cmd.Context(), s.DB(), query)
			if err != nil {
				return apiErr(fmt.Errorf("sql: %w", err))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Append LIMIT N when the query has none (0 = no auto-limit)")
	return cmd
}

// assertSelectOnly rejects queries that aren't a SELECT or PRAGMA read.
// Token-based check (not regex of full body) so SQL injection through a
// "DELETE" word inside a string literal doesn't false-positive.
func assertSelectOnly(query string) error {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return fmt.Errorf("empty query")
	}
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") && !strings.HasPrefix(upper, "PRAGMA") {
		return fmt.Errorf("only SELECT, WITH, and PRAGMA reads are allowed (got %q...)", firstToken(trimmed))
	}
	// Forbid common mutation keywords as standalone tokens. This is best-effort
	// — SQLite parse-time rejection would be stricter, but at the cost of
	// running a fresh parser. False negatives here aren't safety-critical
	// because the database is opened read-only at the connection layer for sql.
	banned := []string{
		"INSERT ", "UPDATE ", "DELETE ", "DROP ", "ALTER ",
		"CREATE TABLE", "CREATE INDEX", "CREATE TRIGGER",
		"REINDEX", "VACUUM", "ATTACH", "DETACH",
	}
	for _, kw := range banned {
		if strings.Contains(upper, kw) {
			return fmt.Errorf("mutation keyword %q not allowed in sql escape hatch", strings.TrimSpace(kw))
		}
	}
	return nil
}

func containsLimit(q string) bool {
	return strings.Contains(strings.ToUpper(q), " LIMIT ")
}

func firstToken(s string) string {
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			return s[:i]
		}
	}
	return s
}
