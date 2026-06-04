// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:client-call — real HTTP via fetchRestaurantListPage -> rappi.Client.FetchHTML.
package cli

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

type restaurantBrandHit struct {
	City        string  `json:"city"`
	Category    string  `json:"category,omitempty"`
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	RatingValue float64 `json:"rating,omitempty"`
	RatingCount int     `json:"review_count,omitempty"`
}

func newRestaurantBrandHit(city string, r rappi.RestaurantListItem) restaurantBrandHit {
	return restaurantBrandHit{
		City: city,
		// PATCH: Brand scans city pages, so use scraped cuisine instead of the empty selector category.
		Category:    r.ServesCuisine,
		Name:        r.Name,
		URL:         r.URL,
		RatingValue: r.RatingValue,
		RatingCount: r.RatingCount,
	}
}

func newRestaurantsBrandCmd(flags *rootFlags) *cobra.Command {
	var (
		brandName string
		cities    []string
	)
	cmd := &cobra.Command{
		Use:   "brand",
		Short: "Find every city × category where a restaurant brand appears in Rappi MX",
		Long: `Fuzzy-match a restaurant brand name against the current Rappi
catalog across the cities passed to --cities (default: all
served cities). Emits a presence matrix of where the brand
appears, by (city, category). Useful for chain-coverage analysis
and asking "where does X expand in MX".`,
		Example:     "  rappi-pp-cli restaurants brand --name \"Sushi Itto\" --cities ciudad-de-mexico,guadalajara,monterrey --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if brandName == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			needle := strings.ToLower(brandName)
			if len(cities) == 0 {
				cities = rappi.CitySlugs()
			}
			// PATCH: Reuse the configured Rappi client across city fetches.
			rappiClient := newRappiHTMLFetcher(flags)
			out := []restaurantBrandHit{}
			var mu sync.Mutex
			var wg sync.WaitGroup
			sem := make(chan struct{}, 4)
			for _, c := range cities {
				wg.Add(1)
				go func(c string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					rows, err := fetchRestaurantListPage(cmd.Context(), rappiClient, c, "")
					if err != nil {
						stderrf("warning: %s fetch failed: %v\n", c, err)
						return
					}
					for _, r := range rows {
						if r.ID == "" {
							continue
						}
						if !strings.Contains(strings.ToLower(r.Name), needle) {
							continue
						}
						h := newRestaurantBrandHit(c, r)
						mu.Lock()
						out = append(out, h)
						mu.Unlock()
					}
				}(c)
			}
			wg.Wait()
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].City != out[j].City {
					return out[i].City < out[j].City
				}
				return out[i].Name < out[j].Name
			})
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Brand presence for %q across %d cities:\n", brandName, len(cities))
			for _, h := range out {
				fmt.Fprintf(w, "  %-18s  %s  %s\n", h.City, h.Name, h.URL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&brandName, "name", "", "Brand name to search for (substring match, case-insensitive)")
	cmd.Flags().StringSliceVar(&cities, "cities", nil, "Comma-separated city slugs to scan (default: all)")
	return cmd
}
