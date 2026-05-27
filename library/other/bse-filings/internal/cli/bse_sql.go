// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/store"

	"github.com/spf13/cobra"
)

// newSQLCmd exposes a read-only SQLite query surface over the local mirror.
// Power users and agents compose joins across announcements / holdings /
// concall_chunks / results_outcomes that the per-command surface does not
// pre-bake. Reads the local SQLite file directly via database/sql; SELECT-only
// by enforcement so the command cannot mutate the store.
func newSQLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run a read-only (SELECT-only) SQL query against the local filings mirror.",
		Long: strings.Trim(`
Execute a SELECT statement against the local SQLite mirror populated by 'sync'.
Tables: announcements, holdings, concall_chunks, results_outcomes. Only SELECT
and WITH (CTE) statements are permitted — the command refuses anything that
could mutate the store.`, "\n"),
		Example: strings.Trim(`
  bse-filings-pp-cli sql "SELECT scrip_cd, COUNT(*) AS filings FROM announcements GROUP BY scrip_cd ORDER BY filings DESC"
  bse-filings-pp-cli sql "SELECT scrip_code, sector FROM holdings" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if err := guardSelectOnly(query); err != nil {
				return usageErr(err)
			}

			// Open the mirror read-only so mode=ro rejects writes at the
			// driver level even if guardSelectOnly's text match is bypassed.
			// Skip EnsureBSETables: read-only mode can't create tables, and a
			// missing table should error honestly ("no such table") rather
			// than be silently conjured by a query tool.
			s, err := store.OpenReadOnly(defaultDBPath("bse-filings-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := s.Query(query)
			if err != nil {
				return apiErr(fmt.Errorf("query: %w", err))
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}
			out := []map[string]any{}
			for rows.Next() {
				cells := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range cells {
					ptrs[i] = &cells[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				row := make(map[string]any, len(cols))
				for i, c := range cols {
					row[c] = normalizeSQLCell(cells[i])
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return flags.printJSON(cmd, out)
		},
	}
	return cmd
}

// guardSelectOnly rejects any statement that is not a single read-only query.
// SQLite would otherwise happily run INSERT/UPDATE/DELETE/DDL through the same
// driver, so the gate is the only thing keeping `sql` non-destructive.
func guardSelectOnly(query string) error {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return fmt.Errorf("empty query")
	}
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return fmt.Errorf("only SELECT/WITH queries are allowed (read-only)")
	}
	for _, banned := range []string{"insert", "update", "delete", "drop", "alter", "create", "replace", "attach", "detach", "pragma", "vacuum", "reindex"} {
		if containsWord(lower, banned) {
			return fmt.Errorf("statement contains a disallowed keyword %q; sql is read-only", banned)
		}
	}
	return nil
}

// containsWord reports whether word appears as a standalone token in s, so a
// column named "created_at" does not trip the "create" guard.
func containsWord(s, word string) bool {
	for _, field := range strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z')
	}) {
		if field == word {
			return true
		}
	}
	return false
}

// normalizeSQLCell turns driver []byte values into strings so JSON output is
// readable rather than base64-encoded byte arrays.
func normalizeSQLCell(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case sql.RawBytes:
		return string(t)
	default:
		return v
	}
}
