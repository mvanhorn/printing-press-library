package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"

	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Show aggregate statistics from the local cache",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Queries the local SQLite cache and returns aggregate counts for lists, items,
and recipes. Runs COUNT(*) and GROUP BY queries to compute totals, checked
vs unchecked ratios, and recipe rating distribution.

Requires sync — run 'anylist-pp-cli sync' first.`,
		Example: `  anylist-pp-cli stats
  anylist-pp-cli stats --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			st, err := store.Open(cfg)
			if err != nil {
				return fmt.Errorf("no local data found — run 'anylist-pp-cli sync' first")
			}
			defer st.Close()

			db := st.DB()
			ctx := cmd.Context()

			type listStat struct {
				Name      string `json:"list"`
				Total     int    `json:"total_items"`
				Checked   int    `json:"checked"`
				Unchecked int    `json:"unchecked"`
			}

			rows, err := db.QueryContext(ctx,
				`SELECT l.name,
				        COUNT(i.id) AS total,
				        SUM(CASE WHEN i.checked = 1 THEN 1 ELSE 0 END) AS checked,
				        SUM(CASE WHEN i.checked = 0 OR i.checked IS NULL THEN 1 ELSE 0 END) AS unchecked
				 FROM lists l
				 LEFT JOIN items i ON i.list_id = l.id
				 GROUP BY l.id, l.name
				 ORDER BY total DESC`)
			if err != nil {
				return fmt.Errorf("querying list stats: %w", err)
			}
			defer rows.Close()

			var listStats []listStat
			for rows.Next() {
				var s listStat
				if err := rows.Scan(&s.Name, &s.Total, &s.Checked, &s.Unchecked); err != nil {
					return fmt.Errorf("scanning: %w", err)
				}
				listStats = append(listStats, s)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating: %w", err)
			}

			var recipeCount int
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM recipes`).Scan(&recipeCount)

			if len(listStats) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No data — run 'anylist-pp-cli sync' first")
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"lists":        listStats,
					"recipe_count": recipeCount,
				}, flags)
			}

			w := cmd.OutOrStdout()
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "LIST\tTOTAL\tCHECKED\tUNCHECKED")
			for _, s := range listStats {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\n", s.Name, s.Total, s.Checked, s.Unchecked)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(w, "\nRecipes: %d\n", recipeCount)
			return nil
		},
	}
	return cmd
}
