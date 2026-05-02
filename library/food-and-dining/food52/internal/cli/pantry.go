// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newPantryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pantry",
		Short: "Manage a local pantry inventory and match it against synced Food52 recipes",
		Long: strings.TrimSpace(`
Track what's in your kitchen. Then run 'pantry match' to find synced Food52
recipes whose ingredients overlap your pantry, ranked by coverage.

The pantry lives in the same SQLite store as synced recipes
(~/Library/Application Support/food52-pp-cli/store.db on macOS). Use
'pantry add', 'pantry list', 'pantry remove' to manage it.

Pantry match operates against recipes in the local store, so run
'sync recipes <tag>' first.
`),
	}
	cmd.AddCommand(newPantryAddCmd(flags))
	cmd.AddCommand(newPantryListCmd(flags))
	cmd.AddCommand(newPantryRemoveCmd(flags))
	cmd.AddCommand(newPantryMatchCmd(flags))
	return cmd
}

func newPantryAddCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <ingredient> [<ingredient>...]",
		Short: "Add one or more ingredients to your local pantry",
		Example: strings.Trim(`
  food52-pp-cli pantry add chicken garlic lemon thyme
  food52-pp-cli pantry add "olive oil" "soy sauce"
`, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreOrErr()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensurePantryTable(db.DB()); err != nil {
				return fmt.Errorf("creating pantry table: %w", err)
			}
			added := 0
			for _, raw := range args {
				ing := normalizeIngredientName(raw)
				if ing == "" {
					continue
				}
				_, err := db.DB().Exec("INSERT OR IGNORE INTO pantry (ingredient) VALUES (?)", ing)
				if err != nil {
					return fmt.Errorf("insert: %w", err)
				}
				added++
			}
			payload := map[string]any{"added": added}
			return emitFromFlags(flags, payload, func() {
				fmt.Printf("Added %d ingredient(s) to pantry.\n", added)
			})
		},
	}
	return cmd
}

func newPantryListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Show all ingredients currently in your local pantry",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  food52-pp-cli pantry list
  food52-pp-cli pantry list --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreOrErr()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensurePantryTable(db.DB()); err != nil {
				return err
			}
			rows, err := db.DB().Query("SELECT ingredient, added_at FROM pantry ORDER BY ingredient")
			if err != nil {
				return err
			}
			defer rows.Close()
			type item struct {
				Ingredient string `json:"ingredient"`
				AddedAt    string `json:"added_at"`
			}
			items := []item{}
			for rows.Next() {
				var it item
				if err := rows.Scan(&it.Ingredient, &it.AddedAt); err != nil {
					return err
				}
				items = append(items, it)
			}
			payload := map[string]any{"count": len(items), "pantry": items}
			return emitFromFlags(flags, payload, func() {
				if len(items) == 0 {
					fmt.Println("Pantry is empty. Add ingredients with: food52-pp-cli pantry add <name>")
					return
				}
				fmt.Printf("%d ingredient(s) in pantry\n", len(items))
				for _, it := range items {
					fmt.Printf("  %s\n", it.Ingredient)
				}
			})
		},
	}
	return cmd
}

func newPantryRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <ingredient> [<ingredient>...]",
		Short: "Remove ingredients from your local pantry",
		Example: strings.Trim(`
  food52-pp-cli pantry remove chicken
  food52-pp-cli pantry remove "olive oil" "soy sauce"
`, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreOrErr()
			if err != nil {
				return err
			}
			defer db.Close()
			removed := 0
			for _, raw := range args {
				ing := normalizeIngredientName(raw)
				if ing == "" {
					continue
				}
				res, err := db.DB().Exec("DELETE FROM pantry WHERE ingredient = ?", ing)
				if err != nil {
					return err
				}
				n, _ := res.RowsAffected()
				removed += int(n)
			}
			payload := map[string]any{"removed": removed}
			return emitFromFlags(flags, payload, func() {
				fmt.Printf("Removed %d ingredient(s) from pantry.\n", removed)
			})
		},
	}
	return cmd
}

