package pricebook

import (
	"math"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/store"
)

// sortTiers orders a markup ladder ascending by From (then To), which is the
// order EvalTier walks.
func sortTiers(tiers []MarkupTier) {
	sort.SliceStable(tiers, func(i, j int) bool {
		if tiers[i].From != tiers[j].From {
			return tiers[i].From < tiers[j].From
		}
		return tiers[i].To < tiers[j].To
	})
}

// EvalTier returns the markup tier whose [From, To] range contains cost.
// found is false when no tier matches (cost below the lowest From or above
// the highest To, or the ladder is empty). The ladder is assumed sorted by
// LoadMarkupLadder; callers passing an unsorted slice should sortTiers first.
func EvalTier(ladder []MarkupTier, cost float64) (tier MarkupTier, found bool) {
	for _, t := range ladder {
		if cost >= t.From && cost <= t.To {
			return t, true
		}
	}
	return MarkupTier{}, false
}

// ExpectedPrice returns the tier-correct price for a cost: cost grossed up by
// the matching tier's Percent. found mirrors EvalTier — when no tier matches,
// price is 0 and found is false so callers can skip rather than reprice to 0.
func ExpectedPrice(ladder []MarkupTier, cost float64) (price float64, tier MarkupTier, found bool) {
	t, ok := EvalTier(ladder, cost)
	if !ok {
		return 0, MarkupTier{}, false
	}
	return round2(cost * (1 + t.Percent/100.0)), t, true
}

// ActualMarkupPercent returns the realised markup percent for a SKU:
// (price - cost) / cost * 100. ok is false when cost is zero or negative, in
// which case markup is undefined (a cost-zero SKU is itself an audit finding,
// surfaced separately).
func ActualMarkupPercent(cost, price float64) (pct float64, ok bool) {
	if cost <= 0 {
		return 0, false
	}
	return (price - cost) / cost * 100.0, true
}

// MarkupRow is one finding from MarkupAudit: a SKU whose realised markup has
// drifted off its cost tier's expected Percent (or whose cost falls in no
// tier at all).
type MarkupRow struct {
	Kind          SKUKind `json:"kind"`
	ID            int64   `json:"id"`
	Code          string  `json:"code"`
	DisplayName   string  `json:"display_name"`
	Cost          float64 `json:"cost"`
	Price         float64 `json:"price"`
	ActualPercent float64 `json:"actual_markup_percent"`
	TierPercent   float64 `json:"tier_markup_percent"`
	ExpectedPrice float64 `json:"expected_price"`
	DeltaPercent  float64 `json:"delta_percent"` // actual - tier
	Reason        string  `json:"reason"`        // "drift" | "no-tier" | "zero-cost"
}

// MarkupAudit joins every active material and equipment SKU against the
// markup tier ladder and returns the ones whose realised markup deviates
// from the tier's expected percent by more than tolerancePct (an absolute
// percentage-point band — tolerancePct of 5 means "flag anything more than
// 5 points off"). Zero-cost and no-tier SKUs are always returned with the
// matching Reason. This join is impossible in one ServiceTitan API call.
func MarkupAudit(db *store.Store, tolerancePct float64) ([]MarkupRow, error) {
	ladder, err := LoadMarkupLadder(db)
	if err != nil {
		return nil, err
	}
	mats, err := LoadMaterials(db)
	if err != nil {
		return nil, err
	}
	eqs, err := LoadEquipment(db)
	if err != nil {
		return nil, err
	}

	var rows []MarkupRow
	consider := func(kind SKUKind, id int64, code, name string, cost, price float64, active bool) {
		if !active {
			return
		}
		if cost <= 0 {
			rows = append(rows, MarkupRow{
				Kind: kind, ID: id, Code: code, DisplayName: name,
				Cost: cost, Price: price, Reason: "zero-cost",
			})
			return
		}
		actual, _ := ActualMarkupPercent(cost, price)
		expPrice, tier, ok := ExpectedPrice(ladder, cost)
		if !ok {
			rows = append(rows, MarkupRow{
				Kind: kind, ID: id, Code: code, DisplayName: name,
				Cost: cost, Price: price, ActualPercent: round2(actual), Reason: "no-tier",
			})
			return
		}
		delta := actual - tier.Percent
		if math.Abs(delta) <= tolerancePct {
			return
		}
		rows = append(rows, MarkupRow{
			Kind: kind, ID: id, Code: code, DisplayName: name,
			Cost: cost, Price: price,
			ActualPercent: round2(actual), TierPercent: round2(tier.Percent),
			ExpectedPrice: expPrice, DeltaPercent: round2(delta), Reason: "drift",
		})
	}

	for _, m := range mats {
		consider(KindMaterial, m.ID, m.Code, m.DisplayName, m.Cost, m.Price, m.Active)
	}
	for _, e := range eqs {
		consider(KindEquipment, e.ID, e.Code, e.DisplayName, e.Cost, e.Price, e.Active)
	}
	return rows, nil
}

// RepriceRow is one proposed price change from Reprice: a drifted SKU plus
// the tier-correct price it should move to.
type RepriceRow struct {
	Kind        SKUKind `json:"kind"`
	ID          int64   `json:"id"`
	Code        string  `json:"code"`
	DisplayName string  `json:"display_name"`
	Cost        float64 `json:"cost"`
	OldPrice    float64 `json:"old_price"`
	NewPrice    float64 `json:"new_price"`
	TierPercent float64 `json:"tier_markup_percent"`
}

// Reprice turns the "drift" findings of a MarkupAudit into concrete price
// changes: for each drifted SKU whose cost falls in a tier, the tier-correct
// price. no-tier and zero-cost rows are skipped — there is no defensible
// price to move them to. The caller decides whether to emit these as a
// dry-run table or push them through bulk-plan.
func Reprice(db *store.Store, tolerancePct float64) ([]RepriceRow, error) {
	audit, err := MarkupAudit(db, tolerancePct)
	if err != nil {
		return nil, err
	}
	ladder, err := LoadMarkupLadder(db)
	if err != nil {
		return nil, err
	}
	var rows []RepriceRow
	for _, a := range audit {
		if a.Reason != "drift" {
			continue
		}
		newPrice, tier, ok := ExpectedPrice(ladder, a.Cost)
		if !ok || newPrice == a.Price {
			continue
		}
		rows = append(rows, RepriceRow{
			Kind: a.Kind, ID: a.ID, Code: a.Code, DisplayName: a.DisplayName,
			Cost: a.Cost, OldPrice: a.Price, NewPrice: newPrice, TierPercent: round2(tier.Percent),
		})
	}
	return rows, nil
}

// round2 rounds to cents. ServiceTitan prices are currency; carrying more
// precision into a proposed price just produces noise in the diff.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
