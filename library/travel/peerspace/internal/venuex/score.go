// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package venuex

import (
	"sort"
	"strings"
)

// Tech checklist item keys for gap analysis.
const (
	GapWiFi            = "wifi"
	GapProjectorAV     = "projector_av"
	GapLateAccess      = "late_access"
	GapFlexibleSeating = "flexible_seating"
	GapTransit         = "transit"
)

// DefaultTechChecklist is the tech-meetup amenity set.
var DefaultTechChecklist = []string{
	GapWiFi, GapProjectorAV, GapLateAccess, GapFlexibleSeating, GapTransit,
}

// checklistKeywords maps gap keys to match substrings (lowercased).
var checklistKeywords = map[string][]string{
	GapWiFi:            {"wifi", "wi-fi", "wi_fi", "internet", "broadband"},
	GapProjectorAV:     {"projector", "av_tech", "av tech", "speakers", "screen", "sound system", "sonorisation", "vidéo", "video", "chromecast", "tv"},
	GapLateAccess:      {"late", "24h", "24/7", "overnight", "evening", "after hours", "night", "soir", "nuit"},
	GapFlexibleSeating: {"chairs", "flexible seating", "seating", "tables", "modular", "chaises", "tables"},
	GapTransit:         {"public_transit", "public transit", "transit", "metro", "subway", "bus", "train", "transport"},
}

// AmenityBlob joins description + amenities for keyword matching.
func AmenityBlob(l Listing) string {
	parts := make([]string, 0, 2+len(l.Amenities))
	parts = append(parts, l.Description, l.Title, l.SpaceType)
	parts = append(parts, l.Amenities...)
	return strings.ToLower(strings.Join(parts, " "))
}

// GapChecklist returns missing tech-event must-haves for a listing.
// checklist names "tech-meetup" (default) or a comma-separated subset of gap keys.
func GapChecklist(l Listing, checklist string) []string {
	keys := resolveChecklist(checklist)
	blob := AmenityBlob(l)
	missing := make([]string, 0)
	for _, k := range keys {
		if !hasAnyKeyword(blob, checklistKeywords[k]) {
			missing = append(missing, k)
		}
	}
	return missing
}

func resolveChecklist(checklist string) []string {
	c := strings.TrimSpace(strings.ToLower(checklist))
	if c == "" || c == "tech-meetup" || c == "tech" || c == "default" {
		return append([]string(nil), DefaultTechChecklist...)
	}
	parts := strings.Split(c, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// normalize aliases
		switch p {
		case "av", "projector", "projector/av", "projector_av":
			p = GapProjectorAV
		case "seating", "flexible":
			p = GapFlexibleSeating
		case "late", "access":
			p = GapLateAccess
		case "wi-fi", "wi_fi":
			p = GapWiFi
		}
		if _, ok := checklistKeywords[p]; ok {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), DefaultTechChecklist...)
	}
	return out
}

func hasAnyKeyword(blob string, kws []string) bool {
	for _, kw := range kws {
		if kw != "" && strings.Contains(blob, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// TechKeywordScore counts how many tech checklist items the listing covers (0-5).
func TechKeywordScore(l Listing) int {
	missing := GapChecklist(l, "tech-meetup")
	return len(DefaultTechChecklist) - len(missing)
}

// ScoreTechFit ranks a listing 0-100 for a tech event with optional constraints.
// guests/budgetMax of 0 mean "unconstrained". vibe is CSV-ish keywords to require.
func ScoreTechFit(l Listing, guests int, budgetMax float64, vibe []string) (score int, gaps []string) {
	score = 40 // base
	gaps = make([]string, 0)

	// Capacity fit (0-25)
	if guests > 0 {
		if l.Guests <= 0 {
			score -= 10
			gaps = append(gaps, "capacity_unknown")
		} else if l.Guests >= guests && l.Guests <= guests*3 {
			score += 25
		} else if l.Guests >= guests {
			score += 15 // oversized but ok
		} else {
			score -= 20
			gaps = append(gaps, "capacity_short")
		}
	} else if l.Guests > 0 {
		score += 5
	}

	// Budget fit (0-20)
	if budgetMax > 0 {
		if l.PriceHourly <= 0 {
			score -= 5
			gaps = append(gaps, "price_unknown")
		} else if l.PriceHourly <= budgetMax {
			// closer to budget still fine; under-budget bonus
			ratio := l.PriceHourly / budgetMax
			score += 10 + int((1-ratio)*10)
		} else {
			score -= 25
			gaps = append(gaps, "over_budget")
		}
	} else if l.PriceHourly > 0 {
		score += 5
	}

	// Tech amenities (0-25)
	techScore := TechKeywordScore(l)
	score += techScore * 5
	for _, g := range GapChecklist(l, "tech-meetup") {
		gaps = append(gaps, g)
	}

	// Vibe keywords (0-10)
	if len(vibe) > 0 {
		blob := AmenityBlob(l)
		hits := 0
		for _, v := range vibe {
			v = strings.TrimSpace(strings.ToLower(v))
			if v == "" {
				continue
			}
			if strings.Contains(blob, v) {
				hits++
			} else {
				gaps = append(gaps, "vibe:"+v)
			}
		}
		if len(vibe) > 0 {
			score += (hits * 10) / len(vibe)
		}
	}

	// Rating bonus (0-10)
	if l.ReviewStars >= 4.5 && l.ReviewCount >= 3 {
		score += 10
	} else if l.ReviewStars >= 4.0 {
		score += 5
	}

	if l.InstantBook {
		score += 5
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	// unique gaps
	gaps = uniqueStrings(gaps)
	return score, gaps
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// ScoredListing is a ranked recommend row.
type ScoredListing struct {
	Listing
	Score int      `json:"score"`
	Gaps  []string `json:"gaps"`
}

// RankListings scores and sorts descending by score; limit 0 means all.
func RankListings(in []Listing, guests int, budgetMax float64, vibe []string, limit int) []ScoredListing {
	out := make([]ScoredListing, 0, len(in))
	for _, l := range in {
		sc, gaps := ScoreTechFit(l, guests, budgetMax, vibe)
		out = append(out, ScoredListing{Listing: l, Score: sc, Gaps: gaps})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].PriceHourly != out[j].PriceHourly {
			// prefer cheaper when scores tie
			if out[i].PriceHourly == 0 {
				return false
			}
			if out[j].PriceHourly == 0 {
				return true
			}
			return out[i].PriceHourly < out[j].PriceHourly
		}
		return out[i].ID < out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ParseVibeCSV splits comma-separated vibe keywords.
func ParseVibeCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return make([]string, 0)
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
