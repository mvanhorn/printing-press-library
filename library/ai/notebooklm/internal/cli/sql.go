// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/store"
	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var query string
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "sql",
		Short: "Run a read-only SQL query against the local notebook cache",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli sql --query 'SELECT id, title FROM notebooks LIMIT 5' --json
  notebooklm-pp-cli sync --json && notebooklm-pp-cli sql --query "SELECT count(*) AS n FROM notebooks" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSON(map[string]any{"rows": []any{}, "count": 0, "dry_run": true})
			}
			if query == "" {
				return usageErr(fmt.Errorf("--query is required"))
			}
			st, err := store.Open(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			rows, err := st.ReadOnlyQuery(cmd.Context(), query, limit)
			if err != nil {
				return usageErr(fmt.Errorf("query local database: %w", err))
			}
			payload := map[string]any{"rows": rows, "count": len(rows)}
			return emitOutput(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Read-only SQL statement (SELECT or WITH)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite cache path")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum rows to return")
	return cmd
}
