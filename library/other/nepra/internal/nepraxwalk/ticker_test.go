// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraxwalk

import (
	"errors"
	"sort"
	"strings"
	"testing"
)

// TestResolveTickerRefusesKEL is the headline safety property. NEPRA writes
// "Kohinoor Energy Limited. (KEL)" for a 131 MW RFO plant; KEL is
// K-Electric's PSX symbol. The bare token must be refused, not picked.
func TestResolveTickerRefusesKEL(t *testing.T) {
	res, err := ResolveTicker("KEL")
	if err == nil {
		t.Fatalf("ResolveTicker(%q) returned %+v with no error; it MUST refuse", "KEL", res)
	}
	if !errors.Is(err, ErrAmbiguousToken) {
		t.Fatalf("error %v does not match ErrAmbiguousToken", err)
	}
	var amb *AmbiguousTokenError
	if !errors.As(err, &amb) {
		t.Fatalf("error %v is not an *AmbiguousTokenError", err)
	}
	if amb.Token != "KEL" {
		t.Errorf("Token = %q, want %q", amb.Token, "KEL")
	}
	if len(amb.Candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(amb.Candidates))
	}
	byKind := map[TokenKind]TokenCandidate{}
	for _, c := range amb.Candidates {
		byKind[c.Kind] = c
	}
	nepra, ok := byKind[TokenNEPRAAcronym]
	if !ok {
		t.Fatal("no NEPRA-acronym candidate")
	}
	if !strings.Contains(nepra.CanonicalName, "Kohinoor") {
		t.Errorf("NEPRA candidate canonical name = %q, want it to name Kohinoor", nepra.CanonicalName)
	}
	if nepra.PSXTicker != "KOHE" {
		t.Errorf("Kohinoor's PSX symbol = %q, want %q", nepra.PSXTicker, "KOHE")
	}
	psx, ok := byKind[TokenPSXTicker]
	if !ok {
		t.Fatal("no PSX-symbol candidate")
	}
	if psx.ParentName != "K-Electric Limited" {
		t.Errorf("PSX candidate parent = %q, want %q", psx.ParentName, "K-Electric Limited")
	}
	// The message must be usable on its own: both readings, so an operator
	// seeing it in a log knows why the join stopped.
	msg := err.Error()
	for _, want := range []string{"KEL", "Kohinoor", "KOHE", "K-Electric"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q does not mention %q", msg, want)
		}
	}
	// Resolving the plant by its full published name still works: refusing the
	// token must not make Kohinoor unreachable.
	m, ok := Resolve(" Kohinoor Energy Limited. (KEL)")
	if !ok {
		t.Fatal("the Kohinoor plant must still resolve by its full published name")
	}
	if m.Row.PSXTicker != "KOHE" {
		t.Errorf("Kohinoor row ticker = %q, want KOHE", m.Row.PSXTicker)
	}
	if got, _ := ResolveTicker("KOHE"); got.PSXTicker != "KOHE" {
		t.Errorf("ResolveTicker(%q) ticker = %q, want the unambiguous route to work", "KOHE", got.PSXTicker)
	}
}

func TestResolveTickerAmbiguous(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantOther string // the company the PSX symbol really belongs to
		wantPlant string // substring of the NEPRA plant it is confused with
	}{
		{"KEL is K-Electric on PSX but Kohinoor at NEPRA", "KEL", "K-Electric Limited", "Kohinoor"},
		{"AGL is Agritech on PSX but Attock Gen at NEPRA", "AGL", "Agritech Limited", "Attock Gen"},
		{"APL is Attock Petroleum on PSX but Atlas Power at NEPRA", "APL", "Attock Petroleum Limited", "Atlas Power"},
		{"SPL is Sitara Peroxide on PSX but Saif Power at NEPRA", "SPL", "Sitara Peroxide Limited", "Saif Power"},
		{"AEL is AEL Textile on PSX but Altern Energy at NEPRA", "AEL", "AEL Textile Limited", "Altern Energy"},
		{"HEPL is two different NEPRA plants", "HEPL", "", "Energy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResolveTicker(tc.token)
			if !errors.Is(err, ErrAmbiguousToken) {
				t.Fatalf("ResolveTicker(%q) err = %v, want ErrAmbiguousToken", tc.token, err)
			}
			var amb *AmbiguousTokenError
			if !errors.As(err, &amb) {
				t.Fatalf("err is not *AmbiguousTokenError")
			}
			if len(amb.Candidates) < 2 {
				t.Errorf("got %d candidates, want at least 2", len(amb.Candidates))
			}
			if amb.Hazard == "" {
				t.Error("no hazard recorded for the refusal")
			}
			joined := amb.Token + " " + err.Error()
			if tc.wantOther != "" && !strings.Contains(joined, tc.wantOther) {
				t.Errorf("refusal does not mention %q: %s", tc.wantOther, joined)
			}
			if !strings.Contains(joined, tc.wantPlant) {
				t.Errorf("refusal does not mention %q: %s", tc.wantPlant, joined)
			}
		})
	}
	// Lower-cased and padded tokens are refused just as firmly.
	if _, err := ResolveTicker("  kel "); !errors.Is(err, ErrAmbiguousToken) {
		t.Errorf(`ResolveTicker("  kel ") err = %v, want ErrAmbiguousToken`, err)
	}
}

