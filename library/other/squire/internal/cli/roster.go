// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored transcendence command (Phase 3). Safe to edit.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type rosterRow struct {
	ShopID      string  `json:"shop_id"`
	Name        string  `json:"name"`
	Route       string  `json:"route"`
	Rating      float64 `json:"rating"`
	NumRatings  int     `json:"num_ratings"`
	Score       float64 `json:"score"`
	BarberCount int     `json:"barber_count"`
}

type rosterResult struct {
	CityID string      `json:"city_id"`
	Ranked []rosterRow `json:"ranked"`
	Note   string      `json:"note,omitempty"`
}

// readRating pulls a rating + count from a city-discovery entity, tolerating the
// several field shapes Squire uses across surfaces.
func readRating(e map[string]any) (float64, int) {
	rating := sqFloat(e, "averageRating")
	if rating == 0 {
		rating = sqFloat(e, "yelpRating")
	}
	count := sqInt(e, "numberOfRatings")
	if count == 0 {
		count = sqInt(e, "reviewCount")
	}
	return rating, count
}

func newNovelRosterCmd(flags *rootFlags) *cobra.Command {
	var flagCityID string
	var flagLat string
	var flagLon string
	var flagMinReviews int
	var flagLimit int
	var flagPages int
	var flagNoEnrich bool

	cmd := &cobra.Command{
		Use:   "roster",
		Short: "Rank the best shops in a city by rating weighted by review volume, with Squire's AI review summary attached.",
		Example: strings.Trim(`
  squire-pp-cli roster --city-id <id> --lat 43.65 --lon -79.38 --limit 10
  squire-pp-cli roster --city-id <id> --lat 21.30 --lon -157.85 --min-reviews 25 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank city shops by rating confidence")
				return nil
			}
			if flagCityID == "" || flagLat == "" || flagLon == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--city-id, --lat and --lon are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows := make([]rosterRow, 0)
			anyRating := false
			for page := 1; page <= flagPages; page++ {
				ents, err := fetchCityShops(ctx, c, flagCityID, flagLat, flagLon, page)
				if err != nil {
					if page == 1 {
						return apiErr(fmt.Errorf("city discovery: %w", err))
					}
					break
				}
				if len(ents) == 0 {
					break
				}
				for _, e := range ents {
					rating, count := readRating(e)
					id := sqString(e, "id")
					// The city-discovery payload often omits ratings; enrich from
					// the per-shop reviews endpoint (entity.id is the shop UUID)
					// unless the caller opts out.
					if !flagNoEnrich && id != "" && rating == 0 {
						if a, n, _, err := fetchReviewMeta(ctx, c, id); err == nil {
							rating, count = a, n
						}
					}
					if count < flagMinReviews {
						continue
					}
					if rating > 0 {
						anyRating = true
					}
					rows = append(rows, rosterRow{
						ShopID:      id,
						Name:        sqString(e, "name"),
						Route:       sqString(e, "route"),
						Rating:      rating,
						NumRatings:  count,
						Score:       rosterScore(rating, count),
						BarberCount: sqInt(e, "barberCount"),
					})
				}
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].Score > rows[j].Score })
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}
			res := rosterResult{CityID: flagCityID, Ranked: rows}
			if !anyRating {
				res.Note = "city discovery did not expose per-shop ratings; shops are listed in API order with score 0"
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&flagCityID, "city-id", "", "City UUID to rank shops within")
	cmd.Flags().StringVar(&flagLat, "lat", "", "Latitude for city discovery")
	cmd.Flags().StringVar(&flagLon, "lon", "", "Longitude for city discovery")
	cmd.Flags().IntVar(&flagMinReviews, "min-reviews", 0, "Exclude shops with fewer than this many ratings")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum shops to return")
	cmd.Flags().IntVar(&flagPages, "pages", 1, "City discovery pages to scan")
	cmd.Flags().BoolVar(&flagNoEnrich, "no-enrich", false, "Skip per-shop review lookups (faster, ranks only on ratings the city payload exposes)")
	return cmd
}
