// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraper

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// The FY2024-25 MEPCO reliability conflict, which is the reason this package
// keeps observations rather than values.
//
// SAIDI is published as 3547.00 in Table 06 (PDF p15) and as 1182.56 in
// Table 18 (PDF p29); SAIFI as 30.67 and 10.23. The ratios are 2.999425 and
// 2.998045. Each figure is attested by its own table, and the document
// contains NO tie-breaker: which is correct is unknowable from the source.
//
// So the requirement is not that the package pick well. It is that the package
// never pick at all — and that the conflict reach the caller intact, through
// every accessor and through JSON.
const (
	mepcoSAIDIHeadline   = 3547.00
	mepcoSAIDIComparison = 1182.56
	mepcoSAIFIHeadline   = 30.67
	mepcoSAIFIComparison = 10.23
)

func mepcoKey(metric Metric) Key {
	return Key{PeriodFY: "FY2024-25", Entity: "MEPCO", Metric: metric}
}

// TestConflictSurvivesToTheCaller is the package's central property, and it
// had NO asserting test: Conflicts and ConflictsAcross appeared only in
// non-asserting scratch harnesses, UndocumentedConflicts appeared nowhere, and
// the two helpers written to pin a figure to the table that printed it
// (findObs, mustFloat) were dead code that staticcheck flagged. A regression
// collapsing the conflict to one number would have left every test green.
func TestConflictSurvivesToTheCaller(t *testing.T) {
	r := loadFixture(t, "fy2024-25.spans.tsv").report(t)

	for _, tc := range []struct {
		metric               Metric
		headline, comparison float64
		wantRatio            float64
		// The two metrics are printed in different tables on different
		// pages, which is itself part of the provenance being asserted.
		headlineTable, comparisonTable string
		headlinePage, comparisonPage   int
	}{
		{"saidi", mepcoSAIDIHeadline, mepcoSAIDIComparison, 2.9994249, "Table 6", "Table 18", 15, 29},
		{"saifi", mepcoSAIFIHeadline, mepcoSAIFIComparison, 2.9980450, "Table 5", "Table 17", 13, 28},
	} {
		t.Run(string(tc.metric), func(t *testing.T) {
			k := mepcoKey(tc.metric)

			// BOTH figures must come back, not one.
			obs := Lookup(r, k)
			if len(obs) != 2 {
				var have []string
				for _, o := range obs {
					have = append(have, o.Prov.TableLabel+"="+o.Value.String())
				}
				t.Fatalf("Lookup(%s) returned %d observations, want 2 (%v); a single value means the conflict was resolved", k, len(obs), have)
			}

			// Each must be pinned to the table that printed it.
			a := findObs(t, r, k, tc.headlineTable)
			b := findObs(t, r, k, tc.comparisonTable)
			if got := mustFloat(t, a.Value, tc.headlineTable+" "+string(tc.metric)); got != tc.headline {
				t.Errorf("%s %s = %v, want %v", tc.headlineTable, tc.metric, got, tc.headline)
			}
			if got := mustFloat(t, b.Value, tc.comparisonTable+" "+string(tc.metric)); got != tc.comparison {
				t.Errorf("%s %s = %v, want %v", tc.comparisonTable, tc.metric, got, tc.comparison)
			}
			if a.Prov.Page != tc.headlinePage || b.Prov.Page != tc.comparisonPage {
				t.Errorf("provenance pages = %d and %d, want %d and %d",
					a.Prov.Page, b.Prov.Page, tc.headlinePage, tc.comparisonPage)
			}
			if a.Prov.Variant == b.Prov.Variant {
				t.Errorf("both figures claim variant %q; the two tables are different kinds", a.Prov.Variant)
			}

			// The conflict must be reported, and reported as unexplained.
			var found *Conflict
			for i, c := range UndocumentedConflicts(r) {
				if c.Key == k {
					found = &UndocumentedConflicts(r)[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("UndocumentedConflicts does not include %s; a caller obeying the contract would never see it", k)
			}
			if found.Class != ConflictUndocumented {
				t.Errorf("%s class = %s, want undocumented", k, found.Class)
			}
			ratio, ok := found.RatioValue()
			if !ok {
				t.Fatalf("%s ratio is undefined", k)
			}
			if math.Abs(ratio-tc.wantRatio) > 1e-6 {
				t.Errorf("%s ratio = %.7f, want %.7f", k, ratio, tc.wantRatio)
			}
			// Neither side may be preferred: no field carries a resolution.
			av, _ := found.A.Value.Float()
			bv, _ := found.B.Value.Float()
			if av == bv {
				t.Error("the conflict's two sides are equal; it was reconciled")
			}
			if mid := (tc.headline + tc.comparison) / 2; av == mid || bv == mid {
				t.Error("a side equals the mean of the two figures; the conflict was averaged")
			}
		})
	}
}

// TestAllFY2024_25ConflictsAreSurfaced pins the whole set, because the defect
// that dropped one of them was a classification threshold, not a missing
// conflict: K-Electric's SAIFI 68.46 vs 68.64 is a 0.26% relative gap, which
// fell under a 0.5% "rounding" tolerance and was filtered out of
// UndocumentedConflicts entirely.
func TestAllFY2024_25ConflictsAreSurfaced(t *testing.T) {
	r := loadFixture(t, "fy2024-25.spans.tsv").report(t)

	want := map[string]struct {
		class    ConflictClass
		a, b     float64
		mustShow bool
	}{
		"FY2024-25/MEPCO/saidi":      {ConflictUndocumented, 3547.00, 1182.56, true},
		"FY2024-25/MEPCO/saifi":      {ConflictUndocumented, 30.67, 10.23, true},
		"FY2024-25/LESCO/saifi":      {ConflictTransposition, 28.16, 28.61, true},
		"FY2024-25/K-Electric/saifi": {ConflictTransposition, 68.46, 68.64, true},
	}

	all := Conflicts(r)
	if len(all) != len(want) {
		for _, c := range all {
			t.Logf("  have %s", c)
		}
		t.Fatalf("Conflicts = %d, want %d", len(all), len(want))
	}
	seen := map[string]bool{}
	for _, c := range all {
		w, ok := want[c.Key.String()]
		if !ok {
			t.Errorf("unexpected conflict %s", c)
			continue
		}
		seen[c.Key.String()] = true
		if c.Class != w.class {
			t.Errorf("%s class = %s, want %s", c.Key, c.Class, w.class)
		}
		av, _ := c.A.Value.Float()
		bv, _ := c.B.Value.Float()
		if av != w.a || bv != w.b {
			t.Errorf("%s figures = %v vs %v, want %v vs %v", c.Key, av, bv, w.a, w.b)
		}
	}
	for k := range want {
		if !seen[k] {
			t.Errorf("conflict %s was not reported at all", k)
		}
	}

	// Every one of them is unexplained by the document, so every one must
	// survive the filter a careful caller uses.
	und := UndocumentedConflicts(r)
	if len(und) != 4 {
		for _, c := range und {
			t.Logf("  undocumented: %s", c)
		}
		t.Errorf("UndocumentedConflicts = %d, want 4", len(und))
	}
	for _, c := range und {
		if c.Class == ConflictRounding || c.Class == ConflictDeclaredVariant {
			t.Errorf("%s is classed %s but appears in UndocumentedConflicts", c.Key, c.Class)
		}
	}
}

// TestRoundingRequiresEvidenceOfRounding separates the two ideas the old 0.5%
// tolerance conflated: a small gap, and one figure printed to two precisions.
func TestRoundingRequiresEvidenceOfRounding(t *testing.T) {
	tests := []struct {
		name         string
		aRaw, bRaw   string
		a, b         float64
		wantRounding bool
		wantSameDigs bool
	}{
		{"one figure at two precisions", "369.159", "369.16", 369.159, 369.16, true, false},
		{"same precision, digits swapped", "68.46", "68.64", 68.46, 68.64, false, true},
		{"same precision, digits swapped, larger", "1000.16", "1000.61", 1000.16, 1000.61, false, true},
		{"same precision, digits swapped, smaller", "28.16", "28.61", 28.16, 28.61, false, true},
		{"a moved decimal point keeps its digits", "39.733", "39733", 39.733, 39733, false, true},
		{"unrelated figures", "3547.00", "1182.56", 3547, 1182.56, false, false},
		{"integer vs one decimal, genuine rounding", "3547", "3546.8", 3547, 3546.8, true, false},
		{"integer vs one decimal, not a rounding", "3547", "3546.2", 3547, 3546.2, false, false},
		{"non-numeric raw makes no precision claim", "Away", "68.64", 0, 68.64, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRoundingOf(tc.a, tc.b, tc.aRaw, tc.bRaw); got != tc.wantRounding {
				t.Errorf("isRoundingOf(%s, %s) = %v, want %v", tc.aRaw, tc.bRaw, got, tc.wantRounding)
			}
			// The test must be symmetric: which side is passed first cannot
			// change whether the document rounded.
			if got := isRoundingOf(tc.b, tc.a, tc.bRaw, tc.aRaw); got != tc.wantRounding {
				t.Errorf("isRoundingOf is not symmetric for (%s, %s)", tc.aRaw, tc.bRaw)
			}
			if got := sameDigits(tc.aRaw, tc.bRaw); got != tc.wantSameDigs {
				t.Errorf("sameDigits(%s, %s) = %v, want %v", tc.aRaw, tc.bRaw, got, tc.wantSameDigs)
			}
		})
	}
}

// TestConflictRatioNeverBreaksJSON is the CRITICAL finding as a test. A zero
// denominator produced math.Inf(1) in a plain float64 field; Go's encoder
// refuses non-finite floats, so json.Marshal of the conflict list returned an
// error and the ubiquitous `b, _ := json.Marshal(...)` emitted an EMPTY
// document — a genuine conflict reported as no conflict. A published zero is
// ordinary in safety_fatalities, load_shedding and pending_connections.
func TestConflictRatioNeverBreaksJSON(t *testing.T) {
	base := Observation{ReportFY: "FY2018-19", PeriodFY: "FY2018-19", Entity: "PESCO", Metric: "safety_fatalities"}
	a, b := base, base
	a.Value, a.Prov = Numeric(3, "3"), Provenance{TableLabel: "Table 11", Page: 20, Variant: "headline"}
	b.Value, b.Prov = Numeric(0, "0"), Provenance{TableLabel: "Table 23", Page: 31, Variant: "five_year_comparison"}

	cs := conflictsAmong([]Observation{a, b})
	if len(cs) != 1 {
		t.Fatalf("conflicts = %d, want 1: 3 vs a published 0 is a real disagreement", len(cs))
	}
	c := cs[0]
	if r, ok := c.RatioValue(); ok {
		t.Errorf("ratio = %v with a zero denominator; it must be reported as undefined", r)
	}
	if c.AbsDiff != 3 {
		t.Errorf("AbsDiff = %v, want 3: the magnitude survives even when the ratio does not", c.AbsDiff)
	}
	if c.Class != ConflictUndocumented {
		t.Errorf("class = %s, want undocumented", c.Class)
	}

	// The encoding must succeed, and must round-trip.
	out, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("json.Marshal(conflicts): %v — a caller using `b, _ :=` would emit nothing and report no conflict", err)
	}
	if !bytes.Contains(out, []byte(`"Ratio":null`)) {
		t.Errorf("an undefined ratio did not serialise as null: %s", out)
	}
	var back []Conflict
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("unmarshalling conflicts: %v", err)
	}
	if len(back) != 1 || back[0].Class != ConflictUndocumented {
		t.Fatalf("round trip changed the conflict: %+v", back)
	}
	if _, ok := back[0].RatioValue(); ok {
		t.Error("an undefined ratio came back from JSON as defined")
	}
	if back[0].AbsDiff != 3 {
		t.Errorf("AbsDiff after round trip = %v, want 3", back[0].AbsDiff)
	}

	// A negative numerator must not come back as a POSITIVE infinity either;
	// undefined is the only correct answer.
	n := a
	n.Value = Numeric(-5, "-5")
	for _, c := range conflictsAmong([]Observation{n, b}) {
		if r, ok := c.RatioValue(); ok {
			t.Errorf("-5 vs 0 gave ratio %v, want undefined", r)
		}
	}
}

