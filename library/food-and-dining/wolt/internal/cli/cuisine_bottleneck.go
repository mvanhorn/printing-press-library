// Copyright 2026 amit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

type cuisineBucket struct {
	Tag        string  `json:"tag"`
	VenueCount int     `json:"venue_count"`
	OpenCount  int     `json:"open_count"`
	AvgETAMin  float64 `json:"avg_eta_min"`
	MinETAMin  int     `json:"min_eta_min"`
	MaxETAMin  int     `json:"max_eta_min"`
}

func newCuisineBottleneckCmd(flags *rootFlags) *cobra.Command {
	var lat, lon float64
	var topN int
	var includeClosed bool
	cmd := &cobra.Command{
		Use:   "cuisine-bottleneck",
		Short: "Show which cuisines have the longest average ETA right now near a lat/lon",
		Long: "Aggregates current delivery ETAs across all nearby venues grouped by\n" +
			"cuisine tag. Helps answer 'what's slow tonight' in one call instead of\n" +
			"hunting through dozens of venue cards.",
		Example: "  wolt-pp-cli cuisine-bottleneck --lat 60.1699 --lon 24.9384 --top 10 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if lat == 0 && lon == 0 {
				return fmt.Errorf("must pass --lat and --lon (or use --profile to set them)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			fullURL := "https://consumer-api.wolt.com/v1/pages/restaurants?" + url.Values{
				"lat": {strconv.FormatFloat(lat, 'f', -1, 64)},
				"lon": {strconv.FormatFloat(lon, 'f', -1, 64)},
			}.Encode()
			raw, err := c.Get(cmd.Context(), fullURL, nil)
			if err != nil {
				return fmt.Errorf("fetching nearby restaurants: %w", err)
			}
			var page struct {
				City     string `json:"city"`
				Sections []struct {
					Items []map[string]any `json:"items"`
				} `json:"sections"`
			}
			if err := json.Unmarshal(raw, &page); err != nil {
				return fmt.Errorf("parsing nearby restaurants: %w", err)
			}
			// PATCH(cuisine-bottleneck-dedup-by-slug): Wolt groups the same
			// venue into multiple sections of /v1/pages/restaurants. Without
			// dedup, a single venue would be counted N times in VenueCount /
			// OpenCount and would contribute N times to AvgETAMin per tag,
			// skewing every aggregation.
			seen := make(map[string]bool)
			buckets := map[string]*cuisineBucket{}
			for _, sec := range page.Sections {
				for _, it := range sec.Items {
					row, ok := extractVenueRowWNow(it)
					if !ok {
						continue
					}
					if seen[row.Slug] {
						continue
					}
					if !includeClosed && !row.Online {
						continue
					}
					if row.EstimateMin <= 0 {
						continue
					}
					seen[row.Slug] = true
					for _, t := range row.Tags {
						b := buckets[t]
						if b == nil {
							b = &cuisineBucket{Tag: t, MinETAMin: row.EstimateMin}
							buckets[t] = b
						}
						b.VenueCount++
						if row.Online {
							b.OpenCount++
						}
						b.AvgETAMin += float64(row.EstimateMin)
						if row.EstimateMin < b.MinETAMin {
							b.MinETAMin = row.EstimateMin
						}
						if row.EstimateMin > b.MaxETAMin {
							b.MaxETAMin = row.EstimateMin
						}
					}
				}
			}
			rows := make([]cuisineBucket, 0, len(buckets))
			for _, b := range buckets {
				if b.VenueCount > 0 {
					b.AvgETAMin = b.AvgETAMin / float64(b.VenueCount)
				}
				rows = append(rows, *b)
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].AvgETAMin > rows[j].AvgETAMin
			})
			if topN > 0 && len(rows) > topN {
				rows = rows[:topN]
			}
			out := struct {
				City    string          `json:"city"`
				Count   int             `json:"count"`
				Buckets []cuisineBucket `json:"cuisine_buckets"`
			}{City: page.City, Count: len(rows), Buckets: rows}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude")
	cmd.Flags().IntVar(&topN, "top", 10, "Show top N slowest cuisines (0 = all)")
	cmd.Flags().BoolVar(&includeClosed, "include-closed", false, "Include venues that are not currently online")
	return cmd
}
