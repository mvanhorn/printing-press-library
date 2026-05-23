// Hand-written novel feature. Not generated.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/halopsa/internal/store"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath  string
		explain bool
	)
	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run a SELECT against the local SQLite store",
		Long: `Execute an ad-hoc SELECT-only query against the synced HaloPSA database.
Mutating statements (INSERT/UPDATE/DELETE/DROP/CREATE/ALTER/ATTACH/REPLACE/REINDEX)
are rejected. Run 'halopsa-pp-cli sync' first to populate the store.`,
		Example: strings.Trim(`
  # Per-status ticket counts
  halopsa-pp-cli sql "SELECT COALESCE(json_extract(data,'$.status_name'),'?') AS status, COUNT(*) FROM tickets GROUP BY status"

  # Top 10 most-active clients
  halopsa-pp-cli sql "SELECT client_name, COUNT(*) FROM tickets GROUP BY client_name ORDER BY 2 DESC LIMIT 10"

  # JSON output
  halopsa-pp-cli sql "SELECT id, summary FROM tickets LIMIT 5" --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(args[0])
			if dryRunOK(flags) {
				return nil
			}
			// SELECT-only gate.
			lower := strings.ToLower(query)
			for _, banned := range []string{"insert ", "update ", "delete ", "drop ", "create ", "alter ", "attach ", "replace ", "reindex "} {
				if strings.Contains(lower, banned) {
					return fmt.Errorf("only SELECT statements are allowed (found %q)", strings.TrimSpace(banned))
				}
			}
			if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") && !strings.HasPrefix(lower, "explain") {
				return fmt.Errorf("query must start with SELECT, WITH, or EXPLAIN")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("halopsa-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'halopsa-pp-cli sync' first.", err)
			}
			defer db.Close()
			if explain {
				query = "EXPLAIN QUERY PLAN " + query
			}
			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("query: %w", err)
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
					continue
				}
				row := map[string]any{}
				for i, c := range cols {
					row[c] = normalizeSQLValue(vals[i])
				}
				out = append(out, row)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"columns": cols,
					"rows":    out,
					"count":   len(out),
				})
			}
			// Table output
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(0 rows)")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(cols, "\t"))
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 12*len(cols)))
			for _, r := range out {
				vals := make([]string, len(cols))
				for i, c := range cols {
					vals[i] = fmt.Sprintf("%v", r[c])
				}
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(vals, "\t"))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n(%d rows)\n", len(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&explain, "explain", false, "Run EXPLAIN QUERY PLAN")
	_ = json.Compact
	return cmd
}

func normalizeSQLValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case sql.NullString:
		if x.Valid {
			return x.String
		}
		return nil
	case sql.NullInt64:
		if x.Valid {
			return x.Int64
		}
		return nil
	case sql.NullFloat64:
		if x.Valid {
			return x.Float64
		}
		return nil
	}
	return v
}
