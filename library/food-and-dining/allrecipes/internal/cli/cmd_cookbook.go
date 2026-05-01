// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written: cookbook + grocery-list — multi-recipe transcendence commands.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"

	"github.com/spf13/cobra"
)

func newCookbookCmd(flags *rootFlags) *cobra.Command {
	var flagCategory, flagCuisine, flagOutput, flagTitle string
	var flagTop int
	cmd := &cobra.Command{
		Use:   "cookbook",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Compile top-rated cached recipes into a markdown cookbook",
		Long: "Reads the local cache, picks the top-N recipes (by Bayesian-smoothed rating)\n" +
			"in a category or cuisine, and writes a single markdown file with TOC,\n" +
			"ingredients, and instructions for each.\n\n" +
			"Recipes must be in the local cache. Pre-fetch with `recipe` or `search`+\n" +
			"`top-rated` first.",
		Example: "  allrecipes-pp-cli cookbook --category dessert --top 10 --output dessert.md\n" +
			"  allrecipes-pp-cli cookbook --cuisine italian --top 20 --title \"Italian Top 20\" --output italian.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if flagCategory == "" && flagCuisine == "" {
				return usageErr(fmt.Errorf("either --category or --cuisine is required"))
			}
			s := openStoreForCommand()
			if s == nil {
				return fmt.Errorf("no local cache available — run 'recipe' or 'search'+'top-rated' first")
			}
			defer s.Close()
			rows, err := recipes.QueryIndex(s, recipes.IndexQuery{
				Category:  flagCategory,
				Cuisine:   flagCuisine,
				Limit:     200,
				OrderBy:   "rating",
				OrderDesc: true,
			})
			if err != nil {
				return err
			}
			sortByBayes(rows)
			if flagTop <= 0 {
				flagTop = 20
			}
			if len(rows) > flagTop {
				rows = rows[:flagTop]
			}
			if len(rows) == 0 {
				return fmt.Errorf("no cached recipes match category=%q cuisine=%q — pre-fetch some first", flagCategory, flagCuisine)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			title := flagTitle
			if title == "" {
				switch {
				case flagCategory != "" && flagCuisine != "":
					title = fmt.Sprintf("%s %s", strings.Title(flagCuisine), strings.Title(flagCategory))
				case flagCategory != "":
					title = fmt.Sprintf("%s Cookbook", strings.Title(flagCategory))
				case flagCuisine != "":
					title = fmt.Sprintf("%s Cookbook", strings.Title(flagCuisine))
				}
			}

			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n\n", title)
			fmt.Fprintf(&b, "*Top %d recipes from Allrecipes, ranked by Bayesian-smoothed rating.*\n\n", len(rows))
			b.WriteString("## Contents\n\n")
			for i, r := range rows {
				fmt.Fprintf(&b, "%d. [%s](#recipe-%d) — %.1f (%d ratings)\n", i+1, r.Name, i+1, r.Rating, r.ReviewCount)
			}
			b.WriteString("\n---\n\n")

			for i, r := range rows {
				rec, err := recipes.FetchRecipe(c, r.URL)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "skipping %s: %v\n", r.Name, err)
					continue
				}
				persistRecipe(rec)
				fmt.Fprintf(&b, "<a id=\"recipe-%d\"></a>\n\n", i+1)
				b.WriteString(recipeMarkdown(rec))
				b.WriteString("\n---\n\n")
			}

			out := b.String()
			if flagOutput != "" {
				if err := os.WriteFile(flagOutput, []byte(out), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", flagOutput, err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d bytes to %s (%d recipes)\n", len(out), flagOutput, len(rows))
				return nil
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	}
	cmd.Flags().StringVar(&flagCategory, "category", "", "Filter cached recipes by category substring (e.g. dessert)")
	cmd.Flags().StringVar(&flagCuisine, "cuisine", "", "Filter cached recipes by cuisine substring (e.g. italian)")
	cmd.Flags().IntVar(&flagTop, "top", 20, "Number of top recipes to include")
	cmd.Flags().StringVar(&flagTitle, "title", "", "Cookbook title (default derived from category/cuisine)")
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "File to write cookbook to (default: stdout)")
	return cmd
}

func newGroceryListCmd(flags *rootFlags) *cobra.Command {
	var flagOutput string
	cmd := &cobra.Command{
		Use:   "grocery-list <url> [<url>...]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Aggregate ingredients from many recipes into a deduped shopping list",
		Long: "Fetches each recipe URL (from cache when available, live otherwise),\n" +
			"parses ingredient lines into qty+unit+name, sums quantities for matching\n" +
			"items, and emits a deduped shopping list. Items with mismatched units\n" +
			"stay separate (we don't lie about converting cups to grams).",
		Example: "  allrecipes-pp-cli grocery-list https://www.allrecipes.com/recipe/9599/quick-and-easy-brownies/ https://www.allrecipes.com/recipe/16354/easy-meatloaf/\n" +
			"  allrecipes-pp-cli grocery-list 9599 16354 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			perRecipe := [][]recipes.ParsedIngredient{}
			titles := []string{}
			for _, raw := range args {
				url := recipes.ResolveRecipeURL(raw)
				rec, err := recipes.FetchRecipe(c, url)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: %v\n", raw, err)
					continue
				}
				persistRecipe(rec)
				perRecipe = append(perRecipe, recipes.ParseIngredients(rec.RecipeIngredient))
				titles = append(titles, rec.Name)
			}
			if len(perRecipe) == 0 {
				return fmt.Errorf("no recipes could be fetched")
			}
			agg := recipes.AggregateGrocery(perRecipe)

			out := map[string]any{
				"recipes":     titles,
				"recipeCount": len(titles),
				"ingredients": agg,
			}

			if flagOutput == "markdown" {
				var b strings.Builder
				b.WriteString("# Grocery List\n\n")
				fmt.Fprintf(&b, "*From %d recipe%s:*\n\n", len(titles), pluralS(len(titles)))
				for _, t := range titles {
					fmt.Fprintf(&b, "- %s\n", t)
				}
				b.WriteString("\n## Ingredients\n\n")
				for _, ing := range agg {
					fmt.Fprintf(&b, "- %s\n", ing.Raw)
				}
				_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
				return err
			}
			data, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagOutput, "output", "", "Output format: 'markdown' for a human-readable list (default: JSON)")
	return cmd
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
