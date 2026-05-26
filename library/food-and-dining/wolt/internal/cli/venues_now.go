// Copyright 2026 amit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type venueNowRow struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Online        bool     `json:"online"`
	EstimateMin   int      `json:"estimate_min"`
	DeliveryPrice int      `json:"delivery_price"`
	Currency      string   `json:"currency,omitempty"`
	PriceRange    int      `json:"price_range"`
	Rating        float64  `json:"rating,omitempty"`
	Tags          []string `json:"tags"`
	ShortDesc     string   `json:"short_description,omitempty"`
}

type venuesNowResult struct {
	City    string        `json:"city"`
	Lat     float64       `json:"lat"`
	Lon     float64       `json:"lon"`
	Count   int           `json:"count"`
	Venues  []venueNowRow `json:"venues"`
	Filters struct {
		MaxETA   int      `json:"max_eta_min,omitempty"`
		Cuisine  []string `json:"cuisine,omitempty"`
		OnlyOpen bool     `json:"only_open"`
		Limit    int      `json:"limit,omitempty"`
	} `json:"filters"`
}

func newVenuesNowCmd(flags *rootFlags) *cobra.Command {
	var lat, lon float64
	var maxETA, limit int
	var cuisineCSV string
	var includeClosed bool
	cmd := &cobra.Command{
		Use:   "venues-now",
		Short: "List venues that are open right now and deliver within --max-eta minutes",
		Long: "Calls Wolt's nearby-restaurants endpoint, then filters in-memory by\n" +
			"open status, ETA cap, and cuisine tags. Pair with --profile to skip --lat/--lon.\n" +
			"Replaces three SPA card-clicks with one structured agent call.",
		Example: "  wolt-pp-cli venues-now --lat 60.1699 --lon 24.9384 --max-eta 25 --cuisine sushi --json",
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
			cuisines := splitCSVLowerWNow(cuisineCSV)
			res := venuesNowResult{City: page.City, Lat: lat, Lon: lon}
			res.Filters.MaxETA = maxETA
			res.Filters.Cuisine = cuisines
			res.Filters.OnlyOpen = !includeClosed
			res.Filters.Limit = limit

			// PATCH(venues-now-dedup-by-slug): Wolt's /v1/pages/restaurants
			// groups the same venue into multiple thematic sections (e.g.
			// "Featured" + a cuisine bucket). Without dedup, the same venue
			// could appear multiple times in the result.
			seen := make(map[string]bool)
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
					// PATCH(venues-now-max-eta-strict): when --max-eta is set, venues
					// with unknown ETA (EstimateMin == 0) cannot satisfy the
					// "deliver within N minutes" contract and must be excluded too.
					// Previously they were silently included.
					if maxETA > 0 && (row.EstimateMin <= 0 || row.EstimateMin > maxETA) {
						continue
					}
					if len(cuisines) > 0 && !tagMatchWNow(row.Tags, cuisines) {
						continue
					}
					seen[row.Slug] = true
					res.Venues = append(res.Venues, row)
				}
			}
			if limit > 0 && len(res.Venues) > limit {
				res.Venues = res.Venues[:limit]
			}
			res.Count = len(res.Venues)
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude")
	cmd.Flags().IntVar(&maxETA, "max-eta", 0, "Max delivery ETA in minutes (0 = no limit)")
	cmd.Flags().StringVar(&cuisineCSV, "cuisine", "", "Comma-separated cuisine tags to match (any-of)")
	cmd.Flags().BoolVar(&includeClosed, "include-closed", false, "Include venues that are not currently online")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap the number of returned venues (0 = no cap)")
	return cmd
}

func splitCSVLowerWNow(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func tagMatchWNow(haystack []string, needles []string) bool {
	for _, h := range haystack {
		hl := strings.ToLower(h)
		for _, n := range needles {
			if strings.Contains(hl, n) {
				return true
			}
		}
	}
	return false
}

func extractVenueRowWNow(it map[string]any) (venueNowRow, bool) {
	v, ok := it["venue"].(map[string]any)
	if !ok {
		v = it
	}
	slug, _ := v["slug"].(string)
	if slug == "" {
		return venueNowRow{}, false
	}
	name, _ := v["name"].(string)
	online, _ := v["online"].(bool)
	row := venueNowRow{Slug: slug, Name: name, Online: online}
	if est, ok := v["estimate"].(float64); ok {
		row.EstimateMin = int(est)
	}
	if dp, ok := v["delivery_price_int"].(float64); ok {
		row.DeliveryPrice = int(dp)
	}
	if dp, ok := v["delivery_price"].(float64); ok && row.DeliveryPrice == 0 {
		row.DeliveryPrice = int(dp)
	}
	if cur, ok := v["currency"].(string); ok {
		row.Currency = cur
	}
	if pr, ok := v["price_range"].(float64); ok {
		row.PriceRange = int(pr)
	}
	if r, ok := v["rating"].(map[string]any); ok {
		if s, ok := r["score"].(float64); ok {
			row.Rating = s
		}
	}
	if tags, ok := v["tags"].([]any); ok {
		for _, t := range tags {
			if ts, ok := t.(string); ok {
				row.Tags = append(row.Tags, ts)
			}
		}
	}
	if sd, ok := v["short_description"].(string); ok {
		row.ShortDesc = sd
	}
	return row, true
}
