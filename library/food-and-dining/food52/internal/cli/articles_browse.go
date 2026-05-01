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

func newArticlesBrowseCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "browse <vertical>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Browse the latest Food52 articles in a vertical (food, life)",
		Example: strings.Trim(`
  food52-pp-cli articles browse food
  food52-pp-cli articles browse life --limit 10 --json
`, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vertical := strings.TrimSpace(args[0])
			if vertical == "" {
				return fmt.Errorf("vertical is required (food or life)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dataSource == "local" {
				return browseArticlesLocal(vertical, "", limit, flags)
			}

			path := "/" + vertical
			html, err := fetchHTML(c, path, nil)
			if err != nil {
				if flags.dataSource == "auto" && isNetworkError(err) {
					return browseArticlesLocal(vertical, "", limit, flags)
				}
				return classifyAPIError(err)
			}
			if c.DryRun {
				return emitJSON(map[string]any{"vertical": vertical, "dry_run": true})
			}
			results, err := food52.ExtractArticlesByVertical(html)
			if err != nil {
				return fmt.Errorf("food52 articles browse %s: %w", vertical, err)
			}
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			payload := map[string]any{
				"vertical": vertical,
				"count":    len(results),
				"results":  results,
			}
			return emitFromFlags(flags, payload, func() {
				fmt.Printf("Articles in %s — %d results\n", vertical, len(results))
				for i, a := range results {
					fmt.Printf("%2d. %s\n    %s\n", i+1, a.Title, a.URL)
				}
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Truncate results to the first N articles (0 = all)")
	return cmd
}

func browseArticlesLocal(vertical, sub string, limit int, flags *rootFlags) error {
	db, err := openStoreOrErr()
	if err != nil {
		return fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()
	rows, err := selectLocalArticles(db, vertical, sub, limit)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"vertical":     vertical,
		"sub_vertical": sub,
		"count":        len(rows),
		"results":      rows,
		"source":       "local",
	}
	return emitFromFlags(flags, payload, func() {
		if len(rows) == 0 {
			fmt.Printf("No locally-synced articles for vertical %q.\n", vertical)
			return
		}
		fmt.Printf("Articles in %s (local) — %d results\n", vertical, len(rows))
		for i, a := range rows {
			fmt.Printf("%2d. %s\n    %s\n", i+1, a.Title, a.URL)
		}
	})
}

func selectLocalArticles(db *store.Store, vertical, sub string, limit int) ([]food52.ArticleSummary, error) {
	q := "SELECT data FROM articles WHERE vertical = ?"
	args := []any{vertical}
	if sub != "" {
		q += " AND json_extract(data, '$.sub_vertical') = ?"
		args = append(args, sub)
	}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("local query: %w", err)
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
