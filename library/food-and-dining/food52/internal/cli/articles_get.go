// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newArticlesGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <slug-or-url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Get a Food52 article (story) by slug or URL",
		Example: strings.Trim(`
  food52-pp-cli articles get best-mothers-day-gift-ideas
  food52-pp-cli articles get https://food52.com/story/best-mothers-day-gift-ideas --json
`, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := articleSlugFromArg(args[0])
			if slug == "" {
				return fmt.Errorf("article slug or URL is required")
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
				row := db.DB().QueryRow("SELECT data FROM articles WHERE slug = ? LIMIT 1", slug)
				var data []byte
				if err := row.Scan(&data); err != nil {
					return fmt.Errorf("article %q not in local store", slug)
				}
				var a food52.Article
				if err := json.Unmarshal(data, &a); err != nil {
					return fmt.Errorf("decoding local article row: %w", err)
				}
				return emitFromFlags(flags, a, func() { renderArticleText(&a) })
			}

			path := "/story/" + slug
			html, err := fetchHTML(c, path, nil)
			if err != nil {
				return classifyAPIError(err)
			}
			if c.DryRun {
				return emitJSON(map[string]any{"slug": slug, "dry_run": true, "url": canonicalArticleURL(slug)})
			}
			a, err := food52.ExtractArticle(html, canonicalArticleURL(slug))
			if err != nil {
				return fmt.Errorf("food52 articles get %s: %w", slug, err)
			}
			return emitFromFlags(flags, a, func() { renderArticleText(a) })
		},
	}
	return cmd
}

func renderArticleText(a *food52.Article) {
	if a == nil {
		return
	}
	fmt.Printf("%s\n%s\n", a.Title, strings.Repeat("=", min(len(a.Title), 78)))
	if a.AuthorName != "" {
		fmt.Printf("by %s\n", a.AuthorName)
	}
	if a.PublishedAt != "" {
		fmt.Println(a.PublishedAt)
	}
	fmt.Println(a.URL)
	if a.Dek != "" {
		fmt.Println()
		fmt.Println(a.Dek)
	}
	fmt.Println()
	fmt.Println(a.Body)
	if len(a.RelatedRecipes) > 0 {
		fmt.Println()
		fmt.Println("Related recipes:")
		for _, slug := range a.RelatedRecipes {
			fmt.Printf("- %s\n", canonicalRecipeURL(slug))
		}
	}
}