// TestReportJSONPreservesEveryConflict is the end-to-end version: the real
// fixture's whole report through JSON, with both MEPCO figures still present
// and still disagreeing on the other side.
func TestReportJSONPreservesEveryConflict(t *testing.T) {
	r := loadFixture(t, "fy2024-25.spans.tsv").report(t)

	out, err := json.Marshal(Conflicts(r))
	if err != nil {
		t.Fatalf("marshalling the fixture's conflicts: %v", err)
	}
	var back []Conflict
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if len(back) != len(Conflicts(r)) {
		t.Fatalf("conflicts through JSON = %d, want %d", len(back), len(Conflicts(r)))
	}
	var sawSAIDI bool
	for _, c := range back {
		if c.Key != mepcoKey("saidi") {
			continue
		}
		sawSAIDI = true
		av, aok := c.A.Value.Float()
		bv, bok := c.B.Value.Float()
		if !aok || !bok {
			t.Fatalf("a MEPCO SAIDI figure lost its number in JSON: %+v", c)
		}
		if av != mepcoSAIDIHeadline || bv != mepcoSAIDIComparison {
			t.Errorf("MEPCO SAIDI through JSON = %v vs %v, want %v vs %v", av, bv, mepcoSAIDIHeadline, mepcoSAIDIComparison)
		}
		if c.A.Prov.TableLabel == "" || c.B.Prov.TableLabel == "" {
			t.Error("provenance was lost in JSON; a figure without its table is unattributable")
		}
	}
	if !sawSAIDI {
		t.Error("the MEPCO SAIDI conflict did not survive the JSON round trip")
	}
}

