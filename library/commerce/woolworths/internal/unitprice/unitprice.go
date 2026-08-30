// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.

// Package unitprice normalises Woolworths "cup price" (unit price) figures so
// they can be compared across products.
//
// The Woolworths product tile carries a CupPrice plus a CupMeasure string that
// says what basis that price is quoted on. Observed values in live traffic
// include "100G", "1KG", "1L", "100ML", "1EA", "EA" and "100 sheets" — and a
// single category listing mixes them freely. Ranking raw CupPrice across such
// a listing is therefore wrong the moment two bases differ: $3.00/100G is
// $30.00/kg and loses to $25.00/1KG, but a naive sort puts the $3.00 tile
// first.
//
// This package solves exactly that, and nothing else:
//
//   - ParseMeasure turns a measure string into a quantity plus a unit kind
//     (mass, volume or count).
//   - Normalize converts a (price, measure) pair to a canonical basis for its
//     kind: price per 1 kg, per 1 L, or per 1 countable unit.
//   - Comparable refuses to compare across kinds. Mass against volume is not
//     a cheaper/dearer question and is never answered as one.
//   - Every entry point returns an explicit ok=false for input it cannot
//     parse. Nothing is coerced to zero, because a zero unit price sorts to
//     the front and would present an unparseable product as the cheapest one.
//
// The package is pure: no I/O, no globals, no clock.
package unitprice

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Kind is the dimension a unit price is quoted in. Prices are only ever
// comparable within one Kind.
type Kind uint8

const (
	// KindUnknown is the zero value and means "not parseable". It is never
	// comparable with anything, including itself.
	KindUnknown Kind = iota
	// KindMass covers g / kg / mg.
	KindMass
	// KindVolume covers mL / L / cL.
	KindVolume
	// KindCount covers discrete countables: each, sheets, packs, capsules,
	// tablets, washes, serves. Count prices additionally require a matching
	// countable label to be comparable: dollars-per-sheet is not
	// dollars-per-capsule.
	KindCount
)

// String renders the kind for output and error messages.
func (k Kind) String() string {
	switch k {
	case KindMass:
		return "mass"
	case KindVolume:
		return "volume"
	case KindCount:
		return "count"
	default:
		return "unknown"
	}
}

// MarshalJSON emits the kind as its name so JSON consumers never see a bare
// integer whose meaning depends on this file's const ordering.
func (k Kind) MarshalJSON() ([]byte, error) {
	return []byte(`"` + k.String() + `"`), nil
}

// Measure is a parsed measure string: a quantity plus the unit it is counted
// in, together with that quantity restated in the kind's base unit.
type Measure struct {
	// Raw is the input string, preserved for diagnostics.
	Raw string `json:"raw"`
	// Kind is the dimension this measure belongs to.
	Kind Kind `json:"kind"`
	// Quantity is the number as written, e.g. 100 for "100G".
	Quantity float64 `json:"quantity"`
	// Unit is the normalised unit label: "g", "mL", or a countable such as
	// "ea" / "sheet".
	Unit string `json:"unit"`
	// BaseQuantity is Quantity expressed in the kind's base unit — grams for
	// mass, millilitres for volume, countable units for count. "1KG" has
	// Quantity 1 and BaseQuantity 1000.
	BaseQuantity float64 `json:"base_quantity"`
}

// Normalized is a price restated on its kind's canonical basis.
type Normalized struct {
	// Price is the price for one canonical basis unit: one kilogram, one
	// litre, or one countable unit.
	Price float64 `json:"price"`
	// Kind is the dimension the price is quoted in.
	Kind Kind `json:"kind"`
	// Unit is the base unit label ("g", "mL") or the countable ("ea").
	Unit string `json:"unit"`
	// Basis is the human label for the denominator, e.g. "1kg", "1L", "1 ea".
	Basis string `json:"basis"`
	// Source is the measure string this was derived from.
	Source string `json:"source,omitempty"`
}

// String renders the normalised price the way a shelf label would.
func (n Normalized) String() string {
	if n.Kind == KindUnknown {
		return "n/a"
	}
	return "$" + strconv.FormatFloat(n.Price, 'f', -1, 64) + "/" + n.Basis
}

