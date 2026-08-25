// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: the `meals` command reads the locally-synced CookUnity menu
// (offline) with composable macro/diet/cuisine/chef/allergen filters, plus a
// `meals get <id>` detail view. Preserved on regen.
package cli

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newMealsPromotedCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath          string
		limit           int
		sortKey         string
		diet            string
		cuisine         string
		chef            string
		minProtein      float64
		maxCalories     int
		maxPrice        float64
		excludeAllergen string
		inStockOnly     bool
	)

	cmd := &cobra.Command{
		Use:   "meals",
		Short: "Browse the locally-synced CookUnity menu offline, with macro/diet/cuisine filters",
		Long: `Browse the locally-synced CookUnity menu offline. Filters compose:

  cookunity-pp-cli meals --diet gluten-free --min-protein 40 --max-calories 600 --sort protein

Run 'cookunity-pp-cli sync' first to populate the local store.`,
		Example:     "  cookunity-pp-cli meals --diet gluten-free --min-protein 40 --json",
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
			db, err := store.OpenReadOnlyContext(ctx, path)
			if err != nil {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "meals") {
				hintIfStale(cmd, db, "meals", flags.maxAge)
			}
			meals, err := loadMealsFromStore(db)
			if err != nil {
				return err
			}
			meals = applyMealFilter(meals, mealFilter{
				diet: diet, cuisine: cuisine, chef: chef,
				minProtein: minProtein, maxCalories: maxCalories, maxPrice: maxPrice,
				excludeAllergen: excludeAllergen, inStockOnly: inStockOnly,
			})
			sortMeals(meals, sortKey)
			if limit > 0 && len(meals) > limit {
				meals = meals[:limit]
			}
			return emitMeals(cmd.OutOrStdout(), meals, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum meals to return (0 = all)")
	cmd.Flags().StringVar(&sortKey, "sort", "", "Sort by: calories | protein | price | rating | name")
	cmd.Flags().StringVar(&diet, "diet", "", "Filter by diet tag (e.g. gluten-free, high-protein, keto, vegan)")
	cmd.Flags().StringVar(&cuisine, "cuisine", "", "Filter by cuisine (e.g. italian, asian, american)")
	cmd.Flags().StringVar(&chef, "chef", "", "Filter by chef name (substring match)")
	cmd.Flags().Float64Var(&minProtein, "min-protein", 0, "Minimum protein grams")
	cmd.Flags().IntVar(&maxCalories, "max-calories", 0, "Maximum calories")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum final price (USD)")
	cmd.Flags().StringVar(&excludeAllergen, "exclude-allergen", "", "Exclude meals containing these allergens (comma-separated)")
	cmd.Flags().BoolVar(&inStockOnly, "in-stock", false, "Only meals currently in stock")

	cmd.AddCommand(newMealsGetCmd(flags))
	return cmd
}

// pp:data-source local
func newMealsGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "get [meal-id]",
		Short:       "Show one meal's full detail (macros, chef, ingredients, allergens) from the local store",
		Example:     "  cookunity-pp-cli meals get 3707 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("meal id is required"))
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("meal id must be a number, got %q", args[0]))
			}
			path := resolveMealDBPath(dbPath)
			if !mealDBExists(path) {
				return missingMirror(cmd, path, flags)
			}
			meals, err := loadMeals(ctx, path)
			if err != nil {
				return err
			}
			for _, m := range meals {
				if m.Id == id {
					return flags.printJSON(cmd, m)
				}
			}
			return notFoundErr(fmt.Errorf("meal %d not found in the local store (try 'cookunity-pp-cli sync')", id))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	return cmd
}
