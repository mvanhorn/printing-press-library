// Shared analytics helpers for market, deal, triage, drops and yield.

package cli

import (
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

// cutInfo describes the price reductions known for one listing, combining
// the locally recorded price history with Immoweb's own old-price field.
type cutInfo struct {
	FirstPrice  float64          `json:"first_price"`
	LatestPrice float64          `json:"latest_price"`
	CutEUR      float64          `json:"cut_eur"`
	CutPct      float64          `json:"cut_pct"`
	Cuts        int              `json:"cuts"`
	Source      string           `json:"source"` // local-history | immoweb-old-price
	LastCutAt   string           `json:"last_cut_at,omitempty"`
	History     []store.PriceObs `json:"history,omitempty"`
}

// priceCut returns the reduction for a listing, or ok=false when no
// reduction is known.
func priceCut(l store.StoredListing, hist []store.PriceObs) (cutInfo, bool) {
	var ci cutInfo
	if len(hist) >= 2 {
		ci.FirstPrice = hist[0].Price
		ci.LatestPrice = hist[len(hist)-1].Price
		for i := 1; i < len(hist); i++ {
			if hist[i].Price < hist[i-1].Price {
				ci.Cuts++
				ci.LastCutAt = hist[i].ObservedAt
			}
		}
		ci.Source = "local-history"
		ci.History = hist
	}
	if l.OldPrice != nil && l.Price != nil && *l.OldPrice > *l.Price {
		if ci.Source == "" || *l.OldPrice > ci.FirstPrice {
			ci.FirstPrice = *l.OldPrice
			ci.LatestPrice = *l.Price
			if ci.Cuts == 0 {
				ci.Cuts = 1
			}
			ci.Source = "immoweb-old-price"
			if ci.LastCutAt == "" {
				ci.LastCutAt = l.ModifiedAt
			}
		}
	}
	if ci.FirstPrice <= 0 || ci.LatestPrice >= ci.FirstPrice {
		return ci, false
	}
	ci.CutEUR = immo.Round0(ci.FirstPrice - ci.LatestPrice)
	ci.CutPct = immo.Round1((ci.FirstPrice - ci.LatestPrice) / ci.FirstPrice * 100)
	return ci, true
}

// daysOf returns days listed from Immoweb's publication date. Search pages
// do not carry that date (only the detail endpoint does), so it is nil for
// listings never opened with show/deal/triage --enrich. The CLI's own
// first-seen time is deliberately not used: on a first pull it would report
// every listing as brand new.
func daysOf(l store.StoredListing, now time.Time) *int {
	if l.CreatedAt == "" {
		return nil
	}
	if d, ok := immo.DaysListed(l.CreatedAt, now); ok {
		return &d
	}
	return nil
}

// ppsValues extracts non-nil €/m² values.
func ppsValues(ls []store.StoredListing) []float64 {
	out := make([]float64, 0, len(ls))
	for _, l := range ls {
		if l.PricePerSqm != nil {
			out = append(out, *l.PricePerSqm)
		}
	}
	return out
}

// priceValues extracts non-nil prices.
func priceValues(ls []store.StoredListing) []float64 {
	out := make([]float64, 0, len(ls))
	for _, l := range ls {
		if l.Price != nil {
			out = append(out, *l.Price)
		}
	}
	return out
}

func medianPtr(vals []float64) *float64 {
	if m, ok := immo.Median(vals); ok {
		r := immo.Round0(m)
		return &r
	}
	return nil
}

func quantilePtr(vals []float64, q float64) *float64 {
	if v, ok := immo.Quantile(vals, q); ok {
		r := immo.Round0(v)
		return &r
	}
	return nil
}

// minComparables is the floor below which verdicts are refused.
const minComparables = 5
