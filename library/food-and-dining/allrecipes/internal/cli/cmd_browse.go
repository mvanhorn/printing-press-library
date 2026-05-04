// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written: category, cuisine, ingredient, occasion — Allrecipes browse
// pages. Each command keeps its `Use:` literal in the cobra.Command so
// static AST scanners (verify-skill) can find them.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"

	"github.com/spf13/cobra"
)

func newCategoryCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "category <slug>",
		Short:       "Browse recipes in a category (e.g. dessert, weeknight)",
		Example:     "  allrecipes-pp-cli category dessert --limit 20 --agent",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrowse(cmd, flags, args, flagLimit, func(slug string) string {
				if strings.HasPrefix(slug, "/") {
					return slug
				}
				return "/recipes/" + slug + "/"
			})
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 24, "Maximum results")
	return cmd
}

func newCuisineCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "cuisine <slug>",
		Short:       "Browse recipes by cuisine (e.g. italian, mexican, thai)",
		Example:     "  allrecipes-pp-cli cuisine italian --limit 20 --agent",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrowse(cmd, flags, args, flagLimit, func(slug string) string {
				if strings.HasPrefix(slug, "/") {
					return slug
				}
				return "/recipes/cuisine/" + slug + "/"
			})
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 24, "Maximum results")
	return cmd
}

func newIngredientBrowseCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "ingredient <name>",
		Short:       "Browse recipes featuring a primary ingredient (e.g. chicken, beef)",
		Example:     "  allrecipes-pp-cli ingredient chicken --limit 20 --agent",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrowse(cmd, flags, args, flagLimit, func(slug string) string {
				if strings.HasPrefix(slug, "/") {
					return slug
				}
				return "/recipes/ingredient/" + slug + "/"
			})
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 24, "Maximum results")
	return cmd
}

func newOccasionCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "occasion <slug>",
		Short:       "Browse recipes by occasion (holiday, weeknight, party, etc.)",
		Example:     "  allrecipes-pp-cli occasion weeknight --limit 20 --agent",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrowse(cmd, flags, args, flagLimit, func(slug string) string {
				if strings.HasPrefix(slug, "/") {
					return slug
				}
				return "/recipes/occasions/" + slug + "/"
			})
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 24, "Maximum results")
	return cmd
}

func runBrowse(cmd *cobra.Command, flags *rootFlags, args []string, flagLimit int, buildPath func(string) string) error {
	slug := strings.Join(args, "-")
	path := buildPath(slug)
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
}

// fmt is imported by other browse commands' Examples in case future variants
// need fmt.Sprintf.
var _ = fmt.Sprintf
