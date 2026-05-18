// Copyright 2026 nathan-kettles. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/nylas/internal/store"
)

var sqlReadOnlyRE = regexp.MustCompile(`(?i)^\s*(select|with|explain|pragma\s+(table_info|table_list|index_list|index_info|database_list))\b`)
var sqlMutationRE = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|create|alter|attach|detach|replace|truncate|vacuum|reindex)\b`)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SQL query against the local mirror",
		Long: `Execute a read-only SQL query against the local SQLite mirror that
sync builds. Only SELECT, WITH, EXPLAIN, and informational PRAGMA queries
are allowed; any statement that could mutate state is refused.

Typical agent usage: compose joins across messages, contacts, events,
and webhooks that no single Nylas API call returns.`,
		Example: strings.Trim(`
  # Top senders across every grant in the last 30 days
  nylas-pp-cli sql "SELECT json_extract(data,'$.from[0].email') AS sender, COUNT(*) AS n FROM grants_messages WHERE synced_at > datetime('now','-30 days') GROUP BY 1 ORDER BY 2 DESC LIMIT 10" --agent

  # Per-grant message counts
  nylas-pp-cli sql "SELECT grants_id, COUNT(*) FROM grants_messages GROUP BY 1" --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if !sqlReadOnlyRE.MatchString(query) {
				return fmt.Errorf("only SELECT/WITH/EXPLAIN/PRAGMA queries are allowed (got: %q)", firstWord(query))
			}
			// Strip single-quoted string literals before the mutation-keyword
			// check so harmless reads that mention those keywords inside LIKE
			// patterns (e.g. `WHERE body LIKE '%insert%'`) are not rejected.
			// The driver-level mode=ro flag in OpenReadOnly is the actual write
			// barrier; this regex is defence-in-depth on top of that.
			if sqlMutationRE.MatchString(stripSQLStringLiterals(query)) {
				return fmt.Errorf("query contains a mutation keyword outside string literals; sql is read-only")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("nylas-pp-cli")
			}
			autoRefreshIfStale(cmd.Context(), dbPath, cmd.ErrOrStderr())
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'nylas-pp-cli sync' first.", err)
			}
			defer db.Close()

			if limit > 0 && !strings.Contains(strings.ToLower(query), " limit ") {
				query = fmt.Sprintf("%s LIMIT %d", query, limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return err
			}
			out := make([]map[string]any, 0, 32)
			for rows.Next() {
				row := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range row {
					ptrs[i] = &row[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				m := make(map[string]any, len(cols))
				for i, c := range cols {
					m[c] = decodeSQLValue(row[i])
				}
				out = append(out, m)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite database (default: $HOME/.nylas-pp-cli.db)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Append LIMIT N if the query has no explicit LIMIT (0 = no cap)")
	return cmd
}

// stripSQLStringLiterals removes the contents of single-quoted string
// literals from a SQL query so a keyword scan can't be tripped by harmless
// text inside a LIKE pattern or WHERE-clause comparison. Handles doubled
// single-quotes ('') as the standard SQL escape for an embedded quote.
// This is a conservative approximation, not a real SQL parser — the
// driver-level mode=ro flag is what actually prevents writes; this helper
// only exists so the defence-in-depth regex doesn't produce false positives.
func stripSQLStringLiterals(q string) string {
	var b strings.Builder
	b.Grow(len(q))
	inStr := false
	for i := 0; i < len(q); i++ {
		c := q[i]
		if c == '\'' {
			if inStr && i+1 < len(q) && q[i+1] == '\'' {
				i++
				continue
			}
			inStr = !inStr
			continue
		}
		if !inStr {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func firstWord(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func decodeSQLValue(v any) any {
	switch t := v.(type) {
	case []byte:
		// Try JSON first; fall through to string.
		var any2 any
		if err := json.Unmarshal(t, &any2); err == nil {
			return any2
		}
		return string(t)
	case sql.NullString:
		if t.Valid {
			return t.String
		}
		return nil
	default:
		return v
	}
}
