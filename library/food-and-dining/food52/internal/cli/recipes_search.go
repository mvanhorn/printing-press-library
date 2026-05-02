// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newRecipesSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		tag     string
		page    int
		perPage int
		sort    string
	)
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Search Food52 recipes via Typesense (Food52's own search backend)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: strings.TrimSpace(`
Search Food52's recipe collection. Uses the public Typesense search-only key
that the CLI auto-discovers from Food52's JS bundle on first run, so there is
no key to provision.

Pair --tag with a slug from ` + "`tags list`" + ` to constrain the result set
(e.g., --tag chicken, --tag vegetarian). Pair --sort with the field+direction
to override the default popularity ranking (e.g., --sort publishedAt:desc for
newest first, --sort rating:desc for highest-rated).
`),
		Example: strings.Trim(`
  food52-pp-cli recipes search brownies --limit 5 --json
  food52-pp-cli recipes search chicken --tag 30-minutes-or-fewer
  food52-pp-cli recipes search "white beans" --sort publishedAt:desc --limit 10
`, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return fmt.Errorf("query is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			httpc := httpClientForFood52(c)

			d, err := food52.LoadDiscovery(httpc)
			if err != nil {
				return fmt.Errorf("food52 discovery: %w", err)
			}
			params := food52.SearchRecipesParams{
				Query:   query,
				Tag:     tag,
				Page:    page,
				PerPage: perPage,
				Sort:    sort,
			}
			res, err := food52.SearchRecipes(httpc, d, params)
			if err != nil {
				if errors.Is(err, food52.ErrTypesenseAuth) {
					// Search-only key was rotated. Re-discover once and retry.
					food52.InvalidateDiscovery()
					d, derr := food52.LoadDiscovery(httpc)
					if derr != nil {
						return fmt.Errorf("rediscovering after typesense auth failure: %w", derr)
					}
					res, err = food52.SearchRecipes(httpc, d, params)
				}
			}
			if err != nil {
				return err
			}
			return emitFromFlags(flags, res, func() {
				fmt.Printf("%d results for %q (page %d/%d)\n", res.Found, res.Query, res.Page, pagesFromFound(res.Found, res.PerPage))
				for i, h := range res.Hits {
					marker := ""
					if h.TestKitchenApproved {
						marker = " ★"
					}
					rating := ""
					if h.AverageRating > 0 {
						rating = fmt.Sprintf(" (%.1f, %d)", h.AverageRating, h.RatingCount)
					}
					fmt.Printf("%2d. %s%s%s\n    %s\n", i+1, h.Title, marker, rating, h.URL)
				}
			})
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Filter results by tag slug (e.g. chicken, vegetarian, 30-minutes-or-fewer)")
	cmd.Flags().IntVar(&page, "page", 1, "Result page (1-indexed)")
	cmd.Flags().IntVar(&perPage, "limit", 36, "Results per page (1-250)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort field:direction (e.g. publishedAt:desc, rating:desc)")
	return cmd
}

func pagesFromFound(found, perPage int) int {
	if perPage <= 0 || found <= 0 {
		return 1
	}
	return (found + perPage - 1) / perPage
}
