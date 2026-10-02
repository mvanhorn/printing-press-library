// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package yamato

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
)

var Sizes = []int{60, 80, 100, 120, 140, 160, 180, 200}
var Weights = []float64{2, 5, 10, 15, 20, 25, 30, 30}

type Parcel struct {
	LengthCM          float64  `json:"length_cm"`
	WidthCM           float64  `json:"width_cm"`
	HeightCM          float64  `json:"height_cm"`
	WeightKG          float64  `json:"weight_kg"`
	TotalCM           float64  `json:"linear_cm"`
	DimensionCategory int      `json:"dimension_category"`
	WeightCategory    int      `json:"weight_category"`
	ChargeableSize    int      `json:"chargeable_size"`
	Supported         bool     `json:"within_size_weight_limits"`
	Reasons           []string `json:"reasons"`
	Basis             string   `json:"charging_basis"`
	Acceptance        string   `json:"acceptance"`
	RuleURL           string   `json:"rule_url"`
	RuleObservedAt    string   `json:"rule_observed_at"`
}

func Classify(l, w, h, kg float64, upright bool) (Parcel, error) {
	p := Parcel{LengthCM: l, WidthCM: w, HeightCM: h, WeightKG: kg, Reasons: []string{}, Basis: "greater of dimension and weight categories", Acceptance: "unknown: contents, packaging, local counter and destination acceptance must be checked", RuleURL: Main + "/ytc/en/send/services/takkyubin/", RuleObservedAt: ObservedAt}
	for _, v := range []float64{l, w, h, kg} {
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return p, fmt.Errorf("--length, --width, --height and --weight must be positive finite numbers in cm/kg")
		}
	}
	// Treat measured decimals as decimals. Binary addition can turn an exact
	// 160cm measurement into 160.00000000000003 and overcharge or reject it.
	var total big.Rat
	for _, dimension := range []float64{l, w, h} {
		term, ok := new(big.Rat).SetString(strconv.FormatFloat(dimension, 'f', -1, 64))
		if !ok {
			return p, fmt.Errorf("invalid decimal parcel measurement")
		}
		total.Add(&total, term)
	}
	p.TotalCM, _ = total.Float64()
	if math.IsInf(p.TotalCM, 0) {
		return p, fmt.Errorf("parcel dimensions exceed the numeric range; provide measurements in cm")
	}
	maxside := math.Max(l, math.Max(w, h))
	maxAllowed := 170.0
	if upright {
		maxAllowed = 100
	}
	if maxside > maxAllowed {
		p.Reasons = append(p.Reasons, fmt.Sprintf("longest side exceeds %.0f cm", maxAllowed))
	}
	if total.Cmp(big.NewRat(200, 1)) > 0 {
		p.Reasons = append(p.Reasons, "linear dimensions exceed 200 cm")
	}
	if kg > 30 {
		p.Reasons = append(p.Reasons, "weight exceeds 30 kg")
	}
	for _, size := range Sizes {
		if total.Cmp(big.NewRat(int64(size), 1)) <= 0 {
			p.DimensionCategory = size
			break
		}
	}
	p.WeightCategory = category(kg, true)
	p.ChargeableSize = max(p.DimensionCategory, p.WeightCategory)
	p.Supported = len(p.Reasons) == 0
	if !p.Supported {
		p.ChargeableSize = 0
	}
	return p, nil
}
func category(v float64, weight bool) int {
	for i, size := range Sizes {
		limit := float64(size)
		if weight {
			limit = Weights[i]
		}
		if v <= limit {
			return size
		}
	}
	return 0
}
func ValidSize(size int) bool {
	_, ok := sort.Find(len(Sizes), func(i int) int { return size - Sizes[i] })
	return ok
}
