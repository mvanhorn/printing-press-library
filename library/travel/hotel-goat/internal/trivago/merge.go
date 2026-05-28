// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

package trivago

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/parser"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var priceRE = regexp.MustCompile(`\d[\d.,]*`)

// ParsePrice extracts the numeric value from a localized price string like
// "$199", "$1,063", "€189,50", or "USD 1,063.50". Returns 0 if no digit
// is present. Handles both US ("1,063.50" = thousands sep + decimal) and
// European ("1.063,50" = thousands sep + decimal) conventions: when both
// `,` and `.` appear the rightmost is the decimal separator; when only
// one appears, 3 trailing digits => thousands separator, 1-2 => decimal.
func ParsePrice(s string) float64 {
	if s == "" {
		return 0
	}
	m := priceRE.FindString(s)
	if m == "" {
		return 0
	}
	hasDot := strings.Contains(m, ".")
	hasComma := strings.Contains(m, ",")
	switch {
	case hasDot && hasComma:
		if strings.LastIndex(m, ",") > strings.LastIndex(m, ".") {
			m = strings.ReplaceAll(m, ".", "")
			m = strings.Replace(m, ",", ".", 1)
		} else {
			m = strings.ReplaceAll(m, ",", "")
		}
	case hasComma:
		idx := strings.LastIndex(m, ",")
		if len(m)-idx-1 == 3 {
			m = strings.ReplaceAll(m, ",", "")
		} else {
			m = strings.Replace(m, ",", ".", 1)
		}
	case hasDot:
		idx := strings.LastIndex(m, ".")
		if len(m)-idx-1 == 3 && strings.Count(m, ".") == 1 {
			m = strings.ReplaceAll(m, ".", "")
		}
	}
	f, _ := strconv.ParseFloat(m, 64)
	return f
}

// Merge folds Trivago results into the Google-derived hotel list. A
// Trivago row matches a Google hotel when they share lat/lng within
// ~250m AND their names overlap by >=0.5 (token-set Jaccard, minus
// stopwords). Matched rows append a `prices` entry on the Google hotel
// so the agent sees both sources in one envelope. Unmatched Trivago
// rows are appended as standalone hotels with PropertyToken
// "trivago:<id>", giving the agent a wider candidate set without
// requiring Google to have indexed every property.
//
// targetCurrency: when non-empty AND different from Trivago's native
// currency (typically EUR), each Trivago price is converted via the
// Frankfurter FX endpoint so the agent sees apples-to-apples numbers.
// The original native amount is preserved in the source label, e.g.
// "trivago/Agoda [EUR €802 ~= USD]". If FX lookup fails, the native
// amount + a "[EUR]" tag fall through unchanged.
func Merge(ctx context.Context, google []parser.Hotel, triv []Accommodation, targetCurrency string) []parser.Hotel {
	if len(triv) == 0 {
		return google
	}
	targetCurrency = strings.ToUpper(strings.TrimSpace(targetCurrency))
	used := make([]bool, len(triv))
	for i := range google {
		h := &google[i]
		bestIdx, bestScore := -1, 0.0
		for j, t := range triv {
			if used[j] {
				continue
			}
			if h.Latitude == 0 || t.Latitude == 0 {
				continue
			}
			if haversineMeters(h.Latitude, h.Longitude, t.Latitude, t.Longitude) > 250 {
				continue
			}
			score := nameOverlap(h.Name, t.Name)
			if score < 0.5 {
				continue
			}
			if score > bestScore {
				bestScore = score
				bestIdx = j
			}
		}
		if bestIdx < 0 {
			continue
		}
		used[bestIdx] = true
		t := triv[bestIdx]
		price := ParsePrice(t.PricePerNight)
		source := "trivago"
		if t.Advertiser != "" {
			source = "trivago/" + t.Advertiser
		}
		effectiveTarget := targetCurrency
		if effectiveTarget == "" {
			effectiveTarget = h.Currency
		}
		displayPrice := price
		displayCurrency := t.Currency
		if effectiveTarget != "" && t.Currency != "" && effectiveTarget != t.Currency && price > 0 {
			if conv, _, ok := Convert(ctx, price, t.Currency, effectiveTarget); ok {
				source = fmt.Sprintf("%s [%s %.0f -> %s]", source, t.Currency, price, effectiveTarget)
				displayPrice = conv
				displayCurrency = effectiveTarget
			} else {
				source = source + " [" + t.Currency + "]"
			}
		}
		h.Prices = append(h.Prices, parser.OTAPrice{
			Source: source,
			Price:  displayPrice,
			Link:   t.BookingURL,
		})
		// Promote Trivago to the headline price when (a) currencies match
		// (or were converted) AND (b) the converted Trivago figure beats
		// Google's. Skip when conversion failed — comparing native EUR
		// against a USD headline is meaningless.
		canCompare := displayCurrency == h.Currency || h.Currency == ""
		if displayPrice > 0 && canCompare &&
			(h.PricePerNight == 0 || displayPrice < h.PricePerNight) {
			h.PricePerNight = displayPrice
			if h.Currency == "" {
				h.Currency = displayCurrency
			}
		}
		if h.BookingURLs.Primary == "" && t.BookingURL != "" {
			h.BookingURLs.Primary = t.BookingURL
		}
	}
	for j, t := range triv {
		if used[j] {
			continue
		}
		google = append(google, accommodationToHotel(ctx, t, targetCurrency))
	}
	return google
}

