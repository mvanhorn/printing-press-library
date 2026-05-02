// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newArticlesForRecipeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "for-recipe <slug-or-url>",
		Short:       "Find synced articles that mention a given recipe in their relatedReading",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: strings.TrimSpace(`
Reverse-indexes the local store: for a given recipe slug, returns every
synced article whose relatedReading list references that recipe. Useful for
pulling in editorial context (origin story, technique deep-dive) for a
recipe you've found.

Run 'sync articles food' (or 'sync articles food baking', etc.) first.
`),
		Example: strings.Trim(`
  food52-pp-cli articles for-recipe sarah-fennel-s-best-lunch-lady-brownie-recipe
  food52-pp-cli articles for-recipe https://food52.com/recipes/mom-s-japanese-curry-chicken-with-radish-and-cauliflower --json
`, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := recipeSlugFromArg(args[0])
			if slug == "" {
				return fmt.Errorf("recipe slug or URL is required")
			}
			db, err := openStoreOrErr()
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().Query("SELECT data FROM articles")
			if err != nil {
				return err
			}
			defer rows.Close()
			var matches []food52.ArticleSummary
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					return err
				}
				var a food52.Article
				if err := json.Unmarshal(data, &a); err != nil {
					continue
				}
				for _, related := range a.RelatedRecipes {
					if related == slug {
						matches = append(matches, food52.ArticleSummary{
							ID:               a.ID,
							Slug:             a.Slug,
							Title:            a.Title,
							URL:              a.URL,
							Dek:              a.Dek,
							AuthorName:       a.AuthorName,
							FeaturedImageURL: a.FeaturedImageURL,
							PublishedAt:      a.PublishedAt,
							Vertical:         a.Vertical,
							SubVertical:      a.SubVertical,
							Tags:             a.Tags,
						})
						break
					}
				}
			}
			payload := map[string]any{
				"recipe_slug": slug,
				"count":       len(matches),
				"articles":    matches,
			}
			return emitFromFlags(flags, payload, func() {
				if len(matches) == 0 {
					fmt.Printf("No synced articles reference %s. Sync articles first: food52-pp-cli sync articles food\n", slug)
					return
				}
				fmt.Printf("%d article(s) reference recipe %s\n", len(matches), slug)
				for i, a := range matches {
					fmt.Printf("%2d. %s\n    %s\n", i+1, a.Title, a.URL)
				}
			})
		},
	}
	return cmd
}
