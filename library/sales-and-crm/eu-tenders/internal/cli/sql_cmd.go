// Copyright 2026 Mathias Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/eu-tenders/internal/store"

	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Execute a raw SELECT query against the local notices store",
		Long: `Run a read-only SQL query against the local SQLite database.

Only SELECT queries are allowed. Returns a JSON array of rows.

Examples:
  eu-tenders-pp-cli sql "SELECT id, buyer_name, title FROM notices LIMIT 10"
  eu-tenders-pp-cli sql "SELECT buyer_country, COUNT(*) as n FROM notices GROUP BY buyer_country ORDER BY n DESC"`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			query := args[0]

			// Allow only SELECT statements.
			trimmed := strings.TrimSpace(query)
			if !strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") {
				return fmt.Errorf("only SELECT queries are allowed\nhint: use 'eu-tenders-pp-cli sync' to write data")
			}

			st, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer st.Close()

			rows, err := st.DB().Query(query)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("columns: %w", err)
			}

			var results []map[string]interface{}
			for rows.Next() {
				vals := make([]interface{}, len(cols))
				ptrs := make([]interface{}, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				row := make(map[string]interface{}, len(cols))
				for i, col := range cols {
					row[col] = vals[i]
				}
				results = append(results, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath(), "SQLite database path")

	return cmd
}