// TestValueJSONNeverPublishesAZeroItWasNotGiven is the JSON projection of the
// package's stated worst failure mode. Num was exported unconditionally, so
// all five non-numeric kinds serialised as "Num":0 — including the truncated
// chart label "19,535." whose entire point is that it must not become a
// number, and TESCO rows NEPRA excluded on the record.
func TestValueJSONNeverPublishesAZeroItWasNotGiven(t *testing.T) {
	for _, v := range []Value{
		{},
		Numeric(3547, "3547.00"),
		Numeric(0, "0.00"),
		Qualitative("Away"),
		Unverified("19,535.", "truncated chart label"),
		Unavailable("404"),
		Excluded("excluded for unreliable metering"),
	} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshalling %s: %v", v.Kind, err)
		}
		if bytes.Equal(b, []byte("{}")) {
			t.Fatalf("%s serialised as {}", v.Kind)
		}
		hasValue := bytes.Contains(b, []byte(`"value"`))
		if v.IsNumeric() != hasValue {
			t.Errorf("%s serialised as %s; a value key must appear only for a numeric kind", v.Kind, b)
		}
		// The kind must travel as a name, not an iota position.
		if !bytes.Contains(b, []byte(`"kind":"`+v.Kind.String()+`"`)) {
			t.Errorf("%s did not serialise its kind as a name: %s", v.Kind, b)
		}
		var back Value
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("unmarshalling %s: %v", b, err)
		}
		wn, wok := v.Float()
		gn, gok := back.Float()
		if wok != gok || wn != gn || back.Kind != v.Kind || back.Raw != v.Raw || back.Reason != v.Reason {
			t.Errorf("round trip of %s changed it: %+v -> %+v", b, v, back)
		}
	}

	// Contradictory and unknown wire values must be refused, not defaulted.
	for _, bad := range []string{
		`{"kind":"absent","value":0}`,
		`{"kind":"numeric"}`,
		`{"kind":"unverified","value":19535}`,
		`{"kind":"probationary"}`,
		`{"kind":1,"value":5}`,
	} {
		var v Value
		if err := json.Unmarshal([]byte(bad), &v); err == nil {
			n, ok := v.Float()
			t.Errorf("%s was accepted as kind %s Float (%v,%v); want an error", bad, v.Kind, n, ok)
		}
	}

	// An Observation's absent Target and Breach must not read as zero.
	o := Observation{ReportFY: "FY2024-25", PeriodFY: "FY2024-25", Entity: "MEPCO", Metric: "saidi",
		Value: Numeric(3547, "3547.00")}
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("marshalling observation: %v", err)
	}
	if bytes.Contains(b, []byte(`"value":0`)) {
		t.Errorf("an absent Target or Breach serialised as a zero value: %s", b)
	}
	if !bytes.Contains(b, []byte(`"Target":{"kind":"absent"}`)) {
		t.Errorf("an absent Target did not serialise as absent: %s", b)
	}
}

