package scengine

// pico_impact_test.go measures what widening the PICO gate's verb recognition
// would do, BEFORE any production code is changed.
//
// Measured law (12/12 across testdata/corpora): the gate fires exactly when
// the claim contains a harm cue. claimSides splits only on claimHarmCues, so a
// benefit-shaped claim ("X improves Y", "X reduces Y") yields nil token lists,
// PICOTokens early-returns nil, and IsPICORelevant's empty-token short circuit
// passes every work. Silently.
//
// This file runs the real PICOTokens + IsPICORelevant against every archived
// corpus, then re-runs the same gate with a LOCAL widened verb splitter
// (fixedPICOTokens below) so the before/after delta is a measurement rather
// than a prediction. Nothing in pico.go or stance.go is touched.
//
// ---------------------------------------------------------------------------
// TWO REASONS THE NUMBERS BELOW ARE NOT A FAITHFUL REPLAY. Read both before
// quoting any figure from this file.
//
// 1. all_studies holds the POST-gate survivors, not the works the gate saw.
//    For a harm-cue claim the excluded works are absent from the archive
//    entirely, so "coffee excluded 15" can never be reproduced here. What IS
//    reproducible is whether the widened splitter treats the survivors the
//    same way the current one does — that is what the idempotency test asserts.
//
// 2. The export truncates every abstract at ~1500 characters, mid-sentence.
//    Measured: 9/14 coffee abstracts, 23/65 omega3 abstracts and 16/29
//    vitamind abstracts sit at the cap. The gate at runtime read the FULL
//    abstract; this file reads the stump. A token that appeared past character
//    1500 is invisible here, so every drop count below is an UPPER BOUND, and
//    a drop whose abstract is truncated is UNRELIABLE rather than informative.
//    Each report therefore separates "dropped, full abstract" (trustworthy)
//    from "dropped, truncated abstract" (suspect).
//
//    This is not hypothetical. Replaying the CURRENT gate over coffee's own
//    survivors excludes "Association of Coffee Consumption With Total and
//    Cause-Specific Mortality Among Nonwhite Populations" — a work that
//    obviously passed at runtime. Its archived abstract is cut at 1500
//    characters and the words "heart" and "disease" fall past the cut.

import (
	"regexp"
	"strings"
	"testing"
)

// abstractTruncationCap is the length at or above which an archived abstract
// is assumed to have been cut by the exporter. The observed cap is 1500; the
// threshold sits slightly below it so a boundary abstract is treated as
// suspect rather than trusted.
const abstractTruncationCap = 1480

// fixedVerbCues is the proposed widened polarity-verb recognizer: the 15 verb
// stems, matched as stems rather than as a fixed inflection list, so
// cause/causes/causing/caused all hit one entry. Direction-neutral by design —
// the gate only needs to know WHERE the claim splits, not which way it points.
// (detectClaimDirection keeps using claimHarmCues; this never replaces it.)
var fixedVerbCues = regexp.MustCompile(`(?i)\b(caus|improv|reduc|increas|decreas|prevent|lower|rais|protect|worsen|treat|affect|promot|contribut|alleviat)\w*`)

// fixedPICOTokens is the proposed PICOTokens, implemented locally so the
// production path stays untouched while it is being measured. It mirrors the
// real one exactly except for which regex locates the split point.
func fixedPICOTokens(claim string) (ivTokens, outTokens []string) {
	lc := strings.ToLower(claim)
	loc := fixedVerbCues.FindStringIndex(lc)
	if loc == nil {
		return nil, nil
	}
	iv := picoSideTokens(lc[:loc[0]])
	out := picoSideTokens(lc[loc[1]:])
	if len(iv) == 0 || len(out) == 0 {
		return nil, nil
	}
	return dropPICOStopwords(iv), dropPICOStopwords(out)
}

// gateOutcome is one corpus measured under one tokenizer.
type gateOutcome struct {
	ivTokens  []string
	outTokens []string
	active    bool // false when the tokenizer could not split the claim
	kept      int  // works that pass the gate

	// dropped splits by whether the archived abstract was truncated, because
	// a drop decided on a stump is an artifact of the export, not a gate
	// decision. droppedFull is the number worth reasoning about.
	droppedFull      int
	droppedTruncated int
	// droppedTitles carries every excluded title, truncated ones marked.
	droppedTitles []string
}

