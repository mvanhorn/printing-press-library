// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newLocalSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		typ   string
		limit int
	)
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search the local store across synced recipes and articles",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: strings.TrimSpace(`
Searches every recipe and article you have synced into the local SQLite store
using a LIKE-based scan over title, slug, and (for articles) body text. Run
'sync recipes <tag>...' and 'sync articles <vertical>' first to populate the
store; this command does not hit Food52.

Use --type to constrain the corpus (recipe or article).
`),
		Example: strings.Trim(`
  food52-pp-cli search miso --json
  food52-pp-cli search "weeknight dinner" --type recipe --limit 10
`, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return fmt.Errorf("query is required")
			}
			db, err := openStoreOrErr()
			if err != nil {
				return err
			}
			defer db.Close()

			var recipes []food52.RecipeSummary
			var articles []food52.ArticleSummary
			if typ == "" || typ == "recipe" {
				recipes, err = searchLocalRecipes(db.DB(), query, limit)
				if err != nil {
					return err
				}
			}
			if typ == "" || typ == "article" {
				articles, err = searchLocalArticles(db.DB(), query, limit)
				if err != nil {
					return err
				}
			}
			payload := map[string]any{
				"query":    query,
				"type":     typ,
				"recipes":  recipes,
				"articles": articles,
				"counts":   map[string]int{"recipes": len(recipes), "articles": len(articles)},
			}
			return emitFromFlags(flags, payload, func() {
				if len(recipes)+len(articles) == 0 {
					fmt.Printf("No matches for %q in the local store. Sync first: food52-pp-cli sync recipes <tag>\n", query)
					return
				}
				if len(recipes) > 0 {
					fmt.Printf("Recipes (%d):\n", len(recipes))
					for i, r := range recipes {
						fmt.Printf("%2d. %s\n    %s\n", i+1, r.Title, r.URL)
					}
				}
				if len(articles) > 0 {
					if len(recipes) > 0 {
						fmt.Println()
					}
					fmt.Printf("Articles (%d):\n", len(articles))
					for i, a := range articles {
						fmt.Printf("%2d. %s\n    %s\n", i+1, a.Title, a.URL)
					}
				}
			})
		},
	}
	cmd.Flags().StringVar(&typ, "type", "", "Constrain to one corpus: recipe or article (default both)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results per corpus")
	return cmd
}

func searchLocalRecipes(db *sql.DB, q string, limit int) ([]food52.RecipeSummary, error) {
	pattern := "%" + strings.ToLower(q) + "%"
	rows, err := db.Query(
		`SELECT data FROM recipes
		 WHERE LOWER(json_extract(data, '$.title')) LIKE ?
		    OR LOWER(json_extract(data, '$.description')) LIKE ?
		    OR LOWER(slug) LIKE ?
		 LIMIT ?`,
		pattern, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []food52.RecipeSummary{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var rs food52.RecipeSummary
		if err := json.Unmarshal(data, &rs); err == nil {
			out = append(out, rs)
		}
	}
	return out, rows.Err()
}

func searchLocalArticles(db *sql.DB, q string, limit int) ([]food52.ArticleSummary, error) {
	pattern := "%" + strings.ToLower(q) + "%"
	rows, err := db.Query(
		`SELECT data FROM articles
		 WHERE LOWER(json_extract(data, '$.title')) LIKE ?
		    OR LOWER(json_extract(data, '$.dek')) LIKE ?
		    OR LOWER(json_extract(data, '$.body')) LIKE ?
		    OR LOWER(slug) LIKE ?
		 LIMIT ?`,
		pattern, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []food52.ArticleSummary{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var a food52.ArticleSummary
		if err := json.Unmarshal(data, &a); err == nil {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}
