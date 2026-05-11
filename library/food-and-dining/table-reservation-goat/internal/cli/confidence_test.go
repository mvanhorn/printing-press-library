// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// epsilon is the tolerance for floating-point comparisons in this
// file. logNorm uses log10 and the curated populations don't produce
// values close to representational boundaries, so 1e-9 is plenty.
const epsilon = 1e-9

// floatNear reports whether got and want differ by no more than tol.
func floatNear(got, want, tol float64) bool {
	return math.Abs(got-want) <= tol
}

// placeByState scans curatedPlaces for an entry whose Name + State pair
// matches; helper for the R14 fixture tests. Returns a zero Place and
// fails the test if no match — fixtures depend on the curated registry
// covering Bellevue/Portland/Springfield/Columbia.
func placeByNameState(t *testing.T, name, state string) Place {
	t.Helper()
	for _, p := range curatedPlaces {
		if strings.EqualFold(p.Name, name) && strings.EqualFold(p.State, state) {
			return p
		}
	}
	t.Fatalf("curated registry missing %s, %s", name, state)
	return Place{}
}

// TestLogNorm pins the population normalization curve. Two anchors per
// the spec: logNorm(0)==0 (don't blow up on missing data) and
// logNorm(NYC pop)==1.0 (NYC is the calibrated ceiling). Mid-range
// values land in (0, 1) and a value above NYC clamps to 1.0.
func TestLogNorm(t *testing.T) {
	cases := []struct {
		name  string
		input int
		want  float64
		tol   float64
	}{
		{"zero returns 0", 0, 0.0, 0.0},
		{"negative returns 0", -100, 0.0, 0.0},
		{"NYC pop returns 1.0", nycPopReference, 1.0, epsilon},
		{"above NYC clamps to 1.0", nycPopReference * 2, 1.0, epsilon},
		// Anchor at log10(100000)/log10(NYC+1) — Seattle scale.
		{"100k", 100_000, math.Log10(100_001) / math.Log10(nycPopReference+1), epsilon},
		// 1M ≈ 0.871 — a sanity-check mid-range anchor.
		{"1M", 1_000_000, math.Log10(1_000_001) / math.Log10(nycPopReference+1), epsilon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := logNorm(tc.input)
			if !floatNear(got, tc.want, tc.tol) {
				t.Errorf("logNorm(%d) = %.6f; want %.6f ±%v", tc.input, got, tc.want, tc.tol)
			}
			if got < 0 || got > 1 {
				t.Errorf("logNorm(%d) = %.6f out of [0, 1]", tc.input, got)
			}
		})
	}
}

// TestPopularityPrior_BellevueRanking pins the three-Bellevue ordering:
// WA (pop=151854) > NE (pop=53178) > KY (pop=5563). All three are
// PlaceTierCity so the metro_bonus contribution is 0; all three match
// the LocCity{"bellevue"} input so match_bonus cancels; the
// differentiator is the pop term.
func TestPopularityPrior_BellevueRanking(t *testing.T) {
	wa := placeByNameState(t, "Bellevue", "WA")
	ne := placeByNameState(t, "Bellevue", "NE")
	ky := placeByNameState(t, "Bellevue", "KY")
	input := &LocationInput{Kind: LocKindCity, Specificity: SpecificityLow, Raw: "bellevue", CityName: "bellevue"}

	priorWA := popularityPrior(wa, input)
	priorNE := popularityPrior(ne, input)
	priorKY := popularityPrior(ky, input)

	if !(priorWA > priorNE && priorNE > priorKY) {
		t.Errorf("expected WA > NE > KY; got WA=%.4f NE=%.4f KY=%.4f",
			priorWA, priorNE, priorKY)
	}
}

