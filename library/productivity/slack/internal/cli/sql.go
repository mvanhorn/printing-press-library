// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: read-only SQL access to the local mirror.

// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelSQLCmd(flags))
	})
}

// sqlStatementIsReadOnly reports whether stmt is a single read-only statement.
// The local mirror is a cache the user can always rebuild with `sync`, but a
// stray UPDATE would silently diverge it from Slack with no way to tell, so the
// gate is a hard refusal rather than a warning.
func sqlStatementIsReadOnly(stmt string) (bool, string) {
	s := strings.TrimSpace(stmt)
	s = strings.TrimSuffix(s, ";")
	if s == "" {
		return false, "empty statement"
	}
	// Reject stacked statements outright; checking only the first verb would
	// let "SELECT 1; DROP TABLE resources" through.
	if strings.Contains(s, ";") {
		return false, "multiple statements are not allowed; run one SELECT at a time"
	}
	head := strings.ToUpper(s)
	for _, ok := range []string{"SELECT ", "SELECT\n", "SELECT\t", "WITH ", "WITH\n", "WITH\t", "EXPLAIN ", "PRAGMA "} {
		if strings.HasPrefix(head, ok) {
			return true, ""
		}
	}
	verb := head
	if i := strings.IndexAny(verb, " \t\n"); i > 0 {
		verb = verb[:i]
	}
	return false, fmt.Sprintf("only SELECT, WITH, EXPLAIN, and PRAGMA are allowed; got %q", verb)
}

type sqlResult struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Count   int              `json:"count"`
}

func newNovelSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SQL query against the local Slack mirror",
		Long: "Use this command to run an arbitrary read-only SQL query against the local SQLite mirror " +
			"when the built-in commands do not shape the answer you need. Only SELECT, WITH, EXPLAIN, and " +
			"PRAGMA are accepted, and only one statement per invocation. " +
			"Do NOT use this command for ordinary full-text lookup of message content; use 'archive recall' instead.",
		Example: strings.Trim(`
  slack-pp-cli sql "SELECT resource_type, COUNT(*) FROM resources GROUP BY resource_type" --json
  slack-pp-cli sql "SELECT id FROM resources WHERE resource_type = 'conversations' LIMIT 5"
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run a read-only SQL query against the local store (no API call, no writes)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a SQL query argument is required"))
			}
			query := strings.Join(args, " ")
			if ok, why := sqlStatementIsReadOnly(query); !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("refusing to run this statement: %s", why))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: slack-pp-cli sync --resources conversations,users && slack-pp-cli archive sync --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), sqlResult{Columns: []string{}, Rows: make([]map[string]any, 0)}, flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, query)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			cols, err := rows.Columns()
			if err != nil {
				_ = rows.Close()
				return fmt.Errorf("reading columns: %w", err)
			}

			// Drain fully before any follow-up work; SQLite holds one connection.
			out := make([]map[string]any, 0)
			for rows.Next() {
				if limit > 0 && len(out) >= limit {
					break
				}
				// Every column scans through sql.RawBytes-safe NullString so a
				// NULL never aborts the row and silently drops it.
				holders := make([]any, len(cols))
				vals := make([]sql.NullString, len(cols))
				for i := range vals {
					holders[i] = &vals[i]
				}
				if err := rows.Scan(holders...); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan row: %w", err)
				}
				rec := make(map[string]any, len(cols))
				for i, c := range cols {
					if vals[i].Valid {
						rec[c] = vals[i].String
					} else {
						rec[c] = nil
					}
				}
				out = append(out, rec)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close rows: %w", err)
			}

			res := sqlResult{Columns: cols, Rows: out, Count: len(out)}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No rows.")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(cols, "\t"))
			for _, r := range out {
				cells := make([]string, 0, len(cols))
				for _, c := range cols {
					if r[c] == nil {
						cells = append(cells, "")
						continue
					}
					cells = append(cells, fmt.Sprintf("%v", r[c]))
				}
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(cells, "\t"))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = no cap)")
	return cmd
}
