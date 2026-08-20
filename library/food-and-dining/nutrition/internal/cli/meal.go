// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/nutridata"

	"github.com/spf13/cobra"
)

type mealItem struct {
	FdcID        int     `json:"fdc_id"`
	Description  string  `json:"description"`
	Grams        float64 `json:"grams"`
	CaloriesKcal float64 `json:"calories_kcal"`
	ProteinG     float64 `json:"protein_g"`
	FatG         float64 `json:"fat_g"`
	CarbsG       float64 `json:"carbs_g"`
	FiberG       float64 `json:"fiber_g"`
}

type mealTotals struct {
	CaloriesKcal float64 `json:"calories_kcal"`
	ProteinG     float64 `json:"protein_g"`
	FatG         float64 `json:"fat_g"`
	CarbsG       float64 `json:"carbs_g"`
	FiberG       float64 `json:"fiber_g"`
}

type mealFailure struct {
	Spec  string `json:"spec"`
	Error string `json:"error"`
}

type mealView struct {
	Items    []mealItem    `json:"items"`
	Totals   mealTotals    `json:"totals"`
	Failures []mealFailure `json:"fetch_failures"`
	Source   string        `json:"source"`
}

func newNovelMealCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meal <fdcId>:<qty> [<fdcId>:<qty>...]",
		Short: "Total nutrition across several foods at given quantities",
		Long: "Total nutrition across several foods in one stateless call. Each argument is " +
			"<fdcId>:<quantity>, where quantity is grams (e.g. 150g or 150) or servings " +
			"(e.g. 1serving, using the food's USDA serving size).\n\n" +
			"Use this command for a one-shot nutrition total of several foods at given " +
			"quantities. Do NOT use it to record what you ate; use 'log' instead.",
		Example:     "  nutrition-pp-cli meal 173414:50g 173944:120g --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would total nutrition across the given foods")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one <fdcId>:<qty> pair is required"))
			}

			type parsedSpec struct {
				spec string
				id   string
				qty  string
			}
			var specs []parsedSpec
			ids := make([]string, 0, len(args))
			seen := map[string]bool{}
			var failures []mealFailure
			for _, a := range args {
				id, qty, ok := strings.Cut(a, ":")
				if !ok || id == "" || qty == "" {
					failures = append(failures, mealFailure{Spec: a, Error: "expected <fdcId>:<qty>"})
					continue
				}
				specs = append(specs, parsedSpec{spec: a, id: id, qty: qty})
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
			if len(ids) == 0 {
				return usageErr(fmt.Errorf("no valid <fdcId>:<qty> pairs"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			foods, err := fetchUSDAFoods(cmd.Context(), c, ids)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			byID := map[string]nutridata.Food{}
			for _, f := range foods {
				byID[strconv.Itoa(f.FdcID)] = f
			}

			view := mealView{Source: "usda", Failures: failures}
			for _, s := range specs {
				f, ok := byID[s.id]
				if !ok {
					view.Failures = append(view.Failures, mealFailure{Spec: s.spec, Error: "food not found in USDA response"})
					continue
				}
				grams, err := parseQuantityGrams(s.qty, f)
				if err != nil {
					view.Failures = append(view.Failures, mealFailure{Spec: s.spec, Error: err.Error()})
					continue
				}
				item := mealItem{
					FdcID:        f.FdcID,
					Description:  f.Description,
					Grams:        round2(grams),
					CaloriesKcal: round2(scaleAmount(f.Calories(), grams)),
					ProteinG:     round2(scaleAmount(f.Protein(), grams)),
					FatG:         round2(scaleAmount(f.Fat(), grams)),
					CarbsG:       round2(scaleAmount(f.Carbs(), grams)),
					FiberG:       round2(scaleAmount(f.Fiber(), grams)),
				}
				view.Items = append(view.Items, item)
				view.Totals.CaloriesKcal += item.CaloriesKcal
				view.Totals.ProteinG += item.ProteinG
				view.Totals.FatG += item.FatG
				view.Totals.CarbsG += item.CarbsG
				view.Totals.FiberG += item.FiberG
			}
			view.Totals.CaloriesKcal = round2(view.Totals.CaloriesKcal)
			view.Totals.ProteinG = round2(view.Totals.ProteinG)
			view.Totals.FatG = round2(view.Totals.FatG)
			view.Totals.CarbsG = round2(view.Totals.CarbsG)
			view.Totals.FiberG = round2(view.Totals.FiberG)
			if view.Items == nil {
				view.Items = []mealItem{}
			}
			if view.Failures == nil {
				view.Failures = []mealFailure{}
			}
			return emitNutritionJSON(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

// parseQuantityGrams converts a quantity token to grams. Supports "150g",
// "150" (grams), and "1serving"/"2servings" (multiples of the food's USDA
// serving size when it is expressed in grams).
func parseQuantityGrams(qty string, f nutridata.Food) (float64, error) {
	q := strings.ToLower(strings.TrimSpace(qty))
	switch {
	case strings.HasSuffix(q, "serving"), strings.HasSuffix(q, "servings"):
		numStr := strings.TrimSuffix(strings.TrimSuffix(q, "s"), "serving")
		n := 1.0
		if strings.TrimSpace(numStr) != "" {
			v, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
			if err != nil {
				return 0, fmt.Errorf("invalid serving count %q", qty)
			}
			n = v
		}
		if f.ServingSize <= 0 || !strings.EqualFold(f.ServingUnit, "g") {
			return 0, fmt.Errorf("no gram serving size known for fdc %d; use grams instead", f.FdcID)
		}
		return n * f.ServingSize, nil
	case strings.HasSuffix(q, "g"):
		v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(q, "g")), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid gram amount %q", qty)
		}
		return v, nil
	default:
		v, err := strconv.ParseFloat(q, 64)
		if err != nil {
			return 0, fmt.Errorf("unrecognized quantity %q (use grams like 150g or servings like 1serving)", qty)
		}
		return v, nil
	}
}