// TestPopularityPrior_SeattleOverHalf pins that Seattle (large metro +
// name match) lands above 0.5. The test description in U4 calls this
// "large city + metro centroid" — the metro_bonus AND match_bonus
// both fire, plus the pop term contributes ~0.25, summing > 0.5.
func TestPopularityPrior_SeattleOverHalf(t *testing.T) {
	seattle := placeByNameState(t, "Seattle", "WA")
	input := &LocationInput{Kind: LocKindCity, Specificity: SpecificityLow, Raw: "seattle", CityName: "seattle"}
	got := popularityPrior(seattle, input)
	if got <= 0.5 {
		t.Errorf("popularityPrior(Seattle, 'seattle') = %.4f; want > 0.5", got)
	}
}

// TestPopularityPrior_MatchBonusRequiresInput verifies the nil and
// empty-input paths: a nil *LocationInput contributes 0 to match_bonus
// (defends the SourceDefault path where no flag was passed).
func TestPopularityPrior_MatchBonusRequiresInput(t *testing.T) {
	seattle := placeByNameState(t, "Seattle", "WA")
	priorNil := popularityPrior(seattle, nil)
	priorEmpty := popularityPrior(seattle, &LocationInput{})
	priorMatch := popularityPrior(seattle, &LocationInput{CityName: "seattle"})

	if priorMatch <= priorNil {
		t.Errorf("name-matched prior (%.4f) should exceed nil-input prior (%.4f)", priorMatch, priorNil)
	}
	if priorMatch <= priorEmpty {
		t.Errorf("name-matched prior (%.4f) should exceed empty-input prior (%.4f)", priorMatch, priorEmpty)
	}
	if !floatNear(priorNil, priorEmpty, epsilon) {
		t.Errorf("nil and empty input priors should be equal; got %.6f vs %.6f", priorNil, priorEmpty)
	}
}

// TestDecideTier_NoCandidates pins the 0-candidate fork — returns
// TierUnknown and a nil slice. Caller emits the location_unknown
// envelope on this path.
func TestDecideTier_NoCandidates(t *testing.T) {
	input := &LocationInput{Kind: LocKindCity, CityName: "narnia", Specificity: SpecificityLow}
	tier, ranked := decideTier(input, nil)
	if tier != TierUnknown {
		t.Errorf("tier: got %v, want TierUnknown", tier)
	}
	if ranked != nil {
		t.Errorf("ranked: got %v, want nil", ranked)
	}
}

// TestDecideTier_OneCandidateAlwaysHigh — 1 candidate is unambiguous
// by definition. Pinned with both a large-pop and tiny-pop Place to
// guarantee the rule doesn't accidentally depend on prior magnitude.
func TestDecideTier_OneCandidateAlwaysHigh(t *testing.T) {
	cases := []struct {
		name  string
		place Place
		input *LocationInput
	}{
		{"large metro", placeByNameState(t, "Seattle", "WA"), &LocationInput{Kind: LocKindCity, CityName: "seattle", Specificity: SpecificityLow}},
		{"tiny city", placeByNameState(t, "Bellevue", "KY"), &LocationInput{Kind: LocKindCity, CityName: "bellevue", Specificity: SpecificityLow}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tier, ranked := decideTier(tc.input, []Place{tc.place})
			if tier != TierHigh {
				t.Errorf("tier: got %v, want TierHigh", tier)
			}
			if len(ranked) != 1 {
				t.Fatalf("ranked: got len %d, want 1", len(ranked))
			}
			if ranked[0].Place.Slug != tc.place.Slug {
				t.Errorf("ranked[0]: got %q, want %q", ranked[0].Place.Slug, tc.place.Slug)
			}
		})
	}
}

// TestDecideTier_HighSpecificityCollapsesAmbiguity — when input has
// SpecificityHigh (city+state, coords, zip), even a multi-candidate
// hit collapses to HIGH because the input itself already
// disambiguated.
func TestDecideTier_HighSpecificityCollapsesAmbiguity(t *testing.T) {
	wa := placeByNameState(t, "Bellevue", "WA")
	ne := placeByNameState(t, "Bellevue", "NE")
	input := &LocationInput{
		Kind:        LocKindCityState,
		Specificity: SpecificityHigh,
		Raw:         "bellevue, wa",
		CityName:    "bellevue",
		State:       "WA",
	}
	tier, ranked := decideTier(input, []Place{wa, ne})
	if tier != TierHigh {
		t.Errorf("tier: got %v, want TierHigh", tier)
	}
	if len(ranked) != 2 {
		t.Fatalf("ranked: got len %d, want 2", len(ranked))
	}
}