// TestHeadlineTableNeverEmitsATargetAsAValue is finding 3: comparison mode was
// entered on the fiscal-year column count alone, so a HEADLINE table whose
// value and target headers both carry the year emitted the TARGET as a VALUE
// under the same Key — manufacturing a conflict between the actual figure and
// the allowed-in-tariff figure, two quantities the domain forbids comparing —
// and wiped Target and Breach to Absent on the way.
func TestHeadlineTableNeverEmitsATargetAsAValue(t *testing.T) {
	cols := []Column{
		{Index: 0, Header: "Reported Figure FY 2024-25", PeriodFY: "FY2024-25", Role: RoleReported},
		{Index: 1, Header: "Target by NEPRA FY 2024-25", PeriodFY: "FY2024-25", Role: RoleTarget},
	}
	row := dataRow{entity: "MEPCO"}
	cells := []string{"3547", "14"}

	headline := &Table{Label: "Table 6", Page: 15, Metric: "saidi", Variant: VariantHeadline, Columns: cols}
	got := buildObservations(row, cells, cols, headline, "FY2024-25", nil)
	for _, o := range got {
		if v, ok := o.Value.Float(); ok && v == 14 {
			t.Errorf("the TARGET 14 was emitted as a VALUE for %s from a headline table", o.Key())
		}
	}
	if len(got) > 1 {
		var vals []string
		for _, o := range got {
			vals = append(vals, o.Value.String())
		}
		t.Errorf("a headline row produced %d observations (%v); comparison mode must require the declared variant", len(got), vals)
	}
	// And the manufactured conflict must not exist.
	if cs := conflictsAmong(got); len(cs) != 0 {
		t.Errorf("a headline row produced %d self-conflicts: %s", len(cs), cs[0])
	}

	// The variant guard must hold ON ITS OWN, with no help from the Role
	// skip. This is the reviewer's measured scenario: role assignment did
	// NOT identify the target column (both came back Role-unset, with
	// Target and Breach wiped to <absent>), so the ONLY thing standing
	// between a headline table and a manufactured actual-vs-target conflict
	// is the declared variant.
	unroled := []Column{
		{Index: 0, Header: "Reported Figure FY 2024-25", PeriodFY: "FY2024-25"},
		{Index: 1, Header: "Target by NEPRA FY 2024-25", PeriodFY: "FY2024-25"},
	}
	hu := &Table{Label: "Table 6", Page: 15, Metric: "saidi", Variant: VariantHeadline, Columns: unroled}
	gotU := buildObservations(row, cells, unroled, hu, "FY2024-25", nil)
	for _, o := range gotU {
		if v, ok := o.Value.Float(); ok && v == 14 {
			t.Errorf("with roles unassigned, the target 14 was emitted as a VALUE for %s", o.Key())
		}
	}
	if len(gotU) > 1 {
		t.Errorf("a headline row with two unroled fiscal-year columns produced %d observations; want 1", len(gotU))
	}
	if cs := conflictsAmong(gotU); len(cs) != 0 {
		t.Errorf("manufactured a conflict between the actual figure and the allowed-in-tariff figure: %s", cs[0])
	}

	// A genuine comparison table must still emit one observation per year,
	// or the guard has broken the feature it protects.
	multiYear := []Column{
		{Index: 0, Header: "FY 2022-23", PeriodFY: "FY2022-23", Role: RoleReported},
		{Index: 1, Header: "FY 2023-24", PeriodFY: "FY2023-24", Role: RoleReported},
		{Index: 2, Header: "FY 2024-25", PeriodFY: "FY2024-25", Role: RoleReported},
	}
	comparison := &Table{Label: "Table 18", Page: 29, Metric: "saidi", Variant: VariantComparison, Columns: multiYear}
	multi := buildObservations(row, []string{"1000", "1100", "1182.56"}, multiYear, comparison, "FY2024-25", nil)
	if len(multi) != 3 {
		t.Fatalf("comparison table produced %d observations, want 3", len(multi))
	}
	years := map[string]bool{}
	for _, o := range multi {
		years[o.PeriodFY] = true
	}
	if len(years) != 3 {
		t.Errorf("comparison observations cover %d distinct years, want 3", len(years))
	}
	// And a target column inside a comparison table must be skipped, not read.
	withTarget := append(append([]Column{}, multiYear...),
		Column{Index: 3, Header: "Target FY 2024-25", PeriodFY: "FY2024-25", Role: RoleTarget})
	ct := &Table{Label: "Table 18", Page: 29, Metric: "saidi", Variant: VariantComparison, Columns: withTarget}
	withT := buildObservations(row, []string{"1000", "1100", "1182.56", "14"}, withTarget, ct, "FY2024-25", nil)
	for _, o := range withT {
		if v, ok := o.Value.Float(); ok && v == 14 {
			t.Error("a RoleTarget column inside a comparison table was emitted as a value")
		}
	}
	if len(withT) != 3 {
		t.Errorf("comparison table with a target column produced %d observations, want 3", len(withT))
	}
}