// massUnits maps a normalised unit token to its size in grams.
var massUnits = map[string]float64{
	"G": 1, "GM": 1, "GR": 1, "GRAM": 1, "GRAMS": 1, "GRAMME": 1, "GRAMMES": 1,
	"KG": 1000, "KGS": 1000, "KILO": 1000, "KILOS": 1000, "KILOGRAM": 1000, "KILOGRAMS": 1000,
	"MG": 0.001, "MILLIGRAM": 0.001, "MILLIGRAMS": 0.001,
}

// volumeUnits maps a normalised unit token to its size in millilitres.
var volumeUnits = map[string]float64{
	"ML": 1, "MLS": 1, "MILLILITRE": 1, "MILLILITRES": 1, "MILLILITER": 1, "MILLILITERS": 1,
	"L": 1000, "LT": 1000, "LTR": 1000, "LTRS": 1000, "LITRE": 1000, "LITRES": 1000,
	"LITER": 1000, "LITERS": 1000,
	"CL": 10, "CENTILITRE": 10, "CENTILITRES": 10,
}

// countUnits maps a normalised unit token to its canonical countable label.
// Two count measures are only comparable when these labels agree, so
// dollars-per-sheet is never ranked against dollars-per-capsule.
var countUnits = map[string]string{
	"EA": "ea", "EACH": "ea", "UNIT": "ea", "UNITS": "ea", "ITEM": "ea", "ITEMS": "ea",
	"PC": "ea", "PCS": "ea", "PIECE": "ea", "PIECES": "ea",
	"SHEET": "sheet", "SHEETS": "sheet",
	"PACK": "pack", "PACKS": "pack", "PK": "pack", "PKT": "pack", "PKTS": "pack",
	"CAPSULE": "capsule", "CAPSULES": "capsule", "CAP": "capsule", "CAPS": "capsule",
	"TABLET": "tablet", "TABLETS": "tablet", "TAB": "tablet", "TABS": "tablet",
	"WASH": "wash", "WASHES": "wash",
	"SERVE": "serve", "SERVES": "serve", "SERVING": "serve", "SERVINGS": "serve",
	"ROLL": "roll", "ROLLS": "roll",
	"BAG": "bag", "BAGS": "bag",
	"DOSE": "dose", "DOSES": "dose",
	"LOAD": "load", "LOADS": "load",
}

