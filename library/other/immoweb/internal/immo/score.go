package immo

import "strings"

// TriageInput carries the signals triage combines for one listing.
type TriageInput struct {
	PricePercentile *float64 // €/m² percentile vs comparables (0 = cheapest)
	DaysListed      *int
	DaysSinceUpdate *int // fallback when the publication date is unknown
	PriceCut        bool
	Private         bool
	EPC             string
}

// TriageFactor is one weighted contribution to the triage score.
type TriageFactor struct {
	Factor string  `json:"factor"`
	Weight float64 `json:"weight"`
	Value  float64 `json:"value"`  // 0..100 before weighting
	Points float64 `json:"points"` // weight * value
	Note   string  `json:"note,omitempty"`
}

// TriageWeights are the default weights; they sum to 1.
var TriageWeights = map[string]float64{
	"price":     0.35,
	"freshness": 0.25,
	"price_cut": 0.15,
	"private":   0.10,
	"epc":       0.15,
}

// EPCValue maps an EPC label to 0..100 (A best). ok=false when unknown.
func EPCValue(label string) (float64, bool) {
	switch strings.ToUpper(strings.TrimSpace(label)) {
	case "A++", "A+", "A":
		return 100, true
	case "B":
		return 85, true
	case "C":
		return 70, true
	case "D":
		return 50, true
	case "E":
		return 30, true
	case "F":
		return 15, true
	case "G":
		return 0, true
	}
	return 0, false
}

// EPCRank orders labels for gap computation (A=1 ... G=7), 0 when unknown.
func EPCRank(label string) int {
	switch strings.ToUpper(strings.TrimSpace(label)) {
	case "A++", "A+", "A":
		return 1
	case "B":
		return 2
	case "C":
		return 3
	case "D":
		return 4
	case "E":
		return 5
	case "F":
		return 6
	case "G":
		return 7
	}
	return 0
}

// TriageScore returns the 0..100 composite score and its factor breakdown.
// Unknown signals score a neutral 50 so they neither help nor hurt.
func TriageScore(in TriageInput) (float64, []TriageFactor) {
	factors := make([]TriageFactor, 0, 5)
	add := func(name string, value float64, note string) {
		w := TriageWeights[name]
		factors = append(factors, TriageFactor{Factor: name, Weight: w, Value: Round1(value), Points: Round1(w * value), Note: note})
	}
	if in.PricePercentile != nil {
		add("price", 100-*in.PricePercentile, "cheaper per m² than comparables scores higher")
	} else {
		add("price", 50, "no comparables with surface; neutral")
	}
	if in.DaysListed != nil {
		v := 100 - float64(*in.DaysListed)*100/30
		if v < 0 {
			v = 0
		}
		add("freshness", v, "listed recently scores higher (0 after 30 days)")
	} else if in.DaysSinceUpdate != nil {
		v := 100 - float64(*in.DaysSinceUpdate)*100/30
		if v < 0 {
			v = 0
		}
		add("freshness", v, "publication date unknown: scored on the last update instead (use --enrich for the real listing age)")
	} else {
		add("freshness", 50, "listing age unknown; neutral")
	}
	if in.PriceCut {
		add("price_cut", 100, "asking price was reduced")
	} else {
		add("price_cut", 0, "")
	}
	if in.Private {
		add("private", 100, "private seller (no agency)")
	} else {
		add("private", 0, "")
	}
	if v, ok := EPCValue(in.EPC); ok {
		add("epc", v, "EPC "+strings.ToUpper(in.EPC))
	} else {
		add("epc", 50, "EPC unknown; run with --enrich to fetch it")
	}
	total := 0.0
	for _, f := range factors {
		total += f.Points
	}
	return Round1(total), factors
}
