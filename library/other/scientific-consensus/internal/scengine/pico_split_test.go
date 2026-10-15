package scengine

// pico_split_test.go pins WHERE PICOTokens splits a claim. Every expectation
// for a hand-written claim below was annotated by reading the claim (what a
// competent reader takes as intervention and outcome), then written in the
// tokenizer's documented form: stemmed to claimStemLen, short alphabetic words
// glued to the preceding token, stopwords and polarity stems dropped. None of
// them was copied from what the code returns.

import (
	"reflect"
	"testing"
)

// picoSplitCase is one claim with its reader-annotated sides.
type picoSplitCase struct {
	claim string
	iv    []string
	out   []string
}

// assertPICOSplit calls the production PICOTokens and compares both sides.
func assertPICOSplit(t *testing.T, c picoSplitCase) {
	t.Helper()
	iv, out := PICOTokens(c.claim)
	if !reflect.DeepEqual(iv, c.iv) || !reflect.DeepEqual(out, c.out) {
		t.Errorf("PICOTokens(%q) = iv=%q out=%q, want iv=%q out=%q",
			c.claim, iv, out, c.iv, c.out)
	}
}

// TestPICOSplitVerbFirstClaims: a claim that opens with a participle
// ("reduced", "increased", "improved") matches polarityVerbCues at offset 0.
// That match leaves the intervention side empty, so it must be skipped and the
// main verb used instead; otherwise the claim yields nothing and the PICO gate
// is off for it.
func TestPICOSplitVerbFirstClaims(t *testing.T) {
	for _, c := range []picoSplitCase{
		{"reduced sodium intake lowers blood pressure", []string{"sodium", "intake"}, []string{"blood", "pressu"}},
		{"increased screen time causes depression", []string{"screen", "time"}, []string{"depres"}},
		{"improved diet quality reduces mortality", []string{"diet", "qualit"}, []string{"mortal"}},
	} {
		assertPICOSplit(t, c)
	}
}

// TestPICOSplitSkipsSubordinateVerb: a polarity verb right after a relative
// pronoun belongs to a clause that modifies the intervention, so the claim
// splits at the main verb and the clause stays on the intervention side.
//
// The "who" claim is also covered by the empty-side rule ("those who" has no
// content token), so it does not isolate the subordinator rule. A "who" claim
// with a noun before the pronoun cannot be annotated cleanly: picoSideTokens
// glues the three-letter "who" onto that noun ("adults who").
func TestPICOSplitSkipsSubordinateVerb(t *testing.T) {
	for _, c := range []picoSplitCase{
		{"a diet that lowers cholesterol prevents heart disease", []string{"diet", "choles"}, []string{"heart", "diseas"}},
		{"fiber, which lowers cholesterol, prevents heart disease", []string{"fiber", "choles"}, []string{"heart", "diseas"}},
		{"those who reduce salt intake lower their blood pressure", []string{"salt", "intake"}, []string{"blood", "pressu"}},
	} {
		assertPICOSplit(t, c)
	}
}

// TestPICOSplitSkipsSubordinateVerbWhom covers "whom" separately because its
// intervention side cannot be compared exactly: "whom" is four letters and is
// not in claimStopwords, so it survives as a content token. That leak is a
// tokenizer gap, not part of the split rule. The split point is what this test
// pins: the outcome side must be exactly the reader's, and every reader token
// of the intervention must stay on the intervention side.
func TestPICOSplitSkipsSubordinateVerbWhom(t *testing.T) {
	claim := "aspirin in adults for whom treatment remains safe prevents stroke"
	wantIV := []string{"aspiri", "adults", "remain", "safe"}
	wantOut := []string{"stroke"}

	iv, out := PICOTokens(claim)
	if !reflect.DeepEqual(out, wantOut) {
		t.Errorf("PICOTokens(%q) out=%q, want %q (iv=%q)", claim, out, wantOut, iv)
	}
	have := make(map[string]bool, len(iv))
	for _, tok := range iv {
		have[tok] = true
	}
	for _, tok := range wantIV {
		if !have[tok] {
			t.Errorf("PICOTokens(%q) iv=%q is missing reader token %q", claim, iv, tok)
		}
	}
	for _, tok := range wantOut {
		if have[tok] {
			t.Errorf("PICOTokens(%q) iv=%q carries outcome token %q", claim, iv, tok)
		}
	}
	if len(iv) != len(wantIV) {
		t.Logf("iv=%q has tokens beyond the reader's %q (known gap: \"whom\" is not a stopword)", iv, wantIV)
	}
}

