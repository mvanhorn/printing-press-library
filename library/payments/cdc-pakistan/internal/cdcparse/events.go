// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cdcparse

import (
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/cliutil"
)

// EligibilityState is a node in CDC's CDS-eligibility lifecycle.
//
// The lifecycle is SIX states, not three. Measured over 7,639 notices and
// circulars spanning 2007-2026:
//
//	declared (1,281) -> intention-to-suspend (127) -> suspended (961)
//	  -> extension (644) -> removal-of-suspension (82)
//	  | revoked (385) | admission-terminated (55)
//
// The intention-to-suspend and removal-of-intention states were missed on a
// first pass and matter: a suspension chain read without them mis-dates the
// start of a restriction.
type EligibilityState string

const (
	StateDeclared            EligibilityState = "declared"
	StateIntentionToSuspend  EligibilityState = "intention-to-suspend"
	StateRemovalOfIntention  EligibilityState = "removal-of-intention"
	StateSuspended           EligibilityState = "suspended"
	StateExtension           EligibilityState = "extension-of-suspension"
	StateRemovalOfSuspension EligibilityState = "removal-of-suspension"
	StateRevoked             EligibilityState = "revoked"
	StateTerminated          EligibilityState = "admission-terminated"
)

// EventKind covers everything a notice or circular can announce, including
// corporate actions that are not eligibility transitions.
type EventKind string

const (
	KindEligibility  EventKind = "eligibility"
	KindIdentity     EventKind = "identity"    // symbol / name change
	KindCorpAction   EventKind = "corp-action" // bonus, rights, IPO, sub-division
	KindRestriction  EventKind = "restriction"
	KindStructural   EventKind = "structural"  // merger, conversion
	KindOperational  EventKind = "operational" // NMS education, RTA blocking, PCM appointment
	KindUnclassified EventKind = "unclassified"
)

// Event is a classified document title.
type Event struct {
	Kind  EventKind        `json:"kind"`
	State EligibilityState `json:"state,omitempty"`
	// Action names a corp-action or structural subtype, e.g. "bonus".
	Action string `json:"action,omitempty"`
	// Confidence is "high" for an unambiguous phrase match and "low" when only a
	// weak signal matched. Callers surface it; the eligibility state machine is
	// the least reliable extractor in this CLI and must say so.
	Confidence string `json:"confidence"`
}

type rule struct {
	re     *regexp.Regexp
	kind   EventKind
	state  EligibilityState
	action string
}

// Order is significant. "extension of suspension" must be tested before
// "suspension", and "removal of intention" before "intention".
var rules = []rule{
	{regexp.MustCompile(`(?i)removal of intention`), KindEligibility, StateRemovalOfIntention, ""},
	{regexp.MustCompile(`(?i)intention to suspend`), KindEligibility, StateIntentionToSuspend, ""},
	{regexp.MustCompile(`(?i)extension of (the )?suspension`), KindEligibility, StateExtension, ""},
	{regexp.MustCompile(`(?i)removal of (the )?suspension`), KindEligibility, StateRemovalOfSuspension, ""},
	{regexp.MustCompile(`(?i)revocation of (the )?cds eligib`), KindEligibility, StateRevoked, ""},
	{regexp.MustCompile(`(?i)suspension of admission`), KindEligibility, StateTerminated, ""},
	{regexp.MustCompile(`(?i)termination of admission`), KindEligibility, StateTerminated, ""},
	{regexp.MustCompile(`(?i)suspension of (the )?cds eligib`), KindEligibility, StateSuspended, ""},
	{regexp.MustCompile(`(?i)declaration of cds eligib`), KindEligibility, StateDeclared, ""},

	{regexp.MustCompile(`(?i)change of (security )?(name and symbol|symbol|name)`), KindIdentity, "", "symbol-name-change"},

	{regexp.MustCompile(`(?i)credit of bonus`), KindCorpAction, "", "bonus"},
	{regexp.MustCompile(`(?i)credit of (unpaid )?rights`), KindCorpAction, "", "rights"},
	{regexp.MustCompile(`(?i)credit of new issue|initial offering`), KindCorpAction, "", "new-issue"},
	{regexp.MustCompile(`(?i)sub-?division of shares`), KindCorpAction, "", "sub-division"},

	{regexp.MustCompile(`(?i)merger|amalgamat|scheme of arrangement`), KindStructural, "", "merger"},
	{regexp.MustCompile(`(?i)conversion of`), KindStructural, "", "conversion"},

	{regexp.MustCompile(`(?i)restriction`), KindRestriction, "", ""},
	{regexp.MustCompile(`(?i)freez`), KindRestriction, "", "freeze"},

	{regexp.MustCompile(`(?i)remote education|nms`), KindOperational, "", "nms-education"},
	{regexp.MustCompile(`(?i)blocking of rta|rta reconciliation|rta authorit`), KindOperational, "", "rta"},
	{regexp.MustCompile(`(?i)professional clearing member|appointment of`), KindOperational, "", "appointment"},
}

// ClassifyTitle maps a document title onto an event.
//
// Two decades of naming drift make this the least reliable extractor in the
// CLI, which is why every result carries a confidence and why unclassified is a
// first-class outcome rather than a silent default.
func ClassifyTitle(title string) Event {
	t := cliutil.CleanText(title)
	for _, r := range rules {
		if r.re.MatchString(t) {
			conf := "high"
			// A bare "restriction" or "appointment of" match is a weak signal.
			if r.kind == KindRestriction && r.action == "" {
				conf = "low"
			}
			if r.action == "appointment" {
				conf = "low"
			}
			return Event{Kind: r.kind, State: r.state, Action: r.action, Confidence: conf}
		}
	}
	return Event{Kind: KindUnclassified, Confidence: "none"}
}

// IsPenetrationReport reports whether a title names a Share-Percentage-in-CDS
// vintage. That series is the CLI's flagship input and lives in the
// `miscellaneous` category, not in `list-of-securities`.
func IsPenetrationReport(title string) bool {
	t := strings.ToLower(cliutil.CleanText(title))
	return strings.Contains(t, "share percentage in cds") ||
		(strings.Contains(t, "share") && strings.Contains(t, "paid up capital"))
}

// ReportPart returns "A", "B" or "" for a penetration report URL. The report is
// split by INSTRUMENT CLASS -- part A is ordinary/preference/modaraba, part B is
// open-end/ETF funds and saving certificates -- and the parts are disjoint, so
// treating either alone as the month is a partial slice.
var partRe = regexp.MustCompile(`(?i)-([AB])\.pdf$`)

func ReportPart(url string) string {
	if m := partRe.FindStringSubmatch(url); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}
