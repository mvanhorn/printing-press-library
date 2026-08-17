// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package venuex

import (
	"math"
	"sort"
	"strings"
)

// Median returns the median of vals. Empty input yields 0.
func Median(vals []float64) float64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	cp := append([]float64(nil), vals...)
	sort.Float64s(cp)
	mid := n / 2
	if n%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

// Percentile returns the p-th percentile (0-100) using nearest-rank.
func Percentile(vals []float64, p float64) float64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	if p <= 0 {
		cp := append([]float64(nil), vals...)
		sort.Float64s(cp)
		return cp[0]
	}
	if p >= 100 {
		cp := append([]float64(nil), vals...)
		sort.Float64s(cp)
		return cp[n-1]
	}
	cp := append([]float64(nil), vals...)
	sort.Float64s(cp)
	// nearest-rank
	rank := int(math.Ceil((p / 100) * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return cp[rank-1]
}

// MarketPulse is a per-city aggregate.
type MarketPulse struct {
	City             string  `json:"city"`
	Count            int     `json:"count"`
	MedianPrice      float64 `json:"median_price"`
	InstantBookPct   float64 `json:"instant_book_pct"`
	CapacityP50      float64 `json:"capacity_p50"`
	MeanPrice        float64 `json:"mean_price,omitempty"`
}

// PulseByCity aggregates listings, optionally filtered to cities (case-insensitive exact or substring).
func PulseByCity(listings []Listing, cities []string, activity string) []MarketPulse {
	filtered := make([]Listing, 0, len(listings))
	for _, l := range listings {
		if !MatchActivity(l, activity) {
			continue
		}
		if len(cities) > 0 && !matchAnyCity(l, cities) {
			continue
		}
		filtered = append(filtered, l)
	}
	// group by city
	byCity := map[string][]Listing{}
	for _, l := range filtered {
		c := strings.TrimSpace(l.City)
		if c == "" {
			c = "(unknown)"
		}
		byCity[c] = append(byCity[c], l)
	}
	// if cities requested, emit empty rows for missing cities
	keys := make([]string, 0, len(byCity))
	if len(cities) > 0 {
		seen := map[string]struct{}{}
		for _, c := range cities {
			// find matching actual city key
			matched := false
			for k := range byCity {
				if strings.EqualFold(k, c) || strings.Contains(strings.ToLower(k), strings.ToLower(c)) {
					if _, ok := seen[k]; !ok {
						keys = append(keys, k)
						seen[k] = struct{}{}
					}
					matched = true
				}
			}
			if !matched {
				keys = append(keys, c)
			}
		}
	} else {
		for k := range byCity {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}
	out := make([]MarketPulse, 0, len(keys))
	for _, k := range keys {
		ls := byCity[k]
		prices := make([]float64, 0, len(ls))
		caps := make([]float64, 0, len(ls))
		ib := 0
		sum := 0.0
		for _, l := range ls {
			if l.PriceHourly > 0 {
				prices = append(prices, l.PriceHourly)
				sum += l.PriceHourly
			}
			if l.Guests > 0 {
				caps = append(caps, float64(l.Guests))
			}
			if l.InstantBook {
				ib++
			}
		}
		var mean float64
		if len(prices) > 0 {
			mean = sum / float64(len(prices))
		}
		var ibPct float64
		if len(ls) > 0 {
			ibPct = 100 * float64(ib) / float64(len(ls))
		}
		out = append(out, MarketPulse{
			City:           k,
			Count:          len(ls),
			MedianPrice:    Median(prices),
			InstantBookPct: math.Round(ibPct*10) / 10,
			CapacityP50:    Median(caps),
			MeanPrice:      math.Round(mean*100) / 100,
		})
	}
	return out
}

func matchAnyCity(l Listing, cities []string) bool {
	for _, c := range cities {
		if MatchCity(l, c) {
			return true
		}
	}
	return false
}

// NeighborhoodStat is a neighborhood rollup.
type NeighborhoodStat struct {
	Neighborhood string  `json:"neighborhood"`
	City         string  `json:"city,omitempty"`
	Count        int     `json:"count"`
	MedianPrice  float64 `json:"median_price"`
	TechScoreAvg float64 `json:"tech_score_avg,omitempty"`
	SampleIDs    []string `json:"sample_ids,omitempty"`
}

// Neighborhoods groups by neighborhood (+ optional tech vibe score).
func Neighborhoods(listings []Listing, city, activity, vibe string) []NeighborhoodStat {
	type acc struct {
		city   string
		count  int
		prices []float64
		tech   []float64
		ids    []string
	}
	wantTech := strings.EqualFold(strings.TrimSpace(vibe), "tech") ||
		strings.Contains(strings.ToLower(vibe), "tech")
	by := map[string]*acc{}
	for _, l := range listings {
		if !MatchCity(l, city) || !MatchActivity(l, activity) {
			continue
		}
		n := strings.TrimSpace(l.Neighborhood)
		if n == "" {
			n = "(unknown)"
		}
		a := by[n]
		if a == nil {
			a = &acc{city: l.City, prices: make([]float64, 0), tech: make([]float64, 0), ids: make([]string, 0)}
			by[n] = a
		}
		a.count++
		if l.PriceHourly > 0 {
			a.prices = append(a.prices, l.PriceHourly)
		}
		if wantTech {
			a.tech = append(a.tech, float64(TechKeywordScore(l)))
		}
		if len(a.ids) < 5 && l.ID != "" {
			a.ids = append(a.ids, l.ID)
		}
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]NeighborhoodStat, 0, len(keys))
	for _, k := range keys {
		a := by[k]
		st := NeighborhoodStat{
			Neighborhood: k,
			City:         a.city,
			Count:        a.count,
			MedianPrice:  Median(a.prices),
			SampleIDs:    a.ids,
		}
		if len(a.tech) > 0 {
			sum := 0.0
			for _, t := range a.tech {
				sum += t
			}
			st.TechScoreAvg = math.Round((sum/float64(len(a.tech)))*100) / 100
		}
		out = append(out, st)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Neighborhood < out[j].Neighborhood
	})
	return out
}

