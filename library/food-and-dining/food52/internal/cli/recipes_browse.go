// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/store"
)

func newRecipesBrowseCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "browse <tag>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Browse Food52 recipes filtered by a tag (e.g. chicken, breakfast, vegetarian)",
		Example: strings.Trim(`
  food52-pp-cli recipes browse chicken
  food52-pp-cli recipes browse vegetarian --limit 10 --json
  food52-pp-cli recipes browse pasta --select 'results.title,results.slug,results.average_rating'
`, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tag := strings.TrimSpace(args[0])
			if tag == "" {
				return fmt.Errorf("tag is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dataSource == "local" {
				return browseRecipesLocal(tag, limit, flags)
			}

			path := "/recipes/" + tag
			html, err := fetchHTML(c, path, nil)
			if err != nil {
				if flags.dataSource == "auto" && isNetworkError(err) {
					return browseRecipesLocal(tag, limit, flags)
				}
				return classifyAPIError(err)
			}
			if c.DryRun {
				return emitJSON(map[string]any{"tag": tag, "dry_run": true, "url": canonicalTagURL(tag)})
			}

			results, tagName, err := food52.ExtractRecipesByTag(html)
			if err != nil {
				return fmt.Errorf("food52 browse %s: %w", tag, err)
			}
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			payload := map[string]any{
				"tag":     tag,
				"name":    tagName,
				"url":     canonicalTagURL(tag),
				"count":   len(results),
				"results": results,
			}
			return emitFromFlags(flags, payload, func() {
				fmt.Printf("%s — %d results\n", tagName, len(results))
				for i, r := range results {
					marker := ""
					if r.TestKitchenApproved {
						marker = " ★"
					}
					fmt.Printf("%2d. %s%s\n    %s\n", i+1, r.Title, marker, r.URL)
				}
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Truncate results to the first N recipes (0 = all)")
	return cmd
}

func browseRecipesLocal(tag string, limit int, flags *rootFlags) error {
	db, err := openStoreOrErr()
	if err != nil {
		return fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()

	rows, err := selectLocalRecipesByTag(db, tag, limit)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"tag":     tag,
		"url":     canonicalTagURL(tag),
		"count":   len(rows),
		"results": rows,
		"source":  "local",
	}
	return emitFromFlags(flags, payload, func() {
		if len(rows) == 0 {
			fmt.Printf("No locally-synced recipes for tag %q. Run: food52-pp-cli sync recipes %s\n", tag, tag)
			return
		}
		fmt.Printf("%s — %d results (local)\n", tag, len(rows))
		for i, r := range rows {
			fmt.Printf("%2d. %s\n    %s\n", i+1, r.Title, r.URL)
		}
	})
}

func selectLocalRecipesByTag(db *store.Store, tag string, limit int) ([]food52.RecipeSummary, error) {
	q := "SELECT data FROM recipes WHERE tag = ?"
	args := []any{tag}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("local query: %w", err)
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
