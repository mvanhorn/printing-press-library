// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The conflict-and-break ledger: the curated half of `conflicts`.
//
// Every entry here is a place two PUBLISHED NEPRA figures for one key
// disagree, a step inside one published series, or a row that fails its own
// arithmetic. Nothing in this file reconciles, prefers, averages or
// de-duplicates a pair. There is deliberately no `value`, `resolved`,
// `preferred`, `best` or `consensus` field at any level, and every entry
// carries `reconciled: false` as a literal, because for the FY2024-25 MEPCO
// figures each side is internally consistent with its own chart and the
// document contains no tie-breaker: which is correct is UNKNOWABLE from the
// source.
//
// The load-bearing distinction in this file is `computable`. An entry is
// either one this build RE-DERIVES from a committed capture of the published
// document (verified), one it can only QUOTE until a live document is fetched
// (live_only), or one nothing that ships can produce at all (not_computable).
// A reader must never be able to mistake a curated claim for a computed one,
// so the flag travels on every entry and the derivation names the exact
// fixture or the exact reason.

package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// conflictsLedgerAsOf is when every measured number in this file was produced
// by running this build's own code against a committed capture. It is not the
// publication date of any NEPRA document.
const conflictsLedgerAsOf = "2026-09-10"

// The three source surfaces.
const (
	conflictSurfacePER      = "per"
	conflictSurfaceGen      = "gen"
	conflictSurfaceCapacity = "capacity"
)

// The three entry kinds.
const (
	conflictKindConflict   = "conflict"
	conflictKindBreak      = "break"
	conflictKindArithmetic = "arithmetic"
)

// The three answers to "can this build re-derive the entry".
const (
	// conflictVerified means this build reproduced the entry's figures by
	// running its own code over a committed capture of the published
	// document. A test pins the numbers.
	conflictVerified = "verified"
	// conflictLiveOnly means the entry is RECORDED but has never been
	// reproduced end to end: the document it needs has no committed capture,
	// so the figures are a citation and `--recompute` is the first test of
	// them.
	conflictLiveOnly = "live_only"
	// conflictNotComputable means nothing that ships can produce the entry.
	// These live in the refusal table, never in the entry list.
	conflictNotComputable = "not_computable"
)

// How this build came by an entry's numbers. This is the field that keeps a
// curated claim from reading as a computed one.
const (
	derivedFromFixture  = "recomputed_by_this_build_from_a_committed_capture"
	derivedFromRegistry = "quoted_from_nepraper.KnownArtifacts; not reproduced end to end by this build"
	derivedFromSurvey   = "quoted_from_the_corpus_survey; not reproduced by this build"
	// derivedFromLive marks a figure this build DID compute itself, by running
	// its own code over the live published document — but for which no
	// committed capture exists, so no offline test pins it and only
	// --recompute can check it again.
	derivedFromLive = "recomputed_by_this_build_from_a_live_fetch; no committed capture covers these pages"
)

// conflictsLedgerBreakFloor is the smallest adjacent step the SHIPPED ledger
// enumerates.
//
// Every break entry is threshold-dependent, so a break count is not quotable
// without its threshold — and a ledger that held only the steps at or above
// the default 100 would answer `--break-ratio 50` with the same three entries
// and look complete. The ledger therefore records every adjacent step at or
// above this floor (MEASURED: 5 steps in the FY2024-25 comparison tables), and
// a `--break-ratio` below the floor is REFUSED rather than answered short.
const conflictsLedgerBreakFloor = 5.0

// conflictsDefaultBreakRatio is the default minimum step that counts as a
// break.
const conflictsDefaultBreakRatio = 100.0

// conflictKey is the (period, entity, metric) triple an entry is about.
type conflictKey struct {
	PeriodFY string `json:"period_fy"`
	// Entity is empty for a figure that is not entity-specific, e.g. a
	// system-wide capacity definition. Empty means NOT ENTITY-SPECIFIC, never
	// "the entity was lost".
	Entity string `json:"entity,omitempty"`
	Metric string `json:"metric"`
}

func (k conflictKey) String() string {
	if k.Entity == "" {
		return k.PeriodFY + "/" + k.Metric
	}
	return k.PeriodFY + "/" + k.Entity + "/" + k.Metric
}

// conflictSide is ONE of the disagreeing figures, with the provenance that
// makes it checkable.
//
// Value is a POINTER and omitempty, so a side that is not a number carries no
// `value` key at all. Every package underneath this one distinguishes "not
// reported" from a measured 0.00 and that distinction has to survive to the
// JSON: a `"value": 0` on an unavailable figure would be the single worst
// defect this command could ship.
type conflictSide struct {
	Raw   string   `json:"raw"`
	Value *float64 `json:"value,omitempty"`
	// ValueKind is numeric | derived | unverified | unavailable |
	// not_reported | qualitative. `derived` marks a figure THIS BUILD
	// computed from the document's own cells; it never replaces a published
	// one.
	ValueKind string `json:"value_kind"`
	Unit      string `json:"unit,omitempty"`
	ReportFY  string `json:"report_fy,omitempty"`
	Table     string `json:"table,omitempty"`
	// Page is the 1-indexed PDF page, which is the printed page plus one in
	// this corpus. Omitted for surfaces that have no pages.
	Page    int    `json:"page,omitempty"`
	Variant string `json:"variant,omitempty"`
	// Basis says how a derived figure was computed, or why an unavailable one
	// cannot be. It is required on every non-numeric side.
	Basis string `json:"basis,omitempty"`
	URL   string `json:"url,omitempty"`
	// Plants is the population behind a capacity sum, because a megawatt
	// total without its row count is not checkable.
	Plants *int `json:"plants,omitempty"`
}

// conflictRegistryEcho is what nepraper's own registry records for this key.
// It is echoed rather than copied into the entry's fields so that a caller can
// see the shipped package's recorded ratio and direction next to the ratio
// computed from the two published figures.
type conflictRegistryEcho struct {
	Ratio float64 `json:"ratio"`
	// Direction is nepraper's own string, verbatim. For the ~1000x MEPCO
	// SAIDI artifact it begins "inferred:" and must never be shortened into a
	// claim.
	Direction   string `json:"direction"`
	Description string `json:"description"`
}

// conflictPopulation is the denominator that makes a count meaningful. A bare
// "78 of 9,180" with no stated row definition is not a finding.
type conflictPopulation struct {
	FiscalYear  string `json:"fiscal_year"`
	Basis       string `json:"basis"`
	Denominator int    `json:"denominator"`
	Flagged     int    `json:"flagged"`
}

// conflictExplanation names the report whose PROSE explains a break.
//
// It must name the report that carries the wording, not the report being read:
// NEPRA explains the IESCO step in the FY2021-22 report, while the same step
// is visible in FY2024-25's Table 17, which carries no such prose.
type conflictExplanation struct {
	ReportFY string `json:"report_fy"`
	Page     int    `json:"page,omitempty"`
	Quote    string `json:"quote,omitempty"`
	// ReadByThisBuild is false when the quote comes from the corpus survey
	// rather than from a document this build parsed.
	ReadByThisBuild bool `json:"read_by_this_build"`
}