// TestTargetAndBreachConflictsAreSurfaced is finding 4. compare() read only
// .Value, so two same-key rows agreeing on the figure while publishing
// Target 14 vs 140 — or Breach "Far Away" vs "Achieved" — reported ZERO
// conflicts. The package keeps the three quantities in separate fields
// precisely because they must never be compared with EACH OTHER, but a
// contradiction WITHIN any one of them is exactly as real as a value conflict.
func TestTargetAndBreachConflictsAreSurfaced(t *testing.T) {
	mk := func(val, tgt, breach Value, table string, page int, variant string) Observation {
		return Observation{
			ReportFY: "FY2024-25", PeriodFY: "FY2024-25", Entity: "MEPCO", Metric: "saidi",
			Value: val, Target: tgt, Breach: breach,
			Prov: Provenance{TableLabel: table, Page: page, Variant: variant},
		}
	}

	t.Run("target disagreement with identical values", func(t *testing.T) {
		a := mk(Numeric(3547, "3547"), Numeric(14, "14"), Absent(), "Table 6", 15, "headline")
		b := mk(Numeric(3547, "3547"), Numeric(140, "140"), Absent(), "Table 18", 29, "five_year_comparison")
		cs := conflictsAmong([]Observation{a, b})
		if len(cs) != 1 {
			t.Fatalf("conflicts = %d, want 1 (the allowed-in-tariff figure contradicts itself): %v", len(cs), cs)
		}
		if cs[0].Field != "target" {
			t.Errorf("Field = %q, want \"target\"", cs[0].Field)
		}
		// The conflict must be about the TARGETS, not the values.
		av, bv := cs[0].fields()
		x, _ := av.Float()
		y, _ := bv.Float()
		if x != 14 || y != 140 {
			t.Errorf("conflict is about %v vs %v, want 14 vs 140", x, y)
		}
		if r, ok := cs[0].RatioValue(); !ok || r != 0.1 {
			t.Errorf("target ratio = (%v,%v), want (0.1,true)", r, ok)
		}
	})

	t.Run("breach verdict disagreement", func(t *testing.T) {
		a := mk(Numeric(3547, "3547"), Absent(), Qualitative("Far Away"), "Table 6", 15, "headline")
		b := mk(Numeric(3547, "3547"), Absent(), Qualitative("Achieved"), "Table 18", 29, "five_year_comparison")
		cs := conflictsAmong([]Observation{a, b})
		if len(cs) != 1 {
			t.Fatalf("conflicts = %d, want 1 (the verdict contradicts itself): %v", len(cs), cs)
		}
		if cs[0].Field != "breach" {
			t.Errorf("Field = %q, want \"breach\"", cs[0].Field)
		}
		if _, ok := cs[0].RatioValue(); ok {
			t.Error("a qualitative disagreement must have no ratio")
		}
	})

	t.Run("all three at once are all reported", func(t *testing.T) {
		a := mk(Numeric(3547, "3547"), Numeric(14, "14"), Qualitative("Far Away"), "Table 6", 15, "headline")
		b := mk(Numeric(1182.56, "1182.56"), Numeric(140, "140"), Qualitative("Achieved"), "Table 18", 29, "five_year_comparison")
		cs := conflictsAmong([]Observation{a, b})
		if len(cs) != 3 {
			t.Fatalf("conflicts = %d, want 3 (one per contradicting field): %v", len(cs), cs)
		}
		fields := map[string]bool{}
		for _, c := range cs {
			fields[c.Field] = true
		}
		for _, want := range []string{"value", "target", "breach"} {
			if !fields[want] {
				t.Errorf("no conflict reported for field %q", want)
			}
		}
	})

	t.Run("an absent target on one side is a coverage gap, not a conflict", func(t *testing.T) {
		// This is why the real fixture stays at 4 conflicts: headline tables
		// publish a target and comparison tables do not.
		a := mk(Numeric(3547, "3547"), Numeric(14, "14"), Absent(), "Table 6", 15, "headline")
		b := mk(Numeric(3547, "3547"), Absent(), Absent(), "Table 18", 29, "five_year_comparison")
		if cs := conflictsAmong([]Observation{a, b}); len(cs) != 0 {
			t.Errorf("conflicts = %d, want 0: an unpublished target is a coverage gap: %v", len(cs), cs)
		}
	})

	t.Run("agreement on all three is silent", func(t *testing.T) {
		a := mk(Numeric(3547, "3547"), Numeric(14, "14"), Qualitative("Far Away"), "Table 6", 15, "headline")
		b := a
		b.Prov = Provenance{TableLabel: "Table 18", Page: 29, Variant: "five_year_comparison"}
		if cs := conflictsAmong([]Observation{a, b}); len(cs) != 0 {
			t.Errorf("conflicts = %d, want 0: %v", len(cs), cs)
		}
	})
}

