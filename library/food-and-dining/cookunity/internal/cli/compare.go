// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: side-by-side comparison of two or more meals.
// Preserved on regen.
package cli

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/types"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "compare [meal-id] [meal-id] ...",
		Short: "Side-by-side macro, price, chef, and allergen comparison of two or more meals.",
		Long: `Compare two or more meals from the locally-synced menu by id.

Run 'cookunity-pp-cli sync' first, then pass meal ids (see 'cookunity-pp-cli meals').`,
		Example:     "  cookunity-pp-cli compare 3707 3210 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "first=3707;second=3210"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				// Verify probes may synthesize fewer than two positionals; a
				// missing-input probe is not a real failure under verify.
				if cliutil.IsVerifyEnv() {
					return nil
				}
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare needs at least two meal ids"))
			}
			path := resolveMealDBPath(dbPath)
			if !mealDBExists(path) {
				return missingMirror(cmd, path, flags)
			}
			meals, err := loadMeals(ctx, path)
			if err != nil {
				return err
			}
			byID := map[int]types.Meal{}
			for _, m := range meals {
				byID[m.Id] = m
			}
			var picked []types.Meal
			var missing []string
			for _, a := range args {
				id, err := strconv.Atoi(a)
				if err != nil {
					return usageErr(fmt.Errorf("meal id must be a number, got %q", a))
				}
				if m, ok := byID[id]; ok {
					picked = append(picked, m)
				} else {
					missing = append(missing, a)
				}
			}
			result := map[string]any{"meals": picked, "compared": len(picked)}
			if len(missing) > 0 {
				result["not_found"] = missing
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	return cmd
}
