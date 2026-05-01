// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newArticlesBrowseSubCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "browse-sub <vertical> <subvertical>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Browse Food52 articles in a vertical/subvertical (e.g. food baking, life travel)",
		Example: strings.Trim(`
  food52-pp-cli articles browse-sub food baking
  food52-pp-cli articles browse-sub food drinks --limit 10 --json
  food52-pp-cli articles browse-sub life travel
`, "\n"),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vertical := strings.TrimSpace(args[0])
			sub := strings.TrimSpace(args[1])
			if vertical == "" || sub == "" {
				return fmt.Errorf("both vertical and subvertical are required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dataSource == "local" {
				return browseArticlesLocal(vertical, sub, limit, flags)
			}

			path := "/" + vertical + "/" + sub
			html, err := fetchHTML(c, path, nil)
			if err != nil {
				if flags.dataSource == "auto" && isNetworkError(err) {
					return browseArticlesLocal(vertical, sub, limit, flags)
				}
				return classifyAPIError(err)
			}
			if c.DryRun {
				return emitJSON(map[string]any{"vertical": vertical, "sub_vertical": sub, "dry_run": true})
			}
			results, err := food52.ExtractArticlesByVertical(html)
			if err != nil {
				return fmt.Errorf("food52 articles browse-sub %s/%s: %w", vertical, sub, err)
			}
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			payload := map[string]any{
				"vertical":     vertical,
				"sub_vertical": sub,
				"count":        len(results),
				"results":      results,
			}
			return emitFromFlags(flags, payload, func() {
				fmt.Printf("Articles in %s/%s — %d results\n", vertical, sub, len(results))
				for i, a := range results {
					fmt.Printf("%2d. %s\n    %s\n", i+1, a.Title, a.URL)
				}
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Truncate results to the first N articles (0 = all)")
	return cmd
}
