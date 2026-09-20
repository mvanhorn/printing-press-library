// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraxwalk

import (
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// Confidence grades the parent_name/psx_ticker attribution on a row. It says
// nothing about the plant identity, which is always certain: canonical names
// and aliases are verbatim published strings.
type Confidence string

const (
	// ConfidenceHigh means the parent is named in the published string itself,
	// or the operator is a wholly owned subsidiary of the named listed company
	// with a primary source cited in Row.Evidence.
	ConfidenceHigh Confidence = "high"
	// ConfidenceMedium means ownership is documented but partial (a JV or
	// minority stake), or the plant-to-company link is domain attribution
	// rather than a quote from the published name.
	ConfidenceMedium Confidence = "medium"
	// ConfidenceLow means the corporate family is evident from branding but the
	// holding vehicle could not be verified. No ticker is asserted on such rows.
	ConfidenceLow Confidence = "low"
	// ConfidenceUnattributed means no parent was researched or research was
	// inconclusive. Not a claim that the plant has no parent.
	ConfidenceUnattributed Confidence = "unattributed"
)

// ListedStatus distinguishes "definitely not on the PSX" from "we do not know",
// so an empty PSXTicker is never silently read as "unlisted".
type ListedStatus string

const (
	// StatusListed means PSXTicker holds a symbol confirmed against
	// dps.psx.com.pk.
	StatusListed ListedStatus = "listed"
	// StatusNotListed means the operator is positively known to have no PSX
	// listing (a federal authority, a GENCO, a provincial vehicle).
	StatusNotListed ListedStatus = "not_listed"
	// StatusUnknown means listing status was not established. This is a real
	// state and must not be collapsed into StatusNotListed.
	StatusUnknown ListedStatus = "unknown"
)

// Observation is one fiscal year's appearance of a plant in a generation
// workbook: where it sat in the sheet and what the sheet said about it.
type Observation struct {
	FY  string `json:"fy"`
	SNo int    `json:"sno"`
	// PublishedName is the cell contents verbatim, including U+00A0 NBSP
	// padding and the newline-plus-indent Excel's HTML export inserts.
	PublishedName string `json:"published_name"`
	Technology    string `json:"technology"`
	Fuel          string `json:"fuel"`
	// InstalledCapacityMWRaw is the published string, thousands separators and
	// all. An empty string means the cell was blank or NBSP-only: NOT REPORTED.
	// It must never be read as zero. The cell can also hold a status sentinel
	// rather than a number — "Export to K.Electric" occupies this column in
	// three FY2020-21 rows — so read [Observation.Capacity] for the state.
	InstalledCapacityMWRaw string `json:"installed_capacity_mw_raw"`
	// InstalledCapacityStateName is [nepraparse.CellState.String] for the
	// capacity cell, recorded at curation time. It is a CHECKED REDUNDANCY,
	// not the runtime source of truth: [Observation.Capacity] re-derives the
	// state from the raw string so behaviour cannot drift from the data, and
	// [Validate] fails the build when the two disagree.
	InstalledCapacityStateName string `json:"installed_capacity_state"`
	// BlockStatusName is [nepraparse.CellState.String] for the plant's
	// TWELVE-MONTH GENERATION BLOCK in this fiscal year: "numeric" for
	// ordinary data, "delicensed" or "decommissioned" for a plant that has
	// stopped generating.
	//
	// This field is the whole reason this struct is not derivable from
	// columns 0-4 of the sheet. The status lives in the monthly block, so the
	// first cut of crosswalk.json — which re-extracted only the first five
	// columns — dropped it, and presented all 13 non-operating FY2023-24
	// plants (3,880 MW, including the DELICENSED 1,292 MW Hub Power Company)
	// as live capacity.
	//
	// An empty value is NOT operating: it means the status was never
	// recorded, and [Observation.Operating] reports it as unknown rather than
	// guessing. Read it through [Observation.BlockStatus].
	BlockStatusName string `json:"block_status"`
}

// Capacity is the installed-capacity cell as a typed [nepraparse.Value]: a
// number when the workbook published one, and otherwise an explicit state
// saying WHY there is no number.
//
// This is what keeps four materially different cells apart, all of which the
// earlier (float64, bool) accessor collapsed into an indistinguishable
// (0, false):
//
//	""                      -> StateNotReported       (blank/NBSP: no measurement)
//	"Export to K.Electric"  -> StateExportToKElectric (a regulatory disposition)
//	"0"                     -> StateNumeric 0         (a MEASURED zero)
//	anything unmodelled     -> StateUnknownText       (visible, never coerced)
//
// The state is re-derived from the published string on every call, so it is
// always the raw data's own answer rather than a stored opinion about it.
func (o Observation) Capacity() nepraparse.Value {
	return nepraparse.ParseValue(o.InstalledCapacityMWRaw)
}

// InstalledCapacityMW parses the published capacity. The second return is
// false when the workbook did not publish a NUMBER for this plant-year, which
// is different from publishing zero; callers that ignore it will invent data.
//
// A false second return is deliberately not self-explanatory: three of the
// four reasons for it are distinct facts about the plant. Call
// [Observation.Capacity] and branch on the state when the reason matters.
func (o Observation) InstalledCapacityMW() (float64, bool) {
	return o.Capacity().Float64()
}

// BlockStatus is the state of the plant's twelve-month generation block. The
// second return is false when no status was recorded, in which case the first
// is [nepraparse.StateUnset] and the plant's operating status is UNKNOWN — not
// operating.
func (o Observation) BlockStatus() (nepraparse.CellState, bool) {
	return nepraparse.CellStateByName(o.BlockStatusName)
}

// Operating reports whether the plant was generating in this fiscal year.
//
// The second return is false when the status is unrecorded or unrecognised. A
// caller must branch on it: treating unknown as operating is what put 3,880 MW
// of delicensed and decommissioned plant into current-capacity totals.
//
// A non-operating plant KEEPS a valid installed capacity — Kotri Power Station
// publishes 174 MW installed and 120 MW dependable while all 26 of its monthly
// cells read DELICENSED — so this must gate how capacity is LABELLED, never
// whether it is read.
func (o Observation) Operating() (operating, known bool) {
	st, ok := o.BlockStatus()
	if !ok {
		return false, false
	}
	return !st.Status(), true
}

// Row is one canonical plant plus every published spelling of it and whatever
// parent attribution survived verification.
type Row struct {
	CanonicalName string   `json:"canonical_name"`
	Aliases       []string `json:"aliases"`
	// ParentName is "" when no parent is asserted. That means unknown, never
	// "has no parent".
	ParentName string `json:"parent_name"`
	// PSXTicker is "" when no ticker is asserted. Consult ListedStatus.
	PSXTicker    string       `json:"psx_ticker"`
	ListedStatus ListedStatus `json:"listed_status"`
	// ValidFromFY is the FIRST fiscal year in which this identity was observed
	// in a fetched workbook. It is not a commissioning date: only FY2017-18 and
	// FY2023-24 were fetched, so "2023-24" means "absent from the FY2017-18
	// workbook", not "did not exist before".
	ValidFromFY string `json:"valid_from_fy"`
	// ValidToFY is "" for an open interval, i.e. still published in the latest
	// observed fiscal year. A non-empty value is the last year observed.
	ValidToFY  string        `json:"valid_to_fy"`
	Confidence Confidence    `json:"confidence"`
	Evidence   string        `json:"evidence"`
	Observed   []Observation `json:"observed"`
}

// HasParent reports whether a parent is asserted at all.
func (r Row) HasParent() bool { return r.ParentName != "" }

// HasTicker reports whether a PSX symbol is asserted at all.
func (r Row) HasTicker() bool { return r.PSXTicker != "" }

// IsOpenEnded reports whether the plant's IDENTITY is still published in the
// latest observed workbook.
//
// IT IS NOT AN OPERATING CLAIM, and it is not a proxy for one. All 133 rows in
// this crosswalk have an empty ValidToFY — including the 12 DELICENSED and 1
// DECOMMISSIONED plants of FY2023-24, which are still LISTED in that workbook
// and so still have an open publication interval while generating nothing.
// Reading this as "still running" is exactly how the Hub Power Company's
// delicensed 1,292 MW got counted as live capacity.
//
// For operating status call [Row.OperatingInFY] or [Row.LatestStatus].
func (r Row) IsOpenEnded() bool { return r.ValidToFY == "" }

// LatestObservedFY is the most recent fiscal year in which this plant was
// observed. The second return is false for a row with no observations, which
// [Validate] rejects.
func (r Row) LatestObservedFY() (string, bool) {
	var latest string
	for _, o := range r.Observed {
		if o.FY > latest {
			latest = o.FY
		}
	}
	return latest, latest != ""
}

// StatusInFY is the plant's generation-block status in one fiscal year. The
// second return is false when the plant was not observed in that year at all,
// which is absence from the publication — never a status.
func (r Row) StatusInFY(fy string) (nepraparse.CellState, bool) {
	o, ok := r.ObservedIn(fy)
	if !ok {
		return nepraparse.StateUnset, false
	}
	st, _ := o.BlockStatus()
	return st, true
}

// OperatingInFY reports whether the plant was generating in one fiscal year.
//
// Both returns must be read. observed is false when the plant is absent from
// that year's workbook, and operating is false for BOTH a non-operating plant
// and an unrecorded status, so a caller that wants certainty should use
// [Row.StatusInFY] and check for [nepraparse.StateUnset].
func (r Row) OperatingInFY(fy string) (operating, observed bool) {
	o, ok := r.ObservedIn(fy)
	if !ok {
		return false, false
	}
	op, _ := o.Operating()
	return op, true
}

// LatestStatus is the plant's generation-block status in the most recent
// fiscal year it was observed in, with that year. This is the answer to "is
// this plant still running?" that [Row.IsOpenEnded] does NOT give.
func (r Row) LatestStatus() (st nepraparse.CellState, fy string, ok bool) {
	fy, ok = r.LatestObservedFY()
	if !ok {
		return nepraparse.StateUnset, "", false
	}
	st, ok = r.StatusInFY(fy)
	return st, fy, ok
}

// ObservedFYs lists the fiscal years this plant was actually seen in, in the
// order observed.
func (r Row) ObservedFYs() []string {
	out := make([]string, 0, len(r.Observed))
	for _, o := range r.Observed {
		out = append(out, o.FY)
	}
	return out
}

// ObservedIn returns the plant's appearance in one fiscal year. The second
// return is false when the plant was not in that year's workbook, which is
// absence from the publication, not a zero.
func (r Row) ObservedIn(fy string) (Observation, bool) {
	for _, o := range r.Observed {
		if o.FY == fy {
			return o, true
		}
	}
	return Observation{}, false
}

// ValidInFY reports whether the row's validity interval covers fy. An empty
// ValidToFY is treated as open-ended.
func (r Row) ValidInFY(fy string) bool {
	if fy == "" {
		return false
	}
	if r.ValidFromFY != "" && fy < r.ValidFromFY {
		return false
	}
	if r.ValidToFY != "" && fy > r.ValidToFY {
		return false
	}
	return true
}

// TokenKind says which namespace a token was found in. The two namespaces
// overlap, which is the whole problem this package guards against.
type TokenKind string

// #nosec G101 -- these name a token *namespace* (a PSX ticker vs a NEPRA
// acronym), not credentials. gosec matches on the "Token" identifier prefix.
const (
	// TokenPSXTicker is a Pakistan Stock Exchange symbol.
	TokenPSXTicker TokenKind = "psx_ticker" // #nosec G101 -- namespace label, not a secret.
	// TokenNEPRAAcronym is the parenthetical NEPRA appends to a plant name.
	TokenNEPRAAcronym TokenKind = "nepra_acronym" // #nosec G101 -- namespace label, not a secret.
)

// TokenCandidate is one reading of an ambiguous token.
type TokenCandidate struct {
	Kind          TokenKind `json:"kind"`
	CanonicalName string    `json:"canonical_name"`
	ParentName    string    `json:"parent_name"`
	PSXTicker     string    `json:"psx_ticker"`
	Note          string    `json:"note"`
}

// AmbiguousToken is a curated refusal: a token with more than one defensible
// reading, recorded in crosswalk.json with the hazard it would cause.
type AmbiguousToken struct {
	Token      string           `json:"token"`
	Reason     string           `json:"reason"`
	Hazard     string           `json:"hazard"`
	Candidates []TokenCandidate `json:"candidates"`
}

// ParentStatus distinguishes a parent whose plants are in this dataset from
// one whose plants are known to be absent from it.
type ParentStatus string

const (
	// StatusHasPlants means the crosswalk has plants for this parent.
	StatusHasPlants ParentStatus = "has_plants"
	// StatusAbsentFromDataset means the parent generates, but its plants are
	// outside these workbooks' scope. Never read as zero generation.
	StatusAbsentFromDataset ParentStatus = "absent_from_dataset"
)

// AbsentParent records a listed operator deliberately known to be missing from
// the generation workbooks, with the probe tokens that were searched.
type AbsentParent struct {
	PSXTicker   string       `json:"psx_ticker"`
	ParentName  string       `json:"parent_name"`
	Status      ParentStatus `json:"status"`
	AbsenceNote string       `json:"absence_note"`
	ProbeTokens []string     `json:"probe_tokens"`
	Caveat      string       `json:"caveat"`
}

// PossibleDuplicate is a human-review note about names that could plausibly be
// the same plant. Verdict is "kept_separate", "merged" or
// "granularity_unresolved"; nothing is merged without one of these.
type PossibleDuplicate struct {
	Names     []string `json:"names"`
	Verdict   string   `json:"verdict"`
	Suspicion string   `json:"suspicion"`
}
