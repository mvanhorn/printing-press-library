package scengine

import (
	"regexp"
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

	// Direction cues for harm-asserting claims: generic comparatives whose
	// trailing window must name a claim content token (see
	// windowHasClaimToken) before they count, so "greater weight gain" ties
	// to a weight-gain claim but "greater adherence" does not.
	directionUpCues   = regexp.MustCompile(`(?i)\b(significantly\s+)?(greater|higher|elevated|more|increas\w+)\b`)
	directionDownCues = regexp.MustCompile(`(?i)\b(significantly\s+)?(less|lower|fewer|smaller|reduc\w+|decreas\w+)\b`)

	// claimTokenSplit tokenizes a lowercased claim for content-token
	// extraction (shared with the CLI relevance gate via ClaimContentTokens).
	claimTokenSplit = regexp.MustCompile(`[^a-z0-9]+`)
)

// ClassifyStance scores a single work's title+abstract relative to the claim.
// The claim's asserted direction is detected first: a HARM-asserting claim
// ("X causes Y") counts a paper reporting the harm as supporting and a paper
// reporting benefit or less of the harm as refuting. BENEFIT-asserting and
// ambiguous claims (including the empty claim) keep the claim-agnostic
// baseline: stance from the reported finding's polarity. confidence is 0..1.
func ClassifyStance(title, abstract, claim string) (Stance, float64) {
	hay := strings.ToLower(title + ". " + abstract)
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
// Reported harm — harm cues, or an upward direction cue whose trailing window
// names a claim token — supports the claim; null findings, reported benefit,
// and downward direction cues on a claim token refute it. Window
// re-inspection is the RE2-safe substitute for lookahead, mirroring
// increaseHarmContext — do not reintroduce (?!...) here.
func classifyAgainstHarmClaim(hay, claim string) (Stance, float64) {
	stems := claimTokenStems(claim)
	pos := len(harmCues.FindAllString(hay, -1))
	for _, loc := range directionUpCues.FindAllStringIndex(hay, -1) {
		if windowHasClaimToken(hay, loc[1], stems) {
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
	for _, m := range supportCues.FindAllString(hay, -1) {
		if strings.Contains(m, "increas") {
			continue
		}
		neg++
	}
	return stanceFromCounts(pos, neg)
}

// windowHasClaimToken reports whether the 40 characters after a direction-cue
// match mention one of the claim's stemmed content tokens — the RE2-safe way
// to tie a generic comparative ("greater", "less") to the claim's outcome.
func windowHasClaimToken(hay string, from int, stems []string) bool {
	end := from + 40
	if end > len(hay) {
		end = len(hay)
	}
	win := hay[from:end]
	for _, s := range stems {
		if strings.Contains(win, s) {
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

// claimTokenStems truncates content tokens to their first five characters so
// inflected forms still match ("sweeteners" -> "sweet" matches "sweetener").
func claimTokenStems(claim string) []string {
	toks := ClaimContentTokens(claim)
	for i, t := range toks {
		if len(t) > 5 {
			toks[i] = t[:5]
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
