// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: which favorited meals are on the current menu.
// Reads the isFavorite flag carried in the stored meal JSON. Preserved on regen.
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/cookunity"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelFavoritesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "favorites",
		Short: "Show which of your favorited meals are actually on the current week's menu.",
		Long: `List the meals you have marked favorite that appear on the current
locally-synced menu. Favorites are read from the isFavorite flag captured
during sync.

Run 'cookunity-pp-cli sync' first to populate the local store.`,
		Example:     "  cookunity-pp-cli favorites --json",
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
			meals, err := loadExtendedMeals(ctx, path)
			if err != nil {
				return err
			}
			favs := make([]cookunity.Meal, 0)
			for _, m := range meals {
				if m.IsFavorite {
					favs = append(favs, m)
				}
			}
			if len(favs) == 0 {
				if flags.asJSON || flags.agent {
					return flags.printJSON(cmd, map[string]any{
						"favorites": []any{},
						"count":     0,
						"note":      "no favorited meals are on the current menu (favorite meals in the CookUnity app, then re-sync)",
					})
				}
			}
			return flags.printJSON(cmd, favs)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	return cmd
}
