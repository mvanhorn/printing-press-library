// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written: pantry, with-ingredient, dietary — local-cache transcendence
// commands that exploit the recipe_index + recipe_ingredients_fts tables.

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/store"

	"github.com/spf13/cobra"
)

func newPantryCmd(flags *rootFlags) *cobra.Command {
	var flagPantryFile, flagPantryArg, flagQuery string
	var flagMinOverlap float64
	var flagMaxMissing, flagLimit int
	cmd := &cobra.Command{
		Use:   "pantry",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Score cached recipes by overlap with a pantry of ingredients",
		Long: `Reads --pantry-file (or --pantry as a comma-separated list of ingredients)
and ranks cached recipes by how many of their ingredients you already have.
Recipes only appear if their fetched JSON-LD has been cached locally; run
` + "`allrecipes-pp-cli search ...` and `allrecipes-pp-cli recipe ...`" + ` first
to populate the cache.

Overlap is computed by token-level matching: a recipe ingredient like
"boneless skinless chicken thighs" matches a pantry token "chicken". The
score is len(have) / len(have+missing); --min-overlap filters by it.`,
		Example: "  allrecipes-pp-cli pantry --pantry-file ~/pantry.txt --query brownies\n" +
			"  allrecipes-pp-cli pantry --pantry chicken,lemon,rice --max-missing 2 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			pantry, err := loadPantry(flagPantryFile, flagPantryArg)
			if err != nil {
				return err
			}
			if len(pantry) == 0 {
				return usageErr(fmt.Errorf("pantry is empty — use --pantry-file <path> or --pantry chicken,rice,lemon"))
			}
			s := openStoreForCommand()
			if s == nil {
				return fmt.Errorf("no local cache available — run 'recipe' or 'search' first to populate it")
			}
			defer s.Close()
			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}
			matches, err := recipes.PantryQuery(s, pantry, flagMinOverlap, flagQuery, limit*2)
			if err != nil {
				return err
			}
			if flagMaxMissing > 0 {
				filtered := matches[:0]
				for _, m := range matches {
					if len(m.Missing) <= flagMaxMissing {
						filtered = append(filtered, m)
					}
				}
				matches = filtered
			}
			if len(matches) > limit {
				matches = matches[:limit]
			}
			return renderJSON(cmd, flags, matches)
		},
	}
	cmd.Flags().StringVar(&flagPantryFile, "pantry-file", "", "Path to a pantry file (one ingredient per line; '#' starts a comment)")
	cmd.Flags().StringVar(&flagPantryArg, "pantry", "", "Comma-separated pantry ingredients (alternative to --pantry-file)")
	cmd.Flags().StringVar(&flagQuery, "query", "", "Filter cached recipes whose name contains this substring")
	cmd.Flags().Float64Var(&flagMinOverlap, "min-overlap", 0.5, "Minimum proportion of ingredients you must already have (0.0-1.0)")
	cmd.Flags().IntVar(&flagMaxMissing, "max-missing", 0, "Discard recipes that need more than N extra ingredients (0 = no limit)")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum results")
	return cmd
}

