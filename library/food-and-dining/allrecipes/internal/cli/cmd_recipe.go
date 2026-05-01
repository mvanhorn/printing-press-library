// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written: top-level recipe-centric commands. These wrap recipes.FetchRecipe
// with progressive output projections (full, ingredients-only, instructions-only,
// nutrition-only, reviews-only, scaled, markdown).

package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"

	"github.com/spf13/cobra"
)

func newRecipeTopCmd(flags *rootFlags) *cobra.Command {
	var flagMarkdown bool
	cmd := &cobra.Command{
		Use:   "recipe <url-or-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Fetch a single recipe by URL, ID, or id/slug shorthand",
		Example: "  allrecipes-pp-cli recipe https://www.allrecipes.com/recipe/9599/quick-and-easy-brownies/\n" +
			"  allrecipes-pp-cli recipe 9599/quick-and-easy-brownies --agent\n" +
			"  allrecipes-pp-cli recipe 9599/quick-and-easy-brownies --markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			r, err := fetchAndCache(flags, args[0])
			if err != nil {
				return err
			}
			if flagMarkdown {
				_, err := fmt.Fprint(cmd.OutOrStdout(), recipeMarkdown(r))
				return err
			}
			return renderJSON(cmd, flags, r)
		},
	}
	cmd.Flags().BoolVar(&flagMarkdown, "markdown", false, "Render as markdown instead of JSON")
	return cmd
}

func newIngredientsCmd(flags *rootFlags) *cobra.Command {
	var flagParsed bool
	cmd := &cobra.Command{
		Use:   "ingredients <url-or-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show parsed ingredients for a recipe",
		Example: "  allrecipes-pp-cli ingredients 9599/quick-and-easy-brownies\n" +
			"  allrecipes-pp-cli ingredients 9599/quick-and-easy-brownies --parsed --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			r, err := fetchAndCache(flags, args[0])
			if err != nil {
				return err
			}
			if flagParsed {
				return renderJSON(cmd, flags, recipes.ParseIngredients(r.RecipeIngredient))
			}
			if flags.asJSON {
				return renderJSON(cmd, flags, r.RecipeIngredient)
			}
			for _, ing := range r.RecipeIngredient {
				fmt.Fprintln(cmd.OutOrStdout(), ing)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagParsed, "parsed", false, "Return ingredients with quantity+unit+name structure")
	return cmd
}

func newInstructionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instructions <url-or-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show numbered instructions for a recipe",
		Example: "  allrecipes-pp-cli instructions 9599/quick-and-easy-brownies\n" +
			"  allrecipes-pp-cli instructions 9599/quick-and-easy-brownies --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			r, err := fetchAndCache(flags, args[0])
			if err != nil {
				return err
			}
			if flags.asJSON {
				return renderJSON(cmd, flags, r.RecipeInstructions)
			}
			for i, step := range r.RecipeInstructions {
				fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n", i+1, step)
			}
			return nil
		},
	}
	return cmd
}

