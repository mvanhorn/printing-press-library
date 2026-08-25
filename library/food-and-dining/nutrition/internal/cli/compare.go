// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/nutridata"

	"github.com/spf13/cobra"
)

type compareRow struct {
	FdcID        int     `json:"fdc_id"`
	Description  string  `json:"description"`
	Basis        string  `json:"basis"`
	CaloriesKcal float64 `json:"calories_kcal"`
	ProteinG     float64 `json:"protein_g"`
	FatG         float64 `json:"fat_g"`
	CarbsG       float64 `json:"carbs_g"`
	FiberG       float64 `json:"fiber_g"`
	// ProteinPer100kcal is grams of protein per 100 kcal (protein density),
	// always computed regardless of the display basis because it is the key
	// cross-food metric no other nutrition CLI exposes.
	ProteinPer100kcal float64 `json:"protein_per_100kcal"`
}

type compareView struct {
	Basis      string       `json:"basis"`
	Count      int          `json:"count"`
	Foods      []compareRow `json:"foods"`
	MissingIDs []string     `json:"missing_ids,omitempty"`
	Source     string       `json:"source"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagBasis string

	cmd := &cobra.Command{
		Use:   "compare <fdcId> <fdcId> [<fdcId>...]",
		Short: "Compare 2-5 foods on a common basis (100g, serving, or 100kcal)",
		Long: "Compare 2-5 foods side by side on a common basis: per 100 g, per serving, or " +
			"per 100 kcal (protein density). Values come from USDA FoodData Central batch fetch " +
			"plus local per-basis math.\n\n" +
			"Use this command to compare foods on a common basis. Do NOT use it for a single " +
			"food's cross-source detail; use 'enrich' instead.",
		Example:     "  nutrition-pp-cli compare 173414 171287 175167 --basis 100kcal --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would fetch USDA foods and build a comparison table")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare needs at least 2 FDC ids"))
			}
			if len(args) > 5 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare accepts at most 5 foods (got %d)", len(args)))
			}
			basis := flagBasis
			if basis == "" {
				basis = "100g"
			}
			switch basis {
			case "100g", "serving", "100kcal":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--basis must be one of 100g, serving, 100kcal"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			foods, err := fetchUSDAFoods(cmd.Context(), c, args)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// USDA's batch endpoint silently omits ids it cannot return;
			// report them rather than presenting a shorter table as complete.
			returned := map[string]bool{}
			for _, f := range foods {
				returned[strconv.Itoa(f.FdcID)] = true
			}
			view := compareView{Basis: basis, Source: "usda"}
			for _, id := range args {
				if !returned[id] {
					view.MissingIDs = append(view.MissingIDs, id)
				}
			}
			for _, f := range foods {
				scale, serving := basisScale(f, basis)
				kcal := f.Calories()
				row := compareRow{
					FdcID:        f.FdcID,
					Description:  f.Description,
					Basis:        basisLabel(basis, serving),
					CaloriesKcal: round2(kcal * scale),
					ProteinG:     round2(f.Protein() * scale),
					FatG:         round2(f.Fat() * scale),
					CarbsG:       round2(f.Carbs() * scale),
					FiberG:       round2(f.Fiber() * scale),
				}
				if kcal > 0 {
					row.ProteinPer100kcal = round2(f.Protein() / kcal * 100.0)
				}
				view.Foods = append(view.Foods, row)
			}
			view.Count = len(view.Foods)
			return emitNutritionJSON(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagBasis, "basis", "100g", "Comparison basis: 100g, serving, or 100kcal")
	return cmd
}

// basisScale returns the multiplier to apply to per-100g nutrient values for the
// requested basis, plus the serving grams used (0 when not serving-based).
func basisScale(f nutridata.Food, basis string) (float64, float64) {
	switch basis {
	case "serving":
		if f.ServingSize > 0 {
			return f.ServingSize / 100.0, f.ServingSize
		}
		return 1.0, 0 // no serving size known; fall back to per-100g
	case "100kcal":
		kcal := f.Calories()
		if kcal > 0 {
			return 100.0 / kcal, 0
		}
		return 1.0, 0
	default: // 100g
		return 1.0, 0
	}
}

func basisLabel(basis string, serving float64) string {
	switch basis {
	case "serving":
		if serving > 0 {
			return fmt.Sprintf("per %.0f g serving", serving)
		}
		return "per 100 g (no serving size available)"
	case "100kcal":
		return "per 100 kcal"
	default:
		return "per 100 g"
	}
}
