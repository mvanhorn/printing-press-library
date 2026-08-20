// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: shared helpers for reading and filtering the locally-synced
// CookUnity meal catalog. Preserved on regen.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/cookunity"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/store"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/types"
)

// resolveMealDBPath returns the db path to use, defaulting to the CLI's
// standard local database when the flag is empty.
func resolveMealDBPath(dbPath string) string {
	if strings.TrimSpace(dbPath) != "" {
		return dbPath
	}
	return defaultDBPath("cookunity-pp-cli")
}

// mealDBExists reports whether the local mirror exists.
func mealDBExists(dbPath string) bool {
	_, err := os.Stat(dbPath)
	return err == nil
}

// missingMirror writes the standard "no local mirror" hint to stderr and, for
// machine output, an empty JSON array to stdout. Returns nil so callers can
// `return missingMirror(...)`: a missing mirror is an empty local cache, not an
// error.
func missingMirror(cmd interface {
	ErrOrStderr() io.Writer
	OutOrStdout() io.Writer
}, dbPath string, flags *rootFlags) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: cookunity-pp-cli sync --param date=<YYYY-MM-DD>\n", dbPath)
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return nil
}

// loadMeals reads all meals from the current-week `meals` table into typed structs.
func loadMeals(ctx context.Context, dbPath string) ([]types.Meal, error) {
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return loadMealsFromStore(db)
}

// loadMealsFromStore reads all current-week meals from an already-open store.
func loadMealsFromStore(db *store.Store) ([]types.Meal, error) {
	rows, err := db.Query(`SELECT "data" FROM "meals"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var meals []types.Meal
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var m types.Meal
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			continue
		}
		meals = append(meals, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return meals, nil
}

// loadExtendedMeals reads current-week meals including the isFavorite flag that
// the generated types.Meal does not carry.
func loadExtendedMeals(ctx context.Context, dbPath string) ([]cookunity.Meal, error) {
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT "data" FROM "meals"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var meals []cookunity.Meal
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var m cookunity.Meal
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			continue
		}
		meals = append(meals, m)
	}
	return meals, rows.Err()
}

// loadSnapshotMeals reads meals for a specific delivery date from the snapshot
// table. Opens read-write because SnapshotMeals lazily creates the snapshot
// table (CREATE TABLE IF NOT EXISTS), which a read-only handle rejects.
func loadSnapshotMeals(ctx context.Context, dbPath, deliveryDate string) ([]types.Meal, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	raws, err := db.SnapshotMeals(deliveryDate)
	if err != nil {
		return nil, err
	}
	meals := make([]types.Meal, 0, len(raws))
	for _, r := range raws {
		var m types.Meal
		if err := json.Unmarshal(r, &m); err != nil {
			continue
		}
		meals = append(meals, m)
	}
	return meals, nil
}

// mealFilter captures the common filter flags shared across meal commands.
type mealFilter struct {
	diet            string
	cuisine         string
	chef            string
	minProtein      float64
	maxCalories     int
	maxPrice        float64
	excludeAllergen string
	inStockOnly     bool
}

func containsFold(haystack, needle string) bool {
	if strings.TrimSpace(needle) == "" {
		return true
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(strings.TrimSpace(needle)))
}

// keep reports whether a meal passes the filter.
func (f mealFilter) keep(m types.Meal) bool {
	if !containsFold(m.DietTags, f.diet) {
		return false
	}
	if !containsFold(m.Cuisines, f.cuisine) {
		return false
	}
	if !containsFold(m.ChefName, f.chef) {
		return false
	}
	if f.minProtein > 0 && m.Protein < f.minProtein {
		return false
	}
	if f.maxCalories > 0 && m.Calories > f.maxCalories {
		return false
	}
	if f.maxPrice > 0 && m.FinalPrice > f.maxPrice {
		return false
	}
	if f.inStockOnly && !m.InStock {
		return false
	}
	if strings.TrimSpace(f.excludeAllergen) != "" {
		for _, a := range strings.Split(f.excludeAllergen, ",") {
			if containsFold(m.Allergens, strings.TrimSpace(a)) {
				return false
			}
		}
	}
	return true
}

func applyMealFilter(meals []types.Meal, f mealFilter) []types.Meal {
	out := make([]types.Meal, 0, len(meals))
	for _, m := range meals {
		if f.keep(m) {
			out = append(out, m)
		}
	}
	return out
}

// sortMeals sorts meals by a named key.
func sortMeals(meals []types.Meal, key string) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "calories":
		sort.SliceStable(meals, func(i, j int) bool { return meals[i].Calories < meals[j].Calories })
	case "protein":
		sort.SliceStable(meals, func(i, j int) bool { return meals[i].Protein > meals[j].Protein })
	case "price":
		sort.SliceStable(meals, func(i, j int) bool { return meals[i].FinalPrice < meals[j].FinalPrice })
	case "rating", "stars":
		sort.SliceStable(meals, func(i, j int) bool { return meals[i].Stars > meals[j].Stars })
	case "name":
		sort.SliceStable(meals, func(i, j int) bool { return meals[i].Name < meals[j].Name })
	}
}

// emitMeals writes a meal slice honoring output flags (--json/--select/--compact/--csv).
func emitMeals(w io.Writer, meals []types.Meal, flags *rootFlags) error {
	data, err := json.Marshal(meals)
	if err != nil {
		return err
	}
	return printOutputWithFlags(w, json.RawMessage(data), flags)
}