// TestAmbiguousTokensAreComplete recomputes ambiguity from the rows themselves
// and asserts the curated ambiguous_tokens list already covers every collision
// the data can produce. A future edit that creates a new collision fails here.
func TestAmbiguousTokensAreComplete(t *testing.T) {
	ix := load()
	if ix.loadErr != nil {
		t.Fatalf("load: %v", ix.loadErr)
	}
	curated := map[string]bool{}
	for _, a := range AmbiguousTokens() {
		curated[a.Token] = true
	}
	for token := range ix.acronyms {
		if curated[token] {
			continue
		}
		if _, collides := derivedCollision(ix, token); collides {
			t.Errorf("token %q collides in the data but is not in ambiguous_tokens", token)
		}
		// Every non-refused acronym must resolve without error.
		if _, err := ResolveTicker(token); err != nil {
			t.Errorf("ResolveTicker(%q) = %v, want a resolution", token, err)
		}
	}
	if got, want := len(curated), 6; got != want {
		t.Errorf("curated ambiguous tokens: got %d, want %d (KEL, AGL, APL, SPL, AEL, HEPL)", got, want)
	}
	// HUBCO appears on two plants (Hub and Narowal) but both are HUBC, so it
	// is deliberately NOT ambiguous.
	res, err := ResolveTicker("HUBCO")
	if err != nil {
		t.Fatalf("ResolveTicker(%q) = %v; one parent on both plants is not an ambiguity", "HUBCO", err)
	}
	if res.PSXTicker != "HUBC" || len(res.Plants) != 2 {
		t.Errorf("HUBCO resolved to ticker %q with %d plants, want HUBC with 2", res.PSXTicker, len(res.Plants))
	}
}

func TestResolveTickerOK(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		wantKind   TokenKind
		wantTicker string
		wantParent string
		wantPlants int
		wantStatus ParentStatus
		wantListed ListedStatus
	}{
		{"PSX symbol with several plants", "HUBC", TokenPSXTicker, "HUBC", "The Hub Power Company Limited", 6, StatusHasPlants, StatusListed},
		{"PSX symbol with one plant", "KAPCO", TokenPSXTicker, "KAPCO", "Kot Addu Power Company Limited", 1, StatusHasPlants, StatusListed},
		{"PSX symbol with two plants under one company", "JDWS", TokenPSXTicker, "JDWS", "JDW Sugar Mills Limited", 2, StatusHasPlants, StatusListed},
		{"NEPRA acronym that is also its own PSX symbol", "NCPL", TokenPSXTicker, "NCPL", "Nishat Chunian Power Limited", 1, StatusHasPlants, StatusListed},
		{"NEPRA acronym for an unlisted plant", "QATPL", TokenNEPRAAcronym, "", "Government of the Punjab", 1, StatusHasPlants, StatusNotListed},
		{"lower case is accepted", "hubc", TokenPSXTicker, "HUBC", "The Hub Power Company Limited", 6, StatusHasPlants, StatusListed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ResolveTicker(tc.token)
			if err != nil {
				t.Fatalf("ResolveTicker(%q) = %v", tc.token, err)
			}
			if res.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", res.Kind, tc.wantKind)
			}
			if res.PSXTicker != tc.wantTicker {
				t.Errorf("PSXTicker = %q, want %q", res.PSXTicker, tc.wantTicker)
			}
			if res.ParentName != tc.wantParent {
				t.Errorf("ParentName = %q, want %q", res.ParentName, tc.wantParent)
			}
			if len(res.Plants) != tc.wantPlants {
				t.Errorf("Plants = %v (%d), want %d", res.Plants, len(res.Plants), tc.wantPlants)
			}
			if res.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", res.Status, tc.wantStatus)
			}
			if res.ListedStatus != tc.wantListed {
				t.Errorf("ListedStatus = %q, want %q", res.ListedStatus, tc.wantListed)
			}
		})
	}
}