// TestTESCOExclusionIsNotBackDated is finding 8. The exclusion wording is
// quoted from the FY2024-25 report, but Lookup returned it for ANY report and
// ANY period, stamped with the QUERIED year — so the FY2014-15 report, which
// includes TESCO with 48 real published figures, was made to assert its own
// exclusion in wording published a decade later.
func TestTESCOExclusionIsNotBackDated(t *testing.T) {
	// The year that states it: the exclusion is returned, attributed to that
	// year, and never as a number.
	stated := &Report{FY: TESCOExclusionStatedFY}
	got := Lookup(stated, Key{PeriodFY: TESCOExclusionStatedFY, Entity: EntityTESCO, Metric: MetricSAIDI})
	if len(got) != 1 {
		t.Fatalf("FY%s TESCO lookup returned %d observations, want 1", TESCOExclusionStatedFY, len(got))
	}
	if got[0].Value.Kind != KindExcluded {
		t.Errorf("kind = %s, want excluded", got[0].Value.Kind)
	}
	if _, ok := got[0].Value.Float(); ok {
		t.Error("an excluded entity yielded a number")
	}
	if got[0].Prov.ReportFY != TESCOExclusionStatedFY || got[0].ReportFY != TESCOExclusionStatedFY {
		t.Errorf("attribution = Prov.ReportFY %q / ReportFY %q, want %q for both",
			got[0].Prov.ReportFY, got[0].ReportFY, TESCOExclusionStatedFY)
	}

	// Any other report must NOT be made to assert it.
	for _, fy := range []string{"FY2014-15", "FY2018-19", "FY2020-21", "FY1899-00", ""} {
		r := &Report{FY: fy}
		if out := Lookup(r, Key{PeriodFY: fy, Entity: EntityTESCO, Metric: MetricSAIDI}); len(out) != 0 {
			t.Errorf("FY%s asserts the TESCO exclusion (%d observations, Prov.ReportFY %q); "+
				"that wording is only in the FY%s report",
				fy, len(out), out[0].Prov.ReportFY, TESCOExclusionStatedFY)
		}
	}

	// The FY2014-15 fixture must still carry TESCO's REAL published data,
	// which is the evidence that the back-dated claim was wrong.
	fx := loadFixture(t, "fy2014-15.spans.tsv")
	real := fx.report(t)
	var tesco int
	for _, o := range real.Observations {
		if o.Entity == EntityTESCO {
			tesco++
		}
	}
	if tesco == 0 {
		t.Error("the FY2014-15 fixture has no TESCO observations; the premise of this finding was that it does")
	}
	// And the unscoped helper still reports the exclusion exists somewhere.
	if ExclusionReason(EntityTESCO).Kind != KindExcluded {
		t.Error("ExclusionReason no longer reports the exclusion at all")
	}
	if ExclusionReasonInFY(EntityTESCO, "FY2014-15").Kind != KindAbsent {
		t.Error("ExclusionReasonInFY asserts the exclusion for a year that never stated it")
	}
}

