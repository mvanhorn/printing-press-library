package immo

import (
	"math"
	"sort"
)

// Median returns the median of vals and ok=false for an empty slice.
func Median(vals []float64) (float64, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2], true
	}
	return (s[n/2-1] + s[n/2]) / 2, true
}

// Quantile returns the q-quantile (0..1) with linear interpolation.
func Quantile(vals []float64, q float64) (float64, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	if q <= 0 {
		return s[0], true
	}
	if q >= 1 {
		return s[len(s)-1], true
	}
	pos := q * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return s[lo], true
	}
	frac := pos - float64(lo)
	return s[lo] + (s[hi]-s[lo])*frac, true
}

// PercentileRank returns the share (0..100) of vals strictly below v, with
// ties counted as half. ok=false for an empty slice.
func PercentileRank(vals []float64, v float64) (float64, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	below, equal := 0, 0
	for _, x := range vals {
		switch {
		case x < v:
			below++
		case x == v:
			equal++
		}
	}
	return math.Round((float64(below)+float64(equal)/2)/float64(len(vals))*1000) / 10, true
}

// Round0 rounds to the nearest integer.
func Round0(v float64) float64 { return math.Round(v) }

// Round1 rounds to one decimal.
func Round1(v float64) float64 { return math.Round(v*10) / 10 }

// Round2 rounds to two decimals.
func Round2(v float64) float64 { return math.Round(v*100) / 100 }

// BedroomBand groups bedroom counts for market breakdowns.
// BedroomBands lists the bedroom bands in display order.
var BedroomBands = []string{"studio", "1", "2", "3", "4+", "unknown"}

func BedroomBand(b *int) string {
	if b == nil {
		return "unknown"
	}
	switch {
	case *b <= 0:
		return "studio"
	case *b >= 4:
		return "4+"
	default:
		return string(rune('0' + *b))
	}
}

// Haversine returns the great-circle distance in kilometres.
func Haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// GrossYield returns 12*rent/price as a percentage.
func GrossYield(monthlyRent, price float64) (float64, bool) {
	if price <= 0 || monthlyRent <= 0 {
		return 0, false
	}
	return Round2(12 * monthlyRent / price * 100), true
}

// PlausiblePricePerSqm filters obvious data-entry errors (surface typed in
// the wrong unit, parking spaces listed as apartments, ...). Rents are
// monthly EUR/m2, sales are EUR/m2.
func PlausiblePricePerSqm(deal string, pps float64) bool {
	if deal == "FOR_RENT" {
		return pps >= 3 && pps <= 150
	}
	return pps >= 300 && pps <= 25000
}

// PricePerSqm returns the rounded EUR/m2 when price and surface are known and
// the result is plausible for the deal type.
func PricePerSqm(deal string, price, surface *float64) *float64 {
	if price == nil || surface == nil || *surface < 10 {
		return nil
	}
	if !PlausiblePricePerSqm(deal, *price / *surface) {
		return nil
	}
	v := math.Round(*price / *surface)
	return &v
}
