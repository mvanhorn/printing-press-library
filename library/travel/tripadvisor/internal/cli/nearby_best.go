// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command. Preserved across regen.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type nearbyBestView struct {
	LatLong       string           `json:"lat_long"`
	Category      string           `json:"category,omitempty"`
	MinRating     float64          `json:"min_rating"`
	Sort          string           `json:"sort"`
	ScannedCount  int              `json:"scanned_locations"`
	MaxScan       int              `json:"max_scan"`
	Results       []taDetail       `json:"results"`
	FetchFailures []taFetchFailure `json:"fetch_failures"`
	Note          string           `json:"note,omitempty"`
}

func newNovelNearbyBestCmd(flags *rootFlags) *cobra.Command {
	var (
		category   string
		minRating  float64
		top        int
		maxScan    int
		sortKey    string
		radius     string
		radiusUnit string
		language   string
	)

	cmd := &cobra.Command{
		Use:   "nearby-best <lat,long>",
		Short: "Find nearby places, rank the highly-rated ones by ranking or rating, and return the top K",
		Long: "From a lat/long point, find nearby locations, fetch their details up to --max-scan (the API is " +
			"metered), drop anything below --min-rating, and return the top --top sorted by --sort.",
		Example: "  tripadvisor-pp-cli nearby-best \"48.8606,2.3376\" --category restaurants --min-rating 4.0 --top 5 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<latLong>=42.35,-71.05;--category=restaurants",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a \"lat,long\" argument is required"))
			}
			switch sortKey {
			case "ranking", "rating", "reviews":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sort must be one of ranking, rating, reviews"))
			}
			latLong := args[0]
			scan := taDogfoodScan(maxScan)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			stubs, err := taNearby(cmd.Context(), c, latLong, category, radius, radiusUnit, language)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			ids := make([]string, 0, len(stubs))
			for _, s := range stubs {
				ids = append(ids, s.LocationID)
			}
			details, failures, scanned := taFetchDetailsBounded(cmd.Context(), c, ids, language, "", scan)

			filtered := make([]taDetail, 0, len(details))
			for _, d := range details {
				if minRating > 0 && d.Rating < minRating {
					continue
				}
				filtered = append(filtered, d)
			}
			taSortDetails(filtered, sortKey)
			filtered = taLimit(filtered, top)

			view := nearbyBestView{
				LatLong:       latLong,
				Category:      category,
				MinRating:     minRating,
				Sort:          sortKey,
				ScannedCount:  scanned,
				MaxScan:       scan,
				Results:       filtered,
				FetchFailures: failures,
			}
			if len(stubs) == 0 {
				view.Note = "no locations found near that point; widen --radius or drop --category"
			} else if len(filtered) == 0 {
				view.Note = fmt.Sprintf("scanned %d nearby locations but none met --min-rating %.1f; lower the threshold or raise --max-scan", scanned, minRating)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d detail fetches failed; ranking computed over the rest\n", len(failures), scanned)
			}
			return emitTANovel(cmd, flags, view, view.Results)
		},
	}

	cmd.Flags().StringVar(&category, "category", "", "Place type: hotels, restaurants, attractions, geos")
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Drop locations rated below this (1-5)")
	cmd.Flags().IntVar(&top, "top", 5, "Number of ranked results to return")
	cmd.Flags().IntVar(&maxScan, "max-scan", 20, "Max nearby locations to fetch details for (metered API)")
	cmd.Flags().StringVar(&sortKey, "sort", "ranking", "Rank by: ranking, rating, reviews")
	cmd.Flags().StringVar(&radius, "radius", "", "Search radius from the point (use with --radius-unit)")
	cmd.Flags().StringVar(&radiusUnit, "radius-unit", "", "Unit for --radius: km, mi, m")
	cmd.Flags().StringVar(&language, "language", "en", "Language code")
	return cmd
}
