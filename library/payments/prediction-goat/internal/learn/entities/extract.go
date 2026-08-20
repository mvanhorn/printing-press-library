// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package entities

import (
	"strings"
	"unicode"
)

// Result is the parsed shape of a CLI query string from the perspective
// of the learning subsystem. Each field carries a distinct semantic role:
//
//   - Entities: identity-bearing tokens (countries, team names, person
//     names, brand names). Used by match validation in the recall path:
//     a learning whose Entities don't overlap with the query's Entities
//     is treated as a mismatch even when non-entity Jaccard is high.
//
//   - Tickers: CLI-specific identifier-shaped tokens, registered via
//     Config.RegisterTickerPattern. Kept separate from Entities so
//     downstream code can distinguish "user typed the literal slug" from
//     "user typed a country name."
//
//   - NonEntityTokens: lowercase content tokens left over after entities,
//     tickers, and stopwords are removed. Used for token-set Jaccard
//     against the corresponding field on stored learnings. Sorted by the
//     caller for stable comparison (normalize.NonEntityNormalized).
//
// Caller is responsible for any further normalization (sorting,
// deduplication). This package extracts; it does not normalize for
// storage.
type Result struct {
	Entities        []string
	Tickers         []string
	NonEntityTokens []string
}

// Extract pulls entity / ticker / non-entity tokens out of a query string
// according to the provided Config. Passing nil cfg uses a Config with
// the default stopword list and no ticker patterns.
//
// # Extraction rules, in order of precedence
//
//  1. Token matches any Config.tickerPatterns -> Tickers
//  2. Token is ALL-CAPS, length >= 2, alphanumeric -> Entities
//     (catches USA, NFL, GPT, BTC, etc.)
//  3. Token starts with uppercase letter:
//     a. At position 0 AND its lowercase form is a stopword -> drop
//     (handles "The odds of Portugal" -> drop "The", keep "Portugal")
//     b. Otherwise -> Entities
//     (mid-sentence capitalization is meaningful: "Will Smith" keeps
//     "Will" as an entity even though "will" is in stopwords)
//  4. Token's lowercase form is a stopword -> drop
//  5. Otherwise -> NonEntityTokens (lowercased)
//
// Punctuation surrounding tokens (.,?!:;'") is stripped before
// classification. Internal hyphens and underscores are preserved so
// tickers and slugs survive whitespace-tokenization.
func Extract(query string, cfg *Config) Result {
	if cfg == nil {
		cfg = NewConfig()
	}
	rawTokens := strings.Fields(query)
	result := Result{}

	for i, raw := range rawTokens {
		tok := trimPunct(raw)
		if tok == "" {
			continue
		}

		// 1. Ticker pattern -- highest precedence so a slug like
		//    "will-portugal-win-..." doesn't get mis-treated as a
		//    capitalized non-stopword.
		if cfg.matchesTicker(tok) {
			result.Tickers = append(result.Tickers, tok)
			continue
		}

		lower := strings.ToLower(tok)

		// 2. ALL-CAPS alphanumeric of length >= 2 -> entity, BUT
		//    explicit stopword registration overrides. This lets a user
		//    say "ODDS is a stopword" and have it drop even when shouted.
		if len(tok) >= 2 && isAllCaps(tok) {
			if cfg.isStopword(lower) {
				continue
			}
			result.Entities = append(result.Entities, tok)
			continue
		}

		// 3. Capitalized (first letter upper, has at least one lowercase).
		if isCapitalized(tok) {
			// 3a. Sentence-initial capitalized stopword: drop. Handles
			//     "The odds of Portugal" -> The dropped, Portugal kept,
			//     and "Will Portugal win" -> Will dropped, Portugal kept.
			if i == 0 && cfg.isStopword(lower) {
				continue
			}
			// 3b. Mid-sentence capitalization or non-stopword leading
			//     word: treat as entity. The user/agent capitalized it
			//     for a reason. "find Will Smith bio" keeps Will + Smith
			//     even though "will" is a default stopword, because
			//     mid-sentence capitalization is meaningful.
			result.Entities = append(result.Entities, tok)
			continue
		}

		// 4. Stopword filter for lowercase tokens.
		if cfg.isStopword(lower) {
			continue
		}

		// 5. Everything else is a non-entity content token.
		result.NonEntityTokens = append(result.NonEntityTokens, lower)
	}

	return result
}

// trimPunct strips a small set of sentence-punctuation characters from
// both ends of a token. Internal hyphens and underscores are preserved
// because they are load-bearing in tickers and slugs ("KXMENWORLDCUP-26",
// "will-portugal-win-the-2026-fifa-world-cup-912").
func trimPunct(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		switch r {
		case '.', ',', '?', '!', ':', ';', '\'', '"', '(', ')', '[', ']', '{', '}':
			return true
		}
		return false
	})
}

// isAllCaps reports whether every letter in s is uppercase. Non-letter
// runes (digits, hyphens) are allowed and don't disqualify; pure-digit
// tokens return false because they aren't entities.
func isAllCaps(s string) bool {
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

// isCapitalized reports whether the first letter rune of s is uppercase
// AND there is at least one lowercase letter somewhere in s. The second
// condition distinguishes "Portugal" (capitalized) from "USA" (ALL-CAPS,
// handled by isAllCaps above). Pure-digit or punctuation-only tokens
// return false.
func isCapitalized(s string) bool {
	firstUpper := false
	hasLower := false
	for i, r := range s {
		if i == 0 {
			if unicode.IsLetter(r) && unicode.IsUpper(r) {
				firstUpper = true
				continue
			}
			return false
		}
		if unicode.IsLetter(r) && unicode.IsLower(r) {
			hasLower = true
		}
	}
	return firstUpper && hasLower
}