func (g gateOutcome) dropped() int { return g.droppedFull + g.droppedTruncated }

// state renders the tokenizer's status for the log.
func (g gateOutcome) state() string {
	if g.active {
		return "ACTIVE "
	}
	return "SKIPPED"
}

// runGate applies IsPICORelevant to every archived study with the given token
// lists and reports what the gate would do.
func runGate(res corpusResult, iv, out []string) gateOutcome {
	g := gateOutcome{ivTokens: iv, outTokens: out, active: len(iv) > 0 && len(out) > 0}
	for _, s := range res.AllStudies {
		if IsPICORelevant(s.Abstract, s.Title, iv, out) {
			g.kept++
			continue
		}
		if len(s.Abstract) >= abstractTruncationCap {
			g.droppedTruncated++
			g.droppedTitles = append(g.droppedTitles, "[TRUNCATED ABSTRACT — unreliable] "+s.Title)
			continue
		}
		g.droppedFull++
		g.droppedTitles = append(g.droppedTitles, s.Title)
	}
	return g
}

// truncatedCount reports how much of a corpus reaches this file as a stump.
func truncatedCount(res corpusResult) int {
	n := 0
	for _, s := range res.AllStudies {
		if len(s.Abstract) >= abstractTruncationCap {
			n++
		}
	}
	return n
}

// TestPICOImpact is the before/after measurement. Log-only: it changes no
// behavior and asserts nothing about the proposed fix, because the fix does
// not exist yet. The assertion lives in TestPICOImpactHarmCorporaIdempotent.
func TestPICOImpact(t *testing.T) {
	for _, name := range allCorpora {
		res := mustLoadCorpus(t, name)

		curIV, curOut := PICOTokens(res.Claim)
		cur := runGate(res, curIV, curOut)

		fixIV, fixOut := fixedPICOTokens(res.Claim)
		fix := runGate(res, fixIV, fixOut)

		role := "GENUINE ESTIMATE (gate was silent; archive is the full pre-PICO corpus)"
		if cur.active {
			role = "IDEMPOTENCY CHECK (gate already ran; excluded works are NOT in the archive)"
		}

		n := len(res.AllStudies)
		t.Logf("\n%s  claim=%q\n"+
			"  role     : %s\n"+
			"  archive  : %d studies, %d with a truncated abstract\n"+
			"  current  : %s iv=%v out=%v  -> drops %d (%d full / %d truncated), keeps %d\n"+
			"  fixed    : %s iv=%v out=%v  -> drops %d (%d full / %d truncated), keeps %d\n"+
			"  delta    : %+d works excluded (%+d of them on a full abstract)",
			name, res.Claim,
			role,
			n, truncatedCount(res),
			cur.state(), cur.ivTokens, cur.outTokens, cur.dropped(), cur.droppedFull, cur.droppedTruncated, cur.kept,
			fix.state(), fix.ivTokens, fix.outTokens, fix.dropped(), fix.droppedFull, fix.droppedTruncated, fix.kept,
			fix.dropped()-cur.dropped(), fix.droppedFull-cur.droppedFull)
	}
}

// TestPICOImpactHarmCorporaIdempotent is the guard rail on the proposed fix.
//
// On a claim the current gate already handles, the widened recognizer must
// find the SAME split point and therefore reach the SAME verdict on every
// archived work. The assertion is on the delta, not on the absolute count:
// replaying the current gate over the archive already produces spurious drops
// (truncated abstracts, see the file header), and the fix cannot be blamed for
// those. A non-zero delta means the widened regex matched earlier in the claim
// than claimHarmCues did and cut the claim in the wrong place.
func TestPICOImpactHarmCorporaIdempotent(t *testing.T) {
	for _, name := range allCorpora {
		res := mustLoadCorpus(t, name)
		curIV, curOut := PICOTokens(res.Claim)
		if len(curIV) == 0 || len(curOut) == 0 {
			continue // gate was silent here; nothing to be idempotent about
		}
		cur := runGate(res, curIV, curOut)

		fixIV, fixOut := fixedPICOTokens(res.Claim)
		fix := runGate(res, fixIV, fixOut)

		if fix.dropped() == cur.dropped() {
			t.Logf("%-13s OK  split unchanged: iv=%v out=%v, verdicts identical on all %d survivors",
				name, fixIV, fixOut, len(res.AllStudies))
			continue
		}

		t.Errorf("%s: widened splitter changes the verdict on %d work(s) the current gate handles — "+
			"split point moved (iv=%v out=%v, was iv=%v out=%v). Now excluded:\n    %s",
			name, fix.dropped()-cur.dropped(), fixIV, fixOut, curIV, curOut,
			strings.Join(fix.droppedTitles, "\n    "))
	}
}

