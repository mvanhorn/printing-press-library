// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newRecipesGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <slug-or-url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Get full structured details for a single Food52 recipe by slug or URL",
		Example: strings.Trim(`
  food52-pp-cli recipes get sarah-fennel-s-best-lunch-lady-brownie-recipe
  food52-pp-cli recipes get https://food52.com/recipes/mom-s-japanese-curry-chicken-with-radish-and-cauliflower --json
`, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := recipeSlugFromArg(args[0])
			if slug == "" {
				return fmt.Errorf("recipe slug or URL is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dataSource == "local" {
				db, err := openStoreOrErr()
				if err != nil {
					return fmt.Errorf("opening local store: %w", err)
				}
				defer db.Close()
				row := db.DB().QueryRow("SELECT data FROM recipes WHERE slug = ? LIMIT 1", slug)
				var data []byte
				if err := row.Scan(&data); err != nil {
					return fmt.Errorf("recipe %q not in local store. Run: food52-pp-cli sync recipes <tag>", slug)
				}
				var r food52.Recipe
				if err := json.Unmarshal(data, &r); err != nil {
					return fmt.Errorf("decoding local recipe row: %w", err)
				}
				return emitFromFlags(flags, r, func() { renderRecipeText(&r) })
			}

			path := "/recipes/" + slug
			html, err := fetchHTML(c, path, nil)
			if err != nil {
				return classifyAPIError(err)
			}
			if c.DryRun {
				return emitJSON(map[string]any{"slug": slug, "dry_run": true, "url": canonicalRecipeURL(slug)})
			}
			r, err := food52.ExtractRecipe(html, canonicalRecipeURL(slug))
			if err != nil {
				return fmt.Errorf("food52 recipes get %s: %w", slug, err)
			}
			return emitFromFlags(flags, r, func() { renderRecipeText(r) })
		},
	}
	return cmd
}

func renderRecipeText(r *food52.Recipe) {
	if r == nil {
		return
	}
	approved := ""
	if r.TestKitchenApproved {
		approved = " ★ Test-Kitchen approved"
	}
	fmt.Printf("%s\n%s\n", r.Title, strings.Repeat("=", min(len(r.Title), 78)))
	if r.AuthorName != "" {
		fmt.Printf("by %s%s\n", r.AuthorName, approved)
	} else if approved != "" {
		fmt.Println(strings.TrimSpace(approved))
	}
	fmt.Println(r.URL)
	if r.AverageRating > 0 {
		fmt.Printf("Rating: %.2f (%d reviews)\n", r.AverageRating, r.RatingCount)
	}
	if r.Yield != "" {
		fmt.Printf("Yield: %s\n", r.Yield)
	}
	if r.PrepTime != "" || r.CookTime != "" || r.TotalTime != "" {
		parts := []string{}
		if r.PrepTime != "" {
			parts = append(parts, "prep "+r.PrepTime)
		}
		if r.CookTime != "" {
			parts = append(parts, "cook "+r.CookTime)
		}
		if r.TotalTime != "" {
			parts = append(parts, "total "+r.TotalTime)
		}
		fmt.Println("Time: " + strings.Join(parts, " · "))
	}
	if r.Description != "" {
		fmt.Println()
		fmt.Println(r.Description)
	}
	fmt.Println()
	fmt.Println("Ingredients")
	fmt.Println("-----------")
	for _, ing := range r.Ingredients {
		fmt.Printf("- %s\n", ing)
	}
	fmt.Println()
	fmt.Println("Steps")
	fmt.Println("-----")
	for i, step := range r.Instructions {
		fmt.Printf("%d. %s\n", i+1, step)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