// Similar finds mechanical neighbors of seed by price/capacity within pct, same-ish city/neighborhood preferred.
func Similar(seed Listing, all []Listing, withinPct float64) []Listing {
	if withinPct <= 0 {
		withinPct = 20
	}
	out := make([]Listing, 0)
	for _, l := range all {
		if l.ID != "" && seed.ID != "" && l.ID == seed.ID {
			continue
		}
		if seed.City != "" && l.City != "" && !strings.EqualFold(seed.City, l.City) {
			// soft: still allow same metro substring
			if !strings.Contains(strings.ToLower(l.City), strings.ToLower(seed.City)) &&
				!strings.Contains(strings.ToLower(seed.City), strings.ToLower(l.City)) {
				continue
			}
		}
		if seed.PriceHourly > 0 && l.PriceHourly > 0 {
			diff := math.Abs(l.PriceHourly-seed.PriceHourly) / seed.PriceHourly * 100
			if diff > withinPct {
				continue
			}
		}
		if seed.Guests > 0 && l.Guests > 0 {
			diff := math.Abs(float64(l.Guests-seed.Guests)) / float64(seed.Guests) * 100
			if diff > withinPct {
				continue
			}
		}
		out = append(out, l)
	}
	// prefer same neighborhood
	sort.SliceStable(out, func(i, j int) bool {
		si := scoreSimilar(seed, out[i])
		sj := scoreSimilar(seed, out[j])
		if si != sj {
			return si > sj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func scoreSimilar(seed, l Listing) int {
	s := 0
	if seed.Neighborhood != "" && strings.EqualFold(seed.Neighborhood, l.Neighborhood) {
		s += 3
	}
	if seed.SpaceType != "" && strings.EqualFold(seed.SpaceType, l.SpaceType) {
		s += 2
	}
	if seed.InstantBook == l.InstantBook {
		s++
	}
	return s
}

// MultiCityTop picks top N per city under guests/budget constraints.
func MultiCityTop(listings []Listing, cities []string, activity string, guests int, budgetMax float64, top int) map[string][]ScoredListing {
	if top <= 0 {
		top = 3
	}
	result := make(map[string][]ScoredListing, len(cities))
	for _, city := range cities {
		filtered := make([]Listing, 0)
		for _, l := range listings {
			if !MatchCity(l, city) || !MatchActivity(l, activity) {
				continue
			}
			if guests > 0 && l.Guests > 0 && l.Guests < guests {
				continue
			}
			if budgetMax > 0 && l.PriceHourly > 0 && l.PriceHourly > budgetMax {
				continue
			}
			filtered = append(filtered, l)
		}
		result[city] = RankListings(filtered, guests, budgetMax, nil, top)
	}
	return result
}

// SnapshotAttrs captures comparable favorite attributes for drift.
type SnapshotAttrs struct {
	ID          string  `json:"id"`
	PriceHourly float64 `json:"price_hourly"`
	Guests      int     `json:"guests"`
	InstantBook bool    `json:"instant_book"`
	Title       string  `json:"title,omitempty"`
	City        string  `json:"city,omitempty"`
}

// AttrsFromListing projects a listing into snapshot attributes.
func AttrsFromListing(l Listing) SnapshotAttrs {
	return SnapshotAttrs{
		ID:          l.ID,
		PriceHourly: l.PriceHourly,
		Guests:      l.Guests,
		InstantBook: l.InstantBook,
		Title:       l.Title,
		City:        l.City,
	}
}

// DriftChange describes one attribute change.
type DriftChange struct {
	ID     string `json:"id"`
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
	Title  string `json:"title,omitempty"`
}

// DiffAttrs compares prior vs current snapshot attribute maps keyed by id.
func DiffAttrs(prior, current map[string]SnapshotAttrs) []DriftChange {
	out := make([]DriftChange, 0)
	for id, cur := range current {
		old, ok := prior[id]
		if !ok {
			continue
		}
		if old.PriceHourly != cur.PriceHourly {
			out = append(out, DriftChange{ID: id, Field: "price_hourly", Before: old.PriceHourly, After: cur.PriceHourly, Title: cur.Title})
		}
		if old.Guests != cur.Guests {
			out = append(out, DriftChange{ID: id, Field: "guests", Before: old.Guests, After: cur.Guests, Title: cur.Title})
		}
		if old.InstantBook != cur.InstantBook {
			out = append(out, DriftChange{ID: id, Field: "instant_book", Before: old.InstantBook, After: cur.InstantBook, Title: cur.Title})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Field < out[j].Field
	})
	return out
}

// DeltaResult is membership change between favorite id sets.
type DeltaResult struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Kept    []string `json:"kept"`
}

// DeltaIDs computes set difference between previous and current favorite ids.
func DeltaIDs(previous, current []string) DeltaResult {
	prevSet := map[string]struct{}{}
	for _, id := range previous {
		if id != "" {
			prevSet[id] = struct{}{}
		}
	}
	curSet := map[string]struct{}{}
	for _, id := range current {
		if id != "" {
			curSet[id] = struct{}{}
		}
	}
	res := DeltaResult{
		Added:   make([]string, 0),
		Removed: make([]string, 0),
		Kept:    make([]string, 0),
	}
	for id := range curSet {
		if _, ok := prevSet[id]; ok {
			res.Kept = append(res.Kept, id)
		} else {
			res.Added = append(res.Added, id)
		}
	}
	for id := range prevSet {
		if _, ok := curSet[id]; !ok {
			res.Removed = append(res.Removed, id)
		}
	}
	sort.Strings(res.Added)
	sort.Strings(res.Removed)
	sort.Strings(res.Kept)
	return res
}

// ExportMarkdown renders a shortlist as a markdown block for Luma/Eventbrite/Slack.
func ExportMarkdown(listings []Listing) string {
	var b strings.Builder
	b.WriteString("## Venue shortlist\n\n")
	if len(listings) == 0 {
		b.WriteString("_No venues in shortlist._\n")
		return b.String()
	}
	for i, l := range listings {
		title := l.Title
		if title == "" {
			title = l.ID
		}
		b.WriteString("### ")
		b.WriteString(title)
		b.WriteString("\n")
		if l.ID != "" {
			b.WriteString("- **ID:** `")
			b.WriteString(l.ID)
			b.WriteString("`\n")
		}
		if l.City != "" {
			b.WriteString("- **Location:** ")
			b.WriteString(l.City)
			if l.Neighborhood != "" {
				b.WriteString(" / ")
				b.WriteString(l.Neighborhood)
			}
			b.WriteString("\n")
		}
		if l.PriceHourly > 0 {
			b.WriteString("- **Price:** ")
			b.WriteString(formatMoney(l.PriceHourly))
			b.WriteString("/hr\n")
		}
		if l.Guests > 0 {
			b.WriteString("- **Capacity:** ")
			b.WriteString(itoa(l.Guests))
			b.WriteString(" guests\n")
		}
		if l.SpaceType != "" {
			b.WriteString("- **Space type:** ")
			b.WriteString(l.SpaceType)
			b.WriteString("\n")
		}
		if l.FormatFit != "" {
			b.WriteString("- **Format fit:** ")
			b.WriteString(l.FormatFit)
			b.WriteString("\n")
		}
		if len(l.Amenities) > 0 {
			max := 8
			if len(l.Amenities) < max {
				max = len(l.Amenities)
			}
			b.WriteString("- **Amenities:** ")
			b.WriteString(strings.Join(l.Amenities[:max], ", "))
			b.WriteString("\n")
		}
		gaps := GapChecklist(l, "tech-meetup")
		if len(gaps) > 0 {
			b.WriteString("- **Gaps:** ")
			b.WriteString(strings.Join(gaps, ", "))
			b.WriteString("\n")
		} else {
			b.WriteString("- **Fit notes:** covers tech-meetup checklist\n")
		}
		if snippet := truncateRunes(firstNonEmpty(l.About, l.Description), 280); snippet != "" {
			b.WriteString("- **About:** ")
			b.WriteString(snippet)
			b.WriteString("\n")
		}
		if snippet := truncateRunes(l.Included, 200); snippet != "" {
			b.WriteString("- **Included:** ")
			b.WriteString(snippet)
			b.WriteString("\n")
		}
		if snippet := truncateRunes(l.Rules, 200); snippet != "" {
			b.WriteString("- **Host rules:** ")
			b.WriteString(snippet)
			b.WriteString("\n")
		}
		if snippet := truncateRunes(l.Parking, 160); snippet != "" {
			b.WriteString("- **Parking:** ")
			b.WriteString(snippet)
			b.WriteString("\n")
		}
		if i < len(listings)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func formatMoney(v float64) string {
	if v == math.Trunc(v) {
		return itoa(int(v))
	}
	return strings.TrimRight(strings.TrimRight(
		// two decimal places without fmt to keep deps light
		sprintf2(v), "0"), ".")
}

func sprintf2(v float64) string {
	// simple fixed 2dp
	neg := v < 0
	if neg {
		v = -v
	}
	i := int(math.Floor(v + 0.005))
	frac := int(math.Round((v - math.Floor(v)) * 100))
	if frac == 100 {
		i++
		frac = 0
	}
	s := itoa(i) + "." + pad2(frac)
	if neg {
		return "-" + s
	}
	return s
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
