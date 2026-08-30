package scengine

import (
	"regexp"
	"sort"
	"strings"
)

// Stance is a work's reported finding relative to an intervention/claim
// "working" or being positively associated. It is a heuristic signal, not a
// trained classifier — output always carries the method used.
type Stance string

const (
	StanceSupporting   Stance = "supporting"   // reports a positive/significant effect or association
	StanceRefuting     Stance = "refuting"     // reports null, no effect, or harm
	StanceMixed        Stance = "mixed"        // both positive and negative signals
	StanceInconclusive Stance = "inconclusive" // no clear directional signal
)

var (
	// Positive/effect cues: the intervention did something beneficial or a
	// positive association was found. "increas*" is matched plainly here; Go's
	// RE2 engine has no negative lookahead, so harm-context phrasing like
	// "increased risk" / "increased mortality" is excluded per-match in
	// ClassifyStance by re-inspecting the matched window against
	// increaseHarmContext (that phrasing belongs to harmCues instead).
	supportCues = regexp.MustCompile(`(?i)\b(improv\w+|increas\w+|reduc\w+ (the )?risk|lower\w* (the )?risk|effective\b|efficac\w+|beneficial|benefit\w*|protect\w+|associated with (a )?(reduc\w+|lower|decreas\w+)|significant\w* (improv|increas|reduc|benefit)|positive (effect|association|impact)|enhanc\w+|alleviat\w+|prevent\w+)`)

	// increaseHarmContext detects when an "increas*" support match is actually
	// harm-context phrasing ("increased risk", "increases the mortality",
	// "increased incidence of ..."). Applied to a short window starting at the
	// support match; a hit means the match is not counted as support. This is
	// the RE2-safe replacement for a negative lookahead — do not reintroduce
	// (?!...) here.
	increaseHarmContext = regexp.MustCompile(`(?i)\bincreas\w+\s+(the\s+|a\s+)?(risk|mortality|morbidity|incidence|harm\w*|adverse|complication\w*|death\w*|odds of)`)

	// Null / no-effect cues.
	nullCues = regexp.MustCompile(`(?i)\b(no (significant )?(association|effect|difference|benefit|evidence|impact|correlation)|not (significant\w*|associated|effective)|did not (significantly )?(improv|increas|reduc|affect|differ|change)|ineffective|no statistically significant|failed to|without (a )?(significant )?(effect|benefit)|null (result|effect|finding))`)

	// Harm / negative-effect cues (treated as refuting an intervention claim).
	harmCues = regexp.MustCompile(`(?i)\b(increas\w+ (the )?(risk|mortality|morbidity|incidence)|harmful|adverse (effect|event|outcome)|worsen\w*|associated with (a )?(higher|increas\w+|greater) (risk|mortality|incidence)|detrimental|toxic\w*|negative (effect|impact|association)|deteriorat\w+)`)

	// Claim-direction cues: what the *claim itself* asserts, not what the
	// paper found. A HARM-asserting claim ("X causes Y", "X increases the
	// risk of Y") inverts the support/refute mapping in ClassifyStance. Both
	// are RE2-safe keyword alternations — no lookahead, matching the cue
	// style above and the increaseHarmContext window technique.
	claimHarmCues    = regexp.MustCompile(`(?i)\b(caus\w+|increas\w+ (the )?risk|rais\w+ (the )?risk|worsen\w*|lead\w* to|harm\w*|damag\w*|toxic\w*)`)
	claimBenefitCues = regexp.MustCompile(`(?i)\b(improv\w+|reduc\w+ (the )?risk|lower\w* (the )?risk|prevent\w+|treat\w+|protect\w+|benefi\w+|enhanc\w+|alleviat\w+|boost\w*|cure\w*|effective\b)`)

	// polarityVerbCues locates the split point in a claim for PICOTokens.
	// Direction-neutral: fires on both harm and benefit verbs so the PICO
	// gate activates for both claim directions. Used only by PICOTokens in
	// pico.go — detectClaimDirection keeps using claimHarmCues/claimBenefitCues.
	polarityVerbCues = regexp.MustCompile(`(?i)\b(caus|improv|reduc|increas|decreas|prevent|lower|rais|protect|worsen|treat|affect|promot|contribut|alleviat)\w*`)

	// Direction cues for harm-asserting claims: generic comparatives whose
	// surrounding scope must name the claim's content before they count (see
	// positiveCueCounts), so "greater weight gain" ties to a weight-gain claim
	// but "greater adherence" does not.
	directionUpCues   = regexp.MustCompile(`(?i)\b(significantly\s+)?(greater|higher|elevated|more|increas\w+)\b`)
	directionDownCues = regexp.MustCompile(`(?i)\b(significantly\s+)?(less|lower|fewer|smaller|reduc\w+|decreas\w+)\b`)

	// negationCues mark a clause as negated. Applied ONLY to a short backward
	// window before a positive cue (negation scope is local): a sentence-wide
	// search would silently drop genuine harm findings that merely follow an
	// unrelated "no" earlier in the same sentence.
	negationCues = regexp.MustCompile(`(?i)\b(no|not|never|without|lack\w*|fail\w*|against|reject\w*|absence)\b`)

	// framingCues mark a sentence as posing a question, restating someone
	// else's hypothesis, or describing a public belief — rhetorical framing,
	// not a reported finding. Applied to the WHOLE enclosing sentence because
	// framing is a property of the sentence, not of the words next to the cue
	// ("Objective: To evaluate whether the vaccine increases the risk ...").
	framingCues = regexp.MustCompile(`(?i)(objective:|\bwhether\b|hypothes\w*|\bconcern\w*|\bbelief\w*|\bmyth\w*|misinformation)`)

	// claimTokenSplit tokenizes a lowercased claim for content-token
	// extraction (shared with the CLI relevance gate via ClaimContentTokens).
	claimTokenSplit = regexp.MustCompile(`[^a-z0-9]+`)

	// strongRefutCues (Option C) are syntactically unambiguous refutations of a
	// causal claim. They are matched against the FULL text and need no pairing:
	// the phrasing itself names the causal relation ("does not cause", "no
	// causal link"), so there is nothing left to tie to the claim. The B2
	// framing gate still applies — "whether vaccines do not cause autism" is a
	// question, not a refutation.
	//
	// Deliberately separate from nullCues: nullCues also feeds the
	// claim-agnostic baseline used by benefit-asserting claims, where "does not
	// cause side effects" is not a null result. These fire on the harm branch
	// only.
	strongRefutCues = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:did|does?|do)\s+not\s+cause\b`),
		regexp.MustCompile(`(?i)\bdoes?\s+not\s+support\s+(?:a\s+)?causal\b`),
		regexp.MustCompile(`(?i)\bnot\s+support\s+(?:a\s+)?causal\b`),
		regexp.MustCompile(`(?i)\bno\s+causal\s+(?:link|association|relationship|evidence)\b`),
		regexp.MustCompile(`(?i)\black\s+of\s+(?:(?:a|any)\s+)?(?:causal\s+)?association\b`),
		regexp.MustCompile(`(?i)\bnot\s+associated?\s+with\s+(?:an?\s+)?increased?\s+risk\b`),
	}

	// metaRefutCues (Option C) are meta-scientific markers: the paper is about
	// a claim having been withdrawn or disproved. They are far weaker evidence
	// than strongRefutCues because a paper ARGUING FOR the link can mention a
	// retraction in its background, so both sides of the claim must appear
	// within metaRefutRadius bytes of the cue before it counts.
	metaRefutCues = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bevidence\s+(?:strongly\s+)?against\b`),
		regexp.MustCompile(`(?i)\bfurther\s+evidence\s+against\b`),
		regexp.MustCompile(`(?i)\brefut(?:e[sd]?|ing|ation)\b`),
		regexp.MustCompile(`(?i)\bdebunk(?:s|ed|ing)?\b`),
		regexp.MustCompile(`(?i)\bretract(?:ed|ion|ing)?\b`),
		regexp.MustCompile(`(?i)\bfraud(?:ulent)?\b`),
	}
)