// TestDecideTier_TwoCandidatesBareInputIsLow exercises the U14 rule
// simplification: a bare LocCity input with 2 candidates ALWAYS routes
// to LOW regardless of population ratio. The old population-ratio
// margin (calibrated to Portland F4) was a design mistake — the ranking
// used the full prior but the tier decision used a different score,
// and the MEDIUM "guess and warn" outcome was wrong-city UX for the
// minority-population case. Pinning here so a regression to the
// population-margin path is caught.
func TestDecideTier_TwoCandidatesBareInputIsLow(t *testing.T) {
	input := &LocationInput{Kind: LocKindCity, Specificity: SpecificityLow, Raw: "x", CityName: "x"}
	cases := []struct {
		name      string
		topPop    int
		runnerPop int
	}{
		// All four cases were previously MEDIUM-or-LOW per population
		// ratio; under U14 they all collapse to LOW because the input
		// itself is bare.
		{"high margin (10x pop)", 10_000, 1_000},
		{"mid margin (0.4)", 10_000, 6_000},
		{"low margin (0.2)", 10_000, 8_000},
		{"near-equal", 10_000, 9_999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			top := Place{Slug: "x-top", Name: "X", State: "AA", Population: tc.topPop, Tier: PlaceTierCity}
			runner := Place{Slug: "x-runner", Name: "X", State: "BB", Population: tc.runnerPop, Tier: PlaceTierCity}
			tier, ranked := decideTier(input, []Place{top, runner})
			if tier != TierLow {
				t.Errorf("tier: got %v, want TierLow (bare input + 2 candidates is always LOW)", tier)
			}
			if len(ranked) != 2 {
				t.Fatalf("ranked: got len %d, want 2", len(ranked))
			}
			// Top should still rank first by popularity prior (larger pop
			// wins when other terms cancel); the LOW tier doesn't suppress
			// ranking so the envelope can list them in order.
			if ranked[0].Place.Slug != "x-top" {
				t.Errorf("ranked[0]: got %q, want %q", ranked[0].Place.Slug, "x-top")
			}
		})
	}
}

// TestDecideTier_TwoCandidatesMediumSpecificityIsMedium pins the
// remaining MEDIUM-tier fork: 2 candidates + SpecificityMedium input
// (a metro qualifier like "X metro" that resolved to multiple Places)
// routes to MEDIUM. Rare in practice because most metro qualifiers
// resolve to a single canonical, but pinning it for the rule.
func TestDecideTier_TwoCandidatesMediumSpecificityIsMedium(t *testing.T) {
	input := &LocationInput{Kind: LocKindMetro, Specificity: SpecificityMedium, Raw: "x metro", MetroSlug: "x"}
	top := Place{Slug: "x-top", Name: "X", State: "AA", Population: 10_000, Tier: PlaceTierCity}
	runner := Place{Slug: "x-runner", Name: "X", State: "BB", Population: 1_000, Tier: PlaceTierCity}
	tier, ranked := decideTier(input, []Place{top, runner})
	if tier != TierMedium {
		t.Errorf("tier: got %v, want TierMedium (medium-spec input + 2 candidates)", tier)
	}
	if len(ranked) != 2 {
		t.Fatalf("ranked: got len %d, want 2", len(ranked))
	}
}

