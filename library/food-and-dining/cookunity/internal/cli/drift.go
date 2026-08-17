// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: week-over-week menu drift. Diffs two synced
// weekly snapshots (added / removed / price-changed meals). Preserved on regen.
package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/store"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/types"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		from   string
		to     string
	)

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "See exactly which meals were added, removed, or repriced between two weekly menus.",
		Long: `Diff two synced weekly menu snapshots by meal id.

Sync at least two delivery dates first, then:
  cookunity-pp-cli drift --from 2026-07-28 --to 2026-08-04

Omit --from/--to to diff the two most recent snapshots automatically.`,
		Example:     "  cookunity-pp-cli drift --from 2026-07-28 --to 2026-08-04 --json",
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
			// Read-write: SnapshotDates lazily creates the snapshot table
			// (CREATE TABLE IF NOT EXISTS), which a read-only handle rejects.
			db, err := store.OpenWithContext(ctx, path)
			if err != nil {
				return err
			}
			dates, err := db.SnapshotDates()
			if cerr := db.Close(); cerr != nil && err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			// Default to the two most recent snapshots.
			if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
				if len(dates) < 2 {
					msg := "need at least two synced weeks to compute drift; sync a second delivery date first"
					if flags.asJSON || flags.agent {
						return flags.printJSON(cmd, map[string]any{"note": msg, "snapshots": dates})
					}
					fmt.Fprintln(cmd.ErrOrStderr(), msg)
					return nil
				}
				// dates are newest-first
				to = dates[0]
				from = dates[1]
			}

			fromMeals, err := loadSnapshotMeals(ctx, path, from)
			if err != nil {
				return err
			}
			toMeals, err := loadSnapshotMeals(ctx, path, to)
			if err != nil {
				return err
			}
			fromByID := indexByID(fromMeals)
			toByID := indexByID(toMeals)

			type priceChange struct {
				Id       int     `json:"id"`
				Name     string  `json:"name"`
				OldPrice float64 `json:"old_price"`
				NewPrice float64 `json:"new_price"`
			}
			added := []types.Meal{}
			removed := []types.Meal{}
			repriced := []priceChange{}
			for id, m := range toByID {
				if _, ok := fromByID[id]; !ok {
					added = append(added, m)
				}
			}
			for id, m := range fromByID {
				if _, ok := toByID[id]; !ok {
					removed = append(removed, m)
				}
			}
			for id, oldM := range fromByID {
				if newM, ok := toByID[id]; ok && oldM.FinalPrice != newM.FinalPrice {
					repriced = append(repriced, priceChange{
						Id: id, Name: newM.Name,
						OldPrice: round2(oldM.FinalPrice), NewPrice: round2(newM.FinalPrice),
					})
				}
			}

			return flags.printJSON(cmd, map[string]any{
				"from":           from,
				"to":             to,
				"added":          added,
				"removed":        removed,
				"repriced":       repriced,
				"added_count":    len(added),
				"removed_count":  len(removed),
				"repriced_count": len(repriced),
			})
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	cmd.Flags().StringVar(&from, "from", "", "Earlier delivery date, YYYY-MM-DD (default: second-newest snapshot)")
	cmd.Flags().StringVar(&to, "to", "", "Later delivery date, YYYY-MM-DD (default: newest snapshot)")
	return cmd
}

func indexByID(meals []types.Meal) map[int]types.Meal {
	m := make(map[int]types.Meal, len(meals))
	for _, meal := range meals {
		m[meal.Id] = meal
	}
	return m
}
