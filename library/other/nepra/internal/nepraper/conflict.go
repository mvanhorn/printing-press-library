package nepraper

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// ConflictClass says what kind of disagreement two same-key observations are.
//
// Classifying is not reconciling: every conflict is returned with both
// observations intact whatever its class. The class exists so a caller can tell
// "the document contradicts itself with no tie-breaker" from "the document
// declared two different definitions".
type ConflictClass int

const (
	// ConflictClassUnset is the zero value and means "not classified". It
	// exists so that a Conflict nobody filled in does not CLAIM to be an
	// undocumented conflict, which is the most serious class this type has.
	// Renumbering the others is safe because [ConflictClass] serialises as a
	// name, never as its iota position.
	ConflictClassUnset ConflictClass = iota
	// ConflictUndocumented is a same-key disagreement the document offers no
	// reason for -- the headline table and the comparison table simply print
	// different numbers. This is the FY2024-25 MEPCO case. Which figure is
	// correct is not determinable from the document.
	ConflictUndocumented
	// ConflictDeclaredVariant is a disagreement between two tables that state
	// different definitions of the same metric, e.g. FY2022-23's SAIFI with
	// and without low-tension interruptions.
	ConflictDeclaredVariant
	// ConflictRounding is a disagreement within 0.5% -- the same figure
	// printed to different precision (369.159 vs 369.16).
	ConflictRounding
	// ConflictKindMismatch is a disagreement where the two observations are
	// not both numeric, e.g. one numeric and one unverified.
	ConflictKindMismatch
	// ConflictTransposition is a disagreement whose two figures are made of
	// the SAME DIGITS in a different arrangement: a transposition
	// (68.46 vs 68.64) or a moved decimal point (39.733 vs 39733).
	//
	// It is a strictly stronger statement than "undocumented" and is NEVER
	// filtered out. The registry already describes the LESCO SAIFI case as
	// "a digit transposition, but the document does not say which way round",
	// Direction "unknowable" — so one of the two figures is simply wrong and
	// the source does not say which. That is the opposite of a benign
	// rounding, and it used to be classed as one whenever the two digits
	// swapped happened to be close together: K-Electric's 68.46 vs 68.64 is
	// a relative gap of 0.26%, under the 0.5% tolerance, so it was dropped.
	ConflictTransposition
)

func (c ConflictClass) String() string {
	switch c {
	case ConflictClassUnset:
		return "unset"
	case ConflictUndocumented:
		return "undocumented"
	case ConflictDeclaredVariant:
		return "declared_variant"
	case ConflictRounding:
		return "rounding"
	case ConflictKindMismatch:
		return "kind_mismatch"
	case ConflictTransposition:
		return "transposition"
	}
	return fmt.Sprintf("ConflictClass(%d)", int(c))
}

// conflictClassByName is the inverse of [ConflictClass.String].
var conflictClassByName = map[string]ConflictClass{
	"unset":            ConflictClassUnset,
	"undocumented":     ConflictUndocumented,
	"declared_variant": ConflictDeclaredVariant,
	"rounding":         ConflictRounding,
	"kind_mismatch":    ConflictKindMismatch,
	"transposition":    ConflictTransposition,
}

// MarshalJSON emits the class name rather than its iota position, so that
// inserting a class does not silently renumber a persisted wire format.
func (c ConflictClass) MarshalJSON() ([]byte, error) { return json.Marshal(c.String()) }

// UnmarshalJSON accepts the class name only. A number is refused because the
// iota positions are not a stable wire format.
func (c *ConflictClass) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return fmt.Errorf("nepraper: conflict class must be a name string, not %s: %w", data, err)
	}
	cc, ok := conflictClassByName[name]
	if !ok {
		return fmt.Errorf("nepraper: unknown conflict class %q", name)
	}
	*c = cc
	return nil
}

