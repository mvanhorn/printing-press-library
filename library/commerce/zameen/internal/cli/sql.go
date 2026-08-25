// sql: run a read-only SQL query against the local store.
// Fulfils absorbed feature "SQL over local listings". pp:data-source local
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/store"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "sql <SELECT ...>",
		Short: "Run a read-only SQL query over the local listings store",
		Long: "Execute a read-only SELECT against the local SQLite store. Listings live in the " +
			"`resources` table (resource_type='listings'); each row's `data` column holds the listing " +
			"JSON, queryable with json_extract(data, '$.price') etc.\n\n" +
			"Only SELECT statements are permitted.",
		Example:     "  zameen-pp-cli sql \"SELECT COUNT(*) AS n FROM resources WHERE resource_type='listings'\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run a read-only SQL query")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a SELECT query is required"))
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			low := strings.ToLower(query)
			if !strings.HasPrefix(low, "select") && !strings.HasPrefix(low, "with") {
				return usageErr(fmt.Errorf("only read-only SELECT/WITH queries are allowed"))
			}
			for _, banned := range []string{"insert ", "update ", "delete ", "drop ", "alter ", "create ", "replace ", "attach ", "pragma "} {
				if strings.Contains(low, banned) {
					return usageErr(fmt.Errorf("query contains a disallowed statement (%q); only read-only SELECT is permitted", strings.TrimSpace(banned)))
				}
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				return emitEmptyMirrorHint(cmd, flags, dbPath)
			}
			db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return apiErr(fmt.Errorf("query failed: %w", err))
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return err
			}
			out := make([]map[string]any, 0)
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
				for i, c := range cols {
					v := vals[i]
					if b, ok := v.([]byte); ok {
						v = string(b)
					}
					row[c] = v
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return emitObject(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