// TestDecideTier_ThreePlusBareInputAlwaysLow — 3+ candidates with
// low/medium specificity always route to LOW regardless of margin.
// Pinned with two synthetic shapes: similar-pop (low natural margin)
// and disparate-pop (high natural margin) to prove the rule overrides
// the margin math.
func TestDecideTier_ThreePlusBareInputAlwaysLow(t *testing.T) {
	input := &LocationInput{Kind: LocKindCity, Specificity: SpecificityLow, Raw: "x", CityName: "x"}
	cases := []struct {
		name string
		pops []int
	}{
		{"similar pops", []int{100_000, 95_000, 90_000}},
		{"disparate pops (margin would be high)", []int{1_000_000, 10_000, 1_000}},
		{"four candidates", []int{1_000_000, 500_000, 200_000, 100_000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ps []Place
			for i, pop := range tc.pops {
				ps = append(ps, Place{
					Slug:       "x-" + string(rune('a'+i)),
					Name:       "X",
					State:      strings.ToUpper(string(rune('a'+i))) + "A",
					Population: pop,
					Tier:       PlaceTierCity,
				})
			}
			tier, ranked := decideTier(input, ps)
			if tier != TierLow {
				t.Errorf("tier: got %v, want TierLow", tier)
			}
			if len(ranked) != len(tc.pops) {
				t.Fatalf("ranked: got len %d, want %d", len(ranked), len(tc.pops))
			}
		})
	}
}

// TestDecideTier_R14_F1 — `restaurants list --query 'sushi bellevue'
// --location seattle`. Single-candidate Seattle hit -> HIGH.
func TestDecideTier_R14_F1(t *testing.T) {
	seattle := placeByNameState(t, "Seattle", "WA")
	input := &LocationInput{
		Kind:        LocKindCity,
		Specificity: SpecificityLow,
		Raw:         "seattle",
		CityName:    "seattle",
	}
	tier, ranked := decideTier(input, []Place{seattle})
	if tier != TierHigh {
		t.Errorf("F1 tier: got %v, want TierHigh", tier)
	}
	if len(ranked) != 1 || ranked[0].Place.Slug != "seattle" {
		t.Errorf("F1 ranked: got %+v", ranked)
	}
}

// TestDecideTier_R14_F2 — `availability check 'I Love Sushi Bellevue'
// --location 'bellevue, wa'`. The LocCityState input has
// SpecificityHigh; even if the registry surfaced multiple Bellevues,
// the rule collapses to HIGH. The state filter would normally narrow
// to just Bellevue WA upstream, so we test both single-candidate and
// pre-filter shapes.
func TestDecideTier_R14_F2(t *testing.T) {
	wa := placeByNameState(t, "Bellevue", "WA")
	input := &LocationInput{
		Kind:        LocKindCityState,
		Specificity: SpecificityHigh,
		Raw:         "bellevue, wa",
		CityName:    "bellevue",
		State:       "WA",
	}
	t.Run("post-state-filter (one candidate)", func(t *testing.T) {
		tier, _ := decideTier(input, []Place{wa})
		if tier != TierHigh {
			t.Errorf("F2 (filtered) tier: got %v, want TierHigh", tier)
		}
	})
	t.Run("pre-state-filter (high spec collapses)", func(t *testing.T) {
		ne := placeByNameState(t, "Bellevue", "NE")
		ky := placeByNameState(t, "Bellevue", "KY")
		tier, ranked := decideTier(input, []Place{wa, ne, ky})
		if tier != TierHigh {
			t.Errorf("F2 (unfiltered, high-spec) tier: got %v, want TierHigh", tier)
		}
		if len(ranked) != 3 {
			t.Fatalf("F2 ranked: got len %d, want 3", len(ranked))
		}
	})
}