const (
	// posPairWindow bounds how far from a positive cue the claim's two sides
	// may sit. Intersected with the enclosing sentence, so it is a cap on an
	// already sentence-scoped window, never a way to reach across a sentence
	// boundary.
	posPairWindow = 120

	// negationLookback is how far back from a positive cue a negation word is
	// searched for. Deliberately short: "does not increase the risk" must be
	// caught, "no benefit was seen, but mortality increased" must not be.
	negationLookback = 25

	// claimStemLen truncates content tokens so inflected forms still match
	// ("sweeteners" -> "sweete" matches "sweetener" and "sweetened").
	//
	// Six, not five. Five is short enough to land on a prefix the literature
	// uses for unrelated words: "intermittent" becomes "inter", which matches
	// intervention, interaction, interval, interquartile, interplay, internal,
	// interpret and interested; "alertness" becomes "alert", which matches the
	// "Safety Alert" of a review about food fermentation. Six splits both pairs
	// while leaving every inflection group in the corpora intact: improves,
	// improved and improvement still share "improv"; caffeine and caffeinated
	// share "caffei"; attention and attentive share "attent".
	//
	// Measured on testdata/corpora: only fasting moves (leakage 5/57 -> 25/57,
	// and the gate drops 3 works -> 20, nine of them decided on a full abstract
	// rather than a truncated stump). No verdict changes on any corpus, and no
	// stance changes on any work that survives the gate. vitaminc is the
	// control: its glued phrase [vitamin c] is never truncated, and it holds.
	claimStemLen = 6

	// metaRefutRadius is how far either side of a meta refutation cue the
	// claim's two sides are searched for. Wider than posPairWindow's sentence
	// clipping because these cues legitimately sit in a title while the claim
	// terms sit in the subtitle.
	metaRefutRadius = 80

	// refutationConfidence is the confidence reported when an Option C cue
	// decides the stance. It matches what a single dominant cue earns from
	// confidenceFrom, so a decisive refutation is not scored as more certain
	// than the counting path can justify.
	refutationConfidence = 0.9
)

