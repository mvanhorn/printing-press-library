// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: set or show the daily macro goal.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelDiaryGoalCmd(flags *rootFlags) *cobra.Command {
	var flagKcal, flagProtein, flagFat, flagCarbs float64
	var flagDB string

	cmd := &cobra.Command{
		Use:         "goal",
		Short:       "Set or show your daily kcal/protein/fat/carbs goal.",
		Long:        "With no flags, print the current daily goal. With any of --kcal/--protein/--fat/--carbs, update the goal. The goal is used by `diary today` to compute remaining budget and by `budget` to constrain search.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli diary goal --kcal 2000 --protein 150\n  openfoodfacts-pp-cli diary goal", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd.Context(), flagDB)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			anySet := cmd.Flags().Changed("kcal") || cmd.Flags().Changed("protein") ||
				cmd.Flags().Changed("fat") || cmd.Flags().Changed("carbs")
			if anySet {
				cur, _, err := loadGoal(cmd.Context(), db.DB())
				if err != nil {
					return err
				}
				if cmd.Flags().Changed("kcal") {
					cur.Kcal = flagKcal
				}
				if cmd.Flags().Changed("protein") {
					cur.Protein = flagProtein
				}
				if cmd.Flags().Changed("fat") {
					cur.Fat = flagFat
				}
				if cmd.Flags().Changed("carbs") {
					cur.Carbs = flagCarbs
				}
				if _, err := db.DB().ExecContext(cmd.Context(),
					`INSERT INTO diary_goal (id, kcal, protein, fat, carbs) VALUES (1,?,?,?,?)
					 ON CONFLICT(id) DO UPDATE SET kcal=excluded.kcal, protein=excluded.protein, fat=excluded.fat, carbs=excluded.carbs`,
					cur.Kcal, cur.Protein, cur.Fat, cur.Carbs); err != nil {
					return fmt.Errorf("saving goal: %w", err)
				}
			}

			goal, ok, err := loadGoal(cmd.Context(), db.DB())
			if err != nil {
				return err
			}
			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"goal_set": ok, "goal": goal}, flags)
			}
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "no goal set — run `diary goal --kcal 2000 --protein 150`")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "daily goal: %.0f kcal, %.1fg protein, %.1fg fat, %.1fg carbs\n",
				goal.Kcal, goal.Protein, goal.Fat, goal.Carbs)
			return nil
		},
	}
	cmd.Flags().Float64Var(&flagKcal, "kcal", 0, "Daily energy goal in kcal")
	cmd.Flags().Float64Var(&flagProtein, "protein", 0, "Daily protein goal in grams")
	cmd.Flags().Float64Var(&flagFat, "fat", 0, "Daily fat goal in grams")
	cmd.Flags().Float64Var(&flagCarbs, "carbs", 0, "Daily carbohydrate goal in grams")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}