// conflictEntry is one ledger row.
type conflictEntry struct {
	ID      string `json:"id"`
	Surface string `json:"surface"`
	Kind    string `json:"kind"`
	// Class is nepraper's own classification for a same-key conflict:
	// undocumented | declared_variant | rounding | kind_mismatch |
	// transposition. Empty for kinds that are not same-key.
	Class string `json:"class,omitempty"`
	// SameKey is false for a break (two adjacent PERIODS of one series) and
	// for an arithmetic failure (a row against itself). Only a conflict can
	// honestly be same_key.
	SameKey bool         `json:"same_key"`
	Key     conflictKey  `json:"key"`
	A       conflictSide `json:"a"`
	B       conflictSide `json:"b"`
	// Also carries further sides that are part of the same disagreement — a
	// third published definition, or a figure that exists but is unavailable
	// to this build.
	Also []conflictSide `json:"also,omitempty"`
	// Ratio is A/B and is NULL when undefined: B is a published zero, or a
	// side is not numeric. It is a pointer because a plain float64 carrying
	// math.Inf made json.Marshal fail, and the ubiquitous `b, _ :=
	// json.Marshal(...)` then wrote an EMPTY document — a real conflict
	// reported as no conflict.
	Ratio *float64 `json:"ratio"`
	// AbsDiff is |A-B| and is null when the two sides are not both numeric.
	AbsDiff *float64 `json:"abs_diff"`
	// Direction is "unknowable" unless a source says otherwise. When it comes
	// from nepraper it is that string verbatim.
	Direction string `json:"direction"`
	// Reconciled is ALWAYS false. It is a literal, not a computation: no
	// consumer of this command ever receives a reconciled number.
	Reconciled bool   `json:"reconciled"`
	Computable string `json:"computable"`
	Derivation string `json:"derivation"`
	// ReproducedByThisBuild is the one-bit form of Derivation, for a consumer
	// that only wants to filter.
	ReproducedByThisBuild bool   `json:"reproduced_by_this_build"`
	MeasuredFrom          string `json:"measured_from,omitempty"`
	// RecomputeNeeds lists the fiscal years --recompute must read.
	RecomputeNeeds []string              `json:"recompute_needs,omitempty"`
	Registry       *conflictRegistryEcho `json:"registry,omitempty"`
	Population     *conflictPopulation   `json:"population,omitempty"`
	ExplainedBy    *conflictExplanation  `json:"explained_by,omitempty"`
	// BreakRatioSeen is the step this break entry actually measured, so a
	// caller can re-apply any threshold to the shipped ledger.
	BreakRatioSeen *float64 `json:"break_ratio_seen,omitempty"`
	Note           string   `json:"note"`
}