// Conflict is two published figures for the same Key that do not agree.
//
// Both observations are present with full provenance. There is deliberately no
// "resolved value", no preferred side and no average: for the FY2024-25 MEPCO
// figures the headline table and the comparison table are each internally
// consistent with their own chart, and the document contains no tie-breaker.
// Which is correct is unknowable from the source.
type Conflict struct {
	Key Key
	A   Observation
	B   Observation
	// Field names WHICH of an observation's three published quantities
	// disagreed: "value" (the DISCO's own reported figure), "target" (what
	// NEPRA set or allowed in tariff) or "breach" (the verdict). The three are
	// different quantities and must never be compared with each other, but a
	// same-key contradiction WITHIN any one of them is a real conflict.
	//
	// Only "value" used to be compared at all, so two rows for one key that
	// agreed on the figure while publishing Target 14 vs 140, or Breach
	// "Far Away" vs "Achieved", reported ZERO conflicts.
	Field string
	Class ConflictClass
	// Ratio is A/B, and is nil when the ratio is UNDEFINED — B is zero, or
	// the two sides are not both numeric. It is the most useful single
	// diagnostic when it exists: 2.999 says "one side is 3x the other",
	// 1000.0 says "a decimal point moved".
	//
	// It is a pointer because it used to be a float64 carrying math.Inf(1)
	// for a zero denominator, and Go's encoder REFUSES non-finite floats:
	// json.Marshal of a conflict list containing one returned an error, so
	// the near-universal `b, _ := json.Marshal(...)` wrote an EMPTY document
	// and a real conflict was reported as no conflict at all. A zero
	// denominator is reachable — safety_fatalities, load_shedding and
	// pending_connections are integer counts where a published 0 is ordinary.
	// Read it through [Conflict.RatioValue]; AbsDiff still carries the
	// magnitude when the ratio does not exist.
	Ratio *float64
	// AbsDiff is |A-B| when both values are numeric.
	AbsDiff float64
}

// RatioValue returns A/B and whether it is defined. A false second return
// means the ratio does not exist (a zero denominator or a non-numeric side),
// NOT that it is zero or infinite.
func (c Conflict) RatioValue() (float64, bool) {
	if c.Ratio == nil {
		return 0, false
	}
	return *c.Ratio, true
}

func (c Conflict) String() string {
	ratio := "undefined"
	if r, ok := c.RatioValue(); ok {
		ratio = fmt.Sprintf("%.6f", r)
	}
	av, bv := c.fields()
	return fmt.Sprintf("%s[%s]: %s [%s p%d %s] vs %s [%s p%d %s] class=%s ratio=%s",
		c.Key, c.Field,
		av, c.A.Prov.TableLabel, c.A.Prov.Page, c.A.Prov.Variant,
		bv, c.B.Prov.TableLabel, c.B.Prov.Page, c.B.Prov.Variant,
		c.Class, ratio)
}

// fields returns the two Values this conflict is actually about, which
// depends on Field.
func (c Conflict) fields() (Value, Value) {
	switch c.Field {
	case "target":
		return c.A.Target, c.B.Target
	case "breach":
		return c.A.Breach, c.B.Breach
	default:
		return c.A.Value, c.B.Value
	}
}

// isRoundingOf reports whether the two figures are ONE number printed to two
// precisions — that is, whether the more precise side rounds exactly to the
// less precise side at the precision the less precise side was printed to.
//
// This replaces a relative-gap tolerance of 0.5%, which was a MAGNITUDE test
// masquerading as a precision test. Measured, the same second-decimal digit
// swap was surfaced or dropped purely according to how big the number was and
// which digits happened to swap:
//
//	  28.16 vs   28.61  relative gap 0.0157  surfaced
//	  68.46 vs   68.64  relative gap 0.0026  DROPPED  <- K-Electric SAIFI, a real conflict
//	 100.16 vs  100.61  relative gap 0.0045  DROPPED
//	1000.16 vs 1000.61  relative gap 0.0005  DROPPED
//
// Under this test 369.159 vs 369.16 is rounding (369.159 rounded to 2 decimals
// IS 369.16) while 68.46 vs 68.64 is not, because both were printed to the
// same precision and so neither can be a rounding of the other.
func isRoundingOf(av, bv float64, aRaw, bRaw string) bool {
	da, aok := printedDecimals(aRaw)
	db, bok := printedDecimals(bRaw)
	if !aok || !bok || da == db {
		// Equal printed precision cannot be a rounding: two figures printed
		// to the same number of decimals that still differ are two different
		// figures.
		return false
	}
	coarse, fine, places := bv, av, db
	if da < db {
		coarse, fine, places = av, bv, da
	}
	pow := math.Pow(10, float64(places))
	rounded := math.Round(fine*pow) / pow
	// A generous epsilon for float representation only, NOT a tolerance on
	// the data: the comparison is against an exact rounding.
	return math.Abs(rounded-coarse) < 1e-9
}

