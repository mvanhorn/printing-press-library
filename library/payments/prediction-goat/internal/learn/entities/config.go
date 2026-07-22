// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package entities

import (
	"regexp"
	"strings"
)

// defaultStopwords are the domain-agnostic English filler words that the
// extractor strips regardless of consumer. Anything domain-specific
// ("odds", "wins" for prediction markets; "stock", "price" for finance)
// belongs in a per-CLI Config via RegisterStopwords, NOT in this list.
//
// Why: the test of "is this a stopword" should be answerable without
// knowing what CLI is calling Extract. If a word here ever causes a real
// false negative in a non-prediction-market CLI, it belongs out of this
// list and into the consumer's per-CLI Config.
var defaultStopwords = map[string]struct{}{
	// Articles
	"a": {}, "an": {}, "the": {},
	// Be verbs
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {},
	// Common prepositions
	"of": {}, "to": {}, "in": {}, "on": {}, "at": {}, "for": {}, "with": {}, "from": {}, "by": {}, "about": {},
	// Question words
	"what": {}, "which": {}, "who": {}, "whom": {}, "whose": {}, "how": {}, "when": {}, "why": {}, "where": {},
	// Modal / auxiliary verbs
	"will": {}, "would": {}, "could": {}, "should": {}, "may": {}, "might": {}, "can": {}, "shall": {},
	"do": {}, "does": {}, "did": {}, "have": {}, "has": {}, "had": {},
	// Conjunctions / pronouns
	"and": {}, "or": {}, "but": {}, "if": {}, "then": {}, "than": {},
	"this": {}, "that": {}, "these": {}, "those": {}, "it": {}, "its": {},
}

// Config holds per-CLI registration for the entity extractor. Construct
// with NewConfig(), then register CLI-specific ticker patterns and
// stopwords. Pass the same Config to every Extract call within a CLI
// process.
type Config struct {
	tickerPatterns []*regexp.Regexp
	stopwords      map[string]struct{}
}

// NewConfig returns a Config preloaded with the domain-agnostic
// default stopword set. No ticker patterns are registered by default --
// each consumer CLI must add its own.
func NewConfig() *Config {
	cfg := &Config{
		stopwords: make(map[string]struct{}, len(defaultStopwords)+16),
	}
	for w := range defaultStopwords {
		cfg.stopwords[w] = struct{}{}
	}
	return cfg
}

// RegisterTickerPattern adds a compiled regex that recognizes one shape
// of identifier this CLI uses. Multiple patterns can be registered;
// they are tried in registration order on each token. Tokens that
// match are returned in Result.Tickers (not Result.Entities).
//
// Patterns must anchor (^...$) when the CLI's identifiers can otherwise
// substring-match an English word. Prediction-goat's Polymarket pattern
// is ^will-[a-z0-9-]+$, anchored so a stray "will" in a sentence doesn't
// match.
func (c *Config) RegisterTickerPattern(re *regexp.Regexp) {
	if re != nil {
		c.tickerPatterns = append(c.tickerPatterns, re)
	}
}

// RegisterStopwords adds domain-shape stopwords on top of the default
// set. Words are lower-cased on registration; matching at extract time
// is case-insensitive.
//
// Use this for the vocabulary that wraps every query in your domain
// without itself being an entity: question shape ("odds", "wins"),
// generic price/probability words ("rate", "chance"), etc.
func (c *Config) RegisterStopwords(words ...string) {
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if w != "" {
			c.stopwords[w] = struct{}{}
		}
	}
}

// isStopword reports whether a lowercase token is a stopword for this
// Config. Caller must lowercase the token first.
func (c *Config) isStopword(lower string) bool {
	_, ok := c.stopwords[lower]
	return ok
}

// matchesTicker reports whether a token matches any registered ticker
// pattern. Patterns are tried in registration order; first match wins.
func (c *Config) matchesTicker(token string) bool {
	for _, re := range c.tickerPatterns {
		if re.MatchString(token) {
			return true
		}
	}
	return false
}
