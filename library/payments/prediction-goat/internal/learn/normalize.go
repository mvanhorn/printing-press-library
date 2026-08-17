// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// normalize.go hosts the prediction-goat-side glue for the entity
// extractor in internal/learn/entities. The extractor itself is
// domain-agnostic; this file wraps it with the prediction-goat
// stopword vocabulary, ticker patterns, and the storage-side
// NormalizedQuery shape that the learning subsystem (teach/recall and
// the v4->v5 schema migration) writes into search_learnings.
//
// Why this isn't in internal/learn/entities: the entities package is
// designed to be lifted into a future generator template. Anything
// prediction-goat-specific (Kalshi/Polymarket ticker shapes, the
// odds/wins/beats stopword set) belongs one layer up, in this consumer
// package, not in the reusable component.
//
// Package-level design notes live in doc.go.
package learn

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/entities"
)

// NormalizedQuery is the entity-aware normalized representation of a
// query string. Replaces the lowercase+stopword scalar normalization
// the v3 learning subsystem used, which destroyed entity tokens (a
// query about "England" lost the entity name and Jaccard-matched a
// stored Portugal learning).
//
//   - Original: the input as received, untouched. Useful for logs
//     and for callers that want to display the user's actual phrasing.
//   - Entities: case-preserving identity-bearing tokens (countries,
//     team names, brand names). Used by the recall match validator
//     to reject a stored learning whose entities don't overlap with
//     the query's entities even at high non-entity Jaccard.
//   - Tickers: CLI-shape identifier tokens (Kalshi tickers, Polymarket
//     slugs). Kept separate from Entities so downstream code can
//     distinguish "user typed Portugal" from "user typed
//     will-portugal-win-the-2026-fifa-world-cup".
//   - NonEntityNormalized: space-joined, alphabetically-sorted,
//     lowercased non-entity content tokens. This is the stable key
//     for Jaccard comparison between the live query and stored
//     learnings, and the value that lands in search_learnings.query_pattern.
//     Sorting is load-bearing: "world cup" and "cup world" must
//     normalize identically so the Jaccard match isn't order-sensitive.
type NormalizedQuery struct {
	Original            string
	Entities            []string
	Tickers             []string
	NonEntityNormalized string
}

// Normalize parses a query into its entity-aware form. Passing a nil
// cfg uses entities.NewConfig() (default domain-agnostic stopwords,
// no ticker patterns) — meaning slugs lose their ticker classification
// and slot back into NonEntityNormalized as plain content tokens.
// Callers in the prediction-goat learning path should pass
// DefaultPredictionGoatConfig() to get the Kalshi/Polymarket ticker
// recognition and the prediction-market stopword vocabulary.
func Normalize(query string, cfg *entities.Config) NormalizedQuery {
	result := entities.Extract(query, cfg)

	// Sort non-entity tokens for stable Jaccard. Sort produces the
	// canonical key written to search_learnings.query_pattern;
	// without it, "england wins world cup" and "world cup england wins"
	// would normalize to different patterns and the Jaccard matcher
	// would underdedupe.
	tokens := append([]string(nil), result.NonEntityTokens...)
	sort.Strings(tokens)

	return NormalizedQuery{
		Original:            query,
		Entities:            append([]string(nil), result.Entities...),
		Tickers:             append([]string(nil), result.Tickers...),
		NonEntityNormalized: strings.Join(tokens, " "),
	}
}

// Prediction-goat domain registrations. Kept as compiled-once
// singletons so every call site that asks for the default config
// shares the same regex instances; sync.Once protects the lazy build.
var (
	predictionGoatConfigOnce sync.Once
	predictionGoatConfig     *entities.Config

	// Kalshi tickers look like KXMENWORLDCUP-26, KXPRES24-DJT, etc.
	// Anchored so a stray "KX..." substring in a sentence doesn't
	// match a non-ticker token. Hyphen-separated suffix segments are
	// optional because some series (KXBTC, KXNFL) have no suffix.
	kalshiTickerRE = regexp.MustCompile(`^KX[A-Z0-9]+(-[A-Z0-9]+)*$`)

	// Polymarket market slugs look like
	// will-portugal-win-the-2026-fifa-world-cup-912. Anchored so a
	// stray "will" in a sentence doesn't substring-match. Lowercase
	// because slugs land in queries lowercased (the user copies them
	// from a URL).
	polymarketSlugRE = regexp.MustCompile(`^will-[a-z0-9-]+$`)
)

// DefaultPredictionGoatConfig returns the entities.Config the
// prediction-goat learning subsystem uses. Registers Kalshi and
// Polymarket ticker patterns, plus the domain stopwords that wrap
// every prediction-market query without themselves being entities
// (odds, win/wins/winning, lose/loses/losing, beat/beats/beating).
//
// Returns a shared instance — callers must not mutate it. Future-
// proofing: if a caller ever needs a per-query Config, NewConfig() +
// the same registrations is the right shape; don't reuse this one.
func DefaultPredictionGoatConfig() *entities.Config {
	predictionGoatConfigOnce.Do(func() {
		cfg := entities.NewConfig()
		cfg.RegisterTickerPattern(kalshiTickerRE)
		cfg.RegisterTickerPattern(polymarketSlugRE)
		cfg.RegisterStopwords(
			"odds",
			"win", "wins", "winning",
			"lose", "loses", "losing",
			"beat", "beats", "beating",
		)
		predictionGoatConfig = cfg
	})
	return predictionGoatConfig
}