func loadPantry(file, csv string) ([]string, error) {
	out := []string{}
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("open pantry file: %w", err)
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			out = append(out, line)
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	if csv != "" {
		for _, p := range strings.Split(csv, ",") {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

func newWithIngredientCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagMaxMinutes int
	var flagMinRating float64
	cmd := &cobra.Command{
		Use:   "with-ingredient <name>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Reverse index: cached recipes that use a given ingredient",
		Long: `Searches the local recipe_ingredients_fts index for recipes containing the
given ingredient name. Recipes only appear if their JSON-LD has been
cached locally — run search/recipe to populate first.`,
		Example: "  allrecipes-pp-cli with-ingredient buttermilk --top 10\n" +
			"  allrecipes-pp-cli with-ingredient \"chicken thighs\" --max-minutes 45 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			ing := strings.Join(args, " ")
			s := openStoreForCommand()
			if s == nil {
				return fmt.Errorf("no local cache available — run 'recipe' or 'search' first to populate it")
			}
			defer s.Close()
			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}
			rows, err := recipes.QueryIndex(s, recipes.IndexQuery{
				IngredientToken: ftsToken(ing),
				MaxTime:         flagMaxMinutes * 60,
				MinRating:       flagMinRating,
				Limit:           limit * 3,
				OrderBy:         "rating",
				OrderDesc:       true,
			})
			if err != nil {
				return err
			}
			sortByBayes(rows)
			if len(rows) > limit {
				rows = rows[:limit]
			}
			return renderJSON(cmd, flags, rows)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "top", 25, "Maximum results")
	cmd.Flags().IntVar(&flagMaxMinutes, "max-minutes", 0, "Optional time cap in minutes")
	cmd.Flags().Float64Var(&flagMinRating, "min-rating", 0, "Optional minimum raw rating")
	return cmd
}

// ftsToken quotes a free-text query for FTS5 to avoid syntax errors on
// punctuation. We wrap in double-quotes and escape internal quotes.
func ftsToken(s string) string {
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}

func newDietaryCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagLimit int
	var flagMaxMinutes int
	cmd := &cobra.Command{
		Use:   "dietary",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Filter cached recipes by gluten-free / vegan / low-carb / vegetarian / dairy-free",
		Long: `Filters the local recipe cache by dietary heuristics:
  - gluten-free: excludes wheat, flour, bread, pasta, soy sauce, beer
  - vegan:       excludes meat, fish, dairy, eggs, honey
  - vegetarian:  excludes meat, fish
  - low-carb:    excludes pasta, rice, bread, sugar, flour
  - dairy-free:  excludes milk, butter, cheese, cream, yogurt
  - keto:        excludes pasta, rice, bread, sugar, flour, fruit (a stricter low-carb)

Heuristics are not authoritative — for medical-grade dietary control,
read the full ingredient list. The dietary tag is a starting point.`,
		Example: "  allrecipes-pp-cli dietary --type gluten-free --top 20 --agent\n" +
			"  allrecipes-pp-cli dietary --type vegan --max-minutes 30 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			banned := bannedIngredientsFor(flagType)
			if len(banned) == 0 {
				return usageErr(fmt.Errorf("--type must be one of: gluten-free, vegan, vegetarian, low-carb, dairy-free, keto"))
			}
			s := openStoreForCommand()
			if s == nil {
				return fmt.Errorf("no local cache available — run 'recipe' or 'search' first to populate it")
			}
			defer s.Close()
			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}
			rows, err := recipes.QueryIndex(s, recipes.IndexQuery{
				MaxTime:   flagMaxMinutes * 60,
				Limit:     500,
				OrderBy:   "rating",
				OrderDesc: true,
			})
			if err != nil {
				return err
			}
			// Filter by checking if any ingredient name contains a banned token.
			out := []recipes.IndexRow{}
			for _, r := range rows {
				if recipeMatchesBanned(s, r.URL, banned) {
					continue
				}
				out = append(out, r)
				if len(out) >= limit {
					break
				}
			}
			sortByBayes(out)
			return renderJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Dietary filter (gluten-free|vegan|vegetarian|low-carb|dairy-free|keto) — required")
	cmd.Flags().IntVar(&flagLimit, "top", 25, "Maximum results")
	cmd.Flags().IntVar(&flagMaxMinutes, "max-minutes", 0, "Optional time cap in minutes (0 = no cap)")
	return cmd
}

// bannedIngredientsFor returns lowercased ingredient name tokens that disqualify
// a recipe for a given dietary type. The list is heuristic — strict adherence
// requires reading the full ingredient list and any "natural flavors" caveat.
func bannedIngredientsFor(diet string) []string {
	switch strings.ToLower(strings.TrimSpace(diet)) {
	case "gluten-free", "gf":
		return []string{"wheat", "flour", "bread", "pasta", "noodle", "soy sauce", "beer", "barley", "rye", "couscous", "panko", "breadcrumb"}
	case "vegan":
		return []string{"chicken", "beef", "pork", "lamb", "fish", "salmon", "tuna", "shrimp", "bacon", "ham", "egg", "milk", "butter", "cheese", "cream", "yogurt", "honey", "gelatin"}
	case "vegetarian":
		return []string{"chicken", "beef", "pork", "lamb", "fish", "salmon", "tuna", "shrimp", "bacon", "ham", "anchov", "gelatin"}
	case "low-carb":
		return []string{"pasta", "rice", "bread", "sugar", "flour", "potato", "noodle", "tortilla", "corn syrup"}
	case "dairy-free":
		return []string{"milk", "butter", "cheese", "cream", "yogurt", "ghee", "buttermilk"}
	case "keto":
		return []string{"pasta", "rice", "bread", "sugar", "flour", "potato", "noodle", "tortilla", "corn syrup", "honey", "fruit juice", "banana", "apple"}
	}
	return nil
}

func recipeMatchesBanned(s *store.Store, url string, banned []string) bool {
	// Lookup ingredients for this URL via the store.
	rows, err := s.DB().Query(`SELECT name FROM recipe_ingredients WHERE recipe_url = ?`, url)
	if err != nil {
		return true // safe default: don't include if we can't verify
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		lc := strings.ToLower(name)
		for _, b := range banned {
			if strings.Contains(lc, b) {
				return true
			}
		}
	}
	return false
}
