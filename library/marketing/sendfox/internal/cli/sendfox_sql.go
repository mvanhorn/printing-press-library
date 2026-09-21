// pp:data-source local
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/store"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) { root.AddCommand(newSendfoxSQL(f)) })
}
func newSendfoxSQL(f *rootFlags) *cobra.Command {
	var query, dbPath string
	var limit int
	cmd := &cobra.Command{Use: "sql [query]", Short: "Run a bounded read-only SQL query against the local mirror", Example: "  sendfox-pp-cli sql 'SELECT resource_type, count(*) AS count FROM resources GROUP BY resource_type' --db ./sendfox.db --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "sql")
		}
		if f.dataSource == "live" {
			return usageErr(fmt.Errorf("sql requires --data-source local"))
		}
		if len(args) == 1 && query == "" {
			query = args[0]
		} else if len(args) > 0 {
			return usageErr(fmt.Errorf("supply one query positional or --query"))
		}
		q := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(query), ";"))
		upper := strings.ToUpper(q)
		if !(strings.HasPrefix(upper, "SELECT ") || strings.HasPrefix(upper, "WITH ")) || strings.Contains(q, ";") {
			return usageErr(fmt.Errorf("one SELECT or read-only WITH query is required; additional statements are forbidden"))
		}
		if limit < 1 || limit > 10000 {
			return usageErr(fmt.Errorf("--limit must be 1..10000"))
		}
		if dbPath == "" {
			dbPath = defaultDBPath("sendfox-pp-cli")
		}
		out := []map[string]any{}
		if _, e := os.Stat(dbPath); os.IsNotExist(e) {
			fmt.Fprintln(cmd.ErrOrStderr(), "No local mirror; run sync first")
			return f.printJSON(cmd, map[string]any{"rows": out, "count": 0, "complete": false})
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		db, e := store.OpenReadOnlyContext(ctx, dbPath)
		if e != nil {
			return e
		}
		defer db.Close()
		hintIfUnsynced(cmd, db, "")
		hintIfStale(cmd, db, "", f.maxAge)
		rows, e := db.DB().QueryContext(ctx, "SELECT * FROM ("+q+") LIMIT ?", limit+1)
		if e != nil {
			return usageErr(e)
		}
		defer rows.Close()
		cols, e := rows.Columns()
		if e != nil {
			return e
		}
		for rows.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if e := rows.Scan(ptrs...); e != nil {
				return e
			}
			r := map[string]any{}
			for i, key := range cols {
				r[key] = values[i]
			}
			out = append(out, r)
		}
		if e := rows.Err(); e != nil {
			return e
		}
		capped := len(out) > limit
		if capped {
			out = out[:limit]
		}
		return f.printJSON(cmd, map[string]any{"rows": out, "count": len(out), "truncated": capped, "limit": limit})
	}}
	cmd.Flags().StringVar(&query, "query", "", "Single SELECT or WITH query; resources table stores JSON in data")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to a local SQLite mirror created by sync")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum result rows; query timeout also bounds computation")
	return cmd
}
