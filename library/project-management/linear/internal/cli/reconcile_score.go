package cli

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

// Scoring for `reconcile`.
//
// Every candidate gets one confidence in 0..1: a weighted mean of text
// similarity signals, multiplied by context modifiers, with one floor rule.
// Every input, weight and multiplier is reported in the evidence list and in
// thresholds_used, so a caller can recompute the number by hand.
//
// No number in this file is a literal. reconcileTuning carries them all and
// every field is bound to a flag.

// reconcileTuning is the complete numeric configuration of the scorer.
type reconcileTuning struct {
	FTSScale               float64
	WeightTitleDice        float64
	WeightTitleContainment float64
	WeightFTS              float64
	WeightBodyOverlap      float64
	WeightSemantic         float64
	ExactFloor             float64
	PenaltyOutOfGroup      float64
	RecencyHalflife        time.Duration
	RecencyFloor           float64
	PenaltyProjectMismatch float64
}

// reconcileCandidate is one issue under consideration, merged from the local
// FTS leg and, when enabled, the live legs.
type reconcileCandidate struct {
	store.IssueCandidate

	// HasFTS is false for a candidate that only ever appeared in a live leg.
	// Such a candidate has no bm25 value, so the fts_norm signal is absent
	// and its weight leaves the denominator.
	HasFTS bool

	// SemanticHit records that the semantic leg returned this candidate.
	SemanticHit bool

	// URL is populated only by the live legs or by hydration. The sync
	// selection carries no url field, so a purely local candidate has none.
	URL string

	// Legs names every retrieval leg that produced this candidate, in
	// deterministic order.
	Legs []string
}

// reconcileScore is the full arithmetic behind one candidate's confidence.
type reconcileScore struct {
	TitleDice        float64
	TitleContainment float64

	FTSNorm    float64
	HasFTSNorm bool

	BodyOverlap    float64
	HasBodyOverlap bool

	SemanticHit    float64
	HasSemanticHit bool

	TitleExact   bool
	FloorApplied bool

	MatchedTerms []string

	InGroup  bool
	MState   float64
	MRecency float64
	MScope   float64

	AgeDays  float64
	AgeBasis string

	Numerator   float64
	Denominator float64
	Similarity  float64
	Confidence  float64
}

// reconcileQueryText is the proposed content, already tokenised once so the
// tokenisation is not repeated per candidate.
type reconcileQueryText struct {
	Title       string
	Body        string
	TitleTokens map[string]struct{}
	TitleOrder  []string
	BodyTokens  map[string]struct{}
	FoldedTitle string
}

// newReconcileQueryText tokenises the proposal once. bodyChars caps how much
// of the body participates, matching the cap applied to the query text that
// was sent to FTS5.
func newReconcileQueryText(title, body string, bodyChars int) reconcileQueryText {
	q := reconcileQueryText{
		Title:       title,
		Body:        truncateRunes(body, bodyChars),
		FoldedTitle: foldTitleForExactMatch(title),
	}
	q.TitleOrder = store.IssueSearchTokens(title)
	q.TitleTokens = tokenSet(q.TitleOrder)
	q.BodyTokens = tokenSet(store.IssueSearchTokens(q.Body))
	return q
}

// groupPredicate is the subset of the internal/groups predicate the scorer
// needs. Declared as an interface so the scorer stays testable and does not
// widen its dependency surface beyond one method.
type groupPredicate interface {
	Match(stateType, stateName string) bool
}

