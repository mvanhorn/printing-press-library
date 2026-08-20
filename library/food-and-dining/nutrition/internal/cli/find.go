// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/nutridata"

	"github.com/spf13/cobra"
)

type findMatch struct {
	FdcID        int     `json:"fdc_id"`
	Description  string  `json:"description"`
	DataType     string  `json:"data_type"`
	CaloriesKcal float64 `json:"calories_kcal_per_100g"`
	ProteinG     float64 `json:"protein_g_per_100g"`
	FatG         float64 `json:"fat_g_per_100g"`
	CarbsG       float64 `json:"carbs_g_per_100g"`
	FiberG       float64 `json:"fiber_g_per_100g"`
}

type findView struct {
	Matches      []findMatch `json:"matches"`
	Count        int         `json:"count"`
	ScannedFoods int         `json:"scanned_foods"`
	MaxScanPages int         `json:"max_scan_pages"`
	Note         string      `json:"note,omitempty"`
	Source       string      `json:"source"`
}

type threshold struct {
	number string
	name   string
	value  float64
	isMax  bool
}

func newNovelFindCmd(flags *rootFlags) *cobra.Command {
	var flagMin []string
	var flagMax []string
	var flagMaxKcal float64
	var flagMinProtein float64
	var flagDataType string
	var flagLimit int
	var flagMaxScanPages int

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Find foods meeting multiple nutrient thresholds at once",
		Long: "Find foods that satisfy multiple nutrient thresholds at once (e.g. protein >= 20 g " +
			"AND kcal <= 165 per 100 g). Scans USDA FoodData Central and filters nutrient arrays " +
			"locally. All thresholds are per 100 g.\n\n" +
			"Use this command for multi-condition nutrient thresholds. Do NOT use it for a simple " +
			"top-N by one nutrient; use 'rank' instead.",
		Example:     "  nutrition-pp-cli find --min protein=20 --max-kcal 165 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would scan USDA foods and filter by nutrient thresholds")
			}

			thresholds, err := buildThresholds(flagMin, flagMax, flagMaxKcal, flagMinProtein)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if len(thresholds) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one threshold is required (e.g. --min protein=20 or --max-kcal 165)"))
			}
			if flagLimit <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be > 0"))
			}
			if flagMaxScanPages <= 0 {
				flagMaxScanPages = 5
			}

			if cliutil.IsDogfoodEnv() && flagMaxScanPages > 1 {
				flagMaxScanPages = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var matches []findMatch
			scanned := 0
			scanCapHit := true
			for page := 1; page <= flagMaxScanPages && len(matches) < flagLimit; page++ {
				params := map[string]string{
					"pageSize":   "200",
					"pageNumber": strconv.Itoa(page),
				}
				if flagDataType != "" {
					params["dataType"] = flagDataType
				}
				raw, err := c.Get(cmd.Context(), "/v1/foods/list", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var arr []json.RawMessage
				if err := json.Unmarshal(raw, &arr); err != nil {
					return fmt.Errorf("parsing foods list page %d: %w", page, err)
				}
				if len(arr) == 0 {
					scanCapHit = false
					break
				}
				for _, item := range arr {
					scanned++
					f, err := nutridata.Normalize(item)
					if err != nil {
						continue
					}
					if !matchesThresholds(f, thresholds) {
						continue
					}
					matches = append(matches, findMatch{
						FdcID:        f.FdcID,
						Description:  f.Description,
						DataType:     f.DataType,
						CaloriesKcal: round2(f.Calories()),
						ProteinG:     round2(f.Protein()),
						FatG:         round2(f.Fat()),
						CarbsG:       round2(f.Carbs()),
						FiberG:       round2(f.Fiber()),
					})
					if len(matches) >= flagLimit {
						break
					}
				}
			}

			view := findView{
				Matches:      matches,
				Count:        len(matches),
				ScannedFoods: scanned,
				MaxScanPages: flagMaxScanPages,
				Source:       "usda",
			}
			if view.Matches == nil {
				view.Matches = []findMatch{}
			}
			if len(matches) == 0 && scanCapHit {
				view.Note = fmt.Sprintf("scanned %d foods across up to %d pages without a match; raise --max-scan-pages to widen the search", scanned, flagMaxScanPages)
			}
			return emitNutritionJSON(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringArrayVar(&flagMin, "min", nil, "Minimum threshold as nutrient=amount per 100g (repeatable), e.g. --min protein=20")
	cmd.Flags().StringArrayVar(&flagMax, "max", nil, "Maximum threshold as nutrient=amount per 100g (repeatable), e.g. --max sugars=5")
	cmd.Flags().Float64Var(&flagMaxKcal, "max-kcal", 0, "Convenience: maximum calories per 100g")
	cmd.Flags().Float64Var(&flagMinProtein, "min-protein", 0, "Convenience: minimum protein grams per 100g")
	cmd.Flags().StringVar(&flagDataType, "data-type", "", "Filter to a USDA dataType (Foundation, SR Legacy, Survey (FNDDS), Branded)")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum matching foods to return")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 5, "Maximum foods-list pages to scan (200 foods/page)")
	return cmd
}

func buildThresholds(mins, maxes []string, maxKcal, minProtein float64) ([]threshold, error) {
	var out []threshold
	parse := func(specs []string, isMax bool) error {
		for _, spec := range specs {
			name, valStr, ok := strings.Cut(spec, "=")
			if !ok {
				return fmt.Errorf("threshold %q must be nutrient=amount", spec)
			}
			num, ok := nutridata.ResolveNutrient(name)
			if !ok {
				return fmt.Errorf("unknown nutrient %q (try protein, kcal, fat, carbs, fiber, sugars, sodium)", name)
			}
			v, err := strconv.ParseFloat(strings.TrimSpace(valStr), 64)
			if err != nil {
				return fmt.Errorf("invalid amount in %q: %w", spec, err)
			}
			out = append(out, threshold{number: num, name: strings.TrimSpace(name), value: v, isMax: isMax})
		}
		return nil
	}
	if err := parse(mins, false); err != nil {
		return nil, err
	}
	if err := parse(maxes, true); err != nil {
		return nil, err
	}
	if maxKcal > 0 {
		out = append(out, threshold{number: nutridata.NutrNumEnergyKcal, name: "kcal", value: maxKcal, isMax: true})
	}
	if minProtein > 0 {
		out = append(out, threshold{number: nutridata.NutrNumProtein, name: "protein", value: minProtein, isMax: false})
	}
	return out, nil
}

func matchesThresholds(f nutridata.Food, ts []threshold) bool {
	for _, t := range ts {
		var amt float64
		var ok bool
		// Energy must resolve across dataTypes (208 / Atwater 957 / 958);
		// Food.Calories() handles the fallback, so don't use raw Amount("208")
		// which misses Foundation foods.
		if t.number == nutridata.NutrNumEnergyKcal {
			amt = f.Calories()
			ok = amt > 0
		} else {
			amt, ok = f.Amount(t.number)
		}
		if !ok {
			return false // cannot confirm the threshold; exclude
		}
		if t.isMax && amt > t.value {
			return false
		}
		if !t.isMax && amt < t.value {
			return false
		}
	}
	return true
}
