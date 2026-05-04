// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written: shared helpers used by the top-level Allrecipes commands.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/store"

	"github.com/spf13/cobra"
)

// fetchAndCache fetches a recipe by URL/ID/shorthand, parses its JSON-LD, and
// writes the parsed result to the local cache. Returns the parsed Recipe.
func fetchAndCache(flags *rootFlags, urlOrID string) (*recipes.Recipe, error) {
	url := recipes.ResolveRecipeURL(urlOrID)
	if url == "" {
		return nil, fmt.Errorf("invalid recipe identifier: %q", urlOrID)
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	r, err := recipes.FetchRecipe(c, url)
	if err != nil {
		return nil, classifyAPIError(err)
	}
	persistRecipe(r)
	return r, nil
}

// renderJSON marshals v to JSON and routes through the standard output flags.
func renderJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

// openStoreForCommand opens the read+write store at the default path. Returns
// nil if the store cannot be opened — callers should treat that as
// "no cache available" and proceed live-only.
func openStoreForCommand() *store.Store {
	s, err := openStoreForRead(context.Background(), "allrecipes-pp-cli")
	if err != nil || s == nil {
		return nil
	}
	if err := recipes.EnsureSchema(s); err != nil {
		s.Close()
		return nil
	}
	return s
}

// persistRecipe is a best-effort cache write. Failures do not fail the command
// because the user already has the data they asked for; what we lose is the
// next call's offline path.
func persistRecipe(r *recipes.Recipe) {
	if r == nil {
		return
	}
	s, err := openStoreForRead(context.Background(), "allrecipes-pp-cli")
	if err != nil || s == nil {
		return
	}
	defer s.Close()
	_ = recipes.EnsureSchema(s)
	_ = recipes.SaveRecipe(s, r)
}

// recipeMarkdown renders a Recipe as a clean markdown document. Used by `export`
// and `cookbook`.
func recipeMarkdown(r *recipes.Recipe) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", r.Name)
	if r.Author != "" {
		fmt.Fprintf(&b, "*by %s*\n\n", r.Author)
	}
	if r.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", r.Description)
	}
	if r.AggregateRating.Count > 0 {
		fmt.Fprintf(&b, "**Rating:** %.1f (%d ratings)\n", r.AggregateRating.Value, r.AggregateRating.Count)
	}
	if r.TotalTime > 0 {
		fmt.Fprintf(&b, "**Total time:** %s", recipes.FormatTime(r.TotalTime))
		if r.PrepTime > 0 || r.CookTime > 0 {
			fmt.Fprintf(&b, " (prep %s, cook %s)", recipes.FormatTime(r.PrepTime), recipes.FormatTime(r.CookTime))
		}
		b.WriteString("\n")
	}
	if r.RecipeYield != "" {
		fmt.Fprintf(&b, "**Yield:** %s\n", r.RecipeYield)
	}
	if len(r.RecipeIngredient) > 0 {
		b.WriteString("\n## Ingredients\n\n")
		for _, ing := range r.RecipeIngredient {
			fmt.Fprintf(&b, "- %s\n", ing)
		}
	}
	if len(r.RecipeInstructions) > 0 {
		b.WriteString("\n## Instructions\n\n")
		for i, step := range r.RecipeInstructions {
			fmt.Fprintf(&b, "%d. %s\n", i+1, step)
		}
	}
	if len(r.Nutrition) > 0 {
		b.WriteString("\n## Nutrition\n\n")
		for k, v := range r.Nutrition {
			fmt.Fprintf(&b, "- **%s:** %s\n", k, v)
		}
	}
	if r.URL != "" {
		fmt.Fprintf(&b, "\n*Source: %s*\n", r.URL)
	}
	return b.String()
}