// scoreCandidate computes one candidate's confidence.
//
// semanticLegCounted must be true only when the semantic leg actually ran and
// the workspace reported the feature enabled. When it is false the signal is
// absent rather than negative, and its weight leaves the denominator.
// projectID is the value of --project, empty when the flag was not given.
func scoreCandidate(
	q reconcileQueryText,
	c reconcileCandidate,
	tuning reconcileTuning,
	group groupPredicate,
	semanticLegCounted bool,
	projectID string,
	now time.Time,
) reconcileScore {
	var s reconcileScore

	candTitleTokens := tokenSet(store.IssueSearchTokens(c.Title))
	shared := intersectCount(q.TitleTokens, candTitleTokens)

	if total := len(q.TitleTokens) + len(candTitleTokens); total > 0 {
		s.TitleDice = 2 * float64(shared) / float64(total)
	}
	if len(q.TitleTokens) > 0 {
		s.TitleContainment = float64(shared) / float64(len(q.TitleTokens))
	}

	// fts_norm squashes the corpus-relative bm25 magnitude into 0..1. bm25
	// is negative for a match and more negative for a better one, so the
	// magnitude is the negated value floored at zero.
	if c.HasFTS {
		s.HasFTSNorm = true
		mag := math.Max(0, -c.Bm25)
		s.FTSNorm = 1 - math.Exp(-mag/tuning.FTSScale)
	}

	candBodyTokens := tokenSet(store.IssueSearchTokens(c.Description))
	if len(q.BodyTokens) > 0 && len(candBodyTokens) > 0 {
		s.HasBodyOverlap = true
		s.BodyOverlap = float64(intersectCount(q.BodyTokens, candBodyTokens)) / float64(len(q.BodyTokens))
	}

	if semanticLegCounted {
		s.HasSemanticHit = true
		if c.SemanticHit {
			s.SemanticHit = 1
		}
	}

	s.TitleExact = q.FoldedTitle != "" && q.FoldedTitle == foldTitleForExactMatch(c.Title)

	// Matched terms are recomputed in Go because FTS5 exposes no per-term
	// match information through SQL. The recomputation is unstemmed, so it
	// can report fewer terms than bm25 actually rewarded, and it never
	// vetoes a decision on its own.
	s.MatchedTerms = matchedQueryTerms(q, candTitleTokens, candBodyTokens)

	// Context modifiers.
	s.InGroup = group == nil || group.Match(c.StateType, c.StateName)
	s.MState = 1
	if !s.InGroup {
		s.MState = tuning.PenaltyOutOfGroup
	}

	s.AgeDays, s.AgeBasis = candidateAgeDays(c, now)
	s.MRecency = 1
	if s.AgeBasis != "unknown" {
		halflifeDays := tuning.RecencyHalflife.Hours() / 24
		if halflifeDays > 0 {
			decay := math.Pow(0.5, s.AgeDays/halflifeDays)
			s.MRecency = tuning.RecencyFloor + (1-tuning.RecencyFloor)*decay
		}
	}

	s.MScope = 1
	if projectID != "" && c.ProjectID != projectID {
		s.MScope = tuning.PenaltyProjectMismatch
	}

	// Weighted mean over the signals that are present. Absent signals leave
	// both numerator and denominator, so a live-only candidate or a bodiless
	// proposal is judged on what exists rather than penalised for a missing
	// input.
	s.Numerator = tuning.WeightTitleDice*s.TitleDice + tuning.WeightTitleContainment*s.TitleContainment
	s.Denominator = tuning.WeightTitleDice + tuning.WeightTitleContainment
	if s.HasFTSNorm {
		s.Numerator += tuning.WeightFTS * s.FTSNorm
		s.Denominator += tuning.WeightFTS
	}
	if s.HasBodyOverlap {
		s.Numerator += tuning.WeightBodyOverlap * s.BodyOverlap
		s.Denominator += tuning.WeightBodyOverlap
	}
	if s.HasSemanticHit {
		s.Numerator += tuning.WeightSemantic * s.SemanticHit
		s.Denominator += tuning.WeightSemantic
	}
	if s.Denominator > 0 {
		s.Similarity = s.Numerator / s.Denominator
	}

	s.Confidence = clamp01(s.Similarity * s.MState * s.MRecency * s.MScope)
	if s.TitleExact && tuning.ExactFloor > s.Confidence {
		s.Confidence = clamp01(tuning.ExactFloor)
		s.FloorApplied = true
	}
	return s
}

// candidateAgeDays reports the candidate's age in days and which timestamp it
// came from: updated_at, falling back to created_at and then synced_at.
func candidateAgeDays(c reconcileCandidate, now time.Time) (float64, string) {
	for _, pick := range []struct {
		basis string
		at    time.Time
	}{
		{"updated_at", c.UpdatedAt},
		{"created_at", c.CreatedAt},
		{"synced_at", c.SyncedAt},
	} {
		if pick.at.IsZero() {
			continue
		}
		days := now.Sub(pick.at).Hours() / 24
		if days < 0 {
			days = 0
		}
		return days, pick.basis
	}
	return 0, "unknown"
}

