package scengine

import "strings"

// picoGateEnabled controls the Phase 3 PICO relevance gate.
// Always true in production; tests may toggle it to measure impact.
var picoGateEnabled = true

// IsPICORelevant checks if a study text (abstract + title) names both sides of
// the claim: at least one intervention token AND at least one outcome token
// must appear, case-insensitive, anywhere in the text.
//
// The relation between the two sides is AND, but WITHIN a side it is OR. That
// asymmetry is the whole design. A claim side is usually a modifier plus a head
// noun ("artificial sweeteners"), and the literature routinely swaps the
// modifier while keeping the head: "low-calorie sweeteners", "low-energy
// sweetener", "non-nutritive sweeteners". Requiring every token of a side — or
// only its first token — throws away exactly the papers most on point. The
// corpus measurement made this concrete: matching on "artif" alone dropped 69%
// of the sweetener corpus, including the meta-analyses on sweeteners and body
// weight. Requiring both SIDES still keeps the gate's purpose intact: a paper
// that names only the intervention or only the outcome is not evidence about
// the relation the claim asserts.
//
// Exported (unlike claimSides) because the gate runs in internal/cli, where
// the fetched works are still available before scoring.
func IsPICORelevant(workAbstract, workTitle string, ivTokens, outTokens []string) bool {
	if !picoGateEnabled {
		return true
	}
	if len(ivTokens) == 0 || len(outTokens) == 0 {
		// Empty claim tokens → assume all studies are relevant (bypass gate).
		// A claim with no polarity verb cannot be split into sides, and a gate
		// that cannot name what it is filtering for must not filter.
		return true
	}

	text := strings.ToLower(normalizeText(workAbstract + " " + workTitle))
	return containsAnyToken(text, ivTokens) && containsAnyToken(text, outTokens)
}

// containsAnyToken reports whether text contains any of the tokens, matching
// case-insensitively. text is expected to be lowercase already.
func containsAnyToken(text string, tokens []string) bool {
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(tok)) {
			return true
		}
	}
	return false
}

// picoStopwords are function words that must never satisfy a side of the gate.
// They occur in essentially every abstract, so a side that reduced to one of
// them would make the gate a no-op for that side while still looking enforced.
var picoStopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "of": {}, "in": {}, "and": {}, "or": {},
	"to": {}, "for": {}, "with": {}, "on": {}, "at": {}, "by": {},
}

// PICOTokens derives the intervention and outcome token lists for the gate from
// a claim, splitting it around its polarity verb: the content tokens before the
// verb name the intervention, the ones after it name the outcome.
//
// The split point comes from polarityVerbCues (direction-neutral, fires on both
// harm and benefit verbs). See the previous comment block in the git history for
// the full rationale on why claimSides was replaced.
//
// Token extraction uses picoSideTokens instead of ClaimContentTokens+stemTokens.
// The difference: a 1–3-character alphabetic token is glued to the preceding
// content token rather than dropped by the 4-char length floor. This keeps the
// "C" in "vitamin C" as the phrase token "vitamin c", which matches only vitamin
// C papers, instead of collapsing to the stem "vitam" which matches every
// vitamin in the literature.
//
// Glued phrases are NOT stemmed. Non-glued tokens are stemmed to claimStemLen
// characters as before. Numeric short tokens (the "3" in "omega-3") are dropped
// rather than glued: the hyphen in "omega-3" would prevent strings.Contains
// from matching "omega 3" against "omega-3", and the "omega" stem already covers
// both forms correctly.
//
// Returns empty slices when the claim cannot be split, which callers must treat
// as "bypass the gate".
func PICOTokens(claim string) (ivTokens, outTokens []string) {
	lc := strings.ToLower(claim)
	loc := polarityVerbCues.FindStringIndex(lc)
	if loc == nil {
		return nil, nil
	}
	intervention := picoSideTokens(lc[:loc[0]])
	outcome := picoSideTokens(lc[loc[1]:])
	if len(intervention) == 0 || len(outcome) == 0 {
		return nil, nil
	}
	return dropPICOStopwords(intervention), dropPICOStopwords(outcome)
}

