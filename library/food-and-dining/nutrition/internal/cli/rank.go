// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/source/nutritionvalue"

	"github.com/spf13/cobra"
)

type rankView struct {
	Nutrient string                   `json:"nutrient"`
	Order    string                   `json:"order"`
	Category string                   `json:"category,omitempty"`
	Count    int                      `json:"count"`
	Foods    []nutritionvalue.RankRow `json:"foods"`
	Source   string                   `json:"source"`
	Note     string                   `json:"note,omitempty"`
}

func newNovelRankCmd(flags *rootFlags) *cobra.Command {
	var flagOrder string
	var flagCategory string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "rank <nutrient>",
		Short: "Rank foods by a single nutrient from NutritionValue.org",
		Long: "List the top (or bottom) foods by a single nutrient, ingested from " +
			"NutritionValue.org's precomputed ranking pages (~60 nutrients).\n\n" +
			"Use this command for top or bottom foods by one nutrient. Do NOT use it for " +
			"compound thresholds like high protein under N kcal; use 'find' instead.",
		Example:     "  nutrition-pp-cli rank potassium --order lowest --limit 15 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would fetch NutritionValue.org ranking page")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a nutrient is required, e.g. rank protein"))
			}
			nutrient := args[0]
			order := flagOrder
			if order == "" {
				order = "highest"
			}
			if order != "highest" && order != "lowest" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--order must be 'highest' or 'lowest'"))
			}

			nv := nutritionvalue.New()
			rows, err := nv.Rank(cmd.Context(), nutrient, order, flagLimit)
			if err != nil {
				return err
			}
			view := rankView{
				Nutrient: nutritionvalue.NutrientPageName(nutrient),
				Order:    order,
				Category: flagCategory,
				Count:    len(rows),
				Foods:    rows,
				Source:   "nutritionvalue",
			}
			if len(rows) == 0 {
				view.Note = fmt.Sprintf("no ranking page found for nutrient %q; try a canonical name like 'protein', 'vitamin c', or 'potassium'", nutrient)
			}
			return emitNutritionJSON(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagOrder, "order", "highest", "Ranking order: highest or lowest")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Optional category label (recorded in output; NutritionValue.org ranking pages are cross-category)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum foods to return")
	return cmd
}
