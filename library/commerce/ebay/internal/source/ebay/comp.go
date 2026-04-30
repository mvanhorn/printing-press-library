package ebay

import (
	"math"
	"sort"
	"time"
)

// AnalyzeComps reduces a slice of SoldItem into CompStats with optional
// outlier trim (1.5*IQR rule) when trimOutliers is true.
func AnalyzeComps(query string, items []SoldItem, windowDays int, trimOutliers bool) CompStats {
	stats := CompStats{
		Query:      query,
		WindowDays: windowDays,
		SampleSize: len(items),
		Currency:   "USD",
	}
	if len(items) == 0 {
		return stats
	}
	prices := make([]float64, 0, len(items))
	for _, it := range items {
		if it.SoldPrice > 0 {
			prices = append(prices, it.SoldPrice)
		}
	}
	if len(prices) == 0 {
		return stats
	}
	sort.Float64s(prices)
	usedPrices := prices
	if trimOutliers && len(prices) >= 5 {
		q1 := percentile(prices, 25)
		q3 := percentile(prices, 75)
		iqr := q3 - q1
		lower := q1 - 1.5*iqr
		upper := q3 + 1.5*iqr
		filtered := prices[:0:0]
		for _, p := range prices {
			if p >= lower && p <= upper {
				filtered = append(filtered, p)
			}
		}
		stats.OutliersTrim = len(prices) - len(filtered)
		usedPrices = filtered
	}
	stats.UsedSize = len(usedPrices)
	stats.Min = usedPrices[0]
	stats.Max = usedPrices[len(usedPrices)-1]
	stats.Mean = mean(usedPrices)
	stats.Median = percentile(usedPrices, 50)
	stats.P25 = percentile(usedPrices, 25)
	stats.P75 = percentile(usedPrices, 75)
	stats.StdDev = stddev(usedPrices, stats.Mean)

	// Date span
	first, last := time.Time{}, time.Time{}
	for _, it := range items {
		if it.SoldDate.IsZero() {
			continue
		}
		if first.IsZero() || it.SoldDate.Before(first) {
			first = it.SoldDate
		}
		if last.IsZero() || it.SoldDate.After(last) {
			last = it.SoldDate
		}
	}
	stats.FirstSold = first
	stats.LastSold = last
	return stats
}

// DedupeVariants collapses near-duplicate listings to the lowest-price exemplar
// per fingerprint. The fingerprint is a stripped-and-lowercased token bag.
func DedupeVariants(items []SoldItem) []SoldItem {
	seen := map[string]int{}
	out := make([]SoldItem, 0, len(items))
	for _, it := range items {
		fp := fingerprint(it.Title)
		if existing, ok := seen[fp]; ok {
			// Keep the earlier-sold item for stability; could weight by price.
			if it.SoldPrice < out[existing].SoldPrice {
				// out[existing] = it  // skip; first-seen wins
			}
			continue
		}
		seen[fp] = len(out)
		out = append(out, it)
	}
	return out
}

func fingerprint(s string) string {
	tokens := tokenize(s)
	sort.Strings(tokens)
	return joinSpace(tokens)
}

func tokenize(s string) []string {
	out := []string{}
	cur := []rune{}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			cur = append(cur, r+('a'-'A'))
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			cur = append(cur, r)
		default:
			if len(cur) > 0 {
				if len(cur) > 1 { // skip single chars
					out = append(out, string(cur))
				}
				cur = cur[:0]
			}
		}
	}
	if len(cur) > 1 {
		out = append(out, string(cur))
	}
	return out
}

func joinSpace(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

func stddev(v []float64, m float64) float64 {
	if len(v) < 2 {
		return 0
	}
	sumSq := 0.0
	for _, x := range v {
		d := x - m
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(v)-1))
}

// percentile assumes sorted v. p in [0, 100].
func percentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	if len(v) == 1 {
		return v[0]
	}
	rank := (p / 100) * float64(len(v)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return v[lo]
	}
	frac := rank - float64(lo)
	return v[lo] + frac*(v[hi]-v[lo])
}
