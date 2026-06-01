package offerup

import "sort"

// PriceStats summarizes the asking-price distribution for a set of listings.
type PriceStats struct {
	Query       string  `json:"query"`
	Location    string  `json:"location,omitempty"`
	Count       int     `json:"count"`
	Min         float64 `json:"min"`
	P25         float64 `json:"p25"`
	Median      float64 `json:"median"`
	P75         float64 `json:"p75"`
	Max         float64 `json:"max"`
	Mean        float64 `json:"mean"`
	FirmCount   int     `json:"firmCount"`
	FirmPercent float64 `json:"firmPercent"`
}

// ComputePriceStats derives the price distribution over listings, ignoring
// zero/free-priced rows (they distort the going rate). Count reflects the
// priced subset.
func ComputePriceStats(query, location string, listings []StoredListing) PriceStats {
	prices := make([]float64, 0, len(listings))
	firm := 0
	for _, l := range listings {
		if l.Price <= 0 {
			continue
		}
		prices = append(prices, l.Price)
		if l.IsFirmPrice {
			firm++
		}
	}
	stats := PriceStats{Query: query, Location: location, Count: len(prices), FirmCount: firm}
	if len(prices) == 0 {
		return stats
	}
	sort.Float64s(prices)
	var sum float64
	for _, p := range prices {
		sum += p
	}
	stats.Min = prices[0]
	stats.Max = prices[len(prices)-1]
	stats.P25 = percentile(prices, 0.25)
	stats.Median = percentile(prices, 0.5)
	stats.P75 = percentile(prices, 0.75)
	stats.Mean = round1(sum / float64(len(prices)))
	stats.FirmPercent = round1(float64(firm) / float64(len(prices)) * 100)
	return stats
}

// Median returns the median of the listings' positive prices, or 0 when none.
func Median(listings []StoredListing) float64 {
	prices := make([]float64, 0, len(listings))
	for _, l := range listings {
		if l.Price > 0 {
			prices = append(prices, l.Price)
		}
	}
	if len(prices) == 0 {
		return 0
	}
	sort.Float64s(prices)
	return percentile(prices, 0.5)
}

// percentile returns the p-quantile (0..1) of a sorted slice via linear
// interpolation between closest ranks.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := p * float64(n-1)
	lo := int(rank)
	frac := rank - float64(lo)
	if lo+1 >= n {
		return sorted[n-1]
	}
	return round1(sorted[lo] + frac*(sorted[lo+1]-sorted[lo]))
}