// printedDecimals counts the digits a figure was published to the right of its
// decimal point. The second return is false when the raw text has no
// recognisable numeric shape, in which case no precision claim can be made.
func printedDecimals(raw string) (int, bool) {
	// Strip the presentation the reports use: thousands separators, the
	// accounting parentheses that mean a negative, and surrounding space.
	t := strings.TrimSpace(raw)
	t = strings.TrimSuffix(strings.TrimPrefix(t, "("), ")")
	t = strings.ReplaceAll(t, ",", "")
	t = strings.TrimPrefix(t, "-")
	t = strings.TrimPrefix(t, "+")
	if t == "" {
		return 0, false
	}
	intPart, frac, hasDot := strings.Cut(t, ".")
	if !allDigits(intPart) || (hasDot && !allDigits(frac)) {
		return 0, false
	}
	if intPart == "" && frac == "" {
		return 0, false
	}
	if !hasDot {
		return 0, true
	}
	return len(frac), true
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// sameDigits reports whether two published figures are made of the same digits
// in a different order or with the decimal point in a different place. It is
// how a transposition is told apart from an unrelated disagreement: 68.46 and
// 68.64 share the multiset {4,6,6,8}, while 3547.00 and 1182.56 share nothing.
//
// Both sides must have a numeric shape, and figures that are digit-identical
// in the same order are excluded (a pure decimal-point move still counts,
// since 39.733 vs 39733 is the known MEPCO artifact).
func sameDigits(aRaw, bRaw string) bool {
	da, aok := digitMultiset(aRaw)
	db, bok := digitMultiset(bRaw)
	if !aok || !bok || da == "" || da != db {
		return false
	}
	return true
}

// digitMultiset returns the sorted digits of a figure, ignoring separators,
// sign, decimal point and trailing zeros — trailing zeros are pure
// presentation, so 30.67 and 3067.0 must compare as the same digits.
func digitMultiset(raw string) (string, bool) {
	if _, ok := printedDecimals(raw); !ok {
		return "", false
	}
	digits := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] >= '0' && raw[i] <= '9' {
			digits = append(digits, raw[i])
		}
	}
	// Drop leading and trailing zeros: they carry position, not identity.
	for len(digits) > 0 && digits[0] == '0' {
		digits = digits[1:]
	}
	for len(digits) > 0 && digits[len(digits)-1] == '0' {
		digits = digits[:len(digits)-1]
	}
	sort.Slice(digits, func(i, j int) bool { return digits[i] < digits[j] })
	return string(digits), true
}

// Conflicts returns every same-key disagreement inside one report.
//
// It reports ALL of them, including rounding-level ones, and classifies rather
// than filters. Use UndocumentedConflicts for the subset the document offers no
// explanation for.
func Conflicts(r *Report) []Conflict {
	if r == nil {
		return nil
	}
	return conflictsAmong(r.Observations)
}

// ConflictsAcross returns same-key disagreements across several reports as well
// as within each one. This is how a figure republished in a later report's
// comparison table gets checked against the year it was first published in --
// for example MEPCO's FY2020-21 SAIDI, printed as 39733 in the FY2020-21 report
// and as 39.733 in FY2024-25's five-year table.
func ConflictsAcross(reports ...*Report) []Conflict {
	var all []Observation
	for _, r := range reports {
		if r == nil {
			continue
		}
		all = append(all, r.Observations...)
	}
	return conflictsAmong(all)
}

// UndocumentedConflicts is the subset of Conflicts the source gives no reason
// for. These are the ones a caller must never silently resolve.
//
// It returns both [ConflictUndocumented] and [ConflictTransposition]: a
// transposition is a disagreement in which one figure is demonstrably wrong
// and the document does not say which, so it is MORE unexplained than a plain
// undocumented gap, not less. Only [ConflictRounding] (one figure, two
// precisions) and [ConflictDeclaredVariant] (two stated definitions) are
// excluded, because the document itself accounts for those.
func UndocumentedConflicts(r *Report) []Conflict {
	var out []Conflict
	for _, c := range Conflicts(r) {
		switch c.Class {
		case ConflictUndocumented, ConflictTransposition:
			out = append(out, c)
		}
	}
	return out
}

func conflictsAmong(obs []Observation) []Conflict {
	byKey := map[Key][]Observation{}
	var order []Key
	for _, o := range obs {
		k := o.Key()
		if _, seen := byKey[k]; !seen {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], o)
	}
	var out []Conflict
	for _, k := range order {
		group := byKey[k]
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				out = append(out, compare(k, group[i], group[j])...)
			}
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Class != out[b].Class {
			return out[a].Class < out[b].Class
		}
		// An undefined ratio sorts last within its class rather than
		// panicking or comparing as zero.
		ra, aok := out[a].RatioValue()
		rb, bok := out[b].RatioValue()
		if aok != bok {
			return aok
		}
		if !aok {
			return out[a].AbsDiff > out[b].AbsDiff
		}
		return math.Abs(ra-1) > math.Abs(rb-1)
	})
	return out
}

