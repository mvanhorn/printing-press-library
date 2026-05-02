package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"

	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SELECT against the local SQLite store",
		Long: strings.Trim(`
Executes the supplied SELECT query against the local store. INSERT / UPDATE /
DELETE / DROP / ALTER / CREATE / PRAGMA write statements are rejected — this
command is read-only by construction.

The local store is populated by `+"`producthunt-pp-cli sync`"+`. Tables include
`+"`resources`"+` (generic resource_type-keyed payloads), `+"`resources_fts`"+` (FTS5
virtual table), `+"`sync_state`"+`, plus per-resource_type tables created on demand.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli sql "SELECT id, content FROM resources WHERE resource_type='post' LIMIT 5"
  producthunt-pp-cli sql "SELECT id, content FROM resources_fts WHERE resources_fts MATCH 'agentic'"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(args[0])
			if !looksReadOnly(query) {
				return fmt.Errorf("sql: only SELECT queries are permitted")
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("producthunt-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			rows, err := db.Query(query)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			cols, _ := rows.Columns()
			out := []map[string]any{}
			for rows.Next() {
				row := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range cols {
					ptrs[i] = &row[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				rec := map[string]any{}
				for i, c := range cols {
					switch v := row[i].(type) {
					case []byte:
						rec[c] = string(v)
					default:
						rec[c] = v
					}
				}
				out = append(out, rec)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			pretty, _ := json.MarshalIndent(out, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(pretty))
			return nil
		},
	}
	return cmd
}

func looksReadOnly(q string) bool {
	upper := strings.ToUpper(strings.TrimSpace(q))
	for _, keyword := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "ATTACH", "DETACH", "REPLACE", "PRAGMA"} {
		if strings.Contains(upper, keyword) {
			return false
		}
	}
	return strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "WITH")
}
