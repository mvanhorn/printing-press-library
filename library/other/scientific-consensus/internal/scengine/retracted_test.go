package scengine

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"testing"
)

// This file pins DetectRetraction at the engine level. The CLI-level gate
// (internal/cli/retraction_gate_test.go) covers filterRetracted with four
// cases; the marker forms, the withdrawn variant, and the index false
// positive below are only guarded here.
//
// Design under test, and why it has two tiers:
//
//	declared — the publisher's marker survives in the title. Measured on a
//	           50-work is_retracted:true sample and a 50-work title-search
//	           sample: every title-marked work was also flagged, 41/41, no
//	           false positive.
//	flagged  — only the index says so. Necessary because the worst case
//	           measured (a retracted meta-analysis on vitamin C) carries NO
//	           title marker. Kept separate because the flag demonstrably
//	           over-marks: the 2020 Lancet Commission dementia report is
//	           is_retracted:true although it only received a table
//	           correction, so the UI wording for this tier must not claim the
//	           publisher retracted anything.
//
// DetectRetraction takes the TITLE only, never title+abstract. The evidence
// is testdata/corpora_full/vaccines.json, which TestCorpusPapersAboutRetraction
// reads: two works there discuss the Wakefield retraction in their abstracts.
// Those are papers ABOUT a retraction, not retracted papers.

// retractionCase is one title with the index flag that accompanied it.
type retractionCase struct {
	name  string
	title string
	flag  bool
	want  Retraction
	why   string
}

var retractionCases = []retractionCase{
	// --- POSITIVE: the three works a live production run actually scored ---
	{
		name: "vitaminC_metaanalysis_flag_only",
		title: "Extra Dose of Vitamin C Based on a Daily Supplementation Shortens the Common Cold: " +
			"A Meta-Analysis of 9 Randomized Controlled Trials",
		flag: true,
		want: RetractionFlagged,
		why:  "no title marker; only the index knows. The single most damaging case measured.",
	},
	{
		name:  "covid_greenfoods_uppercase_colon",
		title: "RETRACTED: Coronavirus disease (COVID\u201019) and immunity booster green foods: A mini review",
		flag:  true,
		want:  RetractionDeclared,
		why:   "publisher marker, uppercase, colon",
	},
	{
		name: "fasting_natcomm_article_variant",
		title: "RETRACTED ARTICLE: Fasting inhibits aerobic glycolysis and proliferation in colorectal cancer " +
			"via the Fdft1-mediated AKT/mTOR/HIF1\u03b1 pathway suppression",
		flag: true,
		want: RetractionDeclared,
		why:  "second publisher convention, measured on a different run",
	},

	// --- POSITIVE: further marker forms seen in the OpenAlex samples ---
	{
		name:  "mixed_case_marker",
		title: "Retracted: Predictive Validity of a Medication Adherence Measure in an Outpatient Setting",
		flag:  true,
		want:  RetractionDeclared,
		why:   "mixed case — this is why the match must be case-insensitive",
	},
	{
		name:  "bracket_form",
		title: "[Retracted] Extra Dose of Vitamin C Based on a Daily Supplementation Shortens the Common Cold",
		flag:  true,
		want:  RetractionDeclared,
		why:   "bracket convention seen on the publisher's own site; not present in OpenAlex display_name",
	},
	{
		name:  "withdrawn_variant",
		title: "WITHDRAWN: Effects of an intervention that was never completed",
		flag:  false,
		want:  RetractionDeclared,
		why:   "withdrawn is a marker in its own right and must not need the index flag",
	},

	// --- NEGATIVE: measured false-positive traps ---
	{
		name:  "paper_about_retractions",
		title: "Misconduct accounts for the majority of retracted scientific publications",
		flag:  false,
		want:  NotRetracted,
		why:   "bibliometrics paper; the word is mid-title, so the start anchor rejects it",
	},
	{
		name:  "topology_homonym",
		title: "Theory of retracts",
		flag:  false,
		want:  NotRetracted,
		why:   "topology term, nothing to do with retraction",
	},
	{
		name:  "retraction_index_paper",
		title: "Retracted Science and the Retraction Index",
		flag:  false,
		want:  NotRetracted,
		why:   "starts with the word but has no colon or bracket — the separator is what rejects it",
	},
	{
		name:  "jupiter_negative_control",
		title: "Rosuvastatin to Prevent Vascular Events in Men and Women with Elevated C-Reactive Protein",
		flag:  false,
		want:  NotRetracted,
		why:   "ordinary trial, verified is_retracted:false",
	},

	// --- The verified index false positive ---
	{
		name:  "lancet_dementia_index_false_positive",
		title: "Dementia prevention, intervention, and care: 2020 report of the Lancet Commission",
		flag:  true,
		want:  RetractionFlagged,
		why: "is_retracted:true but the paper only received a table correction. It must land in " +
			"the softer tier so the UI never claims a retraction that did not happen.",
	},
}

