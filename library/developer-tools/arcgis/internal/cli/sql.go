// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: read-only SQL over synced features in the local store.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelSqlCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SQL query over synced features with no further API calls",
		Long: `Run a read-only SQL SELECT over the local store. Synced features are exposed as
the 'features' view with columns: layer_url, oid, attributes (JSON string),
geometry (JSON string). Use json_extract on attributes to reach fields.

Only SELECT/WITH statements are allowed. Run 'sync <layer-url>' first.`,
		Example:     `  arcgis-pp-cli sql "SELECT json_extract(attributes,'$.OWNER') AS owner, count(*) c FROM features GROUP BY owner ORDER BY c DESC LIMIT 10"`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "query=SELECT 1 AS one"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run a read-only SQL query over the local store")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a SQL query is required"))
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if !isReadOnlySQL(query) {
				return usageErr(fmt.Errorf("only read-only SELECT/WITH queries are allowed"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("arcgis-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local store at %s\nrun: arcgis-pp-cli sync <layer-url> --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			if err := ensureFeaturesView(ctx, db); err != nil {
				// Read-only DB may reject the view create; fall back to a temp view via query rewrite is overkill.
				// The view is created on sync, so this only fails on brand-new read-only opens.
				_ = err
			}
			rows, err := db.DB().QueryContext(ctx, query)
			if err != nil {
				return fmt.Errorf("running query: %w", err)
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
					row[c] = normalizeSQLValue(vals[i])
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no rows")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "store path (default the standard local db)")
	return cmd
}

func isReadOnlySQL(q string) bool {
	s := strings.ToLower(strings.TrimSpace(q))
	if !strings.HasPrefix(s, "select") && !strings.HasPrefix(s, "with") {
		return false
	}
	for _, bad := range []string{"insert ", "update ", "delete ", "drop ", "alter ", "create ", "replace ", "attach ", "pragma ", ";"} {
		if strings.Contains(s+" ", bad) {
			// allow a single trailing semicolon
			if bad == ";" && strings.Count(s, ";") == 1 && strings.HasSuffix(strings.TrimSpace(s), ";") {
				continue
			}
			return false
		}
	}
	return true
}

func normalizeSQLValue(v any) any {
	switch t := v.(type) {
	case []byte:
		// Try to surface JSON columns (attributes/geometry) as parsed objects.
		var js any
		if json.Unmarshal(t, &js) == nil {
			switch js.(type) {
			case map[string]any, []any:
				return js
			}
		}
		return string(t)
	case sql.RawBytes:
		return string(t)
	default:
		return v
	}
}