// TestPICOImpactControls reports the two controls in full detail.
//
// coffee is the harm-claim control. Its note records 15 works excluded by the
// original gate, but those 15 are not in the archive — all_studies is the
// 11 survivors. "Still 15" is therefore NOT measurable from this fixture; what
// is measurable, and what matters, is that the widened splitter reaches the
// same verdict on all 11.
//
// meditation is the benefit-claim control and the real question of this whole
// exercise: the gate has never run on it, so every excluded title printed here
// is a work the fix would newly remove from the corpus. Each one is listed so
// it can be read. A title that genuinely concerns meditation AND anxiety
// appearing in that list is a regression, not progress — unless it is marked
// TRUNCATED, in which case the archive, not the gate, is at fault.
func TestPICOImpactControls(t *testing.T) {
	for _, name := range []string{"coffee", "meditation"} {
		res := mustLoadCorpus(t, name)
		curIV, curOut := PICOTokens(res.Claim)
		cur := runGate(res, curIV, curOut)
		fixIV, fixOut := fixedPICOTokens(res.Claim)
		fix := runGate(res, fixIV, fixOut)

		t.Logf("\n=== %s (%d archived studies, %d truncated) claim=%q\n"+
			"    current: %s iv=%v out=%v drops %d (%d full / %d truncated)\n"+
			"    fixed  : %s iv=%v out=%v drops %d (%d full / %d truncated)",
			name, len(res.AllStudies), truncatedCount(res), res.Claim,
			cur.state(), cur.ivTokens, cur.outTokens, cur.dropped(), cur.droppedFull, cur.droppedTruncated,
			fix.state(), fix.ivTokens, fix.outTokens, fix.dropped(), fix.droppedFull, fix.droppedTruncated)

		if len(fix.droppedTitles) == 0 {
			t.Logf("    no work would be excluded")
			continue
		}
		t.Logf("    excluded under the widened splitter — READ THESE:")
		for _, title := range fix.droppedTitles {
			t.Logf("      - %s", title)
		}
	}
}

// TestPICOImpactVitaminD tests a specific prediction: that widening the verb
// recognizer does NOT meaningfully fix vitamind's token leakage, because the
// intervention side stems to "vitam", which matches every other vitamin in the
// literature (vitamin C, vitamin E, multivitamin) just as happily as vitamin D.
// The single-letter "D" is dropped by ClaimContentTokens' length floor before
// the gate ever sees it.
//
// If that prediction holds, widening the verb list is necessary but not
// sufficient, and the single-letter-token floor is a separate defect worth its
// own measurement.
func TestPICOImpactVitaminD(t *testing.T) {
	res := mustLoadCorpus(t, "vitamind")
	fixIV, fixOut := fixedPICOTokens(res.Claim)
	fix := runGate(res, fixIV, fixOut)

	t.Logf("vitamind claim=%q -> iv=%v out=%v", res.Claim, fixIV, fixOut)
	t.Logf("  gate would drop %d of %d works (%d full abstract / %d truncated)",
		fix.dropped(), len(res.AllStudies), fix.droppedFull, fix.droppedTruncated)

	// Count works that name SOME vitamin but never vitamin D specifically.
	// These are the ones "vitam" waves through and a tighter token would not.
	leaked := 0
	for _, s := range res.AllStudies {
		text := strings.ToLower(s.Title + " " + s.Abstract)
		if !strings.Contains(text, "vitam") {
			continue
		}
		if strings.Contains(text, "vitamin d") || strings.Contains(text, "cholecalciferol") ||
			strings.Contains(text, "25-hydroxyvitamin") || strings.Contains(text, "25(oh)d") {
			continue
		}
		leaked++
		t.Logf("  leaks through \"vitam\" without naming vitamin D: %s", s.Title)
	}
	t.Logf("  %d/%d works match \"vitam\" but never name vitamin D — "+
		"the widened verb list does not address this; the single-letter token floor does",
		leaked, len(res.AllStudies))
}