func TestDetectRetraction(t *testing.T) {
	for _, c := range retractionCases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectRetraction(c.title, c.flag)
			if got != c.want {
				t.Errorf("DetectRetraction() = %q, want %q\n  reason: %s\n  title: %s",
					got, c.want, c.why, c.title)
			}
		})
	}
}

// TestRetractionExcludedFromScoring pins the consequence, not just the label:
// anything the detector names must be kept out of the scored corpus, and
// anything it does not name must stay in.
func TestRetractionExcludedFromScoring(t *testing.T) {
	for _, c := range retractionCases {
		got := DetectRetraction(c.title, c.flag)
		if got.ExcludeFromScore() != (c.want != NotRetracted) {
			t.Errorf("%s: ExcludeFromScore() = %v, but the signal is %q",
				c.name, got.ExcludeFromScore(), c.want)
		}
	}
}

// TestRetractionSignalIsNotAStance guards the trap measured in Consensus():
// its stance switch ends in `default: res.Inconclusive++`, so a stance value
// the switch does not know is silently tallied as inconclusive, while the
// work's design still reaches ApexDesign and its citations still reach
// TotalCitations. Modelling retraction as a Stance would therefore look fixed
// and not be. Both types are string-based, so the conversion below always
// compiles; the guard is the runtime comparison, which fails if a retraction
// value is ever made equal to one of the four stance values.
func TestRetractionSignalIsNotAStance(t *testing.T) {
	for _, r := range []Retraction{RetractionDeclared, RetractionFlagged} {
		s := Stance(r)
		if s == StanceSupporting || s == StanceRefuting ||
			s == StanceMixed || s == StanceInconclusive {
			t.Fatalf("retraction signal %q collides with a Stance value", r)
		}
	}
}

// TestKnownGapRetractionNoticesAreNotDetected documents what this detector
// deliberately does NOT catch: the retraction notice itself is a separate
// indexed work, titled e.g. "Retraction Note: ...". "retraction" is not
// "retracted", so the pattern misses it by design. Such notices are not
// evidence either and should eventually be filtered, but that is a different
// rule with its own measurement. The gap is asserted rather than logged, so a
// change that starts catching notices fails here and has to update this test
// on purpose instead of widening the rule unnoticed.
func TestKnownGapRetractionNoticesAreNotDetected(t *testing.T) {
	notices := []string{
		"Retraction Note: efficacy of vitamin C for the prevention and treatment of upper respiratory tract infection",
		"Retraction: Extra Dose of Vitamin C Based on a Daily Supplementation Shortens the Common Cold",
	}
	for _, n := range notices {
		if got := DetectRetraction(n, false); got != NotRetracted {
			t.Errorf("DetectRetraction(%q, false) = %q; the known gap changed, update this test and its rule together",
				n, got)
		}
	}
}

// retractionCorpusStudy is the subset of a stored consensus study this file
// reads. Named apart from other corpus helpers in this package on purpose.
type retractionCorpusStudy struct {
	Title    string `json:"title"`
	Abstract string `json:"abstract"`
}

// TestCorpusPapersAboutRetraction turns the evidence cited in retracted.go
// into a check. The vaccines corpus holds 37 studies; exactly two discuss the
// Wakefield retraction in their abstracts, and one of those also carries the
// word "retraction" mid-title. None of the 37 is a retracted paper, so the
// detector must name none of them — neither from the title nor, if someone
// ever feeds it the abstract, from the abstract.
func TestCorpusPapersAboutRetraction(t *testing.T) {
	raw, err := os.ReadFile("testdata/corpora_full/vaccines.json")
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var corpus struct {
		AllStudies []retractionCorpusStudy `json:"all_studies"`
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode corpus: %v", err)
	}
	if got := len(corpus.AllStudies); got != 37 {
		t.Fatalf("all_studies = %d, want 37; the corpus changed", got)
	}

	mentions := regexp.MustCompile(`(?i)retract`)
	var aboutRetraction []string
	for _, s := range corpus.AllStudies {
		if got := DetectRetraction(s.Title, false); got != NotRetracted {
			t.Errorf("title flagged as %q: %s", got, s.Title)
		}
		if mentions.MatchString(s.Abstract) {
			aboutRetraction = append(aboutRetraction, s.Title)
			if got := DetectRetraction(s.Abstract, false); got != NotRetracted {
				t.Errorf("abstract of %q flagged as %q", s.Title, got)
			}
		}
	}

	sort.Strings(aboutRetraction)
	want := []string{
		"The MMR Vaccine and Autism",
		"The MMR vaccine and autism: Sensation, refutation, retraction, and fraud",
	}
	if len(aboutRetraction) != len(want) {
		t.Fatalf("abstracts mentioning a retraction = %d %q, want %d", len(aboutRetraction), aboutRetraction, len(want))
	}
	for i := range want {
		if aboutRetraction[i] != want[i] {
			t.Errorf("abstract match %d = %q, want %q", i, aboutRetraction[i], want[i])
		}
	}
}
