// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-edited after generation: replaces the generic HTML page-scrape with a
// JSON-LD parser that produces a structured Recipe.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"

	"github.com/spf13/cobra"
)

func newRecipesGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <recipe_id> <slug>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Fetch a recipe by ID + slug; returns parsed JSON-LD Recipe",
		Example: "  allrecipes-pp-cli recipes get 9599 quick-and-easy-brownies\n" +
			"  allrecipes-pp-cli recipes get 9599 quick-and-easy-brownies --agent --select recipeIngredient,totalTime,aggregateRating",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return usageErr(fmt.Errorf("recipe_id and slug are required\nUsage: %s recipes get <recipe_id> <slug>", cmd.Root().Name()))
			}
			recipeID, slug := args[0], args[1]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			url := recipes.CanonicalRecipeURL(recipeID, slug)
			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{"plan": "GET " + url})
			}
			r, err := recipes.FetchRecipe(c, url)
			if err != nil {
				return classifyAPIError(err)
			}
			persistRecipe(r)
			data, err := json.Marshal(r)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

// persistRecipe is a best-effort cache write. Failures do not fail the command
// because the user already has the data they asked for; what we lose is the
// next call's offline path.
func persistRecipe(r *recipes.Recipe) {
	if r == nil {
		return
	}
	s, err := openStoreForRead("allrecipes-pp-cli")
	if err != nil || s == nil {
		return
	}
	defer s.Close()
	_ = recipes.EnsureSchema(s)
	_ = recipes.SaveRecipe(s, r)
}