func newNutritionCmd(flags *rootFlags) *cobra.Command {
	var flagServings int
	cmd := &cobra.Command{
		Use:   "nutrition <url-or-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show nutrition for a recipe (per serving by default)",
		Example: "  allrecipes-pp-cli nutrition 9599/quick-and-easy-brownies\n" +
			"  allrecipes-pp-cli nutrition 9599/quick-and-easy-brownies --servings 8 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			r, err := fetchAndCache(flags, args[0])
			if err != nil {
				return err
			}
			out := map[string]any{
				"url":       r.URL,
				"name":      r.Name,
				"yield":     r.RecipeYield,
				"nutrition": r.Nutrition,
			}
			if flagServings > 0 {
				orig := parseYieldServings(r.RecipeYield)
				if orig > 0 {
					factor := float64(flagServings) / float64(orig)
					out["scaledServings"] = flagServings
					out["scaleFactor"] = factor
				}
			}
			return renderJSON(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&flagServings, "servings", 0, "Scale nutrition for a target serving count (best-effort)")
	return cmd
}

func newReviewsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reviews <url-or-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show review summary for a recipe",
		Example: "  allrecipes-pp-cli reviews 9599/quick-and-easy-brownies\n" +
			"  allrecipes-pp-cli reviews 9599/quick-and-easy-brownies --agent",
		Long: `Returns the aggregate rating, review count, and "Made It" summary that
Allrecipes publishes via JSON-LD. Per-review text is not extractable from
the public Recipe schema; agents that need full review text should fetch
the recipe page and parse it themselves.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			r, err := fetchAndCache(flags, args[0])
			if err != nil {
				return err
			}
			out := map[string]any{
				"url":         r.URL,
				"name":        r.Name,
				"rating":      r.AggregateRating.Value,
				"reviewCount": r.AggregateRating.Count,
				"description": r.Description,
				"keywords":    r.Keywords,
			}
			return renderJSON(cmd, flags, out)
		},
	}
	return cmd
}

func newScaleCmd(flags *rootFlags) *cobra.Command {
	var flagServings int
	cmd := &cobra.Command{
		Use:   "scale <url-or-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Rescale a recipe's ingredients to a target serving count",
		Example: "  allrecipes-pp-cli scale 9599/quick-and-easy-brownies --servings 8\n" +
			"  allrecipes-pp-cli scale 9599/quick-and-easy-brownies --servings 16 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			if flagServings <= 0 {
				return usageErr(fmt.Errorf("--servings must be a positive integer"))
			}
			r, err := fetchAndCache(flags, args[0])
			if err != nil {
				return err
			}
			origServings := parseYieldServings(r.RecipeYield)
			if origServings == 0 {
				return fmt.Errorf("could not parse original yield %q — cannot scale", r.RecipeYield)
			}
			factor := float64(flagServings) / float64(origServings)
			parsed := recipes.ParseIngredients(r.RecipeIngredient)
			scaled := recipes.ScaleIngredients(parsed, factor)
			out := map[string]any{
				"url":              r.URL,
				"name":             r.Name,
				"originalYield":    r.RecipeYield,
				"originalServings": origServings,
				"targetServings":   flagServings,
				"factor":           factor,
				"ingredients":      scaled,
			}
			return renderJSON(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&flagServings, "servings", 0, "Target serving count (required)")
	return cmd
}

func newExportCmdAlias(flags *rootFlags) *cobra.Command {
	var flagOutput string
	cmd := &cobra.Command{
		Use:   "export-recipe <url-or-id>",
		Short: "Export a recipe as markdown to stdout or to a file",
		Long: `Writes a clean markdown rendering of a recipe — suitable for cooking-mode
reading without ads, story scrolls, or pop-ups. With --output, writes to a
file; otherwise prints to stdout.`,
		Example: "  allrecipes-pp-cli export-recipe 9599/quick-and-easy-brownies > brownies.md\n" +
			"  allrecipes-pp-cli export-recipe 9599/quick-and-easy-brownies --output brownies.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			r, err := fetchAndCache(flags, args[0])
			if err != nil {
				return err
			}
			md := recipeMarkdown(r)
			if flagOutput != "" {
				if err := os.WriteFile(flagOutput, []byte(md), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", flagOutput, err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d bytes to %s\n", len(md), flagOutput)
				return nil
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), md)
			return err
		},
	}
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "File to write markdown to (default: stdout)")
	return cmd
}

func parseYieldServings(yield string) int {
	yield = strings.TrimSpace(yield)
	if yield == "" {
		return 0
	}
	// "16" or "16 servings" or "1 (8x8) pan" — pull leading integer.
	i := 0
	for i < len(yield) && (yield[i] >= '0' && yield[i] <= '9') {
		i++
	}
	if i == 0 {
		return 0
	}
	n, _ := strconv.Atoi(yield[:i])
	return n
}