// conflictRefusal is a question this command will not answer, with the
// measured reason it cannot.
//
// An unsupported (surface, kind) pair is NEVER an empty list: an agent that
// reads an empty result as "no conflicts here" has been misled, and five of
// the nine pairs cannot be produced by anything that ships.
type conflictRefusal struct {
	Surface string `json:"surface,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Subject string `json:"subject,omitempty"`
	// Verdict is not_computable | unavailable | unverifiable.
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
	Instead string `json:"instead,omitempty"`
}

// conflictUnverified is a recorded figure that must never be read as a number.
type conflictUnverified struct {
	PeriodFY string `json:"period_fy"`
	Entity   string `json:"entity,omitempty"`
	Metric   string `json:"metric"`
	RawLabel string `json:"raw_label"`
	// ValueKind is always "unverified" and there is NO numeric field anywhere
	// in this object: "19,535." is a truncated Excel chart label whose
	// trailing digits are absent from the text layer, so parsing it as 19535
	// or 19.535 would be fabrication.
	ValueKind string `json:"value_kind"`
	// EntityAttributed is false for a system-wide chart label. That is a
	// property of the source, not a gap in the record.
	EntityAttributed bool   `json:"entity_attributed"`
	Reason           string `json:"reason"`
	Source           string `json:"source"`
}

// conflictIncompleteness is a class of disagreement the corpus is KNOWN to
// contain that this ledger does not enumerate at figure level.
//
// It exists so the entry list is never mistaken for the complete set. The
// alternative — hand-authoring figures this build cannot attribute to a
// document it has read — would be exactly the guessing this command exists to
// refuse.
type conflictIncompleteness struct {
	Subject string `json:"subject"`
	Reason  string `json:"reason"`
	Instead string `json:"instead,omitempty"`
}

// ---------------------------------------------------------------------------
// Source URLs
// ---------------------------------------------------------------------------

// conflictPERURL returns the absolute URL of a PER, from nepraper's own source
// table. The paths are a TABLE, not a pattern, and FY2020-21's ends in a
// trailing encoded space that is load-bearing: the same URL without it 404s.
func conflictPERURL(fy string) string {
	if s, ok := nepraper.SourceFor(fy); ok {
		return s.URL()
	}
	return ""
}

// conflictGenURL returns the absolute URL of a generation workbook. The token
// is the BARE fiscal year ("2020-21"); the FY-prefixed form is a documented
// 404. Note NEPRA's own typo "Genenration" in the path, which is load-bearing.
func conflictGenURL(fy string) string {
	return nepraHostBase + conflictGenPath(fy)
}

// conflictGenPath substitutes the fiscal year into the generated resource
// template, so the CLI reads one path table rather than two.
func conflictGenPath(fy string) string {
	return strings.ReplaceAll(resourceDetailPaths["generation"], "{fy}", conflictBareFY(fy))
}

// nepraHostBase is NEPRA's origin. The client concatenates BaseURL and path
// verbatim, so an already-encoded NEPRA path passes through intact.
const nepraHostBase = "https://nepra.org.pk"

// conflictBareFY turns "FY2020-21" into "2020-21".
func conflictBareFY(fy string) string {
	return strings.TrimPrefix(nepraper.NormalizeFY(fy), "FY")
}

// ---------------------------------------------------------------------------
// Helpers that keep the null discipline
// ---------------------------------------------------------------------------

func fptr(v float64) *float64 { return &v }
func iptr(v int) *int         { return &v }

// conflictRatio returns A/B, or nil when the ratio does not EXIST.
//
// nil means undefined — a published zero denominator, or a side that is not a
// number — and is emitted as JSON null. It never means zero and never means
// infinite. This is the function that keeps a conflict list from failing to
// marshal at all.
func conflictRatio(a, b *float64) *float64 {
	if a == nil || b == nil || *b == 0 {
		return nil
	}
	r := *a / *b
	if math.IsInf(r, 0) || math.IsNaN(r) {
		return nil
	}
	return &r
}

// conflictAbsDiff returns |A-B|, or nil when the two sides are not both
// numeric. An absent magnitude is absent, not zero.
func conflictAbsDiff(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	return fptr(math.Abs(*a - *b))
}

// conflictSlug reduces a name to an id fragment.
func conflictSlug(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// conflictEntryID builds a stable id. The derivation legs and the shipped
// ledger MUST agree on it, because --recompute matches the two by id.
func conflictEntryID(surface, kind string, key conflictKey, discriminator string) string {
	parts := []string{surface}
	if kind != conflictKindConflict {
		parts = append(parts, kind)
	}
	parts = append(parts, conflictSlug(key.PeriodFY))
	if key.Entity != "" {
		parts = append(parts, conflictSlug(key.Entity))
	}
	parts = append(parts, conflictSlug(key.Metric))
	if discriminator != "" {
		parts = append(parts, conflictSlug(discriminator))
	}
	return strings.Join(parts, "-")
}

// conflictPERDiscriminator names a PER conflict by the two tables that
// printed it. The shipped ledger and the live derivation leg MUST build it the
// same way, because --recompute pairs the two by id.
func conflictPERDiscriminator(field, aTable, bTable string) string {
	d := aTable + "-vs-" + bTable
	if field != "" && field != "value" {
		d = field + "-" + d
	}
	return d
}

// conflictRegistryFor returns nepraper's recorded artifact for a key, or nil.
//
// Reading the registry rather than copying its strings is deliberate: it keeps
// nepraper the single source of truth for the four documented artifacts and
// their Direction wording, and it makes a registry change visible here instead
// of leaving two divergent copies.
func conflictRegistryFor(k conflictKey) *conflictRegistryEcho {
	for _, a := range nepraper.KnownArtifacts() {
		if nepraper.NormalizeFY(a.Key.PeriodFY) != nepraper.NormalizeFY(k.PeriodFY) {
			continue
		}
		if string(a.Key.Entity) != k.Entity || string(a.Key.Metric) != k.Metric {
			continue
		}
		return &conflictRegistryEcho{Ratio: a.Ratio, Direction: a.Direction, Description: a.Description}
	}
	return nil
}

// conflictDirectionFor prefers nepraper's own Direction string when it has
// one. "unknowable" is the honest default: it is what the FY2024-25 pairs are,
// and asserting anything else would be a claim the documents do not support.
func conflictDirectionFor(k conflictKey) string {
	if r := conflictRegistryFor(k); r != nil {
		return r.Direction
	}
	return "unknowable"
}

// ---------------------------------------------------------------------------
// The ledger
// ---------------------------------------------------------------------------

// conflictLedger assembles the shipped ledger. It makes no request and reads
// no file.
func conflictLedger() []conflictEntry {
	var out []conflictEntry
	out = append(out, conflictLedgerPER()...)
	out = append(out, conflictLedgerBreaks()...)
	out = append(out, conflictLedgerGen()...)
	out = append(out, conflictLedgerCapacity()...)
	return out
}

// perFixtureNote names the exact capture every `verified` PER entry was
// re-derived from.
const perFixtureNote = "internal/cli/testdata/per-fy2024-25.spans.tsv.gz -> nepraper.PageFromSpans -> " +
	"nepraper.ParseReliability -> nepraper.Conflicts"

// perLiveNote names the live fetch a figure came from when no committed
// capture covers its pages.
const perLiveNote = "measured on " + conflictsLedgerAsOf + " from the live 1,563,742-byte PDF at " +
	"/M&E/PER/Distribution/2026/PER 2024-25 Distribution Companies.pdf, which yields 630 observations " +
	"against the committed capture's 120; NO offline test pins these two figures"

// conflictLedgerPER holds the same-key PER conflicts.
func conflictLedgerPER() []conflictEntry {
	url2425 := conflictPERURL("FY2024-25")

	// The FY2024-25 within-report conflicts.
	//
	// MEASURED two ways, and they do not agree on the COUNT. Over the
	// committed four-page span capture nepraper.Conflicts returns exactly the
	// first four below and UndocumentedConflicts returns all four (a
	// transposition is never filtered out). Over the LIVE 35-page PDF the same
	// code returns six: the two LESCO pairs at the end are on pages the
	// capture does not include. So "FY2024-25 holds exactly four conflicts" is
	// a property of the fixture, not of the document, and both facts are
	// recorded here rather than one being made to stand for the other.
	type pair struct {
		entity, metric string
		aRaw, bRaw     string
		a, b           float64
		aTable, bTable string
		aPage, bPage   int
		class          string
		// derivation and measuredFrom say which capture the figures came
		// from. The four pages the committed span capture covers are pinned
		// by an offline test; the two conflicts on pages the capture does not
		// reach were measured from the live document and can only be checked
		// again by --recompute.
		derivation, measuredFrom string
		note                     string
	}
	pairs := []pair{
		{
			entity: "MEPCO", metric: "saidi",
			aRaw: "3547.00", bRaw: "1182.56", a: 3547.00, b: 1182.56,
			aTable: "Table 6", bTable: "Table 18", aPage: 15, bPage: 29,
			class: "undocumented", derivation: derivedFromFixture, measuredFrom: perFixtureNote,
			note: "each figure is internally consistent with its own chart and the document carries no " +
				"tie-breaker; the 3x gap flips MEPCO's compliance verdict against the SAIDI target published " +
				"alongside it, so the choice of figure decides the finding",
		},
		{
			entity: "MEPCO", metric: "saifi",
			aRaw: "30.67", bRaw: "10.23", a: 30.67, b: 10.23,
			aTable: "Table 5", bTable: "Table 17", aPage: 13, bPage: 28,
			class: "undocumented", derivation: derivedFromFixture, measuredFrom: perFixtureNote,
			note: "the SAIFI twin of the SAIDI 3x pair, in the same report and the same two table families",
		},
		{
			entity: "LESCO", metric: "saifi",
			aRaw: "28.16", bRaw: "28.61", a: 28.16, b: 28.61,
			aTable: "Table 5", bTable: "Table 17", aPage: 13, bPage: 28,
			class: "transposition", derivation: derivedFromFixture, measuredFrom: perFixtureNote,
			note: "the two figures are the same digit multiset in a different order, so one of them is " +
				"demonstrably wrong and the document does not say which",
		},
		{
			entity: "K-Electric", metric: "saifi",
			aRaw: "68.46", bRaw: "68.64", a: 68.46, b: 68.64,
			aTable: "Table 5", bTable: "Table 17", aPage: 13, bPage: 28,
			class: "transposition", derivation: derivedFromFixture, measuredFrom: perFixtureNote,
			note: "a 0.26% relative gap that used to be filtered out as rounding; both figures are printed to " +
				"the same precision, so neither can be a rounding of the other. nepraper.KnownArtifacts has NO " +
				"entry for this key, so the ratio here is computed from the two published figures rather than " +
				"quoted from the registry",
		},
		// Two further FY2024-25 conflicts, MEASURED live from the published
		// 1,563,742-byte PDF on the ledger date. They are on pages 19/20/31/32,
		// which the committed span capture does not cover, so nepraper's own
		// four-page fixture cannot see them — and the frequently quoted claim
		// that FY2024-25 holds "exactly four conflicts, no more" is a property
		// of that fixture rather than of the document.
		{
			entity: "LESCO", metric: "nominal_voltage",
			aRaw: "5,689", bRaw: "5,107", a: 5689, b: 5107,
			aTable: "Table 11", bTable: "Table 21", aPage: 20, bPage: 32,
			class: "undocumented", derivation: derivedFromLive,
			measuredFrom: perLiveNote,
			note: "582 consumer complaints about nominal voltage in one table against 5,107 in another, in " +
				"one report, with no annotation. Neither figure is on a page the committed capture covers",
		},
		{
			entity: "LESCO", metric: "load_shedding",
			aRaw: "0.06", bRaw: "0.1", a: 0.06, b: 0.1,
			aTable: "Table 10", bTable: "Table 20", aPage: 19, bPage: 31,
			class: "rounding", derivation: derivedFromLive,
			measuredFrom: perLiveNote,
			note: "classed ROUNDING rather than undocumented, and that is the classifier earning its keep: " +
				"0.06 printed to one decimal IS 0.1, so this is one figure at two precisions rather than " +
				"two figures. It is reported anyway, because filtering it out would be a judgement the " +
				"caller should make",
		},
	}

	out := make([]conflictEntry, 0, len(pairs)+2)
	for _, p := range pairs {
		key := conflictKey{PeriodFY: "FY2024-25", Entity: p.entity, Metric: p.metric}
		a := fptr(p.a)
		b := fptr(p.b)
		out = append(out, conflictEntry{
			ID: conflictEntryID(conflictSurfacePER, conflictKindConflict, key,
				conflictPERDiscriminator("value", p.aTable, p.bTable)),
			Surface: conflictSurfacePER,
			Kind:    conflictKindConflict,
			Class:   p.class,
			SameKey: true,
			Key:     key,
			A: conflictSide{
				Raw: p.aRaw, Value: a, ValueKind: "numeric", ReportFY: "FY2024-25",
				Table: p.aTable, Page: p.aPage, Variant: "headline", URL: url2425,
			},
			B: conflictSide{
				Raw: p.bRaw, Value: b, ValueKind: "numeric", ReportFY: "FY2024-25",
				Table: p.bTable, Page: p.bPage, Variant: "five_year_comparison", URL: url2425,
			},
			Ratio:                 conflictRatio(a, b),
			AbsDiff:               conflictAbsDiff(a, b),
			Direction:             conflictDirectionFor(key),
			Reconciled:            false,
			Computable:            conflictVerified,
			Derivation:            p.derivation,
			ReproducedByThisBuild: true,
			MeasuredFrom:          p.measuredFrom,
			RecomputeNeeds:        []string{"FY2024-25"},
			Registry:              conflictRegistryFor(key),
			Note:                  p.note,
		})
	}

	// The ~1000x MEPCO SAIDI artifact. Only the 39.733 side is in the
	// committed capture (FY2024-25 Table 18's 2020-21 column); the 39733 side
	// lives in the FY2020-21 report, which has no capture. ConflictsAcross has
	// therefore never been run end to end on this pair, so it ships live_only
	// and the registry's 1000.0000 ratio is a research measurement, not a
	// reproduced one.
	k1000 := conflictKey{PeriodFY: "FY2020-21", Entity: "MEPCO", Metric: "saidi"}
	a1000, b1000 := fptr(39733), fptr(39.733)
	out = append(out, conflictEntry{
		ID: conflictEntryID(conflictSurfacePER, conflictKindConflict, k1000,
			conflictPERDiscriminator("value", "Table 6", "Table 18")),
		Surface: conflictSurfacePER,
		Kind:    conflictKindConflict,
		Class:   "transposition",
		SameKey: true,
		Key:     k1000,
		A: conflictSide{
			Raw: "39733", Value: a1000, ValueKind: "numeric", ReportFY: "FY2020-21",
			Table: "Table 6", Variant: "headline", URL: conflictPERURL("FY2020-21"),
			Basis: "recorded by nepraper.KnownArtifacts and READ from the live FY2020-21 report on " +
				conflictsLedgerAsOf + "; no committed capture holds it, so no offline test pins it",
		},
		B: conflictSide{
			Raw: "39.733", Value: b1000, ValueKind: "numeric", ReportFY: "FY2024-25",
			Table: "Table 18", Page: 29, Variant: "five_year_comparison", URL: url2425,
			Basis: "this side IS in the committed FY2024-25 capture and was read from it",
		},
		Ratio:                 conflictRatio(a1000, b1000),
		AbsDiff:               conflictAbsDiff(a1000, b1000),
		Direction:             conflictDirectionFor(k1000),
		Reconciled:            false,
		Computable:            conflictVerified,
		Derivation:            derivedFromLive,
		ReproducedByThisBuild: true,
		MeasuredFrom: "reproduced end to end on " + conflictsLedgerAsOf + " by fetching BOTH reports live " +
			"(2,277,838 + 1,563,742 bytes) and running nepraper.ConflictsAcross over the pair: observed " +
			"39733 against 39.733, ratio 1000.0000000000001. No committed capture covers the FY2020-21 " +
			"report, so no OFFLINE test pins this pair and only --recompute --fy 2020-21,2024-25 checks it",
		RecomputeNeeds: []string{"FY2020-21", "FY2024-25"},
		Registry:       conflictRegistryFor(k1000),
		Note: "an exact decimal-point move, propagated into three later reports (FY2021-22 Table 25, " +
			"FY2022-23 Table 20 as 39.73, FY2024-25 Table 18). The FY2020-21 report prints 39733 twice, in " +
			"Table 6 and again in Table 16, so the live read reports this pair a second time under the " +
			"Table 16 label. The direction is nepraper's own string and begins \"inferred\": it is an " +
			"inference from the surrounding series, NOT proven by the documents",
	})

	// The IESCO recovery revision. Requires two live PERs and is not
	// reproducible from anything committed: the FY2018-19 capture holds only
	// pages 7, 10 and 11 (T&D, SAIFI, SAIDI) and carries no recovery table,
	// and FY2019-20 has no capture at all.
	kRec := conflictKey{PeriodFY: "FY2018-19", Entity: "IESCO", Metric: "recovery"}
	aRec, bRec := fptr(88.0), fptr(90)
	out = append(out, conflictEntry{
		ID:      conflictEntryID(conflictSurfacePER, conflictKindConflict, kRec, "revision"),
		Surface: conflictSurfacePER,
		Kind:    conflictKindConflict,
		Class:   "undocumented",
		SameKey: true,
		Key:     kRec,
		A: conflictSide{
			Raw: "88.0", Value: aRec, ValueKind: "numeric", ReportFY: "FY2018-19",
			Variant: "headline", URL: conflictPERURL("FY2018-19"),
			Basis: "recorded by the corpus survey; the committed FY2018-19 capture holds pages 7, 10 and 11 " +
				"only and contains no recovery table, so this build has not read this figure",
		},
		B: conflictSide{
			Raw: "90", Value: bRec, ValueKind: "numeric", ReportFY: "FY2019-20",
			Variant: "five_year_comparison", URL: conflictPERURL("FY2019-20"),
			Basis: "recorded by the corpus survey as republished unchanged in the FY2019-20, FY2020-21 and " +
				"FY2021-22 reports; no committed capture exists for any of them",
		},
		Ratio:                 conflictRatio(aRec, bRec),
		AbsDiff:               conflictAbsDiff(aRec, bRec),
		Direction:             "unknowable",
		Reconciled:            false,
		Computable:            conflictLiveOnly,
		Derivation:            derivedFromSurvey,
		ReproducedByThisBuild: false,
		RecomputeNeeds:        []string{"FY2018-19", "FY2019-20"},
		Note: "a 2.00 pp upward revision to a published recovery rate, never annotated. Under nepraper's " +
			"classifier this is undocumented rather than rounding: 88.0 and 90 are printed to different " +
			"precisions but 88.0 does not round to 90, and their digit multisets differ so it is not a " +
			"transposition either. The first --recompute of these years is also the first test of " +
			"nepraper's parse on them",
	})

	return out
}

// conflictLedgerBreaks holds the adjacent-period steps inside the FY2024-25
// comparison tables.
//
// A break is NOT a same-key disagreement: it is a step between two adjacent
// PERIODS of one published series, so same_key is false on every one of these
// and the threshold that produced it travels with it.
//
// MEASURED from the committed capture: 3 steps at or above 100x, 4 at or above
// 50x, 5 at or above the ledger floor of 5x. Grouping is by (entity, metric,
// TABLE), which is what keeps a break inside ONE published series — comparing
// across tables would manufacture the very conflicts the conflict leg already
// reports.
func conflictLedgerBreaks() []conflictEntry {
	url2425 := conflictPERURL("FY2024-25")
	iescoQuote := "the figure of IESCO has significantly gone upward from 0.05 to 20.5 because previously " +
		"it had miscalculated the SAIFI"

	type step struct {
		entity, metric string
		fromFY, toFY   string
		fromRaw, toRaw string
		from, to       float64
		table          string
		page           int
		ratio          float64
		explained      *conflictExplanation
		note           string
	}
	steps := []step{
		{
			entity: "IESCO", metric: "saidi", fromFY: "FY2020-21", toFY: "FY2021-22",
			fromRaw: "1.36", toRaw: "1027.01", from: 1.36, to: 1027.01,
			table: "Table 18", page: 29, ratio: 755.1544117647,
			explained: &conflictExplanation{
				ReportFY: "FY2021-22", Page: 31, Quote: iescoQuote, ReadByThisBuild: false,
			},
			note: "the largest step in the corpus. NEPRA explains the IESCO revision in the FY2021-22 " +
				"report, NOT in the FY2024-25 report this step was read from, which carries no such prose. " +
				"Note the FY2021-22 prose says 20.5 where its own table says 20.56, so the explanation and " +
				"the table disagree too",
		},
		{
			entity: "IESCO", metric: "saifi", fromFY: "FY2020-21", toFY: "FY2021-22",
			fromRaw: "0.05", toRaw: "20.56", from: 0.05, to: 20.56,
			table: "Table 17", page: 28, ratio: 411.2,
			explained: &conflictExplanation{
				ReportFY: "FY2021-22", Page: 31, Quote: iescoQuote, ReadByThisBuild: false,
			},
			note: "the SAIFI half of the same IESCO revision; the quoted prose names these exact two figures",
		},
		{
			entity: "GEPCO", metric: "saidi", fromFY: "FY2022-23", toFY: "FY2023-24",
			fromRaw: "38.59", toRaw: "4216.56", from: 38.59, to: 4216.56,
			table: "Table 18", page: 29, ratio: 109.2656128531,
			note: "a two-order-of-magnitude step with NO prose anywhere in the report that publishes it. " +
				"GEPCO's SAIFI series steps in the same year (22.01 -> 55.86) but only 2.54x, so the two " +
				"indices were not restated together",
		},
		{
			entity: "MEPCO", metric: "saidi", fromFY: "FY2020-21", toFY: "FY2021-22",
			fromRaw: "39.733", toRaw: "2794", from: 39.733, to: 2794,
			table: "Table 18", page: 29, ratio: 70.319381874,
			note: "the FY2020-21 end of this step is the 39.733 side of the ~1000x MEPCO artifact, so the " +
				"step and the artifact are two readings of one defect and must not be counted as two " +
				"independent findings. Below the default threshold of 100 and above 50",
		},
		{
			entity: "MEPCO", metric: "saifi", fromFY: "FY2020-21", toFY: "FY2021-22",
			fromRaw: "471", toRaw: "43.94", from: 471, to: 43.94,
			table: "Table 17", page: 28, ratio: 10.7191624943,
			note: "a published SAIFI of 471 interruptions, an order of magnitude above every other DISCO in " +
				"that column, dropping to 43.94 the next year. Well below the default threshold; recorded " +
				"because the ledger enumerates every step at or above its declared floor of 5x",
		},
	}

	out := make([]conflictEntry, 0, len(steps))
	for _, s := range steps {
		key := conflictKey{PeriodFY: s.toFY, Entity: s.entity, Metric: s.metric}
		from, to := fptr(s.from), fptr(s.to)
		ratio := s.ratio
		out = append(out, conflictEntry{
			ID:      conflictEntryID(conflictSurfacePER, conflictKindBreak, key, "from-"+s.fromFY),
			Surface: conflictSurfacePER,
			Kind:    conflictKindBreak,
			SameKey: false,
			Key:     key,
			A: conflictSide{
				Raw: s.fromRaw, Value: from, ValueKind: "numeric", ReportFY: "FY2024-25",
				Table: s.table, Page: s.page, Variant: "five_year_comparison", URL: url2425,
				Basis: "the " + s.fromFY + " column of the same published series",
			},
			B: conflictSide{
				Raw: s.toRaw, Value: to, ValueKind: "numeric", ReportFY: "FY2024-25",
				Table: s.table, Page: s.page, Variant: "five_year_comparison", URL: url2425,
				Basis: "the " + s.toFY + " column of the same published series",
			},
			Ratio:                 conflictRatio(to, from),
			AbsDiff:               conflictAbsDiff(to, from),
			Direction:             "unknowable",
			Reconciled:            false,
			Computable:            conflictVerified,
			Derivation:            derivedFromFixture,
			ReproducedByThisBuild: true,
			MeasuredFrom:          perFixtureNote,
			RecomputeNeeds:        []string{"FY2024-25"},
			ExplainedBy:           s.explained,
			BreakRatioSeen:        &ratio,
			Note:                  s.note,
		})
	}
	return out
}

// genFixtureNote names the captures the gen entries were re-derived from.
func genFixtureNote(fy string) string {
	return "internal/cli/testdata/gen-" + conflictBareFY(fy) + ".htm.gz -> nepraparse.ParseWorkbook"
}

// conflictLedgerGen holds the generation-workbook arithmetic failures.
func conflictLedgerGen() []conflictEntry {
	var out []conflictEntry

	// The Balloki Sum mismatch. MEASURED: the only failure among 105 eligible
	// FY2020-21 rows at nepraparse.DefaultSumTolerance 0.06, i.e. 104 of 105
	// reconcile. The derived side is LABELLED derived and never replaces the
	// published Sum.
	kBal := conflictKey{PeriodFY: "FY2020-21", Entity: "(NPPCL) - Balloki", Metric: "generation_gwh"}
	pub, der := fptr(5905.65), fptr(5945.21)
	out = append(out, conflictEntry{
		ID:      conflictEntryID(conflictSurfaceGen, conflictKindArithmetic, kBal, "sum"),
		Surface: conflictSurfaceGen,
		Kind:    conflictKindArithmetic,
		SameKey: false,
		Key:     kBal,
		A: conflictSide{
			Raw: "5,905.65", Value: pub, ValueKind: "numeric", Unit: "GWh", ReportFY: "FY2020-21",
			Table: "published Sum column", Variant: "as_published", URL: conflictGenURL("FY2020-21"),
		},
		B: conflictSide{
			Raw: "", Value: der, ValueKind: "derived", Unit: "GWh", ReportFY: "FY2020-21",
			Table: "sum of the row's own twelve monthly GWh cells", Variant: "derived",
			URL:   conflictGenURL("FY2020-21"),
			Basis: "619.24+694.03+595.60+763.73+591.91+413.01+657.74+0.00+420.82+401.66+403.53+383.94",
		},
		Ratio:                 conflictRatio(pub, der),
		AbsDiff:               conflictAbsDiff(pub, der),
		Direction:             "unknowable",
		Reconciled:            false,
		Computable:            conflictVerified,
		Derivation:            derivedFromFixture,
		ReproducedByThisBuild: true,
		MeasuredFrom:          genFixtureNote("FY2020-21") + " -> (*Workbook).CheckSum(FieldGWh, 0.06)",
		RecomputeNeeds:        []string{"FY2020-21"},
		Population: &conflictPopulation{
			FiscalYear:  "FY2020-21",
			Basis:       "plant rows whose thirteen GWh cells are all numeric (Sum plus twelve months)",
			Denominator: 105, Flagged: 1,
		},
		Note: "the row's own twelve monthly cells do not add up to the row's own annual Sum. Which side is " +
			"wrong is not determinable from the document; the derived side is labelled derived and the " +
			"published Sum is never replaced",
	})

	out = append(out, conflictLedgerZeroLoadFactor()...)
	return out
}

// zeroLoadFactorPopulations are the MEASURED denominators for the
// zero-load-factor scan, per fiscal year.
//
// The denominator is stated because a count without one is not a finding. The
// absorb manifest's "78 of 9,180 plant-months" does NOT reproduce: the scan
// over the three committed captures finds 4 + 5 + 1 = 10 flagged of 3,840
// plant-months whose "% age" and GWh cells are BOTH numeric, and 12 of 4,160
// once the annual Sum pair is included. No cross-year total is printed for
// years this build has not read.
var zeroLoadFactorPopulations = map[string]conflictPopulation{
	"FY2017-18": {FiscalYear: "FY2017-18", Basis: zlfBasis, Denominator: 1164, Flagged: 4},
	"FY2020-21": {FiscalYear: "FY2020-21", Basis: zlfBasis, Denominator: 1260, Flagged: 5},
	"FY2023-24": {FiscalYear: "FY2023-24", Basis: zlfBasis, Denominator: 1416, Flagged: 1},
}

const zlfBasis = "plant-months (12 per plant, the annual Sum pair excluded) whose \"% age\" and GWh cells " +
	"are both numeric; a status sentinel and an NBSP blank are neither flagged nor counted"

// zeroLoadFactorSumPopulations are the same scan over the annual Sum pair
// alone, kept separate because a Sum is not a month.
var zeroLoadFactorSumPopulations = map[string]conflictPopulation{
	"FY2017-18": {FiscalYear: "FY2017-18", Basis: zlfSumBasis, Denominator: 97, Flagged: 1},
	"FY2020-21": {FiscalYear: "FY2020-21", Basis: zlfSumBasis, Denominator: 105, Flagged: 1},
	"FY2023-24": {FiscalYear: "FY2023-24", Basis: zlfSumBasis, Denominator: 118, Flagged: 0},
}

const zlfSumBasis = "plant-YEARS whose annual Sum \"% age\" and GWh cells are both numeric"

// The three verdicts a zero-load-factor row can carry.
const (
	zlfContradiction = "contradiction"
	zlfConsistent    = "consistent_at_published_precision"
	zlfUncomputable  = "uncomputable_no_capacity"
)

// conflictLedgerZeroLoadFactor holds the rows publishing generation against a
// 0.00 utilisation.
//
// NOT all of them are contradictions, and reporting a bare count would say
// they were. AES Lalpir publishes 0.01 GWh on 362 MW, an implied utilisation
// of 0.0037% that ROUNDS to 0.00 at the workbook's own two decimals — it is
// CONSISTENT. Three Gorges Second Wind Farm publishes 0.32 GWh with an
// installed capacity of 0, so no utilisation exists to compare against. Every
// entry therefore carries its own implied utilisation and its own verdict.
func conflictLedgerZeroLoadFactor() []conflictEntry {
	// Only the row's own published INPUTS are recorded here. The implied
	// utilisation and the verdict are computed by the same helpers the live
	// derivation leg uses, so the ledger's numbers are this code's output
	// rather than a transcription of it — there is no second copy to drift.
	type row struct {
		fy       string
		plant    string
		monthIdx int
		isTotal  bool
		capMW    float64
		gwh      float64
	}
	rows := []row{
		{fy: "FY2017-18", plant: "AES Lalpir power limited.", monthIdx: 3, capMW: 362, gwh: 0.01},
		{fy: "FY2017-18", plant: "Neelum Jhelum Hydropower Project (WAPDA)", monthIdx: 2, capMW: 969, gwh: 4.44},
		{fy: "FY2017-18", plant: "Artistic Wind Power (Pvt.) Limited.", monthIdx: 7, capMW: 50, gwh: 0.96},
		{fy: "FY2017-18", plant: "Three Gorges Second Wind Farm (Private) Ltd. (TGS)", monthIdx: 11, capMW: 0, gwh: 0.32},
		{fy: "FY2017-18", plant: "Three Gorges Second Wind Farm (Private) Ltd. (TGS)", isTotal: true, capMW: 0, gwh: 0.32},

		{fy: "FY2020-21", plant: "Fatima Energy Limited. (FEL)", monthIdx: 4, capMW: 120, gwh: 4.61},
		{fy: "FY2020-21", plant: "Fatima Energy Limited. (FEL)", monthIdx: 5, capMW: 120, gwh: 23.24},
		{fy: "FY2020-21", plant: "Fatima Energy Limited. (FEL)", monthIdx: 6, capMW: 120, gwh: 23.43},
		{fy: "FY2020-21", plant: "Fatima Energy Limited. (FEL)", monthIdx: 7, capMW: 120, gwh: 21.20},
		{fy: "FY2020-21", plant: "Fatima Energy Limited. (FEL)", monthIdx: 8, capMW: 120, gwh: 4.52},
		{fy: "FY2020-21", plant: "Fatima Energy Limited. (FEL)", isTotal: true, capMW: 120, gwh: 77.02},

		{fy: "FY2023-24", plant: "Helios Power (Private) Limited (HPPL)", monthIdx: 6, capMW: 50, gwh: 3.42},
	}

	out := make([]conflictEntry, 0, len(rows))
	for _, r := range rows {
		label := "Sum"
		if !r.isTotal {
			label = nepraparse.MonthsInFiscalOrder[r.monthIdx].String()
		}
		key := conflictKey{PeriodFY: r.fy, Entity: r.plant, Metric: "utilisation_pct"}
		published := fptr(0)

		implied, impliedOK := zlfImplied(r.gwh, r.capMW, true, r.fy, r.monthIdx, r.isTotal)
		verdict := zlfVerdictFor(implied, impliedOK)
		basis := zlfBasisFor(r.gwh, r.capMW, true, "numeric", r.fy, r.monthIdx, r.isTotal)
		var derived *float64
		derivedKind := "not_reported"
		if impliedOK {
			derived = fptr(implied)
			derivedKind = "derived"
		}

		pop := zeroLoadFactorPopulations[r.fy]
		if r.isTotal {
			pop = zeroLoadFactorSumPopulations[r.fy]
		}
		out = append(out, conflictEntry{
			ID:      conflictEntryID(conflictSurfaceGen, conflictKindArithmetic, key, "zlf-"+strings.ToLower(label)),
			Surface: conflictSurfaceGen,
			Kind:    conflictKindArithmetic,
			SameKey: false,
			Key:     key,
			A: conflictSide{
				Raw: "0.00", Value: published, ValueKind: "numeric", Unit: "%", ReportFY: r.fy,
				Table: "published \"% age\" cell, " + label, Variant: "as_published",
				URL:   conflictGenURL(r.fy),
				Basis: fmt.Sprintf("published alongside %.2f GWh in the same month pair", r.gwh),
			},
			B: conflictSide{
				Raw: "", Value: derived, ValueKind: derivedKind, Unit: "%", ReportFY: r.fy,
				Table:   "utilisation implied by the row's own GWh, installed capacity and day count",
				Variant: "derived", URL: conflictGenURL(r.fy), Basis: basis,
			},
			Ratio:                 conflictRatio(published, derived),
			AbsDiff:               conflictAbsDiff(published, derived),
			Direction:             "unknowable",
			Reconciled:            false,
			Computable:            conflictVerified,
			Derivation:            derivedFromFixture,
			ReproducedByThisBuild: true,
			MeasuredFrom:          genFixtureNote(r.fy),
			RecomputeNeeds:        []string{r.fy},
			Population:            &pop,
			Note:                  zlfNoteFor(verdict),
		})
	}
	return out
}

// zlfNoteFor states what a zero-load-factor verdict means, in one sentence,
// because a count of flagged cells with no verdict is not a finding.
func zlfNoteFor(verdict string) string {
	note := "the row publishes generation against a utilisation of exactly 0.00"
	switch verdict {
	case zlfConsistent:
		note += ", and the implied utilisation ROUNDS to 0.00 at the workbook's own two decimals: this " +
			"is CONSISTENT, not a contradiction"
	case zlfUncomputable:
		note += ", but the implied utilisation cannot be computed so the claim cannot be checked either way"
	default:
		note += " while its own capacity and day count imply a utilisation far above the published " +
			"precision: the two cells of one pair contradict each other"
	}
	return note + " [verdict: " + verdict + "]"
}

// conflictLedgerCapacity holds the installed-capacity DEFINITIONS conflict for
// 30 June 2024.
//
// The disagreement here is not a typo: four incompatible megawatt figures are
// all correct for different populations, and a consumer that picks one without
// its definition has a wrong number. Two are computable by this build, one
// more is computable and materially different, and three exist only in
// documents this build declares unreadable.
func conflictLedgerCapacity() []conflictEntry {
	url := conflictGenURL("FY2023-24")
	key := conflictKey{PeriodFY: "FY2023-24", Metric: "installed_capacity_mw"}
	reported, operating := fptr(44686), fptr(40806)

	return []conflictEntry{{
		ID:      conflictEntryID(conflictSurfaceCapacity, conflictKindConflict, key, "definitions"),
		Surface: conflictSurfaceCapacity,
		Kind:    conflictKindConflict,
		Class:   "declared_variant",
		SameKey: true,
		Key:     key,
		A: conflictSide{
			Raw: "44686", Value: reported, ValueKind: "numeric", Unit: "MW", ReportFY: "FY2023-24",
			Table: "Installed Capacity (MW) column, every numeric row", Variant: "capacity_reported",
			URL: url, Plants: iptr(133),
			Basis: "every numeric installed capacity published for 30-Jun-2024, operating or not. This is " +
				"the figure that ties to the workbook total",
		},
		B: conflictSide{
			Raw: "40806", Value: operating, ValueKind: "numeric", Unit: "MW", ReportFY: "FY2023-24",
			Table:   "the same column less the rows whose monthly block reads DELICENSED or DECOMMISSIONED",
			Variant: "capacity_operating", URL: url, Plants: iptr(120),
			Basis: "the part of the reported total whose monthly generation block is ordinary data. THIS is " +
				"current generating capacity; the reported total is not",
		},
		Also: []conflictSide{
			{
				Raw: "3880", Value: fptr(3880), ValueKind: "numeric", Unit: "MW", ReportFY: "FY2023-24",
				Variant: "capacity_non_operating_by_sentinel", URL: url, Plants: iptr(13),
				Basis: "the difference between the two sides above: 12 DELICENSED plus 1 DECOMMISSIONED " +
					"rows that publish a real installed capacity and generate nothing. A non-operating " +
					"plant KEEPS a valid capacity, so this gates how capacity is labelled, never whether " +
					"it is read",
			},
			{
				Raw: "4061", Value: fptr(4061), ValueKind: "derived", Unit: "MW", ReportFY: "FY2023-24",
				Variant: "capacity_not_generating_including_unreported", URL: url, Plants: iptr(15),
				Basis: "the 13 status-sentinel rows (3,880 MW) PLUS the 2 rows whose entire monthly block " +
					"is blank while a capacity is published — Reshma Power Generation 97.00 MW and Gulf " +
					"Powergen 84.00 MW, 181.00 MW together. Both 3,880 and 4,061 are correct for different " +
					"populations and must not be merged: a blank block is NOT REPORTED, which is a " +
					"different fact from a declared DELICENSED",
			},
			{
				Raw: "0", Value: fptr(0), ValueKind: "numeric", Unit: "MW", ReportFY: "FY2023-24",
				Variant: "capacity_status_unknown", URL: url, Plants: iptr(0),
				Basis: "a MEASURED zero, not an absence: every FY2023-24 row has a recorded block status, " +
					"so no capacity falls outside the operating/non-operating split",
			},
			{
				Raw: "42,512", ValueKind: "unavailable", Unit: "MW", ReportFY: "FY2023-24",
				Variant: "sir_2024_cppag_system",
				Basis: "NEPRA's own State of Industry Report 2024 states this for the CPPA-G system. It " +
					"lives only inside the SIR PDF corpus — 610 MB across ten reports at 68-164 KB/s, and " +
					"sir2023.pdf extracts 0 bytes of text — which this build declares unverifiable. " +
					"Recorded as unavailable and NOT approximated",
			},
			{
				Raw: "45,888", ValueKind: "unavailable", Unit: "MW", ReportFY: "FY2023-24",
				Variant: "sir_2024_including_k_electric",
				Basis: "the same SIR 2024 figure including K-Electric. Same surface, same unverifiable " +
					"verdict. Note K-Electric's own fleet is absent from the generation workbook entirely, " +
					"so the two sides above cannot include it either",
			},
			{
				Raw: "47,559.97", ValueKind: "unavailable", Unit: "MW",
				Variant: "licence_register_gross_capacity",
				Basis: "Gross Capacity summed over 331 of 335 licence entities, as the research probe " +
					"published it. `licence --all` now reads the register (21 pages, 379 entities) and " +
					"CANNOT reproduce this figure honestly: it is reachable only by reading \"12:00 MW\" " +
					"as 12 and \"3s 6 MW\" as 3, and that entity's own plant-detail row reads 1x16MW + " +
					"1x20MW, i.e. 36 MW. The shipped command reports 361 MW-convertible entities and names " +
					"the other 18 by reason, so no single total is asserted on either side",
			},
		},
		Ratio:                 conflictRatio(reported, operating),
		AbsDiff:               conflictAbsDiff(reported, operating),
		Direction:             "unknowable",
		Reconciled:            false,
		Computable:            conflictVerified,
		Derivation:            derivedFromFixture,
		ReproducedByThisBuild: true,
		MeasuredFrom: genFixtureNote("FY2023-24") + " -> nepraxwalk.Coverage(\"2023-24\", published names), " +
			"every megawatt read through MWSum.Float64()'s measured return",
		RecomputeNeeds: []string{"FY2023-24"},
		Note: "four incompatible megawatt figures for one date, every one of them correct for a different " +
			"population. Three of the seven sides here cannot be recomputed by anything that ships and are " +
			"marked unavailable rather than approximated. The absorb manifest's own triple — 40,614 MW " +
			"active, 4,061 MW status, 44,675 MW total — is NOT reported as a side: it is refuted on " +
			"measurement (40,614 and 44,675 reproduce nothing) and citing our own superseded brief would " +
			"be the one place this command manufactured a conflict",
	}}
}

// ---------------------------------------------------------------------------
// The (surface, kind) refusal matrix
// ---------------------------------------------------------------------------

// conflictSurfaces and conflictKinds are the accepted flag vocabularies.
var (
	conflictSurfaces = []string{conflictSurfacePER, conflictSurfaceGen, conflictSurfaceCapacity}
	conflictKinds    = []string{conflictKindConflict, conflictKindBreak, conflictKindArithmetic}
)

// conflictRefusals is the machine-readable half of the declared scope: the
// (surface, kind) pairs nothing that ships can produce, each with the MEASURED
// reason. It mirrors nepraScopeLimits exactly in shape.
//
// Five of the nine pairs are here. Returning an empty entry list for any of
// them would let an agent read a refusal as a finding of zero.
var conflictRefusals = []conflictRefusal{
	{
		Surface: conflictSurfacePER, Kind: conflictKindArithmetic, Verdict: conflictNotComputable,
		Reason: "no shipped code checks a PER figure against its own arithmetic. internal/nepraper exports " +
			"Conflicts, ConflictsAcross, UndocumentedConflicts and Lookup and no self-consistency API at " +
			"all. The FY2014-15 GEPCO result that is sometimes quoted (841178/2812997 = 29.90% against a " +
			"published 30) was produced by a reviewer's independent Python, not by this build.",
		Instead: "conflicts --surface gen --kind arithmetic, which checks each workbook row against its own Sum",
	},
	{
		Surface: conflictSurfaceGen, Kind: conflictKindConflict, Verdict: conflictNotComputable,
		Reason: "one vintage of each fiscal year's generation workbook is published, so there is no same-key " +
			"SECOND source to disagree with it. Every figure in the file appears exactly once, and the " +
			"seven reachable years are seven different periods rather than seven readings of one period.",
		Instead: "conflicts --surface gen --kind arithmetic for a row against itself, or --surface capacity " +
			"for the one place two definitions of one workbook figure do disagree",
	},
	{
		Surface: conflictSurfaceGen, Kind: conflictKindBreak, Verdict: conflictNotComputable,
		Reason: "a break is a step between two adjacent periods of ONE published series, and one workbook " +
			"covers one fiscal year: there is no adjacent-period series inside it. Stitching two years' " +
			"workbooks into a series would need a cross-year plant identity, and S.No is not one — 0 of " +
			"106 plants keep their serial into FY2023-24, which was re-sorted by technology.",
		Instead: "conflicts --surface per --kind break, whose comparison tables publish five adjacent years " +
			"inside one document",
	},
	{
		Surface: conflictSurfaceCapacity, Kind: conflictKindBreak, Verdict: conflictNotComputable,
		Reason: "the capacity surface is the set of installed-capacity DEFINITIONS for one date, 30 June " +
			"2024. A single date has no adjacent period, so no step exists to measure.",
		Instead: "conflicts --surface capacity --kind conflict, which prints the definitions side by side",
	},
	{
		Surface: conflictSurfaceCapacity, Kind: conflictKindArithmetic, Verdict: conflictNotComputable,
		Reason: "the capacity figures are sums over a population, not rows carrying their own totals, so " +
			"there is no published arithmetic to fail. The operating and non-operating parts do reconstruct " +
			"the reported total exactly (40,806 + 3,880 = 44,686), which is a consistency check that PASSES " +
			"and therefore yields no entry.",
		Instead: "conflicts --surface capacity --kind conflict, whose sides carry every population and its " +
			"row count",
	},
}

// conflictScopeRefusals are the further questions this command is asked and
// cannot answer, beyond the (surface, kind) matrix.
var conflictScopeRefusals = []conflictRefusal{
	{
		Subject: "the FY2024-25 chart-label disagreements (Table 05 says IESCO 15.39 and GEPCO 49.49 while " +
			"the Figure 08 data labels say 15.31 and 49.41)",
		Verdict: conflictNotComputable,
		Reason: "these are the richest same-key conflicts in the corpus and nothing that ships can read " +
			"them. nepraper deliberately rejects chart-axis labels that share a data row's baseline, and " +
			"the p28/p29 chart labels are drawn as vertically rotated single glyphs that no horizontal line " +
			"grouping can reconstruct. Reading them needs a coordinate-aware chart-label reader, which is " +
			"new work in nepraper, not a flag here.",
		Instead: "conflicts --surface per --kind conflict, which reports the four TABLE-level FY2024-25 conflicts",
	},
	{
		Subject: "the FY2023-24 Performance Evaluation Report",
		Verdict: "unavailable",
		Reason: "linked from NEPRA's own Performance Reports index and returns HTTP 404 with a 9-byte body. " +
			"It is recorded UNAVAILABLE, which is a different fact from a report that exists and reports " +
			"nothing, and it is never fetched-and-reported-empty.",
		Instead: "sources --kind per, which lists which report-years are reachable",
	},
	{
		Subject: "a durable store of both vintages of a conflicted figure",
		Verdict: conflictNotComputable,
		Reason: "internal/store carries only the Printing Press learn and playbook tables: there is no NEPRA " +
			"table and no sync command, so --data-source local answers nothing for this command. Every " +
			"derived figure here is live-fetch-only and the shipped ledger is an in-code registry, not a " +
			"database.",
		Instead: "conflicts --recompute, which re-derives from the published documents on demand",
	},
}

// conflictSupported reports whether a (surface, kind) pair can be produced,
// and returns the refusal when it cannot. An empty surface or kind means "all",
// which is always supported.
func conflictSupported(surface, kind string) (bool, conflictRefusal) {
	if surface == "" || kind == "" {
		return true, conflictRefusal{}
	}
	for _, r := range conflictRefusals {
		if r.Surface == surface && r.Kind == kind {
			return false, r
		}
	}
	return true, conflictRefusal{}
}

// conflictSupportedPairs lists the (surface, kind) pairs that do work, so a
// refusal can name the alternatives instead of leaving the caller guessing.
func conflictSupportedPairs() []string {
	var out []string
	for _, s := range conflictSurfaces {
		for _, k := range conflictKinds {
			if ok, _ := conflictSupported(s, k); ok {
				out = append(out, s+"+"+k)
			}
		}
	}
	return out
}

// conflictLedgerIncompleteness declares what the ledger does NOT enumerate, so
// the entry list is never mistaken for the complete set of NEPRA's
// self-contradictions.
var conflictLedgerIncompleteness = []conflictIncompleteness{
	{
		Subject: "21 further conflicts and breaks that a live read of FY2020-21 and FY2024-25 finds",
		Reason: "MEASURED on " + conflictsLedgerAsOf + ": `--recompute --surface per --fy 2020-21,2024-25` " +
			"reproduced all 10 selected ledger entries AND reported 21 further findings the ledger does not " +
			"enumerate — 10 fault-rate pairs (Table 12 printed to 8 decimals against Table 24 at 2), 8 T&D " +
			"loss restatements over 4 DISCOs, the MEPCO SAIDI 1000x pair a second time under its Table 16 " +
			"label, and 2 HESCO new-connection breaks. They are NOT hand-authored here: transcribing 21 " +
			"figures off a terminal is the transcription risk this command exists to avoid, and --recompute " +
			"surfaces every one of them as a new_finding rather than dropping it.",
		Instead: "conflicts --recompute --surface per --fy 2020-21,2024-25",
	},
	{
		Subject: "figure-level conflicts in the FY2014-15, FY2018-19, FY2019-20, FY2021-22 and FY2022-23 reports",
		Reason: "the corpus survey records further classes of disagreement in these years — comma-for-period " +
			"1000x readings, and the FY2022-23 with-LT against without-LT rows. This build has no committed " +
			"capture of the recovery or LT tables and has not read those figures, so they are declared here " +
			"rather than quoted as entries. The count above shows what a live read actually adds, and it is " +
			"large: assume the ledger is a floor, never a complete set.",
		Instead: "conflicts --recompute --surface per --fy 2018-19,2019-20,2021-22,2022-23",
	},
	{
		Subject: "the FY2024-25 conflict COUNT, which depends on which pages were read",
		Reason: "over the committed four-page span capture nepraper.Conflicts returns 4 FY2024-25 conflicts; " +
			"over the live 35-page PDF the same code returns 6, because the two LESCO pairs are on pages " +
			"19/20/31/32. \"Exactly four, no more\" is therefore a property of that fixture and not of the " +
			"document, and both figures are recorded rather than one standing for the other.",
		Instead: "conflicts --recompute --surface per --fy 2024-25",
	},
	{
		Subject: "the zero-load-factor scan for FY2018-19, FY2019-20, FY2021-22 and FY2022-23",
		Reason: "four of the seven reachable generation workbooks have no committed capture, so the scan's " +
			"denominators cover three years and 3,840 plant-months, not seven years. No cross-year rate is " +
			"printed and no count is extrapolated to a wider denominator.",
		Instead: "conflicts --recompute --surface gen --fy 2018-19",
	},
	{
		Subject: "adjacent steps below the ledger's declared break floor",
		Reason: "the shipped ledger enumerates every adjacent step at or above 5.00x measured from the " +
			"FY2024-25 comparison tables. A --break-ratio below that floor cannot be answered from the " +
			"ledger, and is refused rather than answered with a short list that would look complete.",
		Instead: "conflicts --recompute --surface per --kind break --break-ratio <n>",
	},
}

// conflictUnverifiedFigures returns the recorded figures that must never be
// read as numbers, plus the year whose label was never captured.
//
// nepra_scope.go points callers at THIS command for them, so omitting them
// would break a shipped cross-reference.
func conflictUnverifiedFigures() []conflictUnverified {
	src := nepraper.KnownUnverified()
	out := make([]conflictUnverified, 0, len(src))
	for _, u := range src {
		out = append(out, conflictUnverified{
			PeriodFY:         u.PeriodFY,
			Entity:           string(u.Entity),
			Metric:           string(u.Metric),
			RawLabel:         u.RawLabel,
			ValueKind:        "unverified",
			EntityAttributed: u.EntityAttributed(),
			Reason:           u.Reason,
			Source:           u.Source,
		})
	}
	return out
}
