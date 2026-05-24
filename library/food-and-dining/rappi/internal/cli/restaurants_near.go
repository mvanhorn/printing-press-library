// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
// pp:client-call — real HTTP via fetchRestaurantListPage / fetchRestaurantDetail -> rappi.Client.FetchHTML.
package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

func newRestaurantsNearCmd(flags *rootFlags) *cobra.Command {
	var (
		city        string
		category    string
		lat         float64
		lng         float64
		radius      float64
		limit       int
		fetchDetail bool
	)
	cmd := &cobra.Command{
		Use:   "near",
		Short: "Restaurants within a Haversine radius of a lat/lng",
		Long: `Filter restaurants in a (city, category) by great-circle distance
from a lat/lng point. When --lat/--lng aren't provided, falls
back to the centroid of --city. With --fetch-detail the command
pulls each restaurant's detail page to read its real geo
coordinates; without it, distance can only be computed against
the city centroid (low utility for proximity, so --fetch-detail
is recommended).`,
		Example:     "  rappi-pp-cli restaurants near --lat 19.4216 --lng -99.1700 --radius-km 2 --category tacos --city ciudad-de-mexico --agent --fetch-detail",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if city == "" && lat == 0 && lng == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if lat == 0 && lng == 0 {
				lat, lng = resolveCityLatLng(city)
			}
			// PATCH: Reuse the configured Rappi client across list and detail fetches.
			rappiClient := newRappiHTMLFetcher(flags)
			rows, err := fetchRestaurantListPage(cmd.Context(), rappiClient, city, category)
			if err != nil {
				return err
			}
			type result struct {
				ID          string  `json:"id"`
				Name        string  `json:"name"`
				URL         string  `json:"url"`
				DistanceKm  float64 `json:"distance_km"`
				RatingValue float64 `json:"rating,omitempty"`
				RatingCount int     `json:"review_count,omitempty"`
				Latitude    float64 `json:"latitude,omitempty"`
				Longitude   float64 `json:"longitude,omitempty"`
			}
			out := []result{}
			for _, r := range rows {
				if r.ID == "" {
					continue
				}
				var rlat, rlng float64
				if fetchDetail {
					det, err := fetchRestaurantDetail(cmd.Context(), rappiClient, r.ID+"-"+slugFromURL(r.URL), city, category)
					if err != nil {
						stderrf("warning: detail fetch failed for %s: %v\n", r.URL, err)
						continue
					}
					rlat, rlng = det.Latitude, det.Longitude
				}
				// If no geo available, skip rather than fake the distance.
				if rlat == 0 && rlng == 0 {
					continue
				}
				d := haversineKm(lat, lng, rlat, rlng)
				if d > radius {
					continue
				}
				out = append(out, result{
					ID: r.ID, Name: r.Name, URL: r.URL,
					DistanceKm: d, RatingValue: r.RatingValue, RatingCount: r.RatingCount,
					Latitude: rlat, Longitude: rlng,
				})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].DistanceKm < out[j].DistanceKm })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if !fetchDetail {
				stderrf("note: --fetch-detail is required for accurate per-restaurant geo; this run only matched restaurants whose geo was already known.\n")
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Within %.2fkm of (%.4f, %.4f) in %s/%s:\n", radius, lat, lng, city, category)
			for _, r := range out {
				fmt.Fprintf(w, "  %5.2fkm  %-40s %.1f★  %s\n", r.DistanceKm, truncate(r.Name, 40), r.RatingValue, r.URL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&city, "city", "", "City slug (for catalog fetch; centroid fallback for lat/lng)")
	cmd.Flags().StringVar(&category, "category", "", "Cuisine category slug")
	cmd.Flags().Float64Var(&lat, "lat", 0, "Search latitude (defaults to city centroid)")
	cmd.Flags().Float64Var(&lng, "lng", 0, "Search longitude (defaults to city centroid)")
	cmd.Flags().Float64Var(&radius, "radius-km", 2.0, "Haversine radius in kilometers")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max rows to return")
	cmd.Flags().BoolVar(&fetchDetail, "fetch-detail", false, "Fetch each restaurant's detail page for accurate geo (slow but accurate)")
	_ = rappi.Cities // anchor the import for tooling
	return cmd
}