// ClassifyStance scores a single work's title+abstract relative to the claim.
// The claim's asserted direction is detected first: a HARM-asserting claim
// ("X causes Y") counts a paper reporting the harm as supporting and a paper
// reporting benefit or less of the harm as refuting. BENEFIT-asserting and
// ambiguous claims (including the empty claim) keep the claim-agnostic
// baseline: stance from the reported finding's polarity. confidence is 0..1.
func ClassifyStance(title, abstract, claim string) (Stance, float64) {
	hay := strings.ToLower(normalizeText(title + ". " + abstract))
	if detectClaimDirection(claim) == claimHarm {
		return classifyAgainstHarmClaim(hay, claim)
	}
	// Count support matches, excluding "increas*" hits whose immediate context
	// is a harm claim ("increased risk/mortality/..."). RE2 has no negative
	// lookahead, so each match window is re-inspected instead.
	support := 0
	for _, loc := range supportCues.FindAllStringIndex(hay, -1) {
		if strings.Contains(hay[loc[0]:loc[1]], "increas") {
			end := loc[1] + 24 // room for "increased" + " the mortality" etc.
			if end > len(hay) {
				end = len(hay)
			}
			if increaseHarmContext.MatchString(hay[loc[0]:end]) {
				continue
			}
		}
		support++
	}
	null := len(nullCues.FindAllString(hay, -1))
	harm := len(harmCues.FindAllString(hay, -1))

	// Null cues frequently overlap with support phrasing ("no significant
	// reduction in risk"); count net.
	return stanceFromCounts(support, null+harm)
}

// stanceFromCounts maps positive/negative cue counts to a stance + confidence.
// Shared by the claim-agnostic baseline and the harm-claim path so both apply
// identical mixed/threshold/confidence logic.
func stanceFromCounts(pos, neg int) (Stance, float64) {
	total := pos + neg

	if total == 0 {
		return StanceInconclusive, 0.2
	}

	// Mixed when both sides have meaningful signal and neither dominates.
	if pos > 0 && neg > 0 {
		ratio := float64(max(pos, neg)) / float64(total)
		if ratio < 0.65 {
			return StanceMixed, 0.4 + 0.2*ratio
		}
	}

	if pos > neg {
		return StanceSupporting, confidenceFrom(pos, total)
	}
	if neg > pos {
		return StanceRefuting, confidenceFrom(neg, total)
	}
	return StanceMixed, 0.45
}

