// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: offline macro-constrained meal planner.
// Selects a set of meals from the local mirror that collectively fit
// protein / calorie / budget / diet constraints. Preserved on regen.
package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/types"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelPlanCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		proteinMin  float64
		caloriesMax int
		count       int
		budget      float64
		diet        string
		cuisine     string
		excludeAllg string
	)

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Auto-build a set of meals that collectively hit your protein, calorie, budget, and diet targets — offline",
		Long: `Build a meal set from the locally-synced menu that fits your constraints.

Selection is greedy: eligible meals (matching --diet/--cuisine, under
--calories-max per meal, excluding --exclude-allergen) are ranked by protein
per dollar, then chosen highest-first while total price stays within --budget,
until --count meals are selected.

Run 'cookunity-pp-cli sync' first to populate the local store.`,
		Example:     "  cookunity-pp-cli plan --protein-min 40 --calories-max 600 --count 8 --diet gluten-free --agent",
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
			// Eligibility filter (per-meal constraints).
			eligible := applyMealFilter(meals, mealFilter{
				diet: diet, cuisine: cuisine, maxCalories: caloriesMax,
				excludeAllergen: excludeAllg, inStockOnly: true,
			})
			var (
				selected  []types.Meal
				totalCals int
				totalProt float64
				totalCost float64
				picked    = make(map[int]bool)
			)
			// add appends a meal if it fits the count and budget limits and
			// hasn't already been picked; reports whether it was added.
			add := func(m types.Meal) bool {
				if picked[m.Id] {
					return false
				}
				if count > 0 && len(selected) >= count {
					return false
				}
				if budget > 0 && totalCost+m.FinalPrice > budget {
					return false
				}
				picked[m.Id] = true
				selected = append(selected, m)
				totalCals += m.Calories
				totalProt += m.Protein
				totalCost += m.FinalPrice
				return true
			}

			// Phase 1 — satisfy the protein target. When --protein-min is set,
			// front-load the highest-protein meals so the target actually
			// constrains selection, adding until the total reaches the target
			// or the count/budget limits bind.
			if proteinMin > 0 {
				byProtein := make([]types.Meal, len(eligible))
				copy(byProtein, eligible)
				sort.SliceStable(byProtein, func(i, j int) bool {
					return byProtein[i].Protein > byProtein[j].Protein
				})
				for _, m := range byProtein {
					if totalProt >= proteinMin {
						break
					}
					if count > 0 && len(selected) >= count {
						break
					}
					add(m)
				}
			}

			// Phase 2 — fill any remaining slots by protein-per-dollar (value).
			byValue := make([]types.Meal, len(eligible))
			copy(byValue, eligible)
			sort.SliceStable(byValue, func(i, j int) bool {
				return proteinPerDollar(byValue[i]) > proteinPerDollar(byValue[j])
			})
			for _, m := range byValue {
				if count > 0 && len(selected) >= count {
					break
				}
				add(m)
			}

			// Phase 3 — local-search improvement. A pure greedy pass can leave
			// the protein target unmet even when a feasible plan exists (e.g.
			// phase 1 spent the budget on expensive high-protein picks). While
			// under target, repeatedly swap a lower-protein selected meal for a
			// higher-protein eligible one whenever the swap raises total protein
			// and stays within budget. This recovers feasible combinations the
			// greedy ordering misses, hill-climbing until the target is reached
			// or no improving swap remains.
			if proteinMin > 0 && totalProt < proteinMin {
				byProteinDesc := make([]types.Meal, len(eligible))
				copy(byProteinDesc, eligible)
				sort.SliceStable(byProteinDesc, func(i, j int) bool {
					return byProteinDesc[i].Protein > byProteinDesc[j].Protein
				})
				for improved := true; improved && totalProt < proteinMin; {
					improved = false
					for _, cand := range byProteinDesc {
						if picked[cand.Id] {
							continue
						}
						for si := range selected {
							sel := selected[si]
							if cand.Protein <= sel.Protein {
								continue
							}
							newCost := totalCost - sel.FinalPrice + cand.FinalPrice
							if budget > 0 && newCost > budget {
								continue
							}
							picked[sel.Id] = false
							picked[cand.Id] = true
							totalProt += cand.Protein - sel.Protein
							totalCals += cand.Calories - sel.Calories
							totalCost = newCost
							selected[si] = cand
							improved = true
							break
						}
						if improved {
							break
						}
					}
				}
			}

			// Derive one rounded protein total and base BOTH the displayed
			// values and meets_protein on it, so the reported total, the target
			// status, and the note can never contradict each other (a raw total
			// a hair under the target that rounds up now reads as met, matching
			// the displayed figure).
			roundedProt := round1(totalProt)
			meetsProtein := proteinMin == 0 || roundedProt >= proteinMin

			result := map[string]any{
				"meals":            selected,
				"count":            len(selected),
				"total_calories":   totalCals,
				"total_protein":    roundedProt,
				"total_cost":       round2(totalCost),
				"protein_target":   proteinMin,
				"protein_achieved": roundedProt,
				"meets_protein":    meetsProtein,
			}
			if len(selected) == 0 {
				result["note"] = "no meals matched the constraints; loosen --diet/--calories-max or run 'cookunity-pp-cli sync' for the target week"
			} else if proteinMin > 0 && !meetsProtein {
				// Selection is a greedy heuristic with a local-search swap pass,
				// not an exhaustive optimizer, so it can fall short when --count
				// or --budget bind before enough protein is gathered. Report the
				// shortfall honestly rather than implying no feasible plan exists.
				result["note"] = fmt.Sprintf(
					"fell short of the %gg protein target (best plan found: %gg within the --count/--budget limits). Raise --count or --budget, or lower --protein-min.",
					proteinMin, roundedProt)
			}
			return flags.printJSON(cmd, result)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	cmd.Flags().Float64Var(&proteinMin, "protein-min", 0, "Minimum total protein grams the plan should reach; front-loads high-protein meals to hit it (0 = value-ranked only)")
	cmd.Flags().IntVar(&caloriesMax, "calories-max", 0, "Maximum calories per meal (0 = no cap)")
	cmd.Flags().IntVar(&count, "count", 8, "Number of meals to select")
	cmd.Flags().Float64Var(&budget, "budget", 0, "Maximum total price across selected meals (0 = no cap)")
	cmd.Flags().StringVar(&diet, "diet", "", "Restrict to a diet tag (e.g. gluten-free, high-protein, keto)")
	cmd.Flags().StringVar(&cuisine, "cuisine", "", "Restrict to a cuisine")
	cmd.Flags().StringVar(&excludeAllg, "exclude-allergen", "", "Exclude meals with these allergens (comma-separated)")
	return cmd
}

func proteinPerDollar(m types.Meal) float64 {
	if m.FinalPrice <= 0 {
		return 0
	}
	return m.Protein / m.FinalPrice
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
