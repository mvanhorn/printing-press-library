// Hand-authored statistics helpers for the Hotelist transcendence commands.
// Not generated.
package cli

import (
	"math"
	"sort"
)

func meanF(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func medianF(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	n := len(c)
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

func stddevF(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := meanF(xs)
	var ss float64
	for _, x := range xs {
		d := x - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(xs)-1))
}

func minMaxF(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	mn, mx := xs[0], xs[0]
	for _, x := range xs {
		if x < mn {
			mn = x
		}
		if x > mx {
			mx = x
		}
	}
	return mn, mx
}

// ratings / prices extractors (only positive prices count toward value/price
// stats so a missing price never poisons an average).
func ratingsOf(hs []hlHotel) []float64 {
	out := make([]float64, 0, len(hs))
	for _, h := range hs {
		out = append(out, h.Rating)
	}
	return out
}

func pricesOf(hs []hlHotel) []float64 {
	out := make([]float64, 0, len(hs))
	for _, h := range hs {
		if h.Price > 0 {
			out = append(out, h.Price)
		}
	}
	return out
}

func valuesOf(hs []hlHotel) []float64 {
	out := make([]float64, 0, len(hs))
	for _, h := range hs {
		if h.Price > 0 {
			out = append(out, h.Rating/h.Price*100)
		}
	}
	return out
}