// claimDir is the direction a claim asserts about its subject.
type claimDir int

const (
	claimAmbiguous claimDir = iota // unknown/both — use the claim-agnostic baseline
	claimBenefit                   // "X improves/prevents/treats Y" — today's semantics
	claimHarm                      // "X causes/worsens/raises the risk of Y"
)

// detectClaimDirection classifies what the claim asserts. Conservative: harm
// only when harm cues fire and benefit cues do not; anything else (empty
// claim, neither cue, or both) is ambiguous and keeps the baseline behavior,
// so misdetection can never regress benefit-asserting claims.
func detectClaimDirection(claim string) claimDir {
	if strings.TrimSpace(claim) == "" {
		return claimAmbiguous
	}
	harm := claimHarmCues.MatchString(claim)
	benefit := claimBenefitCues.MatchString(claim)
	switch {
	case harm && !benefit:
		return claimHarm
	case benefit && !harm:
		return claimBenefit
	default:
		return claimAmbiguous
	}
}

// classifyAgainstHarmClaim scores a work against a HARM-asserting claim.
// Reported harm supports the claim; null findings, reported benefit, and
// downward direction cues on a claim token refute it.
//
// The positive side is deliberately much harder to earn than the negative one.
// A vaccine-safety literature search is saturated with harm VOCABULARY
// ("adverse event", "toxic", "increased risk") that appears in papers whose
// actual finding is the opposite, so an ungated positive count reads a corpus
// of refutations as agreement. Every positive cue must therefore survive four
// gates (see positiveCueSpans / positiveCueCounts):
//
//	dedup   — harmCues and directionUpCues matching the same span count once
//	framing — cues inside a question/hypothesis/belief sentence are not findings
//	negation— "does not increase the risk" is not a report of increased risk
//	pairing — the claim's intervention side AND outcome side must both be in scope
//
// Under-counting harm is the safer failure here: it yields inconclusive, not a
// false confirmation. Window re-inspection is the RE2-safe substitute for
// lookahead, mirroring increaseHarmContext — do not reintroduce (?!...) here.
func classifyAgainstHarmClaim(hay, claim string) (Stance, float64) {
	stems := claimTokenStems(claim)
	intervention, outcome := claimSides(claim)
	// Pairing needs both sides of the claim. When the claim cannot be split
	// (no polarity verb, or nothing but polarity words on one side) the gate
	// is disabled and directionUpCues fall back to the original trailing-window
	// check, so an unsplittable claim degrades to the previous behavior rather
	// than to silence.
	paired := len(intervention) > 0 && len(outcome) > 0

	pos := 0
	for _, sp := range positiveCueSpans(hay, stems, paired) {
		if positiveCueCounts(hay, sp, intervention, outcome, paired) {
			pos++
		}
	}

	neg := len(nullCues.FindAllString(hay, -1))
	for _, loc := range directionDownCues.FindAllStringIndex(hay, -1) {
		if windowHasClaimToken(hay, loc[1], stems) {
			neg++
		}
	}
	// A reported benefit contradicts a harm claim. "increas*" support matches
	// are direction-ambiguous here and already handled by directionUpCues.
	// Direction-ambiguous stems are excluded outright: in harm literature
	// "positive association" and "enhanced" report the association's sign or
	// magnitude, not a benefit, and "increas*" is already directionUpCues'.
	// The rest must clear the SAME framing/negation/pairing gates the positive
	// side clears. Measured on the smoking corpus (26 works, 2026-08-16): the
	// ungated block produced 16 refuting against 3 supporting, and every one of
	// those 16 carried nullCues=0 — the refutation came entirely from
	// "prevention" (13 hits), "improv*" (11) and "benefit*" (6), which a causal
	// corpus is saturated with precisely BECAUSE the association is real.
	// Gated: 4 refuting, 6 supporting. Doll & Peto's 40-year cohort stops being
	// read as a refutation of the claim it established.
	for _, loc := range supportCues.FindAllStringIndex(hay, -1) {
		m := hay[loc[0]:loc[1]]
		if strings.Contains(m, "increas") ||
			strings.Contains(m, "positive ") ||
			strings.Contains(m, "enhanc") {
			continue
		}
		sp := cueSpan{loc[0], loc[1]}
		sentStart, sentEnd := sentenceBounds(hay, sp.start)
		if framingCues.MatchString(hay[sentStart:sentEnd]) {
			continue
		}
		if negationCues.MatchString(backWindow(hay, sp.start, sentStart)) {
			continue
		}
		if paired && !pairInScope(hay, sp, sentStart, sentEnd, intervention, outcome) {
			continue
		}
		neg++
	}

	// --- Option C: refutation cues (harm branch only; nullCues untouched) ---
	//
	// These run after the counting pass and can override it. That is deliberate
	// for STRONG cues: "the evidence does not support a causal association" is a
	// direct answer to the claim, and it must not be outvoted by harm VOCABULARY
	// counted elsewhere in the same abstract. META cues are gated by pairing
	// because they describe the controversy, not the finding.
	if refuted, ok := refutationOverride(hay, intervention, outcome, paired); ok {
		return refuted, refutationConfidence
	}

	return stanceFromCounts(pos, neg)
}