// TestDecideTier_R14_F3 — `--location bellevue`. Three Bellevues
// surface (WA/NE/KY). Low specificity + 3 candidates -> LOW. Pin the
// ranking: WA leads (highest pop), NE next, KY last.
func TestDecideTier_R14_F3(t *testing.T) {
	wa := placeByNameState(t, "Bellevue", "WA")
	ne := placeByNameState(t, "Bellevue", "NE")
	ky := placeByNameState(t, "Bellevue", "KY")
	input := &LocationInput{
		Kind:        LocKindCity,
		Specificity: SpecificityLow,
		Raw:         "bellevue",
		CityName:    "bellevue",
	}
	// Input order intentionally jumbled to verify the ranker sorts by
	// prior, not by input position.
	tier, ranked := decideTier(input, []Place{ky, wa, ne})
	if tier != TierLow {
		t.Errorf("F3 tier: got %v, want TierLow", tier)
	}
	if len(ranked) != 3 {
		t.Fatalf("F3 ranked: got len %d, want 3", len(ranked))
	}
	if ranked[0].Place.State != "WA" || ranked[1].Place.State != "NE" || ranked[2].Place.State != "KY" {
		t.Errorf("F3 ranking: got [%s, %s, %s]; want [WA, NE, KY]",
			ranked[0].Place.State, ranked[1].Place.State, ranked[2].Place.State)
	}
}

// TestDecideTier_R14_F4 — `--location portland`. Two Portlands surface
// (OR/ME). Under U14's simplified rule, bare LocCity + 2 candidates
// routes to LOW regardless of population gap. The Codex P2-F/P2-G
// adversarial review flagged the prior MEDIUM "guess and warn" outcome
// as wrong-city UX for the minority-population case (Portland ME). The
// envelope path is correct here: agent disambiguates instead of
// silently picking. Ranking still pins OR ahead of ME so the envelope
// candidates list reads in popularity order.
func TestDecideTier_R14_F4(t *testing.T) {
	or := placeByNameState(t, "Portland", "OR")
	me := placeByNameState(t, "Portland", "ME")
	input := &LocationInput{
		Kind:        LocKindCity,
		Specificity: SpecificityLow,
		Raw:         "portland",
		CityName:    "portland",
	}
	tier, ranked := decideTier(input, []Place{me, or})
	if tier != TierLow {
		t.Errorf("F4 tier: got %v, want TierLow (U14: bare 2-candidate is LOW)", tier)
	}
	if len(ranked) != 2 {
		t.Fatalf("F4 ranked: got len %d, want 2", len(ranked))
	}
	if ranked[0].Place.State != "OR" || ranked[1].Place.State != "ME" {
		t.Errorf("F4 ranking: got [%s, %s]; want [OR, ME]",
			ranked[0].Place.State, ranked[1].Place.State)
	}
}

// TestDecideTier_R14_F5 — `--location springfield`. Four Springfields
// surface (MA/IL/MO/OR). 4 candidates + low spec → LOW regardless of
// margin. Ranking pin: MO has the highest curated population (169k),
// MA second (156k).
func TestDecideTier_R14_F5(t *testing.T) {
	ma := placeByNameState(t, "Springfield", "MA")
	il := placeByNameState(t, "Springfield", "IL")
	mo := placeByNameState(t, "Springfield", "MO")
	or := placeByNameState(t, "Springfield", "OR")
	input := &LocationInput{
		Kind:        LocKindCity,
		Specificity: SpecificityLow,
		Raw:         "springfield",
		CityName:    "springfield",
	}
	tier, ranked := decideTier(input, []Place{ma, il, mo, or})
	if tier != TierLow {
		t.Errorf("F5 tier: got %v, want TierLow", tier)
	}
	if len(ranked) != 4 {
		t.Fatalf("F5 ranked: got len %d, want 4", len(ranked))
	}
	if ranked[0].Place.State != "MO" {
		t.Errorf("F5 top: got %q, want MO (largest population)", ranked[0].Place.State)
	}
}