// TestPICOSplitLeavesPrepositionalVerbAlone: "caused" after "damage" sits in a
// prepositional phrase of the outcome, after the main verb, so the first match
// still wins and the split does not move.
func TestPICOSplitLeavesPrepositionalVerbAlone(t *testing.T) {
	assertPICOSplit(t, picoSplitCase{
		"green tea protects the liver from damage caused by alcohol",
		[]string{"green tea"}, []string{"liver", "alcoho"},
	})
}

// TestPICOSplitKnownGapRaisins documents what the split rule deliberately does
// NOT fix: "raisins reduce cholesterol" still yields no tokens, so the PICO gate
// stays off for it. "raisins" starts with "rais", which is both a
// polarityVerbCues stem and a claimPolarityPrefixes entry. The gap is asserted
// rather than logged, so a change that starts splitting this claim fails here
// and has to update this test on purpose.
func TestPICOSplitKnownGapRaisins(t *testing.T) {
	claim := "raisins reduce cholesterol"
	if iv, out := PICOTokens(claim); iv != nil || out != nil {
		t.Errorf("PICOTokens(%q) = iv=%q out=%q; the known gap changed, update this test and its rule together",
			claim, iv, out)
	}
}

// picoSplitFrozen is a corpus claim's token pair and the number of its
// archived works IsPICORelevant rejects. The drop counts are measured on
// truncated abstracts of post-gate survivors (see pico_impact_test.go), so
// they are regression guards, not truth.
type picoSplitFrozen struct {
	iv      []string
	out     []string
	dropped int
}

var picoSplitFrozenCorpora = map[string]picoSplitFrozen{
	"cellphones":   {[]string{"cell", "phones"}, []string{"brain", "cancer"}, 3},
	"coffee":       {[]string{"coffee"}, []string{"heart", "diseas"}, 1},
	"fasting":      {[]string{"interm", "fastin"}, []string{"insuli", "sensit"}, 20},
	"meditation":   {[]string{"medita"}, []string{"anxiet"}, 3},
	"melatonin":    {[]string{"melato", "supple"}, []string{"sleep", "qualit", "shift", "worker"}, 10},
	"omega3":       {[]string{"omega"}, []string{"cardio", "health"}, 34},
	"probiotics":   {[]string{"probio"}, []string{"health"}, 2},
	"redmeat_run1": {[]string{"meat"}, []string{"colon", "cancer"}, 0},
	"redmeat_run2": {[]string{"meat"}, []string{"colon", "cancer"}, 0},
	"saffron":      {[]string{"saffro", "extrac"}, []string{"sympto", "mild", "depres"}, 5},
	"sweeteners":   {[]string{"artifi", "sweete"}, []string{"weight", "gain"}, 1},
	"vaccines":     {[]string{"vaccin"}, []string{"autism"}, 2},
	"vitaminc":     {[]string{"vitamin c"}, []string{"common", "cold"}, 0},
	"vitamind":     {[]string{"vitamin d"}, []string{"respir", "infect"}, 9},
}

// TestPICOSplitCorporaTokensFrozen: the split rule must not move any archived
// corpus claim.
func TestPICOSplitCorporaTokensFrozen(t *testing.T) {
	for _, name := range allCorpora {
		want, ok := picoSplitFrozenCorpora[name]
		if !ok {
			t.Errorf("%s: no frozen token pair recorded", name)
			continue
		}
		res := mustLoadCorpus(t, name)
		iv, out := PICOTokens(res.Claim)
		if !reflect.DeepEqual(iv, want.iv) || !reflect.DeepEqual(out, want.out) {
			t.Errorf("%s: PICOTokens(%q) = iv=%q out=%q, want iv=%q out=%q",
				name, res.Claim, iv, out, want.iv, want.out)
		}
	}
}

// TestPICOSplitCorporaDropCountsFrozen: the gate's verdict on every archived
// work must not move either.
func TestPICOSplitCorporaDropCountsFrozen(t *testing.T) {
	for _, name := range allCorpora {
		want, ok := picoSplitFrozenCorpora[name]
		if !ok {
			t.Errorf("%s: no frozen drop count recorded", name)
			continue
		}
		res := mustLoadCorpus(t, name)
		iv, out := PICOTokens(res.Claim)
		if (len(iv) == 0 || len(out) == 0) && want.dropped == 0 {
			continue
		}
		dropped := 0
		for _, s := range res.AllStudies {
			if !IsPICORelevant(s.Abstract, s.Title, iv, out) {
				dropped++
			}
		}
		if dropped != want.dropped {
			t.Errorf("%s: gate drops %d of %d works, want %d (iv=%q out=%q)",
				name, dropped, len(res.AllStudies), want.dropped, iv, out)
		}
	}
}