// picoSideTokens extracts content tokens from one side of a claim for use in
// IsPICORelevant. It extends ClaimContentTokens with short-qualifier gluing:
// a 1–3-character alphabetic token is appended (with a space) to the preceding
// content token rather than being dropped by the 4-char length floor, so
// "vitamin C" yields the phrase token "vitamin c" instead of "vitam".
//
// The glued phrase is NOT stemmed. "vitamin c" already uniquely identifies the
// substance, and stemming "vitamin c" to "vitam " would silently drop the "c".
// Non-glued tokens are stemmed normally to claimStemLen characters.
//
// Only alphabetic short tokens are glued. Numeric tokens like the "3" in
// "omega-3" are dropped rather than glued, because in abstract text "omega-3"
// is separated by a dash that strings.Contains cannot bridge: "omega 3" would
// not match "omega-3". The "omega" stem covers both forms correctly.
//
// Gluing attaches to the PRECEDING token, so a short qualifier that stands
// BEFORE its head noun is dropped, not glued: len(out) is 0 when it is the
// first token of a side. "probiotics improve gut health" therefore yields
// out=[healt], not out=["gut health"] — and the same goes for "bone density",
// "oral health", "eye strain". This is deliberate. Measured 2026-08-02 on the
// archived full corpora, preserving the qualifier COSTS more than it buys:
//
//	probiotics corpus  out=[healt]        excludes 2 of 9 works
//	                   out=["gut health"] excludes 7 of 9  (+5)
//	coffee corpus      out=[heart disea]  excludes 0 of 11 works
//	                   out=[heart]        excludes 3 of 11 (+3)
//
// The five and three extra exclusions are all on-topic — the ISAPP probiotics
// consensus statement, "Probiotics and prebiotics in intestinal health and
// disease", two coffee/cardiovascular meta-analyses — and they fail only
// because the literature writes "intestinal health" where the claim says "gut
// health", and "cardiovascular" where it says "heart". A loose outcome token is
// what absorbs that vocabulary gap. It does not admit noise on its own either:
// IsPICORelevant is an AND over the two sides, so the intervention token
// ("probi", "coffe") still does the filtering.
//
// What this measurement does NOT establish: the archived corpora hold only works
// that SURVIVED the gate, so the price of tightening is measured while the
// benefit — how much noise the loose token lets in — is not. That asymmetry is
// why this note records a decision rather than an optimum.
//
// Only used by PICOTokens. Do not use in claimSides or claimTokenStems — those
// serve the stance classifier's pairing gate, which is calibrated on the current
// stem-only behavior.
func picoSideTokens(s string) []string {
	raw := claimTokenSplit.Split(strings.ToLower(s), -1)
	out := make([]string, 0, len(raw))
	isGlued := make([]bool, 0, len(raw))

	for _, tok := range raw {
		if tok == "" {
			continue
		}
		if len(tok) <= 3 {
			if _, stop := picoStopwords[tok]; stop {
				continue
			}
			// Glue alphabetic short qualifiers to the preceding token only.
			// "vitamin C" → "vitamin c"; "omega 3" skipped (numeric cannot
			// bridge the hyphen in "omega-3" via strings.Contains).
			if isAlphaOnly(tok) && len(out) > 0 {
				out[len(out)-1] = out[len(out)-1] + " " + tok
				isGlued[len(isGlued)-1] = true
			}
			// No preceding token, or numeric: drop.
			continue
		}
		if _, stop := claimStopwords[tok]; stop {
			continue
		}
		if hasPolarityPrefix(tok) {
			continue
		}
		out = append(out, tok)
		isGlued = append(isGlued, false)
	}

	// Stem non-glued tokens only. Glued phrases like "vitamin c" are used
	// as-is: stemming the phrase would corrupt the qualifier.
	for i, tok := range out {
		if !isGlued[i] && len(tok) > claimStemLen {
			out[i] = tok[:claimStemLen]
		}
	}

	return dropPICOStopwords(out)
}

// isAlphaOnly reports whether s contains only ASCII lowercase letters.
// s is expected to be already lowercased (picoSideTokens lowercases its input).
func isAlphaOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

// dropPICOStopwords removes function words from one side's token list. If the
// side consists of nothing else, the original list is returned unchanged: a
// side that survives the split is real, and silently emptying it here would
// bypass the gate for BOTH sides rather than tighten it.
func dropPICOStopwords(tokens []string) []string {
	kept := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if _, stop := picoStopwords[tok]; stop {
			continue
		}
		kept = append(kept, tok)
	}
	if len(kept) == 0 {
		return tokens
	}
	return kept
}
