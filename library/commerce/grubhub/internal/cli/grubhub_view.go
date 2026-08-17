// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
)

// restaurantRow is the flattened, agent-friendly view of a Grubhub search card
// shared by the `near` and `compare` commands.
type restaurantRow struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Cuisines         []string `json:"cuisines"`
	DeliveryFee      string   `json:"delivery_fee"`
	DeliveryFeeCents int      `json:"delivery_fee_cents"`
	Minimum          string   `json:"minimum"`
	MinimumCents     int      `json:"minimum_cents"`
	ETAMinutes       int      `json:"eta_minutes"`
	Rating           float64  `json:"rating"`
	DistanceMiles    float64  `json:"distance_miles"`
	PriceLevel       int      `json:"price_level"`
	Open             bool     `json:"open"`
	Deals            int      `json:"deals"`
}

func cardToRow(c grubhub.Card) restaurantRow {
	return restaurantRow{
		ID:               c.ID,
		Name:             c.Name,
		Cuisines:         c.Cuisines,
		DeliveryFee:      grubhub.Dollars(c.DeliveryFee.Price),
		DeliveryFeeCents: c.DeliveryFee.Price,
		Minimum:          grubhub.Dollars(c.DeliveryMinimum.Price),
		MinimumCents:     c.DeliveryMinimum.Price,
		ETAMinutes:       c.DeliveryTime,
		Rating:           c.Rating(),
		DistanceMiles:    c.DistanceMiles(),
		PriceLevel:       int(c.PriceRating),
		Open:             c.Open,
		Deals:            c.DealCount(),
	}
}

// cardsToRows converts and optionally filters cards by cuisine and open-now.
func cardsToRows(cards []grubhub.Card, cuisine string, openOnly bool) []restaurantRow {
	cuisine = strings.ToLower(strings.TrimSpace(cuisine))
	rows := make([]restaurantRow, 0, len(cards))
	for _, c := range cards {
		if openOnly && !c.Open {
			continue
		}
		if cuisine != "" && !matchesCuisine(c.Cuisines, cuisine) {
			continue
		}
		rows = append(rows, cardToRow(c))
	}
	return rows
}

func matchesCuisine(cuisines []string, want string) bool {
	for _, cz := range cuisines {
		if strings.Contains(strings.ToLower(cz), want) {
			return true
		}
	}
	return false
}

// sortRows orders rows by a named key. Unknown keys leave server order intact.
func sortRows(rows []restaurantRow, key string) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "fee":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].DeliveryFeeCents < rows[j].DeliveryFeeCents })
	case "min", "minimum":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].MinimumCents < rows[j].MinimumCents })
	case "eta", "time":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].ETAMinutes < rows[j].ETAMinutes })
	case "rating":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Rating > rows[j].Rating })
	case "distance", "dist":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].DistanceMiles < rows[j].DistanceMiles })
	case "deals":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Deals > rows[j].Deals })
	case "name":
		sort.SliceStable(rows, func(i, j int) bool { return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name) })
	}
}