// TestDecideTier_R14_F6 — `--location 'columbia, sc'`. LocCityState +
// state filter narrows to one candidate (Columbia SC) → HIGH.
func TestDecideTier_R14_F6(t *testing.T) {
	sc := placeByNameState(t, "Columbia", "SC")
	input := &LocationInput{
		Kind:        LocKindCityState,
		Specificity: SpecificityHigh,
		Raw:         "columbia, sc",
		CityName:    "columbia",
		State:       "SC",
	}
	tier, ranked := decideTier(input, []Place{sc})
	if tier != TierHigh {
		t.Errorf("F6 tier: got %v, want TierHigh", tier)
	}
	if len(ranked) != 1 || ranked[0].Place.State != "SC" {
		t.Errorf("F6 ranked: got %+v", ranked)
	}
}

// TestBuildEnvelope_BellevueShape pins the full envelope JSON shape
// for the F3 case (3 Bellevues, LOW). Verifies: needs_clarification
// flag, error_kind enum string, candidate name "Bellevue, WA" suffix,
// context_hints carry through, agent_guidance is non-empty.
func TestBuildEnvelope_BellevueShape(t *testing.T) {
	wa := placeByNameState(t, "Bellevue", "WA")
	ne := placeByNameState(t, "Bellevue", "NE")
	ky := placeByNameState(t, "Bellevue", "KY")
	input := &LocationInput{
		Kind:        LocKindCity,
		Specificity: SpecificityLow,
		Raw:         "bellevue",
		CityName:    "bellevue",
	}
	_, ranked := decideTier(input, []Place{wa, ne, ky})
	env := BuildEnvelope(input, ranked, ErrorKindLocationAmbiguous)

	if !env.NeedsClarification {
		t.Error("NeedsClarification: got false, want true")
	}
	if env.ErrorKind != ErrorKindLocationAmbiguous {
		t.Errorf("ErrorKind: got %q, want %q", env.ErrorKind, ErrorKindLocationAmbiguous)
	}
	if env.WhatWasAsked != "bellevue" {
		t.Errorf("WhatWasAsked: got %q, want %q", env.WhatWasAsked, "bellevue")
	}
	if len(env.Candidates) != 3 {
		t.Fatalf("Candidates: got len %d, want 3", len(env.Candidates))
	}
	if env.Candidates[0].Name != "Bellevue, WA" {
		t.Errorf("Candidates[0].Name: got %q, want %q", env.Candidates[0].Name, "Bellevue, WA")
	}
	if env.Candidates[0].State != "WA" {
		t.Errorf("Candidates[0].State: got %q, want %q", env.Candidates[0].State, "WA")
	}
	if len(env.Candidates[0].ContextHints) == 0 {
		t.Error("Candidates[0].ContextHints: got empty, want carried-through hints")
	}
	if env.Candidates[0].ScoreIfPicked == 0 {
		t.Error("Candidates[0].ScoreIfPicked: got 0, want non-zero (prior)")
	}
	if env.Candidates[0].Centroid == [2]float64{0, 0} {
		t.Error("Candidates[0].Centroid: got [0,0], want carried-through centroid")
	}
	if env.AgentGuidance.PreferredRecovery == "" {
		t.Error("AgentGuidance.PreferredRecovery: got empty, want prose")
	}
	if env.AgentGuidance.RerunPattern == "" {
		t.Error("AgentGuidance.RerunPattern: got empty, want command pattern")
	}

	// JSON roundtrip — the shape is a contract for agents.
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"needs_clarification":true`,
		`"error_kind":"location_ambiguous"`,
		`"name":"Bellevue, WA"`,
		`"agent_guidance":`,
		`"preferred_recovery":`,
		`"rerun_pattern":`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q; full payload:\n%s", want, s)
		}
	}
}

// TestBuildEnvelope_EmptyCandidates pins the location_unknown path:
// 0 candidates → envelope with empty candidates list but
// non-empty agent_guidance.
func TestBuildEnvelope_EmptyCandidates(t *testing.T) {
	input := &LocationInput{
		Kind:        LocKindCity,
		Specificity: SpecificityLow,
		Raw:         "narnia",
		CityName:    "narnia",
	}
	env := BuildEnvelope(input, nil, ErrorKindLocationUnknown)
	if !env.NeedsClarification {
		t.Error("NeedsClarification: got false, want true")
	}
	if env.ErrorKind != ErrorKindLocationUnknown {
		t.Errorf("ErrorKind: got %q, want %q", env.ErrorKind, ErrorKindLocationUnknown)
	}
	if len(env.Candidates) != 0 {
		t.Errorf("Candidates: got len %d, want 0", len(env.Candidates))
	}
	if env.AgentGuidance.PreferredRecovery == "" {
		t.Error("AgentGuidance.PreferredRecovery: got empty, want prose")
	}
}

// TestDecorateWithLocationContext_HighTier pins the HIGH case: caller
// gets a LocationResolvedField, no warning.
func TestDecorateWithLocationContext_HighTier(t *testing.T) {
	gc := &GeoContext{
		Origin:     "seattle",
		ResolvedTo: "Seattle, WA",
		Centroid:   [2]float64{47.6062, -122.3321},
		RadiusKm:   75,
		Score:      0.95,
		Source:     SourceExplicitFlag,
	}
	resolved, warning := DecorateWithLocationContext(gc, TierHigh, false)
	if resolved == nil {
		t.Fatal("resolved: got nil, want non-nil")
	}
	if warning != nil {
		t.Errorf("warning: got %+v, want nil", warning)
	}
	if resolved.ResolvedTo != "Seattle, WA" {
		t.Errorf("resolved.ResolvedTo: got %q, want %q", resolved.ResolvedTo, "Seattle, WA")
	}
	if resolved.Source != SourceExplicitFlag {
		t.Errorf("resolved.Source: got %q, want %q", resolved.Source, SourceExplicitFlag)
	}
	if resolved.Tier != ResolutionTierHigh {
		t.Errorf("resolved.Tier: got %q, want %q", resolved.Tier, ResolutionTierHigh)
	}
	if resolved.Reason == "" {
		t.Error("resolved.Reason: got empty, want non-empty")
	}
}

// TestDecorateWithLocationContext_MediumTier pins the MEDIUM case:
// caller gets both fields; warning lists the alternates from
// GeoContext.Alternates.
func TestDecorateWithLocationContext_MediumTier(t *testing.T) {
	gc := &GeoContext{
		Origin:     "portland",
		ResolvedTo: "Portland, OR",
		Centroid:   [2]float64{45.5152, -122.6784},
		RadiusKm:   75,
		Score:      0.7,
		Source:     SourceExplicitFlag,
		Alternates: []Candidate{
			{Name: "Portland, ME", State: "ME", Centroid: [2]float64{43.6591, -70.2568}},
		},
	}
	resolved, warning := DecorateWithLocationContext(gc, TierMedium, false)
	if resolved == nil {
		t.Fatal("resolved: got nil, want non-nil")
	}
	if warning == nil {
		t.Fatal("warning: got nil, want non-nil")
	}
	if resolved.Tier != ResolutionTierMedium {
		t.Errorf("resolved.Tier: got %q, want %q", resolved.Tier, ResolutionTierMedium)
	}
	if warning.Picked != "Portland, OR" {
		t.Errorf("warning.Picked: got %q, want %q", warning.Picked, "Portland, OR")
	}
	if len(warning.Alternates) != 1 || warning.Alternates[0] != "Portland, ME" {
		t.Errorf("warning.Alternates: got %v, want [Portland, ME]", warning.Alternates)
	}
	if warning.Reason == "" {
		t.Error("warning.Reason: got empty, want non-empty")
	}
	if len(resolved.AlternatesConsidered) != 1 {
		t.Errorf("resolved.AlternatesConsidered: got %v, want [Portland, ME]",
			resolved.AlternatesConsidered)
	}
}

// TestDecorateWithLocationContext_LowTierWithBypass pins the forced-
// pick case (LOW + --batch-accept-ambiguous): caller gets both fields,
// the warning flags the bypass.
func TestDecorateWithLocationContext_LowTierWithBypass(t *testing.T) {
	gc := &GeoContext{
		Origin:     "bellevue",
		ResolvedTo: "Bellevue, WA",
		Centroid:   [2]float64{47.6101, -122.2015},
		RadiusKm:   25,
		Score:      0.4,
		Source:     SourceExplicitFlag,
		Alternates: []Candidate{
			{Name: "Bellevue, NE", State: "NE"},
			{Name: "Bellevue, KY", State: "KY"},
		},
	}
	resolved, warning := DecorateWithLocationContext(gc, TierLow, true)
	if resolved == nil {
		t.Fatal("resolved: got nil, want non-nil")
	}
	if warning == nil {
		t.Fatal("warning: got nil, want non-nil")
	}
	if resolved.Tier != ResolutionTierLow {
		t.Errorf("resolved.Tier: got %q, want %q", resolved.Tier, ResolutionTierLow)
	}
	if !strings.Contains(warning.Reason, "forced") {
		t.Errorf("warning.Reason: got %q, want reason mentioning forced pick", warning.Reason)
	}
	if len(warning.Alternates) != 2 {
		t.Errorf("warning.Alternates: got %v, want 2 entries", warning.Alternates)
	}
}

// TestDecorateWithLocationContext_LowTierNoBypass pins the no-bypass
// LOW path: caller is on the envelope path and Decorate should return
// (nil, nil) defensively rather than synthesizing an annotation.
func TestDecorateWithLocationContext_LowTierNoBypass(t *testing.T) {
	gc := &GeoContext{
		Origin:     "bellevue",
		ResolvedTo: "Bellevue, WA",
		Source:     SourceExplicitFlag,
	}
	resolved, warning := DecorateWithLocationContext(gc, TierLow, false)
	if resolved != nil || warning != nil {
		t.Errorf("LOW + no-bypass: got (%+v, %+v); want (nil, nil)", resolved, warning)
	}
}

// TestDecorateWithLocationContext_NilGeoContext pins the no-constraint
// path: nil gc returns (nil, nil).
func TestDecorateWithLocationContext_NilGeoContext(t *testing.T) {
	resolved, warning := DecorateWithLocationContext(nil, TierHigh, false)
	if resolved != nil || warning != nil {
		t.Errorf("nil gc: got (%+v, %+v); want (nil, nil)", resolved, warning)
	}
}

// TestDecorateWithLocationContext_UnknownTier pins the unknown path:
// caller is on the location_unknown envelope path; Decorate returns
// (nil, nil).
func TestDecorateWithLocationContext_UnknownTier(t *testing.T) {
	gc := &GeoContext{Origin: "narnia"}
	resolved, warning := DecorateWithLocationContext(gc, TierUnknown, false)
	if resolved != nil || warning != nil {
		t.Errorf("UNKNOWN tier: got (%+v, %+v); want (nil, nil)", resolved, warning)
	}
}

// TestTierEnum_String pins the lowercase tier names used in JSON
// envelopes and log lines (SKILL.md doc commits to these spellings).
func TestTierEnum_String(t *testing.T) {
	cases := []struct {
		tier TierEnum
		want string
	}{
		{TierHigh, "high"},
		{TierMedium, "medium"},
		{TierLow, "low"},
		{TierUnknown, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.tier.String(); got != tc.want {
				t.Errorf("TierEnum.String: got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestErrorKindConstants pins the error_kind string values. Agents
// branch on these strings without re-parsing prose; renaming any of
// them would break agent integrations.
func TestErrorKindConstants(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{ErrorKindLocationUnknown, "location_unknown"},
		{ErrorKindLocationAmbiguous, "location_ambiguous"},
		{ErrorKindVenueAmbiguous, "venue_ambiguous"},
		{ErrorKindNoResultsInRegion, "no_results_in_region"},
		{ErrorKindResultsOnlyOutside, "results_only_outside_region"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("error kind drift: got %q, want %q", tc.got, tc.want)
			}
		})
	}
}
