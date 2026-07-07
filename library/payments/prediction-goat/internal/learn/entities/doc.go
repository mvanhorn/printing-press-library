// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// Package entities extracts entity tokens from CLI query strings to enable
// entity-aware match validation in the learning subsystem.
//
// # Why patterns over lookup tables
//
// A hard-coded country list (or sport roster, or any other domain-specific
// taxonomy) would tie this package to the prediction-market domain and defeat
// the whole point of factoring learning into a reusable subsystem. Patterns
// (capitalized non-sentence-initial words, ALL-CAPS tokens of length >= 2,
// ticker-shape tokens via per-CLI regex) generalize to any domain whose
// entities have a distinguishing shape -- which is most domains. Country
// names, person names, team names, brand names, stock tickers, podcast hosts:
// all surface via the same heuristics with no per-domain code in core.
//
// # What counts as an entity here
//
// "Entity" means a token that carries identity-bearing semantics, as opposed
// to question-shape tokens ("odds", "wins"), stopwords ("the", "of"), or
// generic content words. The recall pipeline uses entities to validate that a
// learning's resource actually concerns the same subject the query is asking
// about -- so "USA odds" should never recall a Portugal-tagged learning, even
// if their non-entity tokens happen to overlap.
//
// # Extension points for a new CLI
//
// A consumer registers per-CLI configuration via the *Config type and the
// Register* methods on it:
//
//   - RegisterTickerPattern(re *regexp.Regexp): add a regex that recognizes
//     the CLI's identifier shape. Prediction-goat registers two -- one for
//     Kalshi (^KX[A-Z0-9]+...) and one for Polymarket (^will-[a-z0-9-]+...).
//     A stock-tracker CLI might register ^[A-Z]{2,5}$ for tickers like NVDA.
//
//   - RegisterStopwords(words ...string): add domain-shape stopwords that
//     should not be treated as entities even when capitalized. Prediction-
//     goat adds "odds", "win", "wins", "winning", "lose", "losing", "beat" --
//     the question-shape vocabulary that wraps every prediction-market query.
//
// No global state. Each Config instance is independent, so a process that
// runs multiple CLIs (or a test suite that exercises multiple configurations)
// gets clean isolation.
//
// # See also
//
// docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md
// section "Key Technical Decisions" -- "Entity extraction is data-driven,
// not list-driven."
package entities
