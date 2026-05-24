// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
	"github.com/spf13/cobra"
)

type compareHotel struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Country     string   `json:"country"`
	Price       float64  `json:"price"`
	Currency    string   `json:"currency"`
	Score       float64  `json:"score"`
	Latitude    float64  `json:"latitude,omitempty"`
	Longitude   float64  `json:"longitude,omitempty"`
	Amenities   []string `json:"amenities"`
	HighReviews int      `json:"high_reviews"`
	LowReviews  int      `json:"low_reviews"`
}

type compareResult struct {
	Hotel1             compareHotel   `json:"hotel1"`
	Hotel2             compareHotel   `json:"hotel2"`
	AmenityDelta       []string       `json:"amenity_delta"`
	DistanceDelta      float64        `json:"distance_delta"`
	PriceDelta         float64        `json:"price_delta"`
	ScoreDelta         float64        `json:"score_delta"`
	RecentReviewCounts map[string]int `json:"recent_review_counts"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout, country1, country2 string
	cmd := &cobra.Command{
		Use:         "compare <slug1> <slug2>",
		Short:       "Compare two hotels by detail and recent reviews",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if flags.dryRun {
				return flags.printJSON(cmd, compareResult{RecentReviewCounts: map[string]int{}})
			}
			if checkin == "" || checkout == "" {
				return cmd.Help()
			}
			in, err := time.Parse(dateOnly, checkin)
			if err != nil {
				return fmt.Errorf("compare: %w", err)
			}
			outDate, err := time.Parse(dateOnly, checkout)
			if err != nil {
				return fmt.Errorf("compare: %w", err)
			}
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("compare: %w", err)
			}
			type res struct {
				hotel compareHotel
				err   error
			}
			results := make([]res, 2)
			var wg sync.WaitGroup
			for i, item := range []struct{ slug, country string }{{args[0], country1}, {args[1], country2}} {
				wg.Add(1)
				go func(i int, slug, country string) {
					defer wg.Done()
					results[i].hotel, results[i].err = fetchCompareHotel(c, slug, country, in, outDate)
				}(i, item.slug, item.country)
			}
			wg.Wait()
			for _, r := range results {
				if r.err != nil {
					return fmt.Errorf("compare: %w", r.err)
				}
			}
			h1, h2 := results[0].hotel, results[1].hotel
			out := compareResult{Hotel1: h1, Hotel2: h2, AmenityDelta: amenityDelta(h1.Amenities, h2.Amenities), DistanceDelta: hotelDistanceKM(h1, h2), PriceDelta: h1.Price - h2.Price, ScoreDelta: h1.Score - h2.Score, RecentReviewCounts: map[string]int{"hotel1_high": h1.HighReviews, "hotel1_low": h1.LowReviews, "hotel2_high": h2.HighReviews, "hotel2_low": h2.LowReviews}}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date YYYY-MM-DD")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date YYYY-MM-DD")
	cmd.Flags().StringVar(&country1, "country1", "fr", "First hotel country")
	cmd.Flags().StringVar(&country2, "country2", "fr", "Second hotel country")
	return cmd
}

type getter interface {
	Get(string, map[string]string) (json.RawMessage, error)
}

func fetchCompareHotel(c getter, slug, country string, checkin, checkout time.Time) (compareHotel, error) {
	data, err := c.Get(hotelPath(country, slug), hotelParams(checkin, checkout, 2))
	if err != nil {
		return compareHotel{}, err
	}
	prop, err := parseHotel(data)
	if err != nil {
		return compareHotel{}, err
	}
	price, currency := hotelPrice(prop)
	reviews, _ := c.Get("/reviewlist.html", map[string]string{"pagename": slug, "cc1": country, "rows": "25"})
	parsed, _ := booking.ParseReviewList(reviews)
	items := make([]booking.Review, 0)
	_ = json.Unmarshal(parsed, &items)
	h := compareHotel{Name: prop.Name, Slug: slug, Country: country, Price: price, Currency: currency, Score: prop.ReviewScore, Latitude: prop.Latitude, Longitude: prop.Longitude, Amenities: prop.Facilities}
	for _, r := range items {
		if r.Score > 8 {
			h.HighReviews++
		}
		if r.Score < 6 && r.Score > 0 {
			h.LowReviews++
		}
	}
	return h, nil
}

func amenityDelta(a, b []string) []string {
	seen := map[string]int{}
	for _, v := range a {
		seen[v]++
	}
	for _, v := range b {
		seen[v]--
	}
	out := make([]string, 0)
	for k, v := range seen {
		if v != 0 {
			out = append(out, k)
		}
	}
	return out
}

func hotelDistanceKM(a, b compareHotel) float64 {
	if missingCoordinates(a) || missingCoordinates(b) {
		return 0
	}
	const earthRadiusKM = 6371.0
	lat1 := degreesToRadians(a.Latitude)
	lat2 := degreesToRadians(b.Latitude)
	dLat := degreesToRadians(b.Latitude - a.Latitude)
	dLon := degreesToRadians(b.Longitude - a.Longitude)
	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)
	h := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLon*sinLon
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

func missingCoordinates(h compareHotel) bool {
	return h.Latitude == 0 && h.Longitude == 0
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}
