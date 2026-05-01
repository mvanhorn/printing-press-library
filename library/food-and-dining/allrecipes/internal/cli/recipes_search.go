// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-edited after generation: replaces the generic html_extract "links" pass
// with an Allrecipes-specific search-card parser that returns clean
// SearchResult records (URL, title, image, rating, reviewCount).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"

	"github.com/spf13/cobra"
)

func newRecipesSearchCmd(flags *rootFlags) *cobra.Command {
	var flagQ string
	var flagPage int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "search",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Search Allrecipes for recipes matching a query",
		Example: "  allrecipes-pp-cli recipes search --q brownies\n" +
			"  allrecipes-pp-cli recipes search --q \"chicken thighs\" --limit 10 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("q") && !flags.dryRun {
				return fmt.Errorf("required flag \"q\" not set")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{"plan": fmt.Sprintf("GET /search?q=%s&page=%d", flagQ, flagPage)})
			}
			page := flagPage
			if page < 1 {
				page = 1
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 24
			}
			results, err := recipes.FetchSearch(c, flagQ, page, limit)
			if err != nil {
				return classifyAPIError(err)
			}
			data, err := json.Marshal(results)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				rows := make([][]string, 0, len(results))
				for _, r := range results {
					rating := ""
					if r.Rating > 0 {
						rating = fmt.Sprintf("%.1f (%d)", r.Rating, r.ReviewCount)
					} else if r.ReviewCount > 0 {
						rating = fmt.Sprintf("(%d)", r.ReviewCount)
					}
					rows = append(rows, []string{r.Title, rating, r.URL})
				}
				if err := flags.printTable(cmd, []string{"TITLE", "RATING", "URL"}, rows); err != nil {
					return err
				}
				if len(results) >= 25 {
					fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: --limit N or --agent for JSON.\n", len(results))
				}
				return nil
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagQ, "q", "", "Search query (e.g. 'brownies')")
	cmd.Flags().IntVar(&flagPage, "page", 1, "Result page (1-indexed; default 1)")
	cmd.Flags().IntVar(&flagLimit, "limit", 24, "Maximum results to return (default 24)")

	return cmd
}

// trimURL is a tiny helper used by callers that want to display URLs in tables
// without a long query string.
func trimURL(u string) string {
	if i := strings.Index(u, "?"); i >= 0 {
		return u[:i]
	}
	return u
}
