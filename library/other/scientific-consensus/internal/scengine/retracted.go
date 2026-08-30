package scengine

import "regexp"

// Retraction is a work's retraction status. It is deliberately NOT a Stance.
//
// Consensus() tallies stances with a switch that ends in
// `default: res.Inconclusive++`, so a stance value the switch does not
// recognise is silently counted as inconclusive rather than rejected — and the
// work's design still reaches ApexDesign and its citations still reach
// TotalCitations, because both are collected before the switch runs. Modelling
// retraction as a stance would therefore look like a fix while leaving a
// retracted paper setting the evidence tier of the whole result. Retraction is
// an independent axis: a retracted work has a stance, and that stance must not
// be scored.
type Retraction string

const (
	// NotRetracted is the zero value, so a work with no signal needs no
	// special construction anywhere.
	NotRetracted Retraction = ""

	// RetractionDeclared means the publisher's marker survives in the title.
	// Measured precision on two independent 50-work OpenAlex samples: every
	// title-marked work was also flagged by the index, 41/41, no false
	// positive. This tier may be stated as fact to the reader.
	RetractionDeclared Retraction = "retracted-declared"

	// RetractionFlagged means only the source index says so. This tier exists
	// because the single most damaging case measured — a retracted
	// meta-analysis on vitamin C and the common cold, retracted in 2023 for
	// double-counting placebo arms — carries no title marker at all and would
	// otherwise be scored as supporting evidence at the highest evidence tier.
	//
	// It is kept separate from RetractionDeclared because the flag
	// demonstrably over-marks: the 2020 Lancet Commission dementia report is
	// is_retracted:true in OpenAlex although it only received a table
	// correction. Reader-facing wording for this tier must therefore not claim
	// the publisher retracted anything.
	RetractionFlagged Retraction = "retracted-flagged"
)

// retractionMarkerRe matches a publisher retraction marker at the START of a
// title, in any case, followed by a separator within a short window.
//
// Every element is there because a measurement put it there:
//
//   - start anchor: two works in testdata/corpora_full/vaccines.json discuss
//     the Wakefield retraction inside their abstracts. They are papers ABOUT a
//     retraction, and only the anchor keeps them out.
//   - case-insensitive: the marker appears as "RETRACTED:", "RETRACTED
//     ARTICLE:" and "Retracted:" in the same OpenAlex sample.
//   - separator required: "Retracted Science and the Retraction Index" is a
//     bibliometrics paper that starts with the word and has no separator. The
//     separator, not the capitalisation, is what rejects it.
//   - optional bracket: publishers also render the marker as "[Retracted]".
//     Not present in OpenAlex display_name, but cheap to accept.
//   - 20-character window: room for " ARTICLE" and similar qualifiers without
//     letting the pattern run into an unrelated colon later in the title.
//
// Deliberately NOT matched: "Retraction Note: ..." — the notice is a separate
// indexed work, "retraction" is not "retracted", and filtering notices is a
// different rule needing its own measurement.
var retractionMarkerRe = regexp.MustCompile(`(?i)^\s*\[?\s*(retracted|withdrawn)\b[^:\]]{0,20}[:\]]`)

// DetectRetraction reports a work's retraction status from its title and the
// source index's retraction flag.
//
// title is the TITLE ALONE, never title+abstract joined: the pattern is
// start-anchored, so joining would anchor on the title and leave the abstract
// silently unguarded.
//
// indexFlag is OpenAlex's is_retracted. The production select list in
// internal/cli/scwork.go must request the field for this argument to carry any
// information; passing false when it was never fetched is safe but reduces
// DetectRetraction to the title rule, which was measured to miss 1 of 3 real
// cases — including the only meta-analysis among them.
func DetectRetraction(title string, indexFlag bool) Retraction {
	if retractionMarkerRe.MatchString(title) {
		return RetractionDeclared
	}
	if indexFlag {
		return RetractionFlagged
	}
	return NotRetracted
}

// ExcludeFromScore reports whether a work carrying this signal must be kept out
// of the scored corpus. Both tiers exclude: a work whose findings may have been
// withdrawn is not evidence, and the cost measured on two live runs is 2 of 175
// and 1 of 135 works. The asymmetry is the reason — presenting a retracted
// paper as supporting evidence is far worse than dropping a correctly published
// one, provided the work stays visible in the study list with its reason
// attached, which is what the PRISMA convention requires.
func (r Retraction) ExcludeFromScore() bool { return r != NotRetracted }

// Label is the reader-facing English string for this signal, empty when there
// is none. Wording is load-bearing: the flagged tier states what is known (an
// index flag) rather than asserting a retraction that may not have happened.
func (r Retraction) Label() string {
	switch r {
	case RetractionDeclared:
		return "Retracted — excluded from the score"
	case RetractionFlagged:
		return "Flagged as retracted in the source index, not confirmed from the title — excluded from the score"
	default:
		return ""
	}
}
