// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: read-only SQL over the locally-synced CookUnity store for
// offline analytics no web UI offers. Registered via the novel-command hook so
// force regeneration preserves both the source and its wiring. Preserved on regen.
package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/store"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newSQLCmd(flags))
	})
}

// pp:data-source local
func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run a read-only SQL query over the locally-synced menu (offline analytics)",
		Long: `Run a read-only SQL SELECT against the local store. The main table is
"meals" (columns: id, name, chef_name, price, final_price, calories, protein,
carbs, fat, cuisines, diet_tags, allergens, stars, reviews, in_stock, category,
delivery_date, ...). Per-week history is in "meal_snapshots".

Only SELECT/WITH statements are allowed. Run 'cookunity-pp-cli sync' first.`,
		Example: `  cookunity-pp-cli sql "SELECT name, protein, final_price FROM meals ORDER BY protein DESC LIMIT 10" --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a SQL query is required"))
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			lower := strings.ToLower(query)
			// Only allow read statements. The real write barrier is the
			// read-only (mode=ro) handle opened below, which rejects any
			// write — including CTE-wrapped or multi-statement writes — so a
			// keyword blocklist here would only add false positives (e.g. a
			// literal like 'Update Weekly'). The prefix check just gives a
			// clearer up-front error for obvious non-reads.
			if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
				return usageErr(fmt.Errorf("only read-only SELECT/WITH queries are allowed"))
			}

			path := resolveMealDBPath(dbPath)
			if !mealDBExists(path) {
				return missingMirror(cmd, path, flags)
			}
			db, err := store.OpenReadOnlyContext(ctx, path)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, query)
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
				if limit > 0 && len(out) >= limit {
					break
				}
				cells := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range cells {
					ptrs[i] = &cells[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				row := make(map[string]any, len(cols))
				for i, c := range cols {
					row[c] = normalizeSQLCell(cells[i])
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}

// normalizeSQLCell converts driver []byte values to strings for clean JSON.
func normalizeSQLCell(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

var _ = sql.ErrNoRows