func newPantryMatchCmd(flags *rootFlags) *cobra.Command {
	var (
		minCoverage float64
		limit       int
	)
	cmd := &cobra.Command{
		Use:         "match",
		Short:       "Find synced Food52 recipes you can mostly make with what's in your pantry",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: strings.TrimSpace(`
Joins your local pantry against every synced recipe and ranks recipes by
coverage (fraction of recipe ingredients you have). The matcher is
case-insensitive and looks for the pantry ingredient as a substring of the
recipe ingredient line, so "garlic" matches both "2 cloves garlic" and
"3 tablespoons garlic powder".

Pair --min-coverage to require at least N fraction of ingredients available
(0.6 = at least 60% match). Default is 0.5.

Run 'sync recipes <tag>' first — pantry match operates on the local store,
not the live site.
`),
		Example: strings.Trim(`
  food52-pp-cli pantry match --min-coverage 0.6 --limit 10 --json
  food52-pp-cli pantry match
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreOrErr()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensurePantryTable(db.DB()); err != nil {
				return err
			}

			pantry, err := loadPantry(db.DB())
			if err != nil {
				return err
			}
			if len(pantry) == 0 {
				return fmt.Errorf("pantry is empty — run: food52-pp-cli pantry add <ingredient>...")
			}

			recipes, err := loadAllStoredRecipes(db.DB())
			if err != nil {
				return err
			}
			if len(recipes) == 0 {
				return fmt.Errorf("no synced recipes — run: food52-pp-cli sync recipes <tag>...")
			}

			matches := matchPantry(pantry, recipes, minCoverage)
			sort.SliceStable(matches, func(i, j int) bool { return matches[i].Coverage > matches[j].Coverage })
			if limit > 0 && len(matches) > limit {
				matches = matches[:limit]
			}
			payload := map[string]any{
				"pantry":        pantry,
				"recipe_count":  len(recipes),
				"min_coverage":  minCoverage,
				"matches_count": len(matches),
				"matches":       matches,
			}
			return emitFromFlags(flags, payload, func() {
				if len(matches) == 0 {
					fmt.Printf("No recipes meet coverage >= %.2f from pantry of %d ingredients across %d synced recipes.\n", minCoverage, len(pantry), len(recipes))
					return
				}
				fmt.Printf("Top %d matches from pantry of %d ingredients (across %d synced recipes)\n", len(matches), len(pantry), len(recipes))
				for i, m := range matches {
					fmt.Printf("%2d. %s — %.0f%% (%d/%d)\n    %s\n",
						i+1, m.Title, m.Coverage*100, m.MatchedCount, m.TotalIngredients, m.URL)
					if len(m.MissingIngredients) > 0 && len(m.MissingIngredients) <= 6 {
						fmt.Printf("    missing: %s\n", strings.Join(m.MissingIngredients, ", "))
					}
				}
			})
		},
	}
	cmd.Flags().Float64Var(&minCoverage, "min-coverage", 0.5, "Minimum fraction of ingredients required (0.0-1.0)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Cap the result set to N recipes")
	return cmd
}

// PantryMatch is the per-recipe match result emitted from `pantry match`.
type PantryMatch struct {
	Slug               string   `json:"slug"`
	Title              string   `json:"title"`
	URL                string   `json:"url"`
	TotalIngredients   int      `json:"total_ingredients"`
	MatchedCount       int      `json:"matched_count"`
	Coverage           float64  `json:"coverage"`
	MatchedIngredients []string `json:"matched_ingredients,omitempty"`
	MissingIngredients []string `json:"missing_ingredients,omitempty"`
}

func loadPantry(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT ingredient FROM pantry")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var i string
		if err := rows.Scan(&i); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func loadAllStoredRecipes(db *sql.DB) ([]*food52.Recipe, error) {
	rows, err := db.Query("SELECT data FROM recipes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*food52.Recipe{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var r food52.Recipe
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		// Skip rows that are summary-only (no ingredients).
		if len(r.Ingredients) > 0 {
			out = append(out, &r)
		}
	}
	return out, rows.Err()
}

func matchPantry(pantry []string, recipes []*food52.Recipe, minCoverage float64) []PantryMatch {
	pantryLower := make([]string, len(pantry))
	for i, p := range pantry {
		pantryLower[i] = strings.ToLower(p)
	}
	out := []PantryMatch{}
	for _, r := range recipes {
		matched := []string{}
		missing := []string{}
		for _, ing := range r.Ingredients {
			low := strings.ToLower(ing)
			hit := false
			for _, p := range pantryLower {
				if p == "" {
					continue
				}
				if strings.Contains(low, p) {
					hit = true
					break
				}
			}
			if hit {
				matched = append(matched, ing)
			} else {
				missing = append(missing, ing)
			}
		}
		total := len(r.Ingredients)
		if total == 0 {
			continue
		}
		cov := float64(len(matched)) / float64(total)
		if cov < minCoverage {
			continue
		}
		out = append(out, PantryMatch{
			Slug:               r.Slug,
			Title:              r.Title,
			URL:                r.URL,
			TotalIngredients:   total,
			MatchedCount:       len(matched),
			Coverage:           cov,
			MatchedIngredients: matched,
			MissingIngredients: missing,
		})
	}
	return out
}

// normalizeIngredientName lowercases and trims whitespace. We do not try to
// stem or strip qualifiers because pantry matching is substring-based and
// the recipe ingredient lines are messy by nature.
func normalizeIngredientName(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}