// refutationOverride applies the Option C cue sets. It reports StanceRefuting
// and true when a refutation cue fires, or false when none does and the caller
// should fall back to the counting path.
func refutationOverride(hay string, intervention, outcome []string, paired bool) (Stance, bool) {
	// STRONG: full text, no pairing required, but the B2 framing gate still
	// applies — a question that contains the refutation phrasing is not a
	// refutation. Framing is checked over the whole text here (not per
	// sentence) because a strong cue overrides the entire counting pass, so it
	// deserves the more conservative gate.
	if !framingCues.MatchString(hay) {
		for _, re := range strongRefutCues {
			if re.MatchString(hay) {
				return StanceRefuting, true
			}
		}
	}

	// META: both sides of the claim must sit within metaRefutRadius of the cue.
	// An unsplittable claim cannot pair, so meta cues are skipped entirely
	// rather than fired unconditionally.
	if !paired {
		return "", false
	}
	for _, re := range metaRefutCues {
		for _, loc := range re.FindAllStringIndex(hay, -1) {
			if metaRefutPairing(hay, loc[0], intervention, outcome, metaRefutRadius) {
				return StanceRefuting, true
			}
		}
	}
	return "", false
}

// metaRefutPairing reports whether both claim sides appear within radius bytes
// of pos in hay. The claim sides are stemmed token lists here rather than
// regexes, matching how pairInScope ties a cue to the claim.
func metaRefutPairing(hay string, pos int, intervention, outcome []string, radius int) bool {
	start := pos - radius
	if start < 0 {
		start = 0
	}
	end := pos + radius
	if end > len(hay) {
		end = len(hay)
	}
	w := hay[start:end]
	return containsAnyStem(w, intervention) && containsAnyStem(w, outcome)
}

// cueSpan is a byte range in the haystack where a positive cue matched.
type cueSpan struct{ start, end int }

// positiveCueSpans collects every candidate harm-supporting cue occurrence and
// removes duplicates. harmCues and directionUpCues overlap heavily by design
// ("increased risk" matches both), and counting the same words twice inflates
// the positive side past the mixed/dominant threshold in stanceFromCounts.
// Overlapping spans are therefore collapsed to the longest one.
func positiveCueSpans(hay string, stems []string, paired bool) []cueSpan {
	spans := make([]cueSpan, 0, 8)
	for _, loc := range harmCues.FindAllStringIndex(hay, -1) {
		spans = append(spans, cueSpan{loc[0], loc[1]})
	}
	for _, loc := range directionUpCues.FindAllStringIndex(hay, -1) {
		// A bare comparative needs a tie to the claim. With a usable claim
		// split the stricter pair gate in positiveCueCounts supersedes the
		// trailing-window check; without one it is the only tie available.
		if !paired && !windowHasClaimToken(hay, loc[1], stems) {
			continue
		}
		spans = append(spans, cueSpan{loc[0], loc[1]})
	}
	// Longest-first at equal starts so the collapsed span is the widest match.
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end > spans[j].end
	})
	deduped := make([]cueSpan, 0, len(spans))
	lastEnd := -1
	for _, sp := range spans {
		if sp.start < lastEnd {
			continue // same words as an already-counted cue
		}
		deduped = append(deduped, sp)
		lastEnd = sp.end
	}
	return deduped
}