// comparedFields are the three published quantities checked for same-key
// disagreement, in report order.
var comparedFields = [...]struct {
	name string
	pick func(Observation) Value
}{
	{"value", func(o Observation) Value { return o.Value }},
	{"target", func(o Observation) Value { return o.Target }},
	{"breach", func(o Observation) Value { return o.Breach }},
}

// compare reports every field in which two same-key observations disagree. It
// returns a slice because a single pair of rows can contradict itself on the
// figure AND the target AND the verdict, and collapsing those into one
// conflict would hide two of them.
func compare(k Key, a, b Observation) []Conflict {
	var out []Conflict
	for _, f := range comparedFields {
		if c, ok := compareField(k, a, b, f.name, f.pick(a), f.pick(b)); ok {
			out = append(out, c)
		}
	}
	return out
}

func compareField(k Key, a, b Observation, field string, aVal, bVal Value) (Conflict, bool) {
	// An absent cell is a coverage gap, not a disagreement. NEPRA prints "-"
	// for K-Electric's T&D loss in the FY2024-25 headline table and gives the
	// figure in a footnote instead; calling that a conflict with the
	// comparison table would drown the real conflicts in noise.
	if aVal.Kind == KindAbsent || bVal.Kind == KindAbsent {
		return Conflict{}, false
	}
	av, aOK := aVal.Float()
	bv, bOK := bVal.Float()
	c := Conflict{Key: k, A: a, B: b, Field: field}

	switch {
	case aOK && bOK:
		if av == bv {
			return c, false
		}
		c.AbsDiff = math.Abs(av - bv)
		if bv != 0 {
			r := av / bv
			c.Ratio = &r
		}
		switch {
		case declaredVariantPair(a, b):
			c.Class = ConflictDeclaredVariant
		case isRoundingOf(av, bv, aVal.Raw, bVal.Raw):
			c.Class = ConflictRounding
		case sameDigits(aVal.Raw, bVal.Raw):
			c.Class = ConflictTransposition
		default:
			c.Class = ConflictUndocumented
		}
		return c, true
	case aVal.Kind == bVal.Kind && sameDatum(aVal, bVal):
		// Two non-numeric readings of the same datum. Compared CANONICALLY,
		// so NEPRA's "Near to Limit" and "Near Limit" are one verdict rather
		// than a kind mismatch.
		return c, false
	default:
		c.Class = ConflictKindMismatch
		return c, true
	}
}

// sameDatum reports whether two same-kind non-numeric Values say the same
// thing. It prefers the canonical Label when both carry one, because the raw
// text is whatever the PDF's glyphs happened to spell.
func sameDatum(a, b Value) bool {
	if a.Label != "" && b.Label != "" {
		return a.Label == b.Label
	}
	return a.Raw == b.Raw
}

// declaredVariantPair reports whether two observations come from tables that
// state different definitions of the metric. FY2022-23 is the only year in this
// corpus that does so, splitting SAIFI and SAIDI by whether low-tension
// interruptions are counted.
// It is enough for ONE side to carry a declared LT variant. FY2022-23's
// five-year comparison table is unlabelled but reproduces its with-LT figures
// exactly, so the with-LT and without-LT tables both disagree with it by the
// same declared margin; classing those as undocumented would bury the
// genuinely unexplained conflicts.
func declaredVariantPair(a, b Observation) bool {
	lt := func(v string) bool { return v == VariantWithLT || v == VariantWithoutLT }
	if a.Prov.Variant == b.Prov.Variant {
		return false
	}
	if !lt(a.Prov.Variant) && !lt(b.Prov.Variant) {
		return false
	}
	// Scoped to the report that actually DECLARES the split. Without this,
	// one LT-captioned table is enough to downgrade every conflict against it
	// out of UndocumentedConflicts — so a future report that happens to carry
	// an LT caption would silently excuse its own contradictions. FY2022-23 is
	// the only year in this corpus that declares two definitions.
	return ltSplitDeclaredIn(a.ReportFY) || ltSplitDeclaredIn(b.ReportFY)
}

// ltSplitDeclaredFYs are the reports that state two definitions of SAIFI and
// SAIDI, by whether low-tension interruptions are counted. Add a year here
// only on the evidence of that report's own captions.
var ltSplitDeclaredFYs = map[string]bool{"FY2022-23": true}

func ltSplitDeclaredIn(reportFY string) bool { return ltSplitDeclaredFYs[NormalizeFY(reportFY)] }
