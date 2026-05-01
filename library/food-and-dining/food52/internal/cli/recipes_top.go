// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newRecipesTopCmd(flags *rootFlags) *cobra.Command {
	var (
		minRating float64
		tkOnly    bool
		limit     int
	)
	cmd := &cobra.Command{
		Use:   "top <tag>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show Food52 Test-Kitchen-approved + rating-floored recipes for a tag",
		Long: strings.TrimSpace(`
Filter recipes for a tag down to the editorially trusted set. By default
returns only Test-Kitchen-approved recipes; pair --min-rating to add a
community-rating floor on top, or --no-tk to drop the Test-Kitchen filter
and rank by rating alone.

This is the "give me a recipe Food52's editors signed off on" command.
`),
		Example: strings.Trim(`
  food52-pp-cli recipes top chicken --limit 5 --json
  food52-pp-cli recipes top dessert --min-rating 4
  food52-pp-cli recipes top pasta --no-tk --min-rating 4 --limit 10
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
			httpc := httpClientForFood52(c)
			d, err := food52.LoadDiscovery(httpc)
			if err != nil {
				return fmt.Errorf("food52 discovery: %w", err)
			}

			// Pull a wide first page (250 max), filter, then truncate.
			res, err := food52.SearchRecipes(httpc, d, food52.SearchRecipesParams{
				Query:   "*",
				Tag:     tag,
				PerPage: 250,
				Sort:    "rating:desc",
			})
			if err != nil {
				if errors.Is(err, food52.ErrTypesenseAuth) {
					food52.InvalidateDiscovery()
					d, _ = food52.LoadDiscovery(httpc)
					res, err = food52.SearchRecipes(httpc, d, food52.SearchRecipesParams{
						Query: "*", Tag: tag, PerPage: 250, Sort: "rating:desc",
					})
				}
			}
			if err != nil {
				return err
			}
			filtered := []food52.RecipeSummary{}
			for _, h := range res.Hits {
				if tkOnly && !h.TestKitchenApproved {
					continue
				}
				if h.AverageRating < minRating {
					continue
				}
				filtered = append(filtered, h)
			}
			if limit > 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}
			payload := map[string]any{
				"tag":          tag,
				"min_rating":   minRating,
				"tk_only":      tkOnly,
				"count":        len(filtered),
				"results":      filtered,
				"total_pulled": len(res.Hits),
			}
			return emitFromFlags(flags, payload, func() {
				if len(filtered) == 0 {
					fmt.Printf("No recipes match (tag=%s, min_rating=%.1f, tk_only=%v).\n", tag, minRating, tkOnly)
					fmt.Printf("Pulled %d candidates from Typesense.\n", len(res.Hits))
					return
				}
				fmt.Printf("Top %d for %s (tk_only=%v, min_rating=%.1f)\n", len(filtered), tag, tkOnly, minRating)
				for i, h := range filtered {
					marker := ""
					if h.TestKitchenApproved {
						marker = " ★"
					}
					fmt.Printf("%2d. %s%s (%.1f, %d reviews)\n    %s\n", i+1, h.Title, marker, h.AverageRating, h.RatingCount, h.URL)
				}
			})
		},
	}
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Filter to recipes with averageRating >= N (0 = no floor)")
	cmd.Flags().BoolVar(&tkOnly, "tk-only", true, "Filter to Test-Kitchen-approved recipes only (use --tk-only=false to include community recipes)")
	// Provide a friendly inverse alias.
	cmd.Flags().Lookup("tk-only").NoOptDefVal = "true"
	cmd.Flags().BoolP("no-tk", "", false, "Alias for --tk-only=false")
	cmd.Flags().IntVar(&limit, "limit", 5, "Cap the result set to N recipes after filtering")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if noTk, _ := cmd.Flags().GetBool("no-tk"); noTk {
			tkOnly = false
		}
		return nil
	}
	return cmd
}
