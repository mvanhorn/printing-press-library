// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written: category, cuisine, ingredient, occasion — Allrecipes browse
// pages. All four follow the same pattern: build the path, fetch via Surf,
// parse the recipe-card grid.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"

	"github.com/spf13/cobra"
)

func newCategoryCmd(flags *rootFlags) *cobra.Command {
	return newBrowseCmd(flags, browseSpec{
		use:         "category <slug>",
		short:       "Browse recipes in a category (e.g. dessert, weeknight)",
		exampleSlug: "dessert",
		pathPrefix:  "/recipes/79/desserts/",
		buildPath: func(slug string) string {
			// Allrecipes category URLs vary in shape — many follow
			// /recipes/<id>/<slug>/, others /<slug>-recipes/. We accept
			// either a full path (starts with /) or a bare slug and try a
			// reasonable default.
			if strings.HasPrefix(slug, "/") {
				return slug
			}
			return "/recipes/" + slug + "/"
		},
	})
}

func newCuisineCmd(flags *rootFlags) *cobra.Command {
	return newBrowseCmd(flags, browseSpec{
		use:         "cuisine <slug>",
		short:       "Browse recipes by cuisine (e.g. italian, mexican, thai)",
		exampleSlug: "italian",
		buildPath: func(slug string) string {
			if strings.HasPrefix(slug, "/") {
				return slug
			}
			return "/recipes/cuisine/" + slug + "/"
		},
	})
}

func newIngredientBrowseCmd(flags *rootFlags) *cobra.Command {
	return newBrowseCmd(flags, browseSpec{
		use:         "ingredient <name>",
		short:       "Browse recipes featuring a primary ingredient (e.g. chicken, beef)",
		exampleSlug: "chicken",
		buildPath: func(slug string) string {
			if strings.HasPrefix(slug, "/") {
				return slug
			}
			return "/recipes/ingredient/" + slug + "/"
		},
	})
}

func newOccasionCmd(flags *rootFlags) *cobra.Command {
	return newBrowseCmd(flags, browseSpec{
		use:         "occasion <slug>",
		short:       "Browse recipes by occasion (holiday, weeknight, party, etc.)",
		exampleSlug: "weeknight",
		buildPath: func(slug string) string {
			if strings.HasPrefix(slug, "/") {
				return slug
			}
			return "/recipes/occasions/" + slug + "/"
		},
	})
}

type browseSpec struct {
	use         string
	short       string
	exampleSlug string
	pathPrefix  string
	buildPath   func(slug string) string
}

func newBrowseCmd(flags *rootFlags, spec browseSpec) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:     spec.use,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   spec.short,
		Example: fmt.Sprintf("  allrecipes-pp-cli %s %s --limit 20 --agent", strings.Fields(spec.use)[0], spec.exampleSlug),
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := strings.Join(args, "-")
			path := spec.buildPath(slug)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 24
			}
			results, err := recipes.FetchCategoryHTML(c, path, limit)
			if err != nil {
				return classifyAPIError(err)
			}
			data, err := json.Marshal(results)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 24, "Maximum results")
	return cmd
}