// positiveCueCounts reports whether one positive cue occurrence is a genuine
// report of the claimed harm, applying the framing, negation, and pairing
// gates described on classifyAgainstHarmClaim.
func positiveCueCounts(hay string, sp cueSpan, intervention, outcome []string, paired bool) bool {
	sentStart, sentEnd := sentenceBounds(hay, sp.start)

	// Framing: the sentence poses a question or restates a belief.
	if framingCues.MatchString(hay[sentStart:sentEnd]) {
		return false
	}
	// Negation, local scope: "... does not increase the risk ...".
	if negationCues.MatchString(backWindow(hay, sp.start, sentStart)) {
		return false
	}
	// Pairing: both sides of the claim must be in scope, so a cue about the
	// outcome alone ("autism incidence increased sevenfold") or about the
	// intervention alone ("increase in negative vaccine tweets") is not read
	// as evidence that the intervention causes the outcome.
	if paired && !pairInScope(hay, sp, sentStart, sentEnd, intervention, outcome) {
		return false
	}
	return true
}

// pairInScope reports whether the claim's intervention side and outcome side
// both appear within posPairWindow of the cue, clipped to the cue's sentence.
func pairInScope(hay string, sp cueSpan, sentStart, sentEnd int, intervention, outcome []string) bool {
	lo := sp.start - posPairWindow
	if lo < sentStart {
		lo = sentStart
	}
	hi := sp.end + posPairWindow
	if hi > sentEnd {
		hi = sentEnd
	}
	if lo >= hi {
		return false
	}
	win := hay[lo:hi]
	return containsAnyStem(win, intervention) && containsAnyStem(win, outcome)
}

// backWindow returns the text immediately before a cue, clipped to the cue's
// sentence and to negationLookback characters, and advanced to the next word
// boundary so a truncated word ("cannot" -> "not") cannot fake a negation.
func backWindow(hay string, at, sentStart int) string {
	from := at - negationLookback
	if from < sentStart {
		from = sentStart
	}
	if from < 0 {
		from = 0
	}
	if from > sentStart {
		if sp := strings.IndexByte(hay[from:at], ' '); sp >= 0 {
			from += sp
		} else {
			from = at
		}
	}
	return hay[from:at]
}

// sentenceBounds returns the [start, end) range of the sentence containing the
// byte offset at.
func sentenceBounds(hay string, at int) (int, int) {
	if at > len(hay) {
		at = len(hay)
	}
	start := 0
	for i := at - 1; i > 0; i-- {
		if isSentenceBreak(hay, i) {
			start = i + 2
			break
		}
	}
	if start > at {
		start = at
	}
	end := len(hay)
	for i := at; i < len(hay); i++ {
		if isSentenceBreak(hay, i) {
			end = i + 1
			break
		}
	}
	return start, end
}

// isSentenceBreak reports whether hay[i] terminates a sentence: ".", ";", "?"
// or "!" followed by a space and not preceded by a digit, so decimals and
// confidence intervals ("0.3 per 10 000", "1.18; p=0.11") stay inside one
// sentence. ":" is deliberately NOT a break, so a section label stays attached
// to its clause and framingCues can still see "Objective:".
func isSentenceBreak(hay string, i int) bool {
	switch hay[i] {
	case '.', ';', '?', '!':
	default:
		return false
	}
	if i+1 >= len(hay) || hay[i+1] != ' ' {
		return false
	}
	if i > 0 && hay[i-1] >= '0' && hay[i-1] <= '9' {
		return false
	}
	return true
}