// measurePattern splits "100G", "1 KG", "750mL", "EA", "100 sheets" into an
// optional leading number and a unit word. Anything else fails to parse, and
// failing to parse is reported, never rounded down to zero.
var measurePattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)?\s*([A-Za-z][A-Za-z.]*)$`)

// cupTagPattern splits a CentreTag CupTag such as "$2.98/100G" into the price
// and the measure it is quoted against.
var cupTagPattern = regexp.MustCompile(`^\s*\$?\s*([0-9]+(?:\.[0-9]+)?)\s*/\s*(.+?)\s*$`)

// Multipack PackageSize values come in two shapes, and the "x" is what tells
// them apart. Woolworths' own CupPrice is the arbiter, and it is unambiguous
// across every multipack sampled in live traffic:
//
//	"180g 12 pack"    $7.50  ->  $4.17/100G   =  7.50 / 180g     -> 180g is the carton TOTAL
//	"252g 18 pack"    $8.00  ->  $3.17/100G   =  8.00 / 252g     -> 252g is the carton TOTAL
//	"375mL x 10 pack" $10.50 ->  $2.80/1L     = 10.50 / 3750mL   -> 375mL is PER ITEM
//	"40g x 6 pack"    $3.80  ->  $1.58/100G   =  3.80 / 240g     -> 40g is PER ITEM
//
// So a bare space means the leading measure already counts the whole carton and
// the pack count is descriptive; an "x" means the leading measure is one item
// and the count multiplies it. Reading them the same way is off by the carton
// count in one direction or the other, and a twelvefold-too-cheap multipack
// sorts straight to the top as the fake winner this package exists to prevent.

// packTimesPattern matches the per-item shape, "375mL x 10 pack" / "40g x 6
// pack", where the count multiplies the leading measure. It is tried first
// because the trailing-count pattern would otherwise swallow the "x" into its
// base capture.
var packTimesPattern = regexp.MustCompile(`(?i)^(.*?[A-Za-z.])\s*x\s*([0-9]+(?:\.[0-9]+)?)\s*(?:packs?|pk|pkts?)\.?$`)

// packTotalPattern matches the carton-total shape, "180g 12 pack" / "150g 4pk",
// where the leading measure is the whole carton and the count is descriptive.
var packTotalPattern = regexp.MustCompile(`(?i)^(.*?[A-Za-z.])\s+[0-9]+(?:\.[0-9]+)?\s*(?:packs?|pk|pkts?)\.?$`)

// leadingTimesPattern is the mirrored per-item shape, "10 x 375mL", where the
// count leads and the item measure trails.
var leadingTimesPattern = regexp.MustCompile(`(?i)^([0-9]+(?:\.[0-9]+)?)\s*x\s*([0-9].*)$`)

// splitPackMultiplier peels the carton count off a multipack pack-size string,
// returning the measure to parse and what to multiply it by — 1 for the shapes
// whose leading measure already covers the whole carton.
//
// It is only consulted after measurePattern has already failed, so a bare
// "6 pack" — genuinely six countable packs, not a count applied to nothing —
// keeps exactly its existing meaning.
func splitPackMultiplier(t string) (string, float64, bool) {
	var base, countStr string
	switch {
	case packTimesPattern.MatchString(t):
		m := packTimesPattern.FindStringSubmatch(t)
		base, countStr = m[1], m[2]
	case leadingTimesPattern.MatchString(t):
		m := leadingTimesPattern.FindStringSubmatch(t)
		countStr, base = m[1], m[2]
	case packTotalPattern.MatchString(t):
		// The leading measure is already the carton total, so nothing is
		// multiplied; the count is stripped only so the measure can parse.
		base, countStr = packTotalPattern.FindStringSubmatch(t)[1], "1"
	default:
		return "", 0, false
	}
	count, err := strconv.ParseFloat(countStr, 64)
	if err != nil || count <= 0 || math.IsInf(count, 0) || math.IsNaN(count) {
		return "", 0, false
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return "", 0, false
	}
	return base, count, true
}

// ParseMeasure parses a Woolworths measure string such as "100G", "1KG", "1L",
// "100ML", "1EA", "EA", "750mL" or "100 sheets".
//
// A missing leading number means one, so "EA" and "1EA" parse identically.
//
// Multipack pack sizes resolve to what the whole carton holds, because the
// price on the tile buys the whole carton: "180g 12 pack" is 180 g and
// "375mL x 10 pack" is 3750 mL. The "x" is what distinguishes them — see the
// CupPrice evidence on splitPackMultiplier above.
//
// The second return value is false for an empty string, a bare number, a unit
// this package does not recognise, or a non-positive quantity. Callers must
// honour it: a false here means "cannot be ranked", not "ranks at zero".
func ParseMeasure(s string) (Measure, bool) {
	raw := s
	t := strings.TrimSpace(s)
	if t == "" {
		return Measure{Raw: raw}, false
	}
	// Collapse internal runs of whitespace so "100  sheets" behaves like
	// "100 sheets".
	t = strings.Join(strings.Fields(t), " ")
	if strings.EqualFold(t, "none") || strings.EqualFold(t, "null") {
		return Measure{Raw: raw}, false
	}

	cartonCount := 1.0
	m := measurePattern.FindStringSubmatch(t)
	if m == nil {
		base, count, ok := splitPackMultiplier(t)
		if !ok {
			return Measure{Raw: raw}, false
		}
		if m = measurePattern.FindStringSubmatch(base); m == nil {
			return Measure{Raw: raw}, false
		}
		cartonCount = count
	}
	qty := 1.0
	if m[1] != "" {
		parsed, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return Measure{Raw: raw}, false
		}
		qty = parsed
	}
	qty *= cartonCount
	if qty <= 0 || math.IsInf(qty, 0) || math.IsNaN(qty) {
		return Measure{Raw: raw}, false
	}

	token := strings.ToUpper(strings.TrimRight(m[2], "."))
	if factor, ok := massUnits[token]; ok {
		return Measure{Raw: raw, Kind: KindMass, Quantity: qty, Unit: "g", BaseQuantity: qty * factor}, true
	}
	if factor, ok := volumeUnits[token]; ok {
		return Measure{Raw: raw, Kind: KindVolume, Quantity: qty, Unit: "mL", BaseQuantity: qty * factor}, true
	}
	if label, ok := countUnits[token]; ok {
		return Measure{Raw: raw, Kind: KindCount, Quantity: qty, Unit: label, BaseQuantity: qty}, true
	}
	return Measure{Raw: raw}, false
}

// CanonicalBasis returns how many base units one canonical basis holds, and
// the label for that basis. Mass canonicalises to one kilogram, volume to one
// litre, count to one countable unit of the given label.
//
// The second return value is false for KindUnknown.
func CanonicalBasis(k Kind, unit string) (float64, string, bool) {
	switch k {
	case KindMass:
		return 1000, "1kg", true
	case KindVolume:
		return 1000, "1L", true
	case KindCount:
		label := strings.TrimSpace(unit)
		if label == "" {
			label = "ea"
		}
		return 1, "1 " + label, true
	default:
		return 0, "", false
	}
}

// Normalize converts a cup price quoted against cupMeasure into the canonical
// basis for that measure's kind.
//
// "$3.00 per 100G" and "$25.00 per 1KG" both come back as a price per 1kg —
// $30.00 and $25.00 respectively — so the 1KG product correctly ranks cheaper
// even though its raw cup price is the larger number.
//
// The second return value is false when the measure will not parse or when
// price is not strictly positive. Callers must exclude such products from a
// ranking rather than treating them as free.
func Normalize(price float64, cupMeasure string) (Normalized, bool) {
	m, ok := ParseMeasure(cupMeasure)
	if !ok {
		return Normalized{Source: cupMeasure}, false
	}
	return NormalizeMeasure(price, m)
}

// NormalizeMeasure is Normalize for a measure that has already been parsed,
// so a caller that needs the pack size as well does not parse it twice.
func NormalizeMeasure(price float64, m Measure) (Normalized, bool) {
	if m.Kind == KindUnknown || m.BaseQuantity <= 0 {
		return Normalized{Source: m.Raw}, false
	}
	if price <= 0 || math.IsInf(price, 0) || math.IsNaN(price) {
		return Normalized{Source: m.Raw}, false
	}
	canonQty, basis, ok := CanonicalBasis(m.Kind, m.Unit)
	if !ok {
		return Normalized{Source: m.Raw}, false
	}
	value := price / m.BaseQuantity * canonQty
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return Normalized{Source: m.Raw}, false
	}
	return Normalized{
		Price:  round(value, 4),
		Kind:   m.Kind,
		Unit:   m.Unit,
		Basis:  basis,
		Source: m.Raw,
	}, true
}

// Comparable reports whether two normalised prices may be ranked against each
// other. Kinds must match and must not be unknown; for count prices the
// countable label must match too.
func Comparable(a, b Normalized) bool {
	if a.Kind == KindUnknown || b.Kind == KindUnknown {
		return false
	}
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == KindCount && a.Unit != b.Unit {
		return false
	}
	return true
}

// Compare returns -1 when a is cheaper than b, 0 when they are level, and +1
// when a is dearer. The second return value is false when the two are not
// comparable at all — a caller must branch on it rather than reading the int,
// which is zero in that case and would otherwise read as "equally priced".
func Compare(a, b Normalized) (int, bool) {
	if !Comparable(a, b) {
		return 0, false
	}
	switch {
	case a.Price < b.Price:
		return -1, true
	case a.Price > b.Price:
		return 1, true
	default:
		return 0, true
	}
}

// PercentCheaper returns how much cheaper alt is than anchor, as a percentage
// of anchor. A negative result means alt is dearer. The second return value is
// false when the pair is not comparable or anchor has no positive price.
func PercentCheaper(anchor, alt Normalized) (float64, bool) {
	if !Comparable(anchor, alt) || anchor.Price <= 0 {
		return 0, false
	}
	return round((anchor.Price-alt.Price)/anchor.Price*100, 2), true
}

// ParseCupTag splits a multibuy CentreTag cup tag such as "$2.98/100G" into
// its price and its measure string. The measure is returned unparsed so the
// caller can hand it to Normalize and see the same ok=false behaviour as for
// any other measure.
func ParseCupTag(tag string) (float64, string, bool) {
	m := cupTagPattern.FindStringSubmatch(tag)
	if m == nil {
		return 0, "", false
	}
	price, err := strconv.ParseFloat(m[1], 64)
	if err != nil || price <= 0 {
		return 0, "", false
	}
	measure := strings.TrimSpace(m[2])
	if measure == "" {
		return 0, "", false
	}
	return price, measure, true
}

// round trims float noise so a normalised price is stable in JSON output and
// in sort comparisons.
func round(v float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(v*scale) / scale
}
