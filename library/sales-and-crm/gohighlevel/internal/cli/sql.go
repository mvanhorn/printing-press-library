// Copyright 2026 Jen Williams and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `sql` top-level command — run a read-only SELECT against the local
// SQLite mirror. Rejects anything that isn't a SELECT (or WITH ... SELECT).
package cli

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var selectRE = regexp.MustCompile(`(?i)^\s*(select|with|pragma\s+table_info|explain)\b`)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var tsv bool
	cmd := &cobra.Command{
		Use:         "sql <query>",
		Short:       "Run a SELECT against the local SQLite cache (read-only)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  gohighlevel-pp-cli sql "SELECT id, json_extract(data, '$.email') FROM resources WHERE resource_type = 'contacts' LIMIT 5"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			if !selectRE.MatchString(query) {
				return usageErr(fmt.Errorf("sql: only SELECT/WITH/PRAGMA table_info/EXPLAIN queries allowed"))
			}
			// Reject multi-statement input so `SELECT 1; DROP TABLE x` cannot slip past the prefix check.
			// SQLite's QueryContext will reject mutating statements when the connection is read-only, but
			// rejecting `;` here means we don't depend on connection mode and we surface a clear error.
			if i := strings.IndexByte(query, ';'); i >= 0 {
				rest := strings.TrimSpace(query[i+1:])
				if rest != "" {
					return usageErr(fmt.Errorf("sql: multi-statement queries are not allowed (semicolons must only appear at the end of the query)"))
				}
			}
			ctx := cmd.Context()
			s, err := openGHLStoreReadOnly()
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := s.DB().QueryContext(ctx, query)
			if err != nil {
				return apiErr(fmt.Errorf("sql: %w", err))
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return apiErr(err)
			}

			results := []map[string]any{}
			for rows.Next() {
				ptrs := make([]any, len(cols))
				vals := make([]sql.RawBytes, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return apiErr(err)
				}
				row := make(map[string]any, len(cols))
				for i, c := range cols {
					if vals[i] == nil {
						row[c] = nil
						continue
					}
					row[c] = string(vals[i])
				}
				results = append(results, row)
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}
			if tsv && !flags.asJSON {
				w := cmd.OutOrStdout()
				fmt.Fprintln(w, strings.Join(cols, "\t"))
				for _, r := range results {
					parts := make([]string, len(cols))
					for i, c := range cols {
						if v, ok := r[c]; ok && v != nil {
							parts[i] = fmt.Sprintf("%v", v)
						}
					}
					fmt.Fprintln(w, strings.Join(parts, "\t"))
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().BoolVar(&tsv, "tsv", false, "Emit TSV instead of JSON")
	return cmd
}