// claimSides splits a harm-asserting claim around its polarity verb: the
// content tokens before it name the intervention ("artificial sweeteners"),
// the ones after it name the outcome ("weight gain"). Both are stemmed like
// claimTokenStems. Either side may come back empty — callers must treat that
// as "cannot pair" rather than "no match".
func claimSides(claim string) (intervention, outcome []string) {
	lc := strings.ToLower(claim)
	loc := claimHarmCues.FindStringIndex(lc)
	if loc == nil {
		return nil, nil
	}
	return stemTokens(ClaimContentTokens(lc[:loc[0]])), stemTokens(ClaimContentTokens(lc[loc[1]:]))
}

// windowHasClaimToken reports whether the 40 characters after a direction-cue
// match mention one of the claim's stemmed content tokens — the RE2-safe way
// to tie a generic comparative ("greater", "less") to the claim's outcome.
func windowHasClaimToken(hay string, from int, stems []string) bool {
	end := from + 40
	if end > len(hay) {
		end = len(hay)
	}
	return containsAnyStem(hay[from:end], stems)
}

// containsAnyStem reports whether s contains any of the stems.
func containsAnyStem(s string, stems []string) bool {
	for _, st := range stems {
		if strings.Contains(s, st) {
			return true
		}
	}
	return false
}

// claimStopwords are function words ignored when extracting a claim's content
// tokens. Only words of length >= 4 need listing; shorter tokens are dropped
// by the length floor in ClaimContentTokens.
var claimStopwords = map[string]struct{}{
	"that": {}, "this": {}, "with": {}, "than": {}, "from": {}, "does": {},
	"were": {}, "have": {}, "been": {}, "into": {}, "over": {}, "under": {},
	"between": {}, "among": {}, "about": {}, "versus": {}, "when": {},
	"where": {}, "which": {}, "their": {}, "there": {}, "these": {},
	"those": {}, "they": {}, "them": {}, "more": {}, "most": {}, "some": {},
	"each": {}, "much": {},
}

// claimPolarityPrefixes are direction/polarity word stems excluded from a
// claim's content tokens: overlap on "improves" or "causes" alone must never
// make a work relevant to a claim, and direction words are matched by the cue
// regexes, not by token overlap.
var claimPolarityPrefixes = []string{
	"improv", "reduc", "lower", "prevent", "treat", "protect", "benefi",
	"enhanc", "alleviat", "boost", "cure", "effect", "caus", "increas",
	"decreas", "rais", "worsen", "harm", "damag", "toxic", "lead", "risk",
	"great", "high", "less", "fewer", "small", "elevat", "signific",
}

// ClaimContentTokens returns the claim's lowercase content tokens: words of
// four or more characters with stopwords and polarity/direction words
// removed. Shared by the harm-claim window checks here and the CLI's
// relevance gate.
func ClaimContentTokens(claim string) []string {
	toks := claimTokenSplit.Split(strings.ToLower(claim), -1)
	out := make([]string, 0, len(toks))
	for _, tok := range toks {
		if len(tok) < 4 {
			continue
		}
		if _, stop := claimStopwords[tok]; stop {
			continue
		}
		if hasPolarityPrefix(tok) {
			continue
		}
		out = append(out, tok)
	}
	return out
}

func hasPolarityPrefix(tok string) bool {
	for _, p := range claimPolarityPrefixes {
		if strings.HasPrefix(tok, p) {
			return true
		}
	}
	return false
}

// claimTokenStems truncates content tokens to their first claimStemLen
// characters so inflected forms still match ("sweeteners" -> "sweet" matches
// "sweetener").
func claimTokenStems(claim string) []string {
	return stemTokens(ClaimContentTokens(claim))
}

// stemTokens truncates tokens in place to claimStemLen characters.
func stemTokens(toks []string) []string {
	for i, t := range toks {
		if len(t) > claimStemLen {
			toks[i] = t[:claimStemLen]
		}
	}
	return toks
}

func confidenceFrom(dominant, total int) float64 {
	if total == 0 {
		return 0.2
	}
	c := float64(dominant) / float64(total)
	// Scale into a calibrated-feeling 0.3..0.9 band; more cues = more confident.
	conf := 0.3 + 0.6*c
	if total >= 4 {
		conf += 0.05
	}
	if conf > 0.95 {
		conf = 0.95
	}
	return conf
}