func TestResolveTickerUnknown(t *testing.T) {
	for _, token := range []string{"", "   ", "ZZZZ", "OGDC", "PSO"} {
		_, err := ResolveTicker(token)
		if !errors.Is(err, ErrUnknownToken) {
			t.Errorf("ResolveTicker(%q) err = %v, want ErrUnknownToken", token, err)
		}
	}
}

// TestPlantsForParentKElectric is the "absent is not zero" property.
func TestPlantsForParentKElectric(t *testing.T) {
	pp, ok := PlantsForParent("KEL")
	if !ok {
		t.Fatal("PlantsForParent(\"KEL\") returned not-found; K-Electric must be a known but absent parent, so a caller learns the difference")
	}
	if pp.Status != StatusAbsentFromDataset {
		t.Fatalf("Status = %q, want %q", pp.Status, StatusAbsentFromDataset)
	}
	if len(pp.Plants) != 0 {
		t.Errorf("Plants = %v, want empty: K-Electric's fleet is not in these workbooks", pp.Plants)
	}
	if pp.ParentName != "K-Electric Limited" {
		t.Errorf("ParentName = %q, want %q", pp.ParentName, "K-Electric Limited")
	}
	if pp.AbsenceNote == "" {
		t.Fatal("AbsenceNote is empty; an empty plant list with no explanation reads as 'generated nothing'")
	}
	for _, want := range []string{"BQPS", "Korangi", "SITE", "scope"} {
		if !strings.Contains(pp.AbsenceNote, want) {
			t.Errorf("AbsenceNote does not mention %q: %s", want, pp.AbsenceNote)
		}
	}
	if !strings.Contains(pp.AbsenceNote, "NOT a measurement of zero") {
		t.Errorf("AbsenceNote must say the absence is not a zero: %s", pp.AbsenceNote)
	}
	if pp.Caveat == "" || !strings.Contains(pp.Caveat, "Kohinoor") {
		t.Errorf("Caveat = %q, want it to flag the Kohinoor/KEL token collision", pp.Caveat)
	}
	// The absence record is also reachable through ResolveTicker's PSX route,
	// and through the exported list.
	var listed bool
	for _, ab := range AbsentParents() {
		if ab.PSXTicker == "KEL" {
			listed = true
			if len(ab.ProbeTokens) < 3 {
				t.Errorf("ProbeTokens = %v, want the searched tokens recorded", ab.ProbeTokens)
			}
		}
	}
	if !listed {
		t.Error("KEL is not in AbsentParents()")
	}
}

