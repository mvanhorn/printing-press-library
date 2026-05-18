// Copyright 2026 quoxientzero. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/skool/internal/store"
)

var sqlMutationRe = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|attach|detach|pragma|create|replace|reindex|vacuum|truncate)\b`)

// newSQLCmd lets agents run read-only SELECT queries against the local
// SQLite store populated by `sync`. Cross-table joins, group-bys, and
// cross-community comparisons are the headline use case.
func newSQLCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "sql [query...]",
		Short: "Run a read-only SELECT query against the local Skool store",
		Example: `  skool-pp-cli sql "SELECT id, name FROM posts ORDER BY created_at DESC LIMIT 10"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")
			trimmed := strings.TrimSpace(query)
			if !strings.EqualFold(firstWord(trimmed), "select") && !strings.EqualFold(firstWord(trimmed), "with") {
				return usageErr(fmt.Errorf("only SELECT/WITH queries are allowed"))
			}
			if hasMutation(trimmed) {
				return usageErr(fmt.Errorf("mutating SQL is not permitted in skool-pp-cli sql; use the API commands"))
			}

			db := flagDB
			if db == "" {
				db = defaultDBPath("skool-pp-cli")
			}
			// Open read-only at the SQLite driver level. mode=ro rejects every
			// write (including CTE-wrapped INSERT/UPDATE/DELETE) regardless of
			// the application-level hasMutation regex, so the read-only
			// guarantee survives any edge case in stripSQLStrings.
			s, err := store.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("opening database (read-only): %w", err)
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(), trimmed)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("columns: %w", err)
			}
			results := make([]map[string]any, 0, 64)
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				row := make(map[string]any, len(cols))
				for i, c := range cols {
					v := vals[i]
					if b, ok := v.([]byte); ok {
						v = string(b)
					}
					row[c] = v
				}
				results = append(results, row)
				if flagLimit > 0 && len(results) >= flagLimit {
					break
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("rows: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default ~/.local/share/skool-pp-cli/data.db)")
	cmd.Flags().IntVar(&flagLimit, "limit", 1000, "Cap result rows (0 = no cap)")
	return cmd
}

func firstWord(s string) string {
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			return s[:i]
		}
	}
	return s
}

func hasMutation(q string) bool {
	// Strip string literals before sniffing.
	cleaned := stripSQLStrings(q)
	return sqlMutationRe.MatchString(cleaned)
}

func stripSQLStrings(s string) string {
	var b strings.Builder
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !inDouble && ch == '\'' {
			inSingle = !inSingle
			continue
		}
		if !inSingle && ch == '"' {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}
