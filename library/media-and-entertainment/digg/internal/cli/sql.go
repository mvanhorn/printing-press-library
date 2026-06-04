// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(digg-enhancements): library-side new file. Read-only SQL passthrough
// against the local store. Enforcement lives at the driver layer via
// store.OpenReadOnly (sqlite mode=ro), which rejects INSERT/UPDATE/DELETE/
// REPLACE/ATTACH and writable-CTE bypasses (`WITH x AS (DELETE … RETURNING …)
// SELECT * FROM x`) before SQLite parses the statement. A separate
// SELECT/WITH prefix check returns a friendlier error message but is no
// longer the security boundary.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/store"
	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SELECT/WITH query against the local store",
		Example: `  digg-pp-cli sql "SELECT cluster_url_id, title, current_rank FROM digg_clusters ORDER BY current_rank ASC LIMIT 5"
  digg-pp-cli sql "SELECT COUNT(*) AS n FROM digg_clusters"`,
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

			// Friendly prefix gate. The real enforcement is OpenReadOnly below;
			// this just produces a clear error for the common typo case
			// instead of a SQLite "attempt to write a readonly database" error.
			// Statement stacking is left to the driver: modernc.org/sqlite
			// executes a single statement per Query() call. A string-level
			// `;` scan rejected valid queries with semicolons in string
			// literals (e.g. `WHERE name = 'a;b'`).
			query := strings.TrimSpace(args[0])
			upper := strings.ToUpper(query)
			if !strings.HasPrefix(upper, "SELECT ") && !strings.HasPrefix(upper, "WITH ") {
				return fmt.Errorf("only SELECT/WITH queries are allowed")
			}

			if dbPath == "" {
				dbPath = defaultDBPath("digg-pp-cli")
			}

			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database (read-only): %w", err)
			}
			defer s.Close()

			db := s.DB()
			rows, err := db.QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("query error: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("getting columns: %w", err)
			}

			var results []json.RawMessage
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return fmt.Errorf("scanning: %w", err)
				}
				obj := make(map[string]any, len(cols))
				for i, col := range cols {
					v := vals[i]
					if b, ok := v.([]byte); ok {
						v = string(b)
					}
					obj[col] = v
				}
				b, err := json.Marshal(obj)
				if err != nil {
					return fmt.Errorf("marshaling row: %w", err)
				}
				results = append(results, json.RawMessage(b))
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("row iteration: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/digg-pp-cli/data.db)")
	return cmd
}