func TestPlantsForParent(t *testing.T) {
	tests := []struct {
		name       string
		ticker     string
		wantOK     bool
		wantStatus ParentStatus
		wantPlants []string
		wantParent string
	}{
		{
			name:       "HUBC gathers the Hub plant, its Narowal subsidiary and its Thar exposure",
			ticker:     "HUBC",
			wantOK:     true,
			wantStatus: StatusHasPlants,
			wantParent: "The Hub Power Company Limited",
			wantPlants: []string{
				"China Power Hub Generation Company (Private) Limited. (CPHGCL)",
				"Hub Power Company (HUBCO)",
				"Narowal Energy Ltd.",
				"New Bong Escape Hydropower Project",
				"Thar Energy Limited (TEL)",
				"ThalNova Power Thar (Pvt.) Limited (TPTPL)",
			},
		},
		{
			name:       "KAPCO is one row despite the block suffixes elsewhere",
			ticker:     "KAPCO",
			wantOK:     true,
			wantStatus: StatusHasPlants,
			wantParent: "Kot Addu Power Company Limited",
			wantPlants: []string{"Kot Addu Power Company (KAPCO)"},
		},
		{
			name:       "KOHE is the real listed company behind the KEL plant",
			ticker:     "KOHE",
			wantOK:     true,
			wantStatus: StatusHasPlants,
			wantParent: "Kohinoor Energy Limited",
			wantPlants: []string{"Kohinoor Energy Limited. (KEL)"},
		},
		{
			name:       "LUCK reaches Lucky Electric's Bin Qasim coal plant",
			ticker:     "LUCK",
			wantOK:     true,
			wantStatus: StatusHasPlants,
			wantParent: "Lucky Cement Limited",
			wantPlants: []string{"Lucky Electric Power Company Limited (LEPCL)"},
		},
		{
			name:       "NPL and NCPL stay separate companies",
			ticker:     "NPL",
			wantOK:     true,
			wantStatus: StatusHasPlants,
			wantParent: "Nishat Power Limited",
			wantPlants: []string{"Nishat Power Ltd (NPL)"},
		},
		{
			name:       "NCPL is not NPL",
			ticker:     "NCPL",
			wantOK:     true,
			wantStatus: StatusHasPlants,
			wantParent: "Nishat Chunian Power Limited",
			wantPlants: []string{"Nishat Chunian Power Ltd (NCPL)"},
		},
		{
			name:       "K-Electric is known but absent",
			ticker:     "KEL",
			wantOK:     true,
			wantStatus: StatusAbsentFromDataset,
			wantParent: "K-Electric Limited",
			wantPlants: nil,
		},
		{name: "a listed company with no plant here is not-found", ticker: "OGDC", wantOK: false},
		{name: "an empty ticker is not-found", ticker: "", wantOK: false},
		{name: "a NEPRA acronym is not a PSX symbol here", ticker: "QATPL", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pp, ok := PlantsForParent(tc.ticker)
			if ok != tc.wantOK {
				t.Fatalf("PlantsForParent(%q) ok = %v, want %v", tc.ticker, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if pp.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", pp.Status, tc.wantStatus)
			}
			if pp.ParentName != tc.wantParent {
				t.Errorf("ParentName = %q, want %q", pp.ParentName, tc.wantParent)
			}
			got := make([]string, 0, len(pp.Plants))
			for _, p := range pp.Plants {
				got = append(got, p.CanonicalName)
			}
			sort.Strings(got)
			want := append([]string(nil), tc.wantPlants...)
			sort.Strings(want)
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Errorf("plants =\n  %v\nwant\n  %v", got, want)
			}
			for _, p := range pp.Plants {
				if p.Confidence == ConfidenceLow || p.Confidence == ConfidenceUnattributed {
					t.Errorf("plant %q carries ticker %q at confidence %q; weak rows must not assert tickers", p.CanonicalName, p.PSXTicker, p.Confidence)
				}
			}
		})
	}
}

// TestTickerAttributionIsVerifiable pins the listed operators the crosswalk is
// willing to assert, so an unreviewed row cannot slip in.
func TestTickerAttributionIsVerifiable(t *testing.T) {
	want := []string{"ALTN", "ENGRO", "EPQL", "FFC", "HUBC", "JDWS", "KAPCO", "KOHE", "LPL", "LUCK", "NCPL", "NPL", "PKGP", "SPWL", "TICL"}
	got := Tickers()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Tickers() =\n  %v\nwant\n  %v", got, want)
	}
	// Every asserted ticker must come with a parent name and evidence, and no
	// low-confidence row may carry one.
	for _, r := range Rows() {
		if !r.HasTicker() {
			continue
		}
		if !r.HasParent() {
			t.Errorf("%q has ticker %q but no parent name", r.CanonicalName, r.PSXTicker)
		}
		if !strings.Contains(r.Evidence, "dps.psx.com.pk") {
			t.Errorf("%q asserts ticker %q without citing the PSX symbol check in its evidence", r.CanonicalName, r.PSXTicker)
		}
	}
}

// TestConfidenceDistribution reports the crosswalk's honest attribution depth
// and pins it, so a later edit that inflates confidence is visible in a diff.
func TestConfidenceDistribution(t *testing.T) {
	byConf := map[Confidence]int{}
	byListed := map[ListedStatus]int{}
	for _, r := range Rows() {
		byConf[r.Confidence]++
		byListed[r.ListedStatus]++
	}
	wantConf := map[Confidence]int{
		ConfidenceHigh:         29,
		ConfidenceMedium:       29,
		ConfidenceLow:          6,
		ConfidenceUnattributed: 69,
	}
	for k, want := range wantConf {
		if byConf[k] != want {
			t.Errorf("confidence %q: got %d rows, want %d", k, byConf[k], want)
		}
	}
	wantListed := map[ListedStatus]int{
		StatusListed:    21,
		StatusNotListed: 37,
		StatusUnknown:   75,
	}
	for k, want := range wantListed {
		if byListed[k] != want {
			t.Errorf("listed_status %q: got %d rows, want %d", k, byListed[k], want)
		}
	}
	// StatusUnknown must never be silently reported as "not listed".
	for _, r := range Rows() {
		if r.ListedStatus == StatusUnknown && r.HasTicker() {
			t.Errorf("%q has unknown listing status but asserts ticker %q", r.CanonicalName, r.PSXTicker)
		}
	}
}
