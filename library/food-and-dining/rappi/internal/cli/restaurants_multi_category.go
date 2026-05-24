// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
// pp:client-call — real HTTP via fetchRestaurantListPage -> rappi.Client.FetchHTML.
package cli

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

func newRestaurantsMultiCategoryCmd(flags *rootFlags) *cobra.Command {
	var (
		city       string
		categories []string
		minCats    int
	)
	cmd := &cobra.Command{
		Use:   "multi-category",
		Short: "Restaurants listed under two or more cuisine categories in a city",
		Long: `Self-join across category-list snapshots for a given city: emits
restaurants whose URL appears under two or more cuisine slugs.
Surfaces fusion places and ambiguously-categorized chains.`,
		Example:     "  rappi-pp-cli restaurants multi-category --city ciudad-de-mexico --categories hamburguesas,mexicana,tacos --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if city == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cats := categories
			if len(cats) == 0 {
				for _, c := range rappi.RestaurantCategories {
					cats = append(cats, c.Slug)
				}
			}
			// PATCH: Reuse the configured Rappi client across category fetches.
			rappiClient := newRappiHTMLFetcher(flags)
			byURL := map[string]map[string]bool{} // URL -> category set
			byURLName := map[string]string{}      // URL -> first-seen name
			ratingByURL := map[string]float64{}   // URL -> best rating seen
			var mu sync.Mutex
			var wg sync.WaitGroup
			sem := make(chan struct{}, 3)
			for _, cat := range cats {
				wg.Add(1)
				go func(cat string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					rows, err := fetchRestaurantListPage(cmd.Context(), rappiClient, city, cat)
					if err != nil {
						stderrf("warning: %s/%s fetch: %v\n", city, cat, err)
						return
					}
					mu.Lock()
					defer mu.Unlock()
					for _, r := range rows {
						if r.ID == "" {
							continue
						}
						if byURL[r.URL] == nil {
							byURL[r.URL] = map[string]bool{}
						}
						byURL[r.URL][cat] = true
						if _, exists := byURLName[r.URL]; !exists {
							byURLName[r.URL] = r.Name
						}
						if r.RatingValue > ratingByURL[r.URL] {
							ratingByURL[r.URL] = r.RatingValue
						}
					}
				}(cat)
			}
			wg.Wait()
			type result struct {
				Name          string   `json:"name"`
				URL           string   `json:"url"`
				Categories    []string `json:"categories"`
				CategoryCount int      `json:"category_count"`
				Rating        float64  `json:"rating,omitempty"`
			}
			out := []result{}
			for url, set := range byURL {
				if len(set) < minCats {
					continue
				}
				clist := make([]string, 0, len(set))
				for k := range set {
					clist = append(clist, k)
				}
				sort.Strings(clist)
				out = append(out, result{
					Name: byURLName[url], URL: url,
					Categories: clist, CategoryCount: len(clist), Rating: ratingByURL[url],
				})
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].CategoryCount != out[j].CategoryCount {
					return out[i].CategoryCount > out[j].CategoryCount
				}
				return out[i].Name < out[j].Name
			})
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Multi-category restaurants in %s (min %d categories):\n", city, minCats)
			for _, r := range out {
				fmt.Fprintf(w, "  %d cats  %-35s [%s]  %s\n", r.CategoryCount, truncate(r.Name, 35), fmt.Sprint(r.Categories), r.URL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&city, "city", "", "City slug (required)")
	cmd.Flags().StringSliceVar(&categories, "categories", nil, "Cuisine categories to consider (default: all)")
	cmd.Flags().IntVar(&minCats, "min-categories", 2, "Minimum number of categories a restaurant must appear in")
	return cmd
}