// matchedQueryTerms returns the query title tokens that literally appear in
// the candidate's title or description, in query order.
func matchedQueryTerms(q reconcileQueryText, candTitle, candBody map[string]struct{}) []string {
	var out []string
	seen := make(map[string]struct{}, len(q.TitleOrder))
	for _, token := range q.TitleOrder {
		if _, dup := seen[token]; dup {
			continue
		}
		seen[token] = struct{}{}
		if _, ok := candTitle[token]; ok {
			out = append(out, token)
			continue
		}
		if _, ok := candBody[token]; ok {
			out = append(out, token)
		}
	}
	return out
}

// buildEvidence renders one candidate's arithmetic as the evidence list, in
// evaluation order. Absent signals carry a null value and the detail "absent",
// so a reader can see the mean was taken over fewer signals.
func buildEvidence(c reconcileCandidate, s reconcileScore, tuning reconcileTuning, band string, bandDetail string) []reconcileEvidence {
	ev := make([]reconcileEvidence, 0, 15)

	ev = append(ev, reconcileEvidence{
		Signal: "source_leg",
		Value:  strings.Join(c.Legs, "+"),
		Weight: 0,
		Detail: fmt.Sprintf("retrieved from %s", strings.Join(c.Legs, ", ")),
	})

	if s.HasFTSNorm {
		ev = append(ev, reconcileEvidence{
			Signal: "fts_bm25_raw",
			Value:  c.Bm25,
			Weight: 0,
			Detail: "raw bm25, negative for a match and corpus-relative, so it has no absolute meaning across stores of different sizes",
		})
		ev = append(ev, reconcileEvidence{
			Signal: "fts_norm",
			Value:  s.FTSNorm,
			Weight: tuning.WeightFTS,
			Detail: fmt.Sprintf("1 - exp(-%.4f / %.4f)", math.Max(0, -c.Bm25), tuning.FTSScale),
		})
	} else {
		ev = append(ev, reconcileEvidence{
			Signal: "fts_bm25_raw",
			Value:  nil,
			Weight: 0,
			Detail: "absent",
		})
		ev = append(ev, reconcileEvidence{
			Signal: "fts_norm",
			Value:  nil,
			Weight: tuning.WeightFTS,
			Detail: "absent, candidate came from the live leg only so it carries no local bm25",
		})
	}

	ev = append(ev, reconcileEvidence{
		Signal: "title_dice",
		Value:  s.TitleDice,
		Weight: tuning.WeightTitleDice,
		Detail: "symmetric title token overlap, 2 * shared / (query + candidate)",
	})
	ev = append(ev, reconcileEvidence{
		Signal: "title_containment",
		Value:  s.TitleContainment,
		Weight: tuning.WeightTitleContainment,
		Detail: "asymmetric title token containment, shared / query",
	})

	exactDetail := "folded titles differ, floor not applied"
	if s.TitleExact && s.FloorApplied {
		exactDetail = "folded titles identical, confidence lifted to the floor"
	} else if s.TitleExact {
		exactDetail = "folded titles identical, confidence already at or above the floor"
	}
	ev = append(ev, reconcileEvidence{
		Signal: "title_exact",
		Value:  s.TitleExact,
		Weight: tuning.ExactFloor,
		Detail: exactDetail + ". Not a member of the mean, it acts as a floor",
	})

	if s.HasBodyOverlap {
		ev = append(ev, reconcileEvidence{
			Signal: "body_overlap",
			Value:  s.BodyOverlap,
			Weight: tuning.WeightBodyOverlap,
			Detail: "body token containment, shared / query body",
		})
	} else {
		ev = append(ev, reconcileEvidence{
			Signal: "body_overlap",
			Value:  nil,
			Weight: tuning.WeightBodyOverlap,
			Detail: "absent, one side has no body tokens",
		})
	}

	if s.HasSemanticHit {
		ev = append(ev, reconcileEvidence{
			Signal: "semantic_hit",
			Value:  s.SemanticHit,
			Weight: tuning.WeightSemantic,
			Detail: "1 when the semantic leg returned this candidate. The payload carries no score, so the signal is boolean",
		})
	} else {
		ev = append(ev, reconcileEvidence{
			Signal: "semantic_hit",
			Value:  nil,
			Weight: tuning.WeightSemantic,
			Detail: "absent, the semantic leg did not run or the workspace reported it disabled",
		})
	}

	ev = append(ev, reconcileEvidence{
		Signal: "matched_terms",
		Value:  s.MatchedTerms,
		Weight: 0,
		Detail: "recomputed in Go with the FTS tokeniser and unstemmed, so it can under-report the terms bm25 rewarded. Never vetoes a decision on its own",
	})
	ev = append(ev, reconcileEvidence{
		Signal: "matched_columns",
		Value:  c.MatchedColumns,
		Weight: 0,
		Detail: "indexed columns FTS5 highlighted for this row",
	})

	groupDetail := "candidate state is inside the resolved candidate group"
	if !s.InGroup {
		groupDetail = "candidate state is outside the resolved candidate group, penalised rather than dropped because a closed twin is still evidence the work was filed"
	}
	ev = append(ev, reconcileEvidence{
		Signal: "state_group",
		Value:  s.InGroup,
		Weight: s.MState,
		Detail: fmt.Sprintf("%s (state %q, type %q)", groupDetail, c.StateName, c.StateType),
	})

	recencyDetail := "no usable timestamp, recency multiplier neutral"
	if s.AgeBasis != "unknown" {
		recencyDetail = fmt.Sprintf("%.2f days old by %s (%s)", s.AgeDays, s.AgeBasis, candidateAgeTimestamp(c, s.AgeBasis))
	}
	ev = append(ev, reconcileEvidence{
		Signal: "recency",
		Value:  s.AgeDays,
		Weight: s.MRecency,
		Detail: recencyDetail,
	})

	scopeDetail := "no project scope requested, multiplier neutral"
	if s.MScope != 1 {
		scopeDetail = "candidate sits in a different project than --project"
	} else if c.ProjectID != "" {
		scopeDetail = "candidate project matches --project, or no scope was requested"
	}
	ev = append(ev, reconcileEvidence{
		Signal: "project_scope",
		Value:  nullableString(c.ProjectID),
		Weight: s.MScope,
		Detail: scopeDetail,
	})

	ev = append(ev, reconcileEvidence{
		Signal: "similarity",
		Value:  s.Similarity,
		Weight: 0,
		Detail: fmt.Sprintf("%.4f / %.4f = %.4f, then * %.4f state * %.4f recency * %.4f scope", s.Numerator, s.Denominator, s.Similarity, s.MState, s.MRecency, s.MScope),
	})
	ev = append(ev, reconcileEvidence{
		Signal: "band",
		Value:  band,
		Weight: 0,
		Detail: bandDetail,
	})
	return ev
}

