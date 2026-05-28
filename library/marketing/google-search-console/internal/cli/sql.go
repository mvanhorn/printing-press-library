// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence layer.

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// nonSelectRE detects any SQL keyword that mutates state. We reject the query
// before passing it to SQLite — defense in depth on top of read-only DB
// permission would be ideal, but SQLite's single-file model means the same
// connection that reads also writes; refusing the verbs is simpler.
var nonSelectRE = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|create|attach|detach|replace|truncate|vacuum|pragma|reindex)\b`)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var queryFlag string

	cmd := &cobra.Command{
		Use:         "sql [select-statement]",
		Short:       "Run a SELECT-only SQL query against the local store",
		Long:        "Opens the local SQLite store at ~/.cache/google-search-console-pp-cli/store.db and runs an arbitrary SELECT statement. Mutating verbs (INSERT/UPDATE/DELETE/DROP/ALTER/CREATE/REPLACE/TRUNCATE/VACUUM/PRAGMA) are rejected. Useful for ad-hoc joins (e.g. cross-table cohort questions) the named transcendence commands don't cover.",
		Example:     "  google-search-console-pp-cli sql \"SELECT site_url, COUNT(*) FROM search_analytics_rows GROUP BY site_url\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && queryFlag == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := queryFlag
			if query == "" {
				query = strings.TrimSpace(strings.Join(args, " "))
			}
			if query == "" {
				return usageErr(fmt.Errorf("query required"))
			}

			// Strip simple SQL line comments, then enforce SELECT-only.
			stripped := stripSQLComments(query)
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(stripped)), "select") &&
				!strings.HasPrefix(strings.ToLower(strings.TrimSpace(stripped)), "with") {
				return usageErr(fmt.Errorf("only SELECT (or WITH ... SELECT) statements are allowed"))
			}
			if nonSelectRE.MatchString(stripped) {
				return usageErr(fmt.Errorf("statement contains a mutating SQL keyword; only SELECT is allowed"))
			}

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			rows, err := s.DB().QueryContext(ctx, query)
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
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				row := make(map[string]any, len(cols))
				for i, col := range cols {
					row[col] = normalizeSQLValue(vals[i])
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&queryFlag, "query", "", "SQL query (alternative to passing as argument)")

	return cmd
}

func stripSQLComments(q string) string {
	lines := strings.Split(q, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if i := strings.Index(l, "--"); i >= 0 {
			l = l[:i]
		}
		out = append(out, l)
	}
	return strings.Join(out, " ")
}

func normalizeSQLValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	default:
		return x
	}
}
