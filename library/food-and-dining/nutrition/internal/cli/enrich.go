// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/nutridata"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/source/nutritionvalue"

	"github.com/spf13/cobra"
)

// enrichView is the merged USDA + NutritionValue.org record.
type enrichView struct {
	FdcID        int     `json:"fdc_id"`
	Description  string  `json:"description"`
	DataType     string  `json:"data_type"`
	CaloriesKcal float64 `json:"calories_kcal_per_100g"`
	ProteinG     float64 `json:"protein_g_per_100g"`
	FatG         float64 `json:"fat_g_per_100g"`
	CarbsG       float64 `json:"carbs_g_per_100g"`
	FiberG       float64 `json:"fiber_g_per_100g"`
	// NutritionValue.org-derived analytics the USDA API does not expose.
	NetCarbsG    *float64 `json:"net_carbs_g_per_100g,omitempty"`
	Omega3G      *float64 `json:"omega_3_g_per_100g,omitempty"`
	Omega6G      *float64 `json:"omega_6_g_per_100g,omitempty"`
	Omega63Ratio *float64 `json:"omega_6_3_ratio,omitempty"`
	Source       string   `json:"source"`
	NVMatched    bool     `json:"nutritionvalue_matched"`
	NVNote       string   `json:"nutritionvalue_note,omitempty"`
}

func newNovelEnrichCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enrich <fdcId>",
		Short: "Merge NutritionValue.org derived analytics onto a USDA food",
		Long: "Fetch a USDA FoodData Central record by FDC id and merge NutritionValue.org's " +
			"derived analytics onto it: net carbs, omega-6/omega-3 ratio, and per-nutrient " +
			"detail the USDA API does not expose. NutritionValue.org food ids are the same FDC " +
			"ids, so the two sources join exactly.\n\n" +
			"Use this command to get NutritionValue.org derived analytics (net carbs, omega " +
			"ratio) merged onto one food. Do NOT use it for raw nutrient arrays or portion " +
			"scaling; use 'food' instead.",
		Example:     "  nutrition-pp-cli enrich 173414 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would fetch USDA food and merge NutritionValue.org analytics")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an FDC id is required, e.g. enrich 173414"))
			}
			fdcID := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			food, _, err := fetchUSDAFood(cmd.Context(), c, fdcID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			view := enrichView{
				FdcID:        food.FdcID,
				Description:  food.Description,
				DataType:     food.DataType,
				CaloriesKcal: round2(food.Calories()),
				ProteinG:     round2(food.Protein()),
				FatG:         round2(food.Fat()),
				CarbsG:       round2(food.Carbs()),
				FiberG:       round2(food.Fiber()),
				Source:       "usda+nutritionvalue",
			}

			nv := nutritionvalue.New()
			detail, nvErr := nv.FoodByID(cmd.Context(), fdcID, food.Description)
			if nvErr != nil {
				// NutritionValue.org enrichment is best-effort; a miss still
				// returns the USDA record with an honest note rather than failing.
				view.NVNote = "NutritionValue.org lookup failed: " + nvErr.Error()
			} else {
				view.NVMatched = true
				view.NetCarbsG = detail.NetCarbs
				view.Omega3G = detail.Omega3
				view.Omega6G = detail.Omega6
				view.Omega63Ratio = detail.OmegaRatio
			}
			// Fall back to computing net carbs from USDA (carbs - fiber) when
			// NutritionValue.org did not surface it.
			if view.NetCarbsG == nil {
				if _, ok := food.Amount(nutridata.NutrNumCarb); ok {
					nc := round2(food.Carbs() - food.Fiber())
					if nc < 0 {
						nc = 0
					}
					view.NetCarbsG = &nc
					if view.NVNote == "" {
						view.NVNote = "net carbs computed from USDA (carbs - fiber); NutritionValue.org value unavailable"
					}
				}
			}

			return emitNutritionJSON(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}
