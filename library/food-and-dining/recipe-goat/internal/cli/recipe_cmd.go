package cli

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/recipe-goat/internal/recipes"

	"github.com/spf13/cobra"
)

func newRecipeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "recipe",
		Short:   "Fetch, open, and inspect recipes from curated sites",
		Long:    "Subcommands to fetch a recipe by URL, open a saved recipe in the browser, or inspect reviews/cost.",
		Example: "  recipe-goat-pp-cli recipe get https://www.seriouseats.com/the-best-chili-recipe",
	}
	cmd.AddCommand(newRecipeGetCmd(flags))
	cmd.AddCommand(newRecipeOpenCmd(flags))
	cmd.AddCommand(newRecipeReviewsCmd(flags))
	cmd.AddCommand(newRecipeCostCmd(flags))
	return cmd
}

func newRecipeGetCmd(flags *rootFlags) *cobra.Command {
	var (
		servings   int
		units      string
		asMarkdown bool
		printMode  bool
		nutrition  bool
		reviews    bool
	)
	cmd := &cobra.Command{
		Use:         "get <url>",
		Short:       "Fetch and render a recipe from its source URL with optional scaling, unit conversion, and USDA nutrition backfill",
		Example:     "  recipe-goat-pp-cli recipe get https://www.budgetbytes.com/creamy-mushroom-pasta/ --servings 6 --print",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			if _, err := url.ParseRequestURI(target); err != nil {
				return usageErr(fmt.Errorf("invalid URL: %s", target))
			}
			ctx, cancel := flags.withContext()
			defer cancel()
			client := httpClientForSites(flags.timeout)
			r, err := recipes.Fetch(ctx, client, target)
			if err != nil {
				return apiErr(fmt.Errorf("fetch: %w", err))
			}
			// Scale.
			if servings > 0 {
				from := recipes.ParseYield(r.RecipeYield)
				if from > 0 {
					r.RecipeIngredient = recipes.ScaleIngredients(r.RecipeIngredient, from, servings)
					r.RecipeYield = fmt.Sprintf("%d", servings)
				}
			}
			// Unit conversion (after scaling so the numbers we convert are
			// the final ones the user sees).
			unitsNorm := strings.ToLower(strings.TrimSpace(units))
			if unitsNorm == "metric" || unitsNorm == "us" {
				r.RecipeIngredient = recipes.ConvertIngredients(r.RecipeIngredient, unitsNorm)
				r.RecipeInstructions = recipes.ConvertInstructionsTemps(r.RecipeInstructions, unitsNorm)
			} else if unitsNorm != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: --units value %q not recognized; use 'metric' or 'us'\n", units)
			}
			// Nutrition backfill.
			nutritionSource := ""
			if nutrition {
				nut, src, _ := recipes.BackfillNutrition(ctx, client, r)
				nutritionSource = src
				if nut != nil {
					r.Nutrition = nut
				}
			}
			if reviews {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: review digest is planned (not yet wired)")
			}
			if flags.asJSON {
				return flags.printJSON(cmd, r)
			}
			if printMode {
				// Print-friendly: no ANSI, no colors (respect --no-color already).
				printRecipeCard(cmd.OutOrStdout(), r, false)
			} else {
				printRecipeCard(cmd.OutOrStdout(), r, asMarkdown)
			}
			if nutrition && nutritionSource != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n[nutrition source: %s]\n", nutritionSource)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&servings, "servings", 0, "Scale ingredients to this many servings")
	cmd.Flags().StringVar(&units, "units", "", "Output units: metric|us (best-effort conversion for common cooking units)")
	cmd.Flags().BoolVar(&asMarkdown, "md", false, "Render as Markdown")
	cmd.Flags().BoolVar(&printMode, "print", false, "Print-friendly plain output")
	cmd.Flags().BoolVar(&nutrition, "nutrition", false, "Backfill macros when the source omits them — requires free USDA_FDC_API_KEY (get one at https://fdc.nal.usda.gov/api-key-signup)")
	cmd.Flags().BoolVar(&reviews, "reviews", false, "Include review digest (wip)")
	return cmd
}

func newRecipeOpenCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "open <id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Open a saved recipe in the default browser",
		Example: "  recipe-goat-pp-cli recipe open 12",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("recipe id must be an integer: %s", args[0]))
			}
			s, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer s.Close()
			r, err := s.GetRecipeByID(id)
			if err != nil {
				return err
			}
			if r == nil {
				return notFoundErr(fmt.Errorf("recipe id %d not found", id))
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would open: %s\n", r.URL)
				return nil
			}
			var c *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				c = exec.Command("open", r.URL)
			case "windows":
				c = exec.Command("rundll32", "url.dll,FileProtocolHandler", r.URL)
			default:
				c = exec.Command("xdg-open", r.URL)
			}
			if err := c.Start(); err != nil {
				return fmt.Errorf("open: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "opened: %s\n", r.URL)
			return nil
		},
	}
}

func newRecipeReviewsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "reviews <id>",
		Short:       "Show aggregated review-derived modifications for a saved recipe",
		Example:     "  recipe-goat-pp-cli recipe reviews 12",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("recipe id must be an integer"))
			}
			s, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer s.Close()
			r, err := s.GetRecipeByID(id)
			if err != nil {
				return err
			}
			if r == nil {
				return notFoundErr(fmt.Errorf("recipe id %d not found", id))
			}
			payload := map[string]any{
				"recipe_id": r.ID,
				"title":     r.Title,
				"url":       r.URL,
				"status":    "review digest not yet wired in v1; planned: aggregated modifications from source reviews",
				"hint":      "run 'recipe-goat-pp-cli recipe reviews --help' for more details",
			}
			if flags.asJSON {
				return flags.printJSON(cmd, payload)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Review digest for recipe #%d (%s)\n", r.ID, r.Title)
			fmt.Fprintln(cmd.OutOrStdout(), "Source:", r.URL)
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), "review digest not yet wired in v1.")
			fmt.Fprintln(cmd.OutOrStdout(), "planned: aggregated modifications from source reviews (\"added an egg\", \"halved the sugar\", etc.)")
			return nil
		},
	}
}

func newRecipeCostCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "cost <id>",
		Short:       "Estimate per-serving cost for a saved recipe using ingredient count and serving size",
		Example:     "  recipe-goat-pp-cli recipe cost 12",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("recipe id must be an integer"))
			}
			s, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer s.Close()
			r, err := s.GetRecipeByID(id)
			if err != nil {
				return err
			}
			if r == nil {
				return notFoundErr(fmt.Errorf("recipe id %d not found", id))
			}
			ingredientCount := len(r.Ingredients)
			servings := r.Servings
			if servings <= 0 {
				servings = 4
			}
			estimated := 0.75 * float64(ingredientCount) / float64(servings)
			low := estimated * 0.6
			high := estimated * 1.4
			payload := map[string]any{
				"recipe_id":         r.ID,
				"title":             r.Title,
				"ingredient_count":  ingredientCount,
				"servings":          servings,
				"estimated_per_svg": estimated,
				"range_low":         low,
				"range_high":        high,
				"method":            "placeholder: 0.75 * ingredients / servings; ±40%",
				"note":              "accurate cost requires ingredient-price data (Budget Bytes + USDA retail averages); integration wip",
			}
			if flags.asJSON {
				return flags.printJSON(cmd, payload)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cost estimate for recipe #%d (%s)\n", r.ID, r.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "Ingredients: %d  Servings: %d\n", ingredientCount, servings)
			fmt.Fprintf(cmd.OutOrStdout(), "Estimated: $%.2f per serving (range $%.2f–$%.2f, ±40%%)\n", estimated, low, high)
			fmt.Fprintln(cmd.OutOrStdout(), "Method: placeholder heuristic — accurate cost requires ingredient-price data (wip).")
			return nil
		},
	}
}
