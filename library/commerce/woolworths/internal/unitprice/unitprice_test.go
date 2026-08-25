// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.

package unitprice

import (
	"encoding/json"
	"math"
	"sort"
	"testing"
)

func TestKindString(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		want string
	}{
		{"mass", KindMass, "mass"},
		{"volume", KindVolume, "volume"},
		{"count", KindCount, "count"},
		{"unknown", KindUnknown, "unknown"},
		{"out of range", Kind(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.want {
				t.Fatalf("Kind(%d).String() = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestKindMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		want string
	}{
		{"mass", KindMass, `"mass"`},
		{"volume", KindVolume, `"volume"`},
		{"count", KindCount, `"count"`},
		{"unknown", KindUnknown, `"unknown"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.kind)
			if err != nil {
				t.Fatalf("json.Marshal(%v) error = %v", tt.kind, err)
			}
			if string(got) != tt.want {
				t.Fatalf("json.Marshal(%v) = %s, want %s", tt.kind, got, tt.want)
			}
		})
	}
}

func TestParseMeasure(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantOK   bool
		wantKind Kind
		wantQty  float64
		wantUnit string
		wantBase float64
	}{
		// Every value in this block was observed in live Woolworths traffic.
		{"per 100 grams", "100G", true, KindMass, 100, "g", 100},
		{"per kilogram", "1KG", true, KindMass, 1, "g", 1000},
		{"per litre", "1L", true, KindVolume, 1, "mL", 1000},
		{"per 100 millilitres", "100ML", true, KindVolume, 100, "mL", 100},
		{"per each with count", "1EA", true, KindCount, 1, "ea", 1},
		{"per each bare", "EA", true, KindCount, 1, "ea", 1},
		{"per 100 sheets", "100 sheets", true, KindCount, 100, "sheet", 100},
		// Shapes the pack-size field uses.
		{"pack size millilitres", "750mL", true, KindVolume, 750, "mL", 750},
		{"pack size grams lowercase", "500g", true, KindMass, 500, "g", 500},
		{"fractional litres", "1.5L", true, KindVolume, 1.5, "mL", 1500},
		{"spaced kilogram", "1 kg", true, KindMass, 1, "g", 1000},
		{"milligrams", "500MG", true, KindMass, 500, "g", 0.5},
		{"centilitres", "10CL", true, KindVolume, 10, "mL", 100},
		{"capsules", "10 capsules", true, KindCount, 10, "capsule", 10},
		{"washes", "30 washes", true, KindCount, 30, "wash", 30},
		{"trailing dot unit", "100 g.", true, KindMass, 100, "g", 100},
		{"collapsed whitespace", "100   sheets", true, KindCount, 100, "sheet", 100},
		// Multipack pack sizes, all observed in live PackageSize values. The
		// tile price buys the whole carton, so the measure is the carton total
		// — and the "x" is what says whether the leading figure is already
		// that total or only one item. Each expectation below is what
		// Woolworths' own CupPrice implies for that product.
		//   "180g 12 pack"    $7.50  -> $4.17/100G  = 7.50 / 180g
		{"carton total grams", "180g 12 pack", true, KindMass, 180, "g", 180},
		//   "252g 18 pack"    $8.00  -> $3.17/100G  = 8.00 / 252g
		{"carton total grams larger count", "252g 18 pack", true, KindMass, 252, "g", 252},
		{"carton total abbreviated pk", "150g 4pk", true, KindMass, 150, "g", 150},
		//   "40g x 6 pack"    $3.80  -> $1.58/100G  = 3.80 / 240g
		{"per item grams with x", "40g x 6 pack", true, KindMass, 240, "g", 240},
		//   "375mL x 10 pack" $10.50 -> $2.80/1L    = 10.50 / 3750mL
		{"per item millilitres with x", "375mL x 10 pack", true, KindVolume, 3750, "mL", 3750},
		//   "1L x 4 pack"     $10.90 -> $2.73/1L    = 10.90 / 4L
		{"per item litres with x", "1L x 4 pack", true, KindVolume, 4, "mL", 4000},
		{"per item leading count", "10 x 375mL", true, KindVolume, 3750, "mL", 3750},
		// A bare carton count is still six countable packs, not a count
		// applied to nothing.
		{"bare pack count unchanged", "6 pack", true, KindCount, 6, "pack", 6},
		// Everything below must report failure rather than a zero measure.
		{"empty", "", false, KindUnknown, 0, "", 0},
		{"whitespace only", "   ", false, KindUnknown, 0, "", 0},
		{"literal none", "None", false, KindUnknown, 0, "", 0},
		{"bare number", "100", false, KindUnknown, 0, "", 0},
		{"unknown unit", "100 zorkmids", false, KindUnknown, 0, "", 0},
		{"length unit is not a grocery basis", "5 metres", false, KindUnknown, 0, "", 0},
		{"zero quantity", "0G", false, KindUnknown, 0, "", 0},
		{"punctuation soup", "$$$", false, KindUnknown, 0, "", 0},
		{"two numbers", "100 200", false, KindUnknown, 0, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseMeasure(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ParseMeasure(%q) ok = %v, want %v (got %+v)", tt.in, ok, tt.wantOK, got)
			}
			if !tt.wantOK {
				if got.Kind != KindUnknown {
					t.Fatalf("ParseMeasure(%q) failed but returned kind %v; a failed parse must not carry a usable kind", tt.in, got.Kind)
				}
				if got.BaseQuantity != 0 {
					t.Fatalf("ParseMeasure(%q) failed but returned BaseQuantity %v; must stay zero-valued and be rejected via ok", tt.in, got.BaseQuantity)
				}
				return
			}
			if got.Kind != tt.wantKind {
				t.Fatalf("ParseMeasure(%q).Kind = %v, want %v", tt.in, got.Kind, tt.wantKind)
			}
			if !nearly(got.Quantity, tt.wantQty) {
				t.Fatalf("ParseMeasure(%q).Quantity = %v, want %v", tt.in, got.Quantity, tt.wantQty)
			}
			if got.Unit != tt.wantUnit {
				t.Fatalf("ParseMeasure(%q).Unit = %q, want %q", tt.in, got.Unit, tt.wantUnit)
			}
			if !nearly(got.BaseQuantity, tt.wantBase) {
				t.Fatalf("ParseMeasure(%q).BaseQuantity = %v, want %v", tt.in, got.BaseQuantity, tt.wantBase)
			}
			if got.Raw != tt.in {
				t.Fatalf("ParseMeasure(%q).Raw = %q, want the input preserved", tt.in, got.Raw)
			}
		})
	}
}

func TestCanonicalBasis(t *testing.T) {
	tests := []struct {
		name      string
		kind      Kind
		unit      string
		wantQty   float64
		wantBasis string
		wantOK    bool
	}{
		{"mass canonicalises to a kilogram", KindMass, "g", 1000, "1kg", true},
		{"volume canonicalises to a litre", KindVolume, "mL", 1000, "1L", true},
		{"count canonicalises to one each", KindCount, "ea", 1, "1 ea", true},
		{"count keeps its countable label", KindCount, "sheet", 1, "1 sheet", true},
		{"count with no label defaults to each", KindCount, "", 1, "1 ea", true},
		{"unknown has no basis", KindUnknown, "", 0, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qty, basis, ok := CanonicalBasis(tt.kind, tt.unit)
			if ok != tt.wantOK {
				t.Fatalf("CanonicalBasis(%v,%q) ok = %v, want %v", tt.kind, tt.unit, ok, tt.wantOK)
			}
			if qty != tt.wantQty || basis != tt.wantBasis {
				t.Fatalf("CanonicalBasis(%v,%q) = (%v,%q), want (%v,%q)", tt.kind, tt.unit, qty, basis, tt.wantQty, tt.wantBasis)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name      string
		price     float64
		measure   string
		wantOK    bool
		wantPrice float64
		wantBasis string
		wantKind  Kind
	}{
		// The headline case: two mass tiles quoted on different bases.
		{"three dollars per 100g is thirty per kilo", 3.00, "100G", true, 30, "1kg", KindMass},
		{"twenty five per kilo stays twenty five", 25.00, "1KG", true, 25, "1kg", KindMass},
		{"one dollar per 100ml is ten per litre", 1.00, "100ML", true, 10, "1L", KindVolume},
		{"two sixty per litre stays two sixty", 2.60, "1L", true, 2.6, "1L", KindVolume},
		{"per each passes through", 1.75, "1EA", true, 1.75, "1 ea", KindCount},
		{"per 100 sheets becomes per sheet", 0.50, "100 sheets", true, 0.005, "1 sheet", KindCount},
		{"pack size works as a measure", 5.00, "500g", true, 10, "1kg", KindMass},
		// Failure modes. None of these may return a usable zero price.
		{"unparseable measure", 3.00, "per widget", false, 0, "", KindUnknown},
		{"empty measure", 3.00, "", false, 0, "", KindUnknown},
		{"zero price", 0, "100G", false, 0, "", KindUnknown},
		{"negative price", -1, "100G", false, 0, "", KindUnknown},
		{"NaN price", math.NaN(), "100G", false, 0, "", KindUnknown},
		{"Inf price", math.Inf(1), "100G", false, 0, "", KindUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Normalize(tt.price, tt.measure)
			if ok != tt.wantOK {
				t.Fatalf("Normalize(%v,%q) ok = %v, want %v (got %+v)", tt.price, tt.measure, ok, tt.wantOK, got)
			}
			if !tt.wantOK {
				if got.Price != 0 || got.Kind != KindUnknown {
					t.Fatalf("Normalize(%v,%q) failed but returned %+v; a failed normalisation must not look like a real price", tt.price, tt.measure, got)
				}
				return
			}
			if !nearly(got.Price, tt.wantPrice) {
				t.Fatalf("Normalize(%v,%q).Price = %v, want %v", tt.price, tt.measure, got.Price, tt.wantPrice)
			}
			if got.Basis != tt.wantBasis {
				t.Fatalf("Normalize(%v,%q).Basis = %q, want %q", tt.price, tt.measure, got.Basis, tt.wantBasis)
			}
			if got.Kind != tt.wantKind {
				t.Fatalf("Normalize(%v,%q).Kind = %v, want %v", tt.price, tt.measure, got.Kind, tt.wantKind)
			}
		})
	}
}

// TestNormalizeCrossBasisMassOrdering is the correctness assertion this whole
// package exists for: a $3.00/100G tile is dearer than a $25.00/1KG tile, even
// though 3.00 < 25.00. A raw CupPrice sort gets this exactly backwards.
func TestNormalizeCrossBasisMassOrdering(t *testing.T) {
	per100g, ok := Normalize(3.00, "100G")
	if !ok {
		t.Fatalf("Normalize(3.00, \"100G\") returned ok=false")
	}
	perKG, ok := Normalize(25.00, "1KG")
	if !ok {
		t.Fatalf("Normalize(25.00, \"1KG\") returned ok=false")
	}

	if per100g.Basis != perKG.Basis {
		t.Fatalf("bases differ after normalisation: %q vs %q", per100g.Basis, perKG.Basis)
	}
	if !nearly(per100g.Price, 30) {
		t.Fatalf("$3.00/100G normalised to %v/%s, want 30/1kg", per100g.Price, per100g.Basis)
	}
	if !nearly(perKG.Price, 25) {
		t.Fatalf("$25.00/1KG normalised to %v/%s, want 25/1kg", perKG.Price, perKG.Basis)
	}

	cmp, ok := Compare(perKG, per100g)
	if !ok {
		t.Fatalf("Compare on two mass prices returned not-comparable")
	}
	if cmp != -1 {
		t.Fatalf("Compare(1KG tile, 100G tile) = %d, want -1 (the 1KG tile is cheaper per kilo)", cmp)
	}

	// And the naive comparison the CLI must never make.
	if !(3.00 < 25.00) {
		t.Fatalf("sanity: raw cup prices are 3.00 and 25.00")
	}
	t.Logf("raw cup prices rank 100G first (3.00 < 25.00); normalised prices rank 1KG first ($%.2f/%s < $%.2f/%s)",
		perKG.Price, perKG.Basis, per100g.Price, per100g.Basis)
}

// TestComparableRefusesCrossKind proves mass is never silently ranked against
// volume, which would otherwise produce a confident but meaningless ordering.
func TestComparableRefusesCrossKind(t *testing.T) {
	mass, ok := Normalize(3.00, "100G")
	if !ok {
		t.Fatalf("Normalize mass returned ok=false")
	}
	volume, ok := Normalize(1.00, "100ML")
	if !ok {
		t.Fatalf("Normalize volume returned ok=false")
	}
	count, ok := Normalize(1.75, "1EA")
	if !ok {
		t.Fatalf("Normalize count returned ok=false")
	}

	pairs := []struct {
		name string
		a, b Normalized
	}{
		{"mass vs volume", mass, volume},
		{"volume vs mass", volume, mass},
		{"mass vs count", mass, count},
		{"count vs volume", count, volume},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			if Comparable(p.a, p.b) {
				t.Fatalf("Comparable(%s) = true; %v and %v are different dimensions and must never be ranked together", p.name, p.a.Kind, p.b.Kind)
			}
			if cmp, ok := Compare(p.a, p.b); ok {
				t.Fatalf("Compare(%s) = (%d,true); want not-comparable", p.name, cmp)
			}
			if pct, ok := PercentCheaper(p.a, p.b); ok {
				t.Fatalf("PercentCheaper(%s) = (%v,true); want not-comparable", p.name, pct)
			}
		})
	}
}

func TestComparableCountLabels(t *testing.T) {
	perSheet, _ := Normalize(0.50, "100 sheets")
	perEach, _ := Normalize(0.50, "1EA")
	perCapsule, _ := Normalize(0.50, "10 capsules")
	otherSheet, _ := Normalize(0.90, "50 sheets")

	if Comparable(perSheet, perEach) {
		t.Fatalf("dollars-per-sheet and dollars-per-each are both count but not the same countable; must not compare")
	}
	if Comparable(perSheet, perCapsule) {
		t.Fatalf("dollars-per-sheet and dollars-per-capsule must not compare")
	}
	if !Comparable(perSheet, otherSheet) {
		t.Fatalf("two per-sheet prices must compare")
	}
	cmp, ok := Compare(perSheet, otherSheet)
	if !ok || cmp != -1 {
		t.Fatalf("Compare(0.50/100 sheets, 0.90/50 sheets) = (%d,%v), want (-1,true)", cmp, ok)
	}
}

// TestUnparseableIsNotRankedCheapest guards the exact failure mode a zero
// fallback would produce: an unparseable measure sorting to the front of a
// price-ascending list.
func TestUnparseableIsNotRankedCheapest(t *testing.T) {
	type candidate struct {
		name    string
		price   float64
		measure string
	}
	input := []candidate{
		{"mystery pack", 3.00, "per widget"},
		{"cheap kilo bag", 25.00, "1KG"},
		{"pricey small jar", 3.00, "100G"},
		{"blank measure", 1.00, ""},
	}

	ranked := make([]Normalized, 0, len(input))
	names := make([]string, 0, len(input))
	excluded := make([]string, 0, len(input))
	for _, c := range input {
		n, ok := Normalize(c.price, c.measure)
		if !ok {
			excluded = append(excluded, c.name)
			continue
		}
		ranked = append(ranked, n)
		names = append(names, c.name)
	}

	if len(excluded) != 2 {
		t.Fatalf("excluded = %v, want the two unparseable candidates", excluded)
	}
	if len(ranked) != 2 {
		t.Fatalf("ranked %d candidates, want 2", len(ranked))
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		cmp, ok := Compare(ranked[i], ranked[j])
		return ok && cmp < 0
	})
	if !nearly(ranked[0].Price, 25) {
		t.Fatalf("cheapest ranked price = %v/%s, want 25/1kg (the 1KG bag); an unparseable row must never lead", ranked[0].Price, ranked[0].Basis)
	}
	for _, name := range excluded {
		for _, kept := range names {
			if name == kept {
				t.Fatalf("excluded candidate %q also appears in the ranking", name)
			}
		}
	}
	t.Logf("ranked=%v excluded=%v", ranked, excluded)
}

func TestCompare(t *testing.T) {
	cheap, _ := Normalize(10, "1KG")
	dear, _ := Normalize(20, "1KG")
	same, _ := Normalize(1, "100G")
	volume, _ := Normalize(10, "1L")

	tests := []struct {
		name    string
		a, b    Normalized
		wantCmp int
		wantOK  bool
	}{
		{"cheaper first", cheap, dear, -1, true},
		{"dearer first", dear, cheap, 1, true},
		{"equal across bases", cheap, same, 0, true},
		{"cross kind", cheap, volume, 0, false},
		{"unknown left", Normalized{}, cheap, 0, false},
		{"unknown right", cheap, Normalized{}, 0, false},
		{"both unknown", Normalized{}, Normalized{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp, ok := Compare(tt.a, tt.b)
			if cmp != tt.wantCmp || ok != tt.wantOK {
				t.Fatalf("Compare = (%d,%v), want (%d,%v)", cmp, ok, tt.wantCmp, tt.wantOK)
			}
		})
	}
}

func TestPercentCheaper(t *testing.T) {
	anchor, _ := Normalize(30, "1KG")
	half, _ := Normalize(15, "1KG")
	dearer, _ := Normalize(45, "1KG")
	viaSmallPack, _ := Normalize(1.50, "100G")
	volume, _ := Normalize(30, "1L")

	tests := []struct {
		name    string
		anchor  Normalized
		alt     Normalized
		wantPct float64
		wantOK  bool
	}{
		{"half price is fifty percent cheaper", anchor, half, 50, true},
		{"cross basis alternative", anchor, viaSmallPack, 50, true},
		{"dearer alternative is negative", anchor, dearer, -50, true},
		{"identical is zero", anchor, anchor, 0, true},
		{"cross kind refuses", anchor, volume, 0, false},
		{"unknown anchor refuses", Normalized{}, half, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pct, ok := PercentCheaper(tt.anchor, tt.alt)
			if ok != tt.wantOK {
				t.Fatalf("PercentCheaper ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && !nearly(pct, tt.wantPct) {
				t.Fatalf("PercentCheaper = %v, want %v", pct, tt.wantPct)
			}
		})
	}
}

func TestParseCupTag(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantPrice   float64
		wantMeasure string
		wantOK      bool
	}{
		{"grams tag", "$2.98/100G", 2.98, "100G", true},
		{"litre tag", "$2.60/1L", 2.6, "1L", true},
		{"no dollar sign", "1.38/100G", 1.38, "100G", true},
		{"spaced", " $ 2.00 / 100G ", 2.00, "100G", true},
		{"each tag", "$1.75/1EA", 1.75, "1EA", true},
		{"empty", "", 0, "", false},
		{"no slash", "$2.98", 0, "", false},
		{"no price", "$/100G", 0, "", false},
		{"zero price", "$0.00/100G", 0, "", false},
		{"no measure", "$2.98/", 0, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, measure, ok := ParseCupTag(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ParseCupTag(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if !nearly(price, tt.wantPrice) || measure != tt.wantMeasure {
				t.Fatalf("ParseCupTag(%q) = (%v,%q), want (%v,%q)", tt.in, price, measure, tt.wantPrice, tt.wantMeasure)
			}
		})
	}
}

// TestParseCupTagRoundTrip checks that a multibuy cup tag lands on the same
// canonical basis as the shelf cup price it must be compared against.
func TestParseCupTagRoundTrip(t *testing.T) {
	price, measure, ok := ParseCupTag("$2.98/100G")
	if !ok {
		t.Fatalf("ParseCupTag returned ok=false")
	}
	offer, ok := Normalize(price, measure)
	if !ok {
		t.Fatalf("Normalize of the parsed tag returned ok=false")
	}
	shelf, ok := Normalize(3.57, "100G")
	if !ok {
		t.Fatalf("Normalize of the shelf price returned ok=false")
	}
	if !Comparable(offer, shelf) {
		t.Fatalf("an offer tag and a shelf cup price on the same measure must be comparable")
	}
	pct, ok := PercentCheaper(shelf, offer)
	if !ok {
		t.Fatalf("PercentCheaper returned not-comparable")
	}
	t.Logf("offer=%s shelf=%s saving=%.2f%%", offer, shelf, pct)
	if pct <= 0 {
		t.Fatalf("offer cup tag $2.98/100G should beat a $3.57/100G shelf price; got %.2f%%", pct)
	}
}

func TestNormalizedString(t *testing.T) {
	n, _ := Normalize(25, "1KG")
	if got := n.String(); got != "$25/1kg" {
		t.Fatalf("Normalized.String() = %q, want %q", got, "$25/1kg")
	}
	if got := (Normalized{}).String(); got != "n/a" {
		t.Fatalf("zero Normalized.String() = %q, want %q", got, "n/a")
	}
}

func TestNormalizeMeasureRejectsZeroValueMeasure(t *testing.T) {
	if _, ok := NormalizeMeasure(5, Measure{}); ok {
		t.Fatalf("NormalizeMeasure on a zero Measure returned ok=true")
	}
	if _, ok := NormalizeMeasure(5, Measure{Kind: KindMass, BaseQuantity: 0, Unit: "g"}); ok {
		t.Fatalf("NormalizeMeasure with BaseQuantity 0 returned ok=true; that would divide by zero")
	}
}

func nearly(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