func accommodationToHotel(ctx context.Context, t Accommodation, targetCurrency string) parser.Hotel {
	price := ParsePrice(t.PricePerNight)
	rating, _ := strconv.ParseFloat(strings.TrimSpace(t.ReviewRating), 64)
	source := "trivago"
	if t.Advertiser != "" {
		source = "trivago/" + t.Advertiser
	}
	currency := t.Currency
	displayPrice := price
	if targetCurrency != "" && t.Currency != "" && targetCurrency != t.Currency && price > 0 {
		if conv, _, ok := Convert(ctx, price, t.Currency, targetCurrency); ok {
			source = fmt.Sprintf("%s [%s %.0f -> %s]", source, t.Currency, price, targetCurrency)
			displayPrice = conv
			currency = targetCurrency
		} else {
			source = source + " [" + t.Currency + "]"
		}
	}
	var imgs []string
	if t.Image != "" {
		imgs = []string{t.Image}
	}
	return parser.Hotel{
		PropertyToken: "trivago:" + t.ID,
		Name:          t.Name,
		Address:       t.Address,
		Latitude:      t.Latitude,
		Longitude:     t.Longitude,
		HotelClass:    t.HotelRating,
		Rating:        rating,
		Reviews:       t.ReviewCount,
		PricePerNight: displayPrice,
		Currency:      currency,
		Amenities:     splitCSV(t.Amenities),
		Prices: []parser.OTAPrice{{
			Source: source,
			Price:  displayPrice,
			Link:   t.BookingURL,
		}},
		BookingURLs: parser.BookingURLs{Primary: t.BookingURL, HotelURL: t.URL},
		Images:      imgs,
		Description: t.Description,
		Thumbnail:   t.Image,
	}
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	rad := math.Pi / 180.0
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return r * c
}

var (
	stopwords  = map[string]bool{"the": true, "hotel": true, "hotels": true, "and": true, "by": true, "at": true, "in": true, "of": true, "a": true, "an": true}
	tokenSplit = regexp.MustCompile(`[^a-z0-9]+`)
)

// nameOverlap returns the token-set overlap ratio using the LARGER set as
// the denominator (standard Jaccard-ish, not min-denominator). Min would
// score a single-token chain name like "Marriott" against any longer
// "Marriott …" property as 1.0, then the 250m radius gate would silently
// merge two distinct properties in dense city blocks. Max denominator
// keeps legitimate matches strong ("Park Hyatt Paris" vs "Park Hyatt
// Paris Vendôme" = 3/4 = 0.75) while killing the single-token false
// positive ("Marriott" vs "JW Marriott Bonvoy" = 1/3 = 0.33).
//
// We also require >=2 informative tokens on the shorter side: a Google
// row reduced to a single brand token after stopword stripping never
// auto-merges, since chain-only names carry no per-property signal.
//
// Stopwords ("hotel", "the", "by", ...) are stripped so they don't
// inflate matches between unrelated properties.
func nameOverlap(a, b string) float64 {
	ta := tokenSet(a)
	tb := tokenSet(b)
	short := len(ta)
	if len(tb) < short {
		short = len(tb)
	}
	if short < 2 {
		return 0
	}
	common := 0
	for t := range ta {
		if tb[t] {
			common++
		}
	}
	base := len(ta)
	if len(tb) > base {
		base = len(tb)
	}
	return float64(common) / float64(base)
}

func tokenSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, t := range tokenSplit.Split(strings.ToLower(s), -1) {
		if t == "" || stopwords[t] {
			continue
		}
		out[t] = true
	}
	return out
}
