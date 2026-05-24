// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
// pp:client-call — real HTTP via fetchRestaurantListPage / fetchRestaurantDetail -> rappi.Client.FetchHTML.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newRestaurantsByNeighborhoodCmd(flags *rootFlags) *cobra.Command {
	var (
		city        string
		category    string
		fetchDetail bool
		topPer      int
	)
	cmd := &cobra.Command{
		Use:   "by-neighborhood",
		Short: "Group restaurants by neighborhood (extracted from address) within a city",
		Long: `Group restaurants in a (city, category) by neighborhood. Neighborhoods
are extracted from each restaurant's address ("COL. <name>" segment
in Mexican addresses). With --fetch-detail the command pulls each
restaurant's detail page to read its full address; without it the
neighborhood is only known for restaurants whose neighborhood was
populated upstream (likely none, in the absence of a prior sync).`,
		Example:     "  rappi-pp-cli restaurants by-neighborhood --city ciudad-de-mexico --category pizza --fetch-detail --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if city == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			// PATCH: Reuse the configured Rappi client across list and detail fetches.
			rappiClient := newRappiHTMLFetcher(flags)
			rows, err := fetchRestaurantListPage(cmd.Context(), rappiClient, city, category)
			if err != nil {
				return err
			}
			type restaurantInfo struct {
				Name         string  `json:"name"`
				URL          string  `json:"url"`
				Rating       float64 `json:"rating,omitempty"`
				Neighborhood string  `json:"neighborhood,omitempty"`
				Address      string  `json:"address,omitempty"`
			}
			byNeighborhood := map[string][]restaurantInfo{}
			for _, r := range rows {
				if r.ID == "" {
					continue
				}
				neighborhood := ""
				address := ""
				if fetchDetail {
					det, err := fetchRestaurantDetail(cmd.Context(), rappiClient, r.ID+"-"+slugFromURL(r.URL), city, category)
					if err != nil {
						stderrf("warning: detail fetch for %s: %v\n", r.URL, err)
						continue
					}
					neighborhood = strings.TrimSpace(det.Neighborhood)
					address = det.AddressStreet
				}
				if neighborhood == "" {
					neighborhood = "(unknown)"
				}
				byNeighborhood[neighborhood] = append(byNeighborhood[neighborhood], restaurantInfo{
					Name: r.Name, URL: r.URL, Rating: r.RatingValue,
					Neighborhood: neighborhood, Address: address,
				})
			}
			type group struct {
				Neighborhood string           `json:"neighborhood"`
				Count        int              `json:"count"`
				TopRated     []restaurantInfo `json:"top_rated"`
			}
			groups := []group{}
			for n, infos := range byNeighborhood {
				sort.SliceStable(infos, func(i, j int) bool { return infos[i].Rating > infos[j].Rating })
				top := infos
				if topPer > 0 && len(top) > topPer {
					top = top[:topPer]
				}
				groups = append(groups, group{Neighborhood: n, Count: len(infos), TopRated: top})
			}
			sort.SliceStable(groups, func(i, j int) bool { return groups[i].Count > groups[j].Count })
			if !fetchDetail {
				stderrf("note: without --fetch-detail every row falls into (unknown). Re-run with --fetch-detail for real neighborhoods.\n")
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), groups, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Restaurants by neighborhood in %s/%s:\n", city, category)
			for _, g := range groups {
				fmt.Fprintf(w, "  %-25s  %d restaurants\n", g.Neighborhood, g.Count)
				for _, r := range g.TopRated {
					fmt.Fprintf(w, "      %.1f★  %s\n", r.Rating, r.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&city, "city", "", "City slug (required)")
	cmd.Flags().StringVar(&category, "category", "", "Cuisine category slug")
	cmd.Flags().BoolVar(&fetchDetail, "fetch-detail", false, "Fetch each restaurant's detail page to read the full address (slow but accurate)")
	cmd.Flags().IntVar(&topPer, "top-per", 3, "Top-rated restaurants to surface per neighborhood (0 = all)")
	return cmd
}
