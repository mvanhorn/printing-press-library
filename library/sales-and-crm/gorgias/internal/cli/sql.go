// `sql` command — read-only SQL escape hatch against the local SQLite mirror.
// Mirrors the MCP server's `sql` tool semantics but exposes the capability to
// the CLI surface so a human (or scripted pipeline) can run ad-hoc aggregations
// against synced data without going through the MCP gateway.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/store"
	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var queryFlag string
	var dbPathFlag string

	cmd := &cobra.Command{
		Use:   "sql [QUERY]",
		Short: "Run a read-only SQL query against the local SQLite mirror.",
		Long: "Runs an ad-hoc SQL query against the local SQLite mirror populated by `sync`.\n\n" +
			"All data is stored as JSON blobs in a single `resources` table with columns\n" +
			"`(resource_type, id, data, updated_at)`. Use `json_extract(data, '$.field')`\n" +
			"to read nested fields. Full-text search lives in companion virtual tables\n" +
			"named `<resource>_fts` (e.g. `tickets_fts`) when the resource was synced.\n\n" +
			"Read-only by design: only SELECT and WITH queries are accepted. Comments and\n" +
			"leading whitespace are stripped before the keyword check, so `/* DROP */ SELECT`\n" +
			"is still rejected as a DROP. The mirror is opened in read-only mode; any\n" +
			"mutation must go through the API commands.\n\n" +
			"Query can be passed as a positional arg, via `--query`, or piped on stdin.\n" +
			"Output is a JSON envelope `{meta: {columns, row_count, db_path, source}, results: [...]}`.\n\n" +
			"If the DB does not exist, run `gorgias-pp-cli sync` first.",
		Example: `  # Count closed tickets in the local mirror
  gorgias-pp-cli sql "SELECT count(*) FROM resources WHERE resource_type = 'tickets' AND json_extract(data, '\$.status') = 'closed'"

  # Read query from stdin
  echo "SELECT count(*) FROM resources WHERE resource_type = 'tickets'" | gorgias-pp-cli sql`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := queryFlag
			if query == "" && len(args) > 0 {
				query = args[0]
			}
			if query == "" {
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) == 0 {
					b, err := io.ReadAll(os.Stdin)
					if err == nil {
						query = string(b)
					}
				}
			}
			if query == "" {
				return cmd.Help()
			}
			if err := cliutil.ValidateReadOnlySQL(query); err != nil {
				return err
			}
			path := dbPathFlag
			if path == "" {
				path = defaultDBPath("gorgias-pp-cli")
			}
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return fmt.Errorf("opening mirror at %s: %w (run `sync` first)", path, err)
			}
			defer db.Close()

			ctx, cancel := context.WithTimeout(cmd.Context(), flags.timeout)
			defer cancel()
			rows, err := db.DB().QueryContext(ctx, query)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			cols, _ := rows.Columns()
			results := make([]map[string]any, 0)
			for rows.Next() {
				values := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range values {
					ptrs[i] = &values[i]
				}
				if scanErr := rows.Scan(ptrs...); scanErr != nil {
					return fmt.Errorf("scan row: %w", scanErr)
				}
				row := make(map[string]any, len(cols))
				for i, col := range cols {
					v := values[i]
					if b, ok := v.([]byte); ok {
						v = string(b)
					}
					row[col] = v
				}
				results = append(results, row)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			envelope := map[string]any{
				"meta": map[string]any{
					"source":    "local",
					"db_path":   path,
					"row_count": len(results),
					"columns":   cols,
				},
				"results": results,
			}
			data, err := json.Marshal(envelope)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&queryFlag, "query", "", "SQL query to run (alternative to passing as positional arg).")
	cmd.Flags().StringVar(&dbPathFlag, "db", "", "Path to the SQLite mirror (default: ~/.local/share/gorgias-pp-cli/data.db).")
	return cmd
}

// The read-only SQL gate is defined once in internal/cliutil/sqlgate.go and
// shared between this CLI command and the MCP `sql` tool. Keeping both
// surfaces on the same gate is intentional: any divergence would create an
// allowlist-bypass surface, not a stylistic complaint.