func candidateAgeTimestamp(c reconcileCandidate, basis string) string {
	switch basis {
	case "updated_at":
		return c.UpdatedAt.UTC().Format(time.RFC3339)
	case "created_at":
		return c.CreatedAt.UTC().Format(time.RFC3339)
	case "synced_at":
		return c.SyncedAt.UTC().Format(time.RFC3339)
	}
	return ""
}

// foldTitleForExactMatch lowercases, drops every character that is not a
// letter or a digit, and collapses whitespace runs to a single space. Used
// only by the title_exact floor.
func foldTitleForExactMatch(s string) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if pendingSpace && b.Len() > 0 {
				b.WriteRune(' ')
			}
			pendingSpace = false
			b.WriteRune(r)
		default:
			pendingSpace = true
		}
	}
	return b.String()
}

func tokenSet(tokens []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		set[t] = struct{}{}
	}
	return set
}

func intersectCount(a, b map[string]struct{}) int {
	if len(b) < len(a) {
		a, b = b, a
	}
	n := 0
	for k := range a {
		if _, ok := b[k]; ok {
			n++
		}
	}
	return n
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// truncateRunes returns the leading n bytes of s, backed off to the nearest
// rune boundary so a multi-byte character is never split. n <= 0 returns "".
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// utf8RuneStart reports whether b can begin a UTF-8 encoded rune.
func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
