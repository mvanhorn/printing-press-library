// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: best-value leaderboard. Ranks meals by
// nutritional value per dollar computed from the local mirror. Preserved on regen.
package cli

import (
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/types"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelValueCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath  string
		metric  string
		limit   int
		diet    string
		cuisine string
	)

	cmd := &cobra.Command{
		Use:   "value",
		Short: "Rank meals by nutritional value per dollar (protein-per-dollar, calories-per-dollar) computed from the local mirror",
		Long: `Rank the locally-synced menu by nutritional value per dollar.

Metrics: protein-per-dollar (default), calories-per-dollar.
Run 'cookunity-pp-cli sync' first to populate the local store.`,
		Example:     "  cookunity-pp-cli value --metric protein-per-dollar --limit 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}
			path := resolveMealDBPath(dbPath)
			if !mealDBExists(path) {
				return missingMirror(cmd, path, flags)
			}
			meals, err := loadMeals(ctx, path)
			if err != nil {
				return err
			}
			meals = applyMealFilter(meals, mealFilter{diet: diet, cuisine: cuisine})

			ratio := func(m types.Meal) float64 {
				if m.FinalPrice <= 0 {
					return 0
				}
				if metric == "calories-per-dollar" {
					return float64(m.Calories) / m.FinalPrice
				}
				return m.Protein / m.FinalPrice
			}
			sort.SliceStable(meals, func(i, j int) bool { return ratio(meals[i]) > ratio(meals[j]) })
			if limit > 0 && len(meals) > limit {
				meals = meals[:limit]
			}

			type row struct {
				types.Meal
				Value  float64 `json:"value_per_dollar"`
				Metric string  `json:"metric"`
			}
			rows := make([]row, 0, len(meals))
			for _, m := range meals {
				rows = append(rows, row{Meal: m, Value: round2(ratio(m)), Metric: metric})
			}
			return flags.printJSON(cmd, rows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	cmd.Flags().StringVar(&metric, "metric", "protein-per-dollar", "Ranking metric: protein-per-dollar | calories-per-dollar")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum meals to return")
	cmd.Flags().StringVar(&diet, "diet", "", "Restrict to a diet tag")
	cmd.Flags().StringVar(&cuisine, "cuisine", "", "Restrict to a cuisine")
	return cmd
}
