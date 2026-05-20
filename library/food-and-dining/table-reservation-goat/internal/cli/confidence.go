// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: location-native-redesign — popularity prior + ambiguity-aware
// tier decision + DisambiguationEnvelope shape + DecorateWithLocationContext
// helper (U4). The prior ranks Place candidates within a result; the
// tier decision routes to one of three user-facing outcomes (HIGH:
// silent resolution, MEDIUM: pick-with-warning, LOW: emit
// disambiguation envelope). The decorator produces the annotation
// shape callers embed in their response after applyGeoFilter has
// finished filtering result slices.

import (
	"fmt"
	"math"
	"strings"
)

// Weight constants for the popularity prior. The four components sum
// to 1.0 so a perfectly-matching, high-population, metro-centroid Place
// with full provider coverage scores 1.0; the worst case (zero pop,
// no coverage, not a metro, no match) scores 0. The split is calibrated
// against R14 fixtures F1-F7:
//
//   - w_pop: dominates ranking among similar-tier candidates (e.g.,
//     ranking three Bellevues by population so the WA hit leads).
//   - w_cov: live provider coverage carries equal weight when present;
//     curated entries have zero coverage so this term contributes 0
//     until Tock SSR hydration runs. Keeping it as a peer of pop means
//     a high-coverage smaller market can outrank a low-coverage larger
//     one once hydration data is available.
//   - w_tier: metro-centroid Places get a flat bonus so a metro
//     centroid outranks a same-name city when both qualify (used in
//     the prior, but the tier-decision margin is computed over the
//     population component only — see decideTier).
//   - w_match: name match — input.CityName == place.Name (case-
//     insensitive). Fires for ambiguous-name lookups where every
//     candidate matches, in which case the term cancels in margin
//     calculations.
//
// Tuning happens against R14 fixtures F1-F7; do not edit these without
// re-running the calibration tests in confidence_test.go.
const (
	wPop   = 0.3
	wCov   = 0.3
	wTier  = 0.2
	wMatch = 0.2
)

// nycPopReference is the normalization base for log_norm. Set to the
// 2020 Census NYC value (~8.8M) so log_norm(NYC) == 1.0 by construction
// and every other US Place lands in [0, 1). Pinned in
// confidence_test.go.
const nycPopReference = 8_804_190

// TierEnum — the user-facing confidence tier returned by decideTier.
// HIGH: resolve silently and return results. MEDIUM: resolve but
// attach a location_warning listing alternates. LOW: emit the
// DisambiguationEnvelope unless the caller opted into AcceptAmbiguous.
// Unknown is the no-candidates sentinel for the location_unknown
// envelope path.
type TierEnum int

const (
	TierUnknown TierEnum = iota
	TierLow
	TierMedium
	TierHigh
)

// String returns the lowercase tier name, used in JSON envelopes and
// log lines. Matches the SKILL.md doc's tier names (high/medium/low/
// unknown) so the agent-facing surface stays stable.
func (t TierEnum) String() string {
	switch t {
	case TierHigh:
		return "high"
	case TierMedium:
		return "medium"
	case TierLow:
		return "low"
	default:
		return "unknown"
	}
}

// ResolutionTier maps the internal TierEnum value into the exported
// ResolutionTier wire string. Used when a GeoContext is constructed
// from a (tier, ranked) pair returned by decideTier so the agent-facing
// JSON shape commits to a categorical tier alongside the numeric Score.
func (t TierEnum) ResolutionTier() ResolutionTier {
	switch t {
	case TierHigh:
		return ResolutionTierHigh
	case TierMedium:
		return ResolutionTierMedium
	case TierLow:
		return ResolutionTierLow
	default:
		return ResolutionTierUnknown
	}
}

// ScoredCandidate pairs a Place with its computed popularity prior.
// Used internally during ranking and surfaced (as the source for
// envelope Candidate entries) when callers need the prior alongside
// the place.
type ScoredCandidate struct {
	Place Place
	Prior float64
}

