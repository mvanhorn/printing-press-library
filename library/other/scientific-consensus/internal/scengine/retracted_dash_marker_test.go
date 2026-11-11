package scengine

import "testing"

// This file pins the spaced-dash separator of retractionMarkerRe.
//
// Evidence: a 1,490-title OpenAlex sample (titles starting with
// retract*/withdraw*, both is_retracted values, plus five 200-work random
// samples of is_retracted:true; 2026-11-11). The colon/bracket rule missed the
// titles below whose marker is followed by a dash instead. Widening to "any
// 20 characters, then a dash" caught the same titles in that sample, but it
// also takes ordinary titles that only start with the word, so the dash is
// accepted only right after the marker word or after one listed noun.
//
// The positive titles are measured; the negative ones are synthetic, because
// the sample held no such title. Without them, the wider pattern passes every
// test in this package.

var retractionDashCases = []retractionCase{
	// --- POSITIVE: measured dash markers the colon/bracket rule missed ---
	{
		name: "noun_then_ascii_dash",
		title: "Retracted Paper - Low-Dose Vitamin D Prevents Muscular Atrophy and Reduces Falls and Hip " +
			"Fractures in Women after Stroke: A Randomized Controlled Trial",
		flag: true,
		want: RetractionDeclared,
		why: "doi 10.1159/000087203, seen in a live Corpova export. The only colon is 90 characters in, " +
			"so the colon rule left it in the softer flagged tier although the publisher marked it.",
	},
	{
		name:  "marker_then_en_dash",
		title: "RETRACTED – Reynolds-number scaling and convergence time scale in two-dimensional Rayleigh–Bénard convection",
		flag:  true,
		want:  RetractionDeclared,
		why:   "OpenAlex sample: marker word, a non-ASCII dash, then the title. Guards the en dash in the class.",
	},
	{
		name:  "withdrawn_then_ascii_dash_no_flag",
		title: "WITHDRAWN - Mixed Methods Sampling",
		flag:  false,
		want:  RetractionDeclared,
		why:   "OpenAlex W2165249494: is_retracted is false, so the title is the only signal left",
	},
	{
		name:  "marker_then_em_dash",
		title: "Withdrawn — Omega-3 supplementation and depressive symptoms: a randomised trial",
		flag:  false,
		want:  RetractionDeclared,
		why:   "synthetic: the third dash form the class accepts",
	},

	// --- NEGATIVE: ordinary titles that start with the word, then a dash ---
	{
		name:  "plural_noun_is_not_a_marker",
		title: "Retracted papers - a bibliometric analysis of oncology journals",
		flag:  false,
		want:  NotRetracted,
		why:   "synthetic: a paper ABOUT retractions. \"papers\" is not in the noun list, so the dash is not a separator here.",
	},
	{
		name:  "free_words_before_dash",
		title: "Withdrawn patients - outcomes in a cohort of 300 adults leaving dialysis",
		flag:  false,
		want:  NotRetracted,
		why:   "synthetic: \"withdrawn\" as an ordinary adjective. A free window before the dash would take it.",
	},
	{
		name:  "free_words_before_en_dash",
		title: "Retracted publications – what can we learn from them?",
		flag:  false,
		want:  NotRetracted,
		why:   "synthetic: plural noun, en dash; same trap as above",
	},
	{
		name:  "hyphen_without_space",
		title: "Retracted-paper citation decay in oncology",
		flag:  false,
		want:  NotRetracted,
		why:   "synthetic: a hyphenated compound is not a separator; the dash needs a space after it",
	},
	{
		name:  "withdrawal_is_not_withdrawn",
		title: "Withdrawal of antidepressants - a review",
		flag:  false,
		want:  NotRetracted,
		why:   "synthetic: the word boundary keeps \"withdrawal\" out",
	},
}

func TestDetectRetractionDashSeparator(t *testing.T) {
	for _, c := range retractionDashCases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectRetraction(c.title, c.flag)
			if got != c.want {
				t.Errorf("DetectRetraction() = %q, want %q\n  reason: %s\n  title: %s",
					got, c.want, c.why, c.title)
			}
			if got.ExcludeFromScore() != (c.want != NotRetracted) {
				t.Errorf("ExcludeFromScore() = %v for signal %q", got.ExcludeFromScore(), got)
			}
		})
	}
}

// TestKnownGapSingularNounAboutRetraction documents the cost this rule
// accepts: a paper about retraction whose title starts with the marker word,
// a listed singular noun and a spaced dash reads exactly like a marker. The
// 1,490-title sample held no such title, while it held real markers of the
// "Retracted Paper - <title>" shape. The gap is asserted rather than logged,
// so a change that closes it has to update this test on purpose.
func TestKnownGapSingularNounAboutRetraction(t *testing.T) {
	title := "Retracted article - a case study of citations after retraction"
	if got := DetectRetraction(title, false); got != RetractionDeclared {
		t.Errorf("DetectRetraction(%q, false) = %q; the known gap changed, update this test and its rule together",
			title, got)
	}
}