// TestZeroValuesMakeNoClaims is finding 9e: a Conflict nobody filled in used
// to report Class == ConflictUndocumented, the most serious class the type
// has.
func TestZeroValuesMakeNoClaims(t *testing.T) {
	var c Conflict
	if c.Class != ConflictClassUnset {
		t.Errorf("Conflict{}.Class = %s, want unset: an unfilled conflict must not claim to be undocumented", c.Class)
	}
	if c.Class.String() != "unset" {
		t.Errorf("ConflictClassUnset.String() = %q, want %q", c.Class.String(), "unset")
	}
	if _, ok := c.RatioValue(); ok {
		t.Error("Conflict{} reports a defined ratio")
	}
	// An unset class must not survive into the caller-facing filter.
	for _, cl := range []ConflictClass{ConflictClassUnset} {
		if cl == ConflictUndocumented || cl == ConflictTransposition {
			t.Errorf("%s would be returned by UndocumentedConflicts", cl)
		}
	}
	var v Value
	if v.Kind != KindAbsent {
		t.Errorf("Value{}.Kind = %s, want absent", v.Kind)
	}
	if _, ok := v.Float(); ok {
		t.Error("Value{} yields a number")
	}
}

// TestQualitativeVerdictsCompareCanonically is finding 9i. withRawLabel
// overwrote Raw with the canonical label, destroying the evidence of what the
// PDF actually spelled and making two spellings of one verdict compare as a
// KIND mismatch.
func TestQualitativeVerdictsCompareCanonically(t *testing.T) {
	a := ParseFigure("NeartoLimit")
	b := ParseFigure("Near to Limit")
	if a.Raw == b.Raw {
		t.Fatalf("premise wrong: both raws are %q, so there is nothing to canonicalise", a.Raw)
	}
	if a.Label != b.Label || a.Label == "" {
		t.Fatalf("canonical labels differ: %q vs %q", a.Label, b.Label)
	}
	// Raw must survive as the evidence.
	if a.Raw != "NeartoLimit" {
		t.Errorf("Raw = %q, want the source text %q", a.Raw, "NeartoLimit")
	}

	base := Observation{ReportFY: "FY2024-25", PeriodFY: "FY2024-25", Entity: "MEPCO", Metric: "saidi",
		Value: Numeric(3547, "3547")}
	x, y := base, base
	x.Breach, y.Breach = a, b
	x.Prov = Provenance{TableLabel: "Table 6", Page: 15, Variant: "headline"}
	y.Prov = Provenance{TableLabel: "Table 18", Page: 29, Variant: "five_year_comparison"}
	if cs := conflictsAmong([]Observation{x, y}); len(cs) != 0 {
		t.Errorf("two spellings of one verdict reported %d conflicts (%s); they are the same verdict", len(cs), cs[0])
	}

	// But two genuinely different verdicts must still conflict.
	z := base
	z.Breach = ParseFigure("FarAway")
	z.Prov = y.Prov
	if cs := conflictsAmong([]Observation{x, z}); len(cs) != 1 {
		t.Errorf("\"Near to Limit\" vs \"Far Away\" reported %d conflicts, want 1", len(cs))
	}
}

