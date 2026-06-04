// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:client-call — real HTTP via fetchRestaurantListPage -> rappi.Client.FetchHTML.
package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

func newRestaurantsTopCmd(flags *rootFlags) *cobra.Command {
	var (
		city       string
		category   string
		minRating  float64
		minReviews int
		limit      int
	)
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Top-rated restaurants with a minimum rating AND a minimum review-count floor",
		Long: `List the top N restaurants in a city or city + category, filtered
by both a minimum rating threshold AND a minimum review-count
floor. The review-count floor is the listicle-grade filter Rappi's
UI hides — without it, a brand-new restaurant with three perfect
ratings outranks well-established ones with thousands of reviews.`,
		Example:     "  rappi-pp-cli restaurants top --city ciudad-de-mexico --category hamburguesas --min-rating 4.5 --min-reviews 100 --limit 10 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if city == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			// PATCH: Use the root request settings for live Rappi fetches.
			rappiClient := newRappiHTMLFetcher(flags)
			rows, err := fetchRestaurantListPage(cmd.Context(), rappiClient, city, category)
			if err != nil {
				return err
			}
			out := []rappi.RestaurantListItem{}
			for _, r := range rows {
				if r.RatingValue < minRating {
					continue
				}
				if r.RatingCount < minReviews {
					continue
				}
				if r.ID == "" {
					// skip header/section anchors that aren't real restaurants
					continue
				}
				out = append(out, r)
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].RatingValue != out[j].RatingValue {
					return out[i].RatingValue > out[j].RatingValue
				}
				return out[i].RatingCount > out[j].RatingCount
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Top %d in %s (min-rating %.1f, min-reviews %d):\n", len(out), city, minRating, minReviews)
			for i, r := range out {
				fmt.Fprintf(w, "  %2d. %-40s %.1f★ (%d reviews)  %s\n",
					i+1, truncate(r.Name, 40), r.RatingValue, r.RatingCount, r.URL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&city, "city", "", "City slug (required)")
	cmd.Flags().StringVar(&category, "category", "", "Cuisine category slug (optional; defaults to all categories in the city)")
	cmd.Flags().Float64Var(&minRating, "min-rating", 4.5, "Minimum aggregateRating.ratingValue")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 100, "Minimum aggregateRating.reviewCount")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max rows to return")
	return cmd
}