// logNorm normalizes a population (or coverage count) into [0, 1].
// Uses log10(p+1) / log10(nycPopReference+1) so:
//   - logNorm(0) == 0 (don't blow up on missing data).
//   - logNorm(nycPopReference) == 1.0 (NYC is the calibrated ceiling).
//   - Sub-NYC US populations land in (0, 1).
//
// log scaling compresses the long tail (a 10x population gap doesn't
// produce a 10x prior gap), which is the intended behavior for
// ranking similar-tier candidates within an ambiguous-name lookup.
func logNorm(value int) float64 {
	if value <= 0 {
		return 0
	}
	num := math.Log10(float64(value) + 1)
	den := math.Log10(float64(nycPopReference) + 1)
	if den <= 0 {
		return 0
	}
	r := num / den
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// metroCentroidBonus returns 1.0 when the place is a metro centroid,
// else 0. Used as the is_metro_centroid_bonus term in popularityPrior;
// gives metros a flat lift over same-name cities so a metro centroid
// outranks a co-named smaller city.
func metroCentroidBonus(p Place) float64 {
	if p.Tier == PlaceTierMetroCentroid {
		return 1.0
	}
	return 0.0
}

// exactMatchBonus returns 1.0 when the input's CityName matches the
// place's Name (case-insensitive). Fires for every ambiguous-name
// candidate (so the term cancels in margin calculations) but lifts the
// absolute prior so name-matched candidates outrank near-misses when
// they coexist (e.g., a reverse-geo hit and a name match in the same
// candidate set). Returns 0 when input is nil or CityName is empty;
// downstream callers may pass a nil LocationInput for the no-flag
// SourceDefault path.
func exactMatchBonus(p Place, input *LocationInput) float64 {
	if input == nil {
		return 0
	}
	if input.CityName == "" {
		return 0
	}
	if strings.EqualFold(p.Name, input.CityName) {
		return 1.0
	}
	return 0
}

// popularityPrior produces a [0, 1] score used to rank Place
// candidates within a result. The formula is the spec's four-term
// combination:
//
//	prior = w_pop  * log_norm(place.Population)
//	      + w_cov  * log_norm(place.ProviderCoverage["tock"])
//	      + w_tier * is_metro_centroid_bonus
//	      + w_match* exact_match_bonus(place, input)
//
// Curated Places have a nil ProviderCoverage map; the term contributes
// 0 until Tock SSR hydration fills it. Note this prior is for RANKING;
// the tier decision uses a separate population-ratio margin (see
// decideTier) because non-discriminating bonus terms dilute the
// ratio-on-full-prior the spec originally specified.
func popularityPrior(p Place, input *LocationInput) float64 {
	popTerm := wPop * logNorm(p.Population)

	covTerm := 0.0
	if p.ProviderCoverage != nil {
		covTerm = wCov * logNorm(p.ProviderCoverage["tock"])
	}

	tierTerm := wTier * metroCentroidBonus(p)
	matchTerm := wMatch * exactMatchBonus(p, input)

	return popTerm + covTerm + tierTerm + matchTerm
}

// decideTier ranks candidates by popularityPrior and routes the result
// to a tier. U14 simplified the rule after Codex P2-F/P2-G adversarial
// review flagged the prior population-ratio margin as a wrong-city UX
// trap (the ranking used the full prior, the margin used population
// alone — two different scores deciding two related questions, and
// the MEDIUM "guess and warn" outcome was wrong-city for the minority-
// population candidate when the agent ignored the warning).
//
// New rules:
//
//   - 0 candidates -> TierUnknown (caller emits location_unknown envelope).
//   - 1 candidate -> TierHigh (unambiguous by definition; regardless of
//     input specificity).
//   - 2+ candidates AND input.Specificity == SpecificityHigh -> TierHigh
//     (city+state, coords, zip; the caller was specific so collapse to
//     the top match. Upstream filters typically narrow to 1 first, but
//     defensive against multi-hit edge cases).
//   - 2+ candidates AND input.Specificity == SpecificityMedium -> TierMedium
//     (metro qualifier or other medium-specificity input. Rare in
//     practice — most metro qualifiers resolve to a single canonical via
//     alias Lookup upstream — but pinned for the rule).
//   - 2+ candidates AND input.Specificity == SpecificityLow -> TierLow
//     (bare ambiguous input; the envelope path lets the agent
//     disambiguate rather than silently picking the wrong city).
//
// Returns the tier and the ranked candidates (sorted desc by prior).
// Tie-break order matches sort.SliceStable: equal priors keep their
// input order, which lets caller-provided ordering act as the final
// tie-break.
func decideTier(input *LocationInput, candidates []Place) (TierEnum, []ScoredCandidate) {
	if len(candidates) == 0 {
		return TierUnknown, nil
	}

	ranked := make([]ScoredCandidate, len(candidates))
	for i, p := range candidates {
		ranked[i] = ScoredCandidate{Place: p, Prior: popularityPrior(p, input)}
	}
	// Stable insertion-sort by Prior desc — small N (typically ≤ 4 for
	// our R14 fixtures) so an O(n²) pass is fine and avoids importing
	// "sort" just for this loop. Stable so equal priors preserve the
	// caller's input order.
	for i := 1; i < len(ranked); i++ {
		for j := i; j > 0 && ranked[j].Prior > ranked[j-1].Prior; j-- {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
		}
	}

	if len(ranked) == 1 {
		return TierHigh, ranked
	}

	// 2+ candidates: branch on input specificity. Nil input is treated
	// as SpecificityLow (the default zero-value semantics in
	// LocationInput) so defensive coverage routes to LOW.
	if input != nil {
		switch input.Specificity {
		case SpecificityHigh:
			return TierHigh, ranked
		case SpecificityMedium:
			return TierMedium, ranked
		}
	}
	return TierLow, ranked
}

// Error-kind enum values for DisambiguationEnvelope.ErrorKind. Stable
// across surfaces so agents can branch on the kind string without
// re-parsing prose.
const (
	// ErrorKindLocationUnknown — caller passed a --location value that
	// did not resolve to any Place. The envelope candidates list is
	// empty; agent_guidance suggests asking the user for a clearer
	// location.
	ErrorKindLocationUnknown = "location_unknown"

	// ErrorKindLocationAmbiguous — multiple Places share the input
	// name and the tier decision routed to LOW. Envelope candidates
	// enumerate the alternatives ranked by popularityPrior.
	ErrorKindLocationAmbiguous = "location_ambiguous"

	// ErrorKindVenueAmbiguous — venue-name (not location-name) lookup
	// surfaced multiple matches across regions. Reserved for U7's
	// availability_check disambiguation; included here so callers can
	// emit a consistent envelope shape.
	ErrorKindVenueAmbiguous = "venue_ambiguous"

	// ErrorKindNoResultsInRegion — the location resolved cleanly but
	// the filtered result set is empty. Envelope candidates suggest
	// expanding the search radius or trying a nearby region.
	ErrorKindNoResultsInRegion = "no_results_in_region"

	// ErrorKindResultsOnlyOutside — results exist but all fall outside
	// the resolved radius; the post-filter would hard-reject every
	// row. Envelope surfaces the outside-region hits as alternatives
	// so the caller can decide whether to widen the radius.
	ErrorKindResultsOnlyOutside = "results_only_outside_region"
)

// AgentGuidance is the recovery-hint sub-shape embedded in a
// DisambiguationEnvelope. PreferredRecovery is short prose the agent
// can echo to the user; RerunPattern is a literal command template
// (with `<chosen-name>` placeholder) the agent can substitute and
// re-execute once it has a disambiguated value.
type AgentGuidance struct {
	PreferredRecovery string `json:"preferred_recovery"`
	RerunPattern      string `json:"rerun_pattern"`
}

// DisambiguationEnvelope is the JSON shape returned in place of
// results when the tier decision routes to LOW (without
// AcceptAmbiguous). The contract is stable across read commands so an
// agent's parser only needs to handle one shape.
type DisambiguationEnvelope struct {
	NeedsClarification bool          `json:"needs_clarification"`
	ErrorKind          string        `json:"error_kind"`
	WhatWasAsked       string        `json:"what_was_asked"`
	Candidates         []Candidate   `json:"candidates"`
	AgentGuidance      AgentGuidance `json:"agent_guidance"`
}

// BuildEnvelope assembles a DisambiguationEnvelope from ranked
// candidates. Each ScoredCandidate is projected into a Candidate (the
// shape shared with GeoContext.Alternates) — Name carries the
// ", State" suffix where State is set, so the agent can render and
// rerun the result without a second lookup.
//
// The agent_guidance text is intentionally generic — the rerun pattern
// uses `<command>` as a literal placeholder because BuildEnvelope is
// called from multiple commands and the caller can substitute the
// command name into the pattern when emitting.
func BuildEnvelope(input *LocationInput, ranked []ScoredCandidate, errorKind string) DisambiguationEnvelope {
	whatWasAsked := ""
	if input != nil {
		whatWasAsked = input.Raw
	}

	candidates := make([]Candidate, len(ranked))
	for i, sc := range ranked {
		p := sc.Place
		name := p.Name
		if p.State != "" {
			name = fmt.Sprintf("%s, %s", p.Name, p.State)
		}
		tockCov := 0
		if p.ProviderCoverage != nil {
			tockCov = p.ProviderCoverage["tock"]
		}
		candidates[i] = Candidate{
			Name:              name,
			State:             p.State,
			ContextHints:      p.ContextHints,
			TockBusinessCount: tockCov,
			ScoreIfPicked:     sc.Prior,
			Centroid:          [2]float64{p.Lat, p.Lng},
		}
	}

	guidance := AgentGuidance{
		PreferredRecovery: "Check conversation context for geographic clues. If the user mentioned a state or nearby city, re-run with that.",
		RerunPattern:      "<command> --location '<chosen-name>'",
	}

	return DisambiguationEnvelope{
		NeedsClarification: true,
		ErrorKind:          errorKind,
		WhatWasAsked:       whatWasAsked,
		Candidates:         candidates,
		AgentGuidance:      guidance,
	}
}

// LocationResolvedField is the annotation shape callers embed in
// their response when the location resolved to a Place. Source carries
// the GeoContext.Source value so consumers can branch on
// explicit-flag-vs-extracted-vs-default without parsing prose.
//
// Score is the popularity prior (a [0,1] mechanical number, not a
// confidence — see ResolutionTier). Tier is the agent-facing
// categorical decision; SKILL.md directs agents to branch on Tier
// rather than the raw Score.
type LocationResolvedField struct {
	Input                string         `json:"input"`
	ResolvedTo           string         `json:"resolved_to"`
	Score                float64        `json:"score"`
	Tier                 ResolutionTier `json:"tier"`
	Reason               string         `json:"reason"`
	AlternatesConsidered []string       `json:"alternates_considered,omitempty"`
	Source               Source         `json:"source"`
}

// LocationWarningField is the annotation shape callers attach in
// addition to LocationResolvedField when the resolution had material
// ambiguity (MEDIUM tier) or the caller forced a pick over LOW
// (AcceptAmbiguous=true). Picked echoes the chosen name; Alternates
// lists the other candidates that were considered; Reason explains
// why a warning fires.
type LocationWarningField struct {
	Picked     string   `json:"picked"`
	Alternates []string `json:"alternates"`
	Reason     string   `json:"reason"`
}

// DecorateWithLocationContext composes the location_resolved /
// location_warning annotation fields for a response, given the
// resolved GeoContext, the tier from decideTier, and whether the
// caller forced past a LOW result with --batch-accept-ambiguous.
//
// Return shape:
//   - HIGH: (resolved, nil) — silent success.
//   - MEDIUM: (resolved, warning) — both fields; the warning lists the
//     alternates so the caller can show "picked X over Y, Z".
//   - LOW with AcceptAmbiguous=true: (resolved, warning) — resolved
//     carries the low confidence, warning flags the bypassed
//     disambiguation.
//   - LOW without AcceptAmbiguous: caller returns the envelope
//     directly; DecorateWithLocationContext is not called on this
//     path. Defensive: returns (nil, nil) for TierLow with
//     acceptAmbiguousBypass=false rather than panicking.
//   - UNKNOWN: (nil, nil) — caller is on the location_unknown
//     envelope path.
//
// Nil-safe on gc: returns (nil, nil) when gc is nil (the SourceDefault
// no-constraint path) since there is nothing to annotate.
func DecorateWithLocationContext(gc *GeoContext, tier TierEnum, acceptAmbiguousBypass bool) (*LocationResolvedField, *LocationWarningField) {
	if gc == nil {
		return nil, nil
	}

	switch tier {
	case TierUnknown:
		return nil, nil
	case TierLow:
		if !acceptAmbiguousBypass {
			return nil, nil
		}
	}

	resolved := &LocationResolvedField{
		Input:                gc.Origin,
		ResolvedTo:           gc.ResolvedTo,
		Score:                gc.Score,
		Tier:                 tier.ResolutionTier(),
		Reason:               resolvedReason(tier, acceptAmbiguousBypass),
		AlternatesConsidered: alternateNames(gc.Alternates),
		Source:               gc.Source,
	}

	if tier == TierHigh {
		return resolved, nil
	}

	warning := &LocationWarningField{
		Picked:     gc.ResolvedTo,
		Alternates: alternateNames(gc.Alternates),
		Reason:     warningReason(tier, acceptAmbiguousBypass),
	}
	return resolved, warning
}

// resolvedReason maps tier + bypass into the user-facing reason text
// embedded in LocationResolvedField. Strings are short on purpose so
// the agent doesn't have to truncate them.
func resolvedReason(tier TierEnum, acceptAmbiguousBypass bool) string {
	switch tier {
	case TierHigh:
		return "unambiguous match"
	case TierMedium:
		return "picked best match over close alternates"
	case TierLow:
		if acceptAmbiguousBypass {
			return "forced pick over ambiguous candidates (--batch-accept-ambiguous)"
		}
		return "ambiguous"
	}
	return ""
}

// warningReason maps tier + bypass into the LocationWarningField
// reason text. Only called when DecorateWithLocationContext is about
// to return a non-nil warning, so the TierHigh case is unreachable.
func warningReason(tier TierEnum, acceptAmbiguousBypass bool) string {
	switch tier {
	case TierMedium:
		return "picked best match; alternates were close enough to be worth surfacing"
	case TierLow:
		if acceptAmbiguousBypass {
			return "forced pick over ambiguous candidates; disambiguation envelope bypassed"
		}
	}
	return ""
}

// inferTierFromGeoContext approximates the tier from a returned
// GeoContext when the caller doesn't have direct access to the
// decideTier output (i.e., they got back a GeoContext from
// ResolveLocation, not the (tier, ranked) pair from decideTier).
//
// Resolution path:
//
//  1. If gc.Tier is set (non-empty), trust it — buildGeoContext now
//     stamps the tier directly from decideTier, so the categorical
//     decision is authoritative and we don't need to re-infer.
//  2. Legacy fallback (gc.Tier == ""): use the original
//     score-based heuristic for callers constructing GeoContext
//     literals without going through buildGeoContext (older test
//     fixtures, future hand-constructed geo contexts).
//
// Legacy decision table (gc.Tier == ""):
//
//   - nil gc                 -> TierUnknown (defensive)
//   - 0 alternates           -> TierHigh (one candidate ranked, OR
//     LocKindCityState narrowed by state)
//   - >0 alternates, !accept -> TierMedium (LOW would have been
//     envelope at ResolveLocation time, so a non-envelope GeoContext
//     with alternates must be MEDIUM)
//   - >0 alternates, accept  -> score cutoff: >= 0.35 is MEDIUM,
//     < 0.35 is LOW (forced pick over a wide-margin LOW result)
//
// The MEDIUM-vs-LOW-with-bypass distinction affects only the prose
// reason strings emitted on LocationResolvedField / LocationWarningField;
// no downstream behavior branches on it. The 0.35 cutoff is calibrated
// downward from the popularity-prior's natural minimum so a wrongly-
// classified LOW-with-bypass renders as "forced pick" prose rather than
// silently downgrading the warning text.
//
// Lives in confidence.go (not in restaurants_list.go where it was
// originally introduced in U6) so availability_check and other
// command-wiring helpers can reuse it without an inverse import.
func inferTierFromGeoContext(gc *GeoContext, acceptedAmbiguous bool) TierEnum {
	if gc == nil {
		return TierUnknown
	}
	// Trust gc.Tier when set — buildGeoContext stamps it from decideTier.
	switch gc.Tier {
	case ResolutionTierHigh:
		return TierHigh
	case ResolutionTierMedium:
		return TierMedium
	case ResolutionTierLow:
		return TierLow
	case ResolutionTierUnknown:
		return TierUnknown
	}
	// Legacy fallback: gc.Tier is zero-value (""), use the score-based
	// heuristic from before the field existed.
	if len(gc.Alternates) == 0 {
		return TierHigh
	}
	if !acceptedAmbiguous {
		// ResolveLocation returned gc (not envelope) despite alternates
		// and no bypass — must be MEDIUM (LOW would have been envelope).
		return TierMedium
	}
	// acceptedAmbiguous=true: score-based split.
	if gc.Score >= 0.35 {
		return TierMedium
	}
	return TierLow
}

// alternateNames projects the GeoContext.Alternates Candidate list
// into a flat []string of "Name, ST" entries. Returns nil for an empty
// input so JSON marshaling omits the field (omitempty on
// LocationResolvedField.AlternatesConsidered).
func alternateNames(alts []Candidate) []string {
	if len(alts) == 0 {
		return nil
	}
	out := make([]string, 0, len(alts))
	for _, c := range alts {
		if c.Name == "" {
			continue
		}
		out = append(out, c.Name)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
