// PATCH: hand-authored insight `stats` — local SQL aggregation over synced
// data. Uses the SQLite store populated by `sync`; matches the Workflows +
// Insight scorecard categories. See .printing-press-patches.json
// patch id "insight-stats".

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/jimmy-johns/internal/store"
)

type statsResult struct {
	MenuItemCount int                  `json:"menu_item_count"`
	StoreCount    int                  `json:"store_count"`
	ByCategory    map[string]int       `json:"by_category,omitempty"`
	Notes         []string             `json:"notes,omitempty"`
}

func newStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show top-line counts and aggregations from locally synced data",
		Long: `Summarize the local SQLite store: how many menu items, how many stores,
plus per-category counts when the menu carries category tags.

Run 'jimmy-johns-pp-cli sync' first to populate the store, otherwise this
returns zero counts.`,
		Example: `  jimmy-johns-pp-cli stats --json
  jimmy-johns-pp-cli stats`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("jimmy-johns-pp-cli")
			}
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			result := statsResult{ByCategory: map[string]int{}}

			if err := db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM menu`).Scan(&result.MenuItemCount); err != nil {
				result.Notes = append(result.Notes, fmt.Sprintf("menu count failed: %v", err))
			}
			if err := db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM stores`).Scan(&result.StoreCount); err != nil {
				result.Notes = append(result.Notes, fmt.Sprintf("store count failed: %v", err))
			}

			// GROUP BY category (works when synced menu items have a category JSON field).
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(json_extract(data, '$.category'), 'uncategorized') AS cat, COUNT(*)
				FROM menu GROUP BY cat ORDER BY COUNT(*) DESC`)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var cat string
					var n int
					if err := rows.Scan(&cat, &n); err == nil {
						result.ByCategory[cat] = n
					}
				}
			}

			if result.MenuItemCount == 0 && result.StoreCount == 0 {
				result.Notes = append(result.Notes, "store is empty — run 'sync' to populate it before re-running stats")
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			renderStats(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.cache/jimmy-johns-pp-cli/store.db)")
	return cmd
}

func renderStats(w io.Writer, r statsResult) {
	fmt.Fprintf(w, "Menu items:  %d\n", r.MenuItemCount)
	fmt.Fprintf(w, "Stores:      %d\n", r.StoreCount)
	if len(r.ByCategory) > 0 {
		fmt.Fprintln(w, "\nBy category:")
		for cat, n := range r.ByCategory {
			fmt.Fprintf(w, "  %-20s  %d\n", cat, n)
		}
	}
	for _, n := range r.Notes {
		fmt.Fprintf(w, "Note: %s\n", n)
	}
}