// TestKnownUnverifiedRecordsItsGap is finding 9b: the recorded truncated
// figures all carry an empty Entity, and the corpus survey describes a
// four-year range where only three labels were read. The gap must be VISIBLE
// and must never be filled by interpolation.
func TestKnownUnverifiedRecordsItsGap(t *testing.T) {
	figs := KnownUnverified()
	if len(figs) != 3 {
		t.Fatalf("recorded truncated figures = %d, want 3 (only three labels were actually read)", len(figs))
	}
	for _, f := range figs {
		if f.UnverifiedValue().Kind != KindUnverified {
			t.Errorf("%s %s is not unverified", f.PeriodFY, f.Metric)
		}
		if _, ok := f.UnverifiedValue().Float(); ok {
			t.Errorf("%s %s yielded a number from a truncated label", f.PeriodFY, f.Metric)
		}
		// The empty Entity must be a deliberate, readable state.
		if f.EntityAttributed() {
			t.Errorf("%s claims an entity attribution (%q) it should not have", f.PeriodFY, f.Entity)
		}
	}
	// The believed-but-uncaptured year is recorded rather than invented.
	if len(UnverifiedGapFYs) == 0 {
		t.Error("no known gap is recorded, but the corpus survey covers a wider range than the captured labels")
	}
	captured := map[string]bool{}
	for _, f := range figs {
		captured[f.PeriodFY] = true
	}
	for _, fy := range UnverifiedGapFYs {
		if captured[fy] {
			t.Errorf("FY%s is listed as a gap but a label was captured for it", fy)
		}
	}
}

// TestDeclaredVariantIsScopedToItsReport is finding 9c: one LT-captioned table
// was enough to excuse any conflict against it, in any year.
func TestDeclaredVariantIsScopedToItsReport(t *testing.T) {
	mk := func(reportFY, variant string, n float64) Observation {
		return Observation{
			ReportFY: reportFY, PeriodFY: reportFY, Entity: "MEPCO", Metric: "saifi",
			Value: Numeric(n, "x"), Target: Absent(), Breach: Absent(),
			Prov: Provenance{TableLabel: "T", Variant: variant},
		}
	}
	// FY2022-23 declares the split, so the disagreement is explained.
	a := mk("FY2022-23", VariantWithLT, 30.67)
	b := mk("FY2022-23", VariantComparison, 10.23)
	cs := conflictsAmong([]Observation{a, b})
	if len(cs) != 1 || cs[0].Class != ConflictDeclaredVariant {
		t.Errorf("FY2022-23 LT split: got %v, want one declared_variant conflict", cs)
	}
	// A year that does NOT declare it must not be excused.
	c := mk("FY2024-25", VariantWithLT, 30.67)
	d := mk("FY2024-25", VariantComparison, 10.23)
	cs2 := conflictsAmong([]Observation{c, d})
	if len(cs2) != 1 {
		t.Fatalf("FY2024-25: got %d conflicts, want 1", len(cs2))
	}
	if cs2[0].Class == ConflictDeclaredVariant {
		t.Error("a report that never declared the LT split was allowed to excuse its own contradiction")
	}
}

// TestAvailabilityStatesAreDistinguishable is finding 9f.
func TestAvailabilityStatesAreDistinguishable(t *testing.T) {
	// The 404 year.
	un := AvailabilityFor("FY2023-24")
	if un.Kind != KindUnavailable {
		t.Errorf("FY2023-24 = %s, want unavailable", un.Kind)
	}
	// A year with no record at all is also unavailable, but must say why.
	none := AvailabilityFor("FY1899-00")
	if none.Kind != KindUnavailable {
		t.Errorf("FY1899-00 = %s, want unavailable", none.Kind)
	}
	if none.Reason == un.Reason {
		t.Error("a missing RECORD and a real 404 give the identical reason")
	}
	if !strings.Contains(none.Reason, "absence of a RECORD") {
		t.Errorf("the no-record reason does not distinguish itself: %q", none.Reason)
	}
	// A published year must not look like a missing figure with no explanation.
	pub := AvailabilityFor("FY2024-25")
	if pub.Kind != KindAbsent {
		t.Errorf("FY2024-25 = %s, want absent (no availability problem)", pub.Kind)
	}
	if pub.Reason == "" {
		t.Error("a published year returns a bare Absent with no explanation")
	}
	if _, ok := pub.Float(); ok {
		t.Error("an availability answer yielded a number")
	}
}
