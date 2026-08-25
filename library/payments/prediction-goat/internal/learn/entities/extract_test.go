// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package entities

import (
	"reflect"
	"regexp"
	"testing"
)

// predictionGoatConfig builds a Config mirroring how the prediction-goat
// consumer will register itself in U3 (when the entities package gets
// wired into the recall pipeline). Tests use this so the behavior we
// validate matches the real call site.
func predictionGoatConfig() *Config {
	cfg := NewConfig()
	cfg.RegisterTickerPattern(regexp.MustCompile(`^KX[A-Z0-9]+(-[A-Z0-9]+)*$`))
	cfg.RegisterTickerPattern(regexp.MustCompile(`^will-[a-z0-9-]+$`))
	cfg.RegisterStopwords("odds", "win", "wins", "winning", "lose", "loses", "losing", "beat", "beats", "beating")
	return cfg
}

func TestExtract_RealWorldQueries(t *testing.T) {
	t.Parallel()
	cfg := predictionGoatConfig()

	cases := []struct {
		name            string
		query           string
		wantEntities    []string
		wantTickers     []string
		wantNonEntities []string
	}{
		{
			name:            "USA all-caps in question",
			query:           "odds USA wins world cup",
			wantEntities:    []string{"USA"},
			wantNonEntities: []string{"world", "cup"},
		},
		{
			name:            "Capitalized country with possessive",
			query:           "what are Portugal's odds at the world cup",
			wantEntities:    []string{"Portugal's"},
			wantNonEntities: []string{"world", "cup"}, // 'at', 'what', 'are', 'the', 'odds' all stopwords
		},
		{
			name:            "England wins phrasing",
			query:           "odds england wins the world cup",
			wantEntities:    nil, // lowercase "england" is NOT capitalized -> falls to non-entity
			wantNonEntities: []string{"england", "world", "cup"},
		},
		{
			name:            "England capitalized",
			query:           "odds England wins the world cup",
			wantEntities:    []string{"England"},
			wantNonEntities: []string{"world", "cup"},
		},
		{
			name:         "Kalshi ticker in query",
			query:        "what is KXMENWORLDCUP-26 about",
			wantTickers:  []string{"KXMENWORLDCUP-26"},
			wantEntities: nil,
		},
		{
			name:            "Polymarket slug in query",
			query:           "look up will-usa-win-the-2026-fifa-world-cup-467",
			wantTickers:     []string{"will-usa-win-the-2026-fifa-world-cup-467"},
			wantNonEntities: []string{"look", "up"}, // 'look'/'up' aren't default stopwords, fall through to non-entity
		},
		{
			name:            "Sentence-initial stopword dropped",
			query:           "The odds of Portugal",
			wantEntities:    []string{"Portugal"},
			wantNonEntities: nil, // 'odds', 'of' both stopwords
		},
		{
			name:            "Mid-sentence stopword-shape capitalized stays an entity",
			query:           "find Will Smith bio",
			wantEntities:    []string{"Will", "Smith"},
			wantNonEntities: []string{"find", "bio"}, // 'find' not in default stopwords
		},
		{
			name:            "Question-shape stopwords dropped",
			query:           "what are the odds Portugal wins",
			wantEntities:    []string{"Portugal"},
			wantNonEntities: nil, // 'what', 'are', 'the', 'odds', 'wins' all stopwords
		},
		{
			name:            "Empty input",
			query:           "",
			wantEntities:    nil,
			wantNonEntities: nil,
		},
		{
			name:            "Only stopwords",
			query:           "the of for",
			wantEntities:    nil,
			wantNonEntities: nil,
		},
		{
			name:            "Punctuation stripped",
			query:           "odds, Portugal? wins!",
			wantEntities:    []string{"Portugal"},
			wantNonEntities: nil,
		},
		{
			name:            "Multi-token entities",
			query:           "odds New Zealand wins world cup",
			wantEntities:    []string{"New", "Zealand"},
			wantNonEntities: []string{"world", "cup"},
		},
		{
			name:            "Bitcoin all-caps acronym",
			query:           "BTC odds at 100k",
			wantEntities:    []string{"BTC"},
			wantNonEntities: []string{"100k"},
		},
		{
			name:            "Single uppercase letter is not an entity",
			query:           "A B candidate",
			wantEntities:    nil,                            // 'A'/'B' fail ALL-CAPS len>=2 AND isCapitalized (need lowercase rest)
			wantNonEntities: []string{"b", "candidate"},     // 'a' is stopword and dropped; 'B' falls through to non-entity (lowercased)
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Extract(tc.query, cfg)
			if !reflect.DeepEqual(nilIfEmpty(got.Entities), nilIfEmpty(tc.wantEntities)) {
				t.Errorf("Entities = %v, want %v", got.Entities, tc.wantEntities)
			}
			if !reflect.DeepEqual(nilIfEmpty(got.Tickers), nilIfEmpty(tc.wantTickers)) {
				t.Errorf("Tickers = %v, want %v", got.Tickers, tc.wantTickers)
			}
			if !reflect.DeepEqual(nilIfEmpty(got.NonEntityTokens), nilIfEmpty(tc.wantNonEntities)) {
				t.Errorf("NonEntityTokens = %v, want %v", got.NonEntityTokens, tc.wantNonEntities)
			}
		})
	}
}

func TestExtract_NilConfig(t *testing.T) {
	t.Parallel()
	// nil config should still work, using only default stopwords. No
	// ticker patterns means a slug becomes a non-entity content token.
	got := Extract("odds Portugal wins", nil)
	if got.Entities[0] != "Portugal" {
		t.Errorf("nil config entity = %q, want Portugal", got.Entities[0])
	}
}

func TestRegisterTickerPattern_MultiplePatterns(t *testing.T) {
	t.Parallel()
	cfg := NewConfig()
	cfg.RegisterTickerPattern(regexp.MustCompile(`^KX[A-Z0-9]+$`))
	cfg.RegisterTickerPattern(regexp.MustCompile(`^[A-Z]{2,5}$`))

	got := Extract("KXBTC and NVDA", cfg)
	// Both should land in Tickers (NVDA matches the second pattern).
	if len(got.Tickers) != 2 {
		t.Fatalf("Tickers = %v, want 2 entries", got.Tickers)
	}
}

func TestRegisterTickerPattern_NilIgnored(t *testing.T) {
	t.Parallel()
	cfg := NewConfig()
	cfg.RegisterTickerPattern(nil)
	cfg.RegisterTickerPattern(regexp.MustCompile(`^KX[A-Z0-9]+$`))

	got := Extract("KXBTC odds", cfg)
	if len(got.Tickers) != 1 {
		t.Errorf("nil patterns should be ignored without panic; got Tickers = %v", got.Tickers)
	}
}

func TestRegisterStopwords_CaseInsensitive(t *testing.T) {
	t.Parallel()
	cfg := NewConfig()
	cfg.RegisterStopwords("ODDS", "Wins") // case-insensitive registration

	// Both should match against lowercase tokens.
	got := Extract("odds Portugal wins ODDS", cfg)
	// Only "Portugal" should survive (entity); odds/wins/ODDS all filter.
	if len(got.NonEntityTokens) != 0 {
		t.Errorf("expected odds/wins all filtered, got non-entity %v", got.NonEntityTokens)
	}
	if len(got.Entities) != 1 || got.Entities[0] != "Portugal" {
		t.Errorf("expected Portugal entity, got %v", got.Entities)
	}
}

func TestRegisterStopwords_EmptyAndWhitespace(t *testing.T) {
	t.Parallel()
	cfg := NewConfig()
	cfg.RegisterStopwords("", "  ", "valid")
	// Empty and whitespace-only entries should be silently skipped.
	got := Extract("valid token here", cfg)
	// "valid" is now a stopword. "token", "here" are not in default set.
	if len(got.NonEntityTokens) != 2 {
		t.Errorf("got %v, want 2 non-entity tokens", got.NonEntityTokens)
	}
}

func TestConfigIsolation(t *testing.T) {
	t.Parallel()
	// Two configs should not share state. If they did, registering a
	// stopword on cfg1 would leak into cfg2's behavior.
	cfg1 := NewConfig()
	cfg2 := NewConfig()
	cfg1.RegisterStopwords("zebra")

	r1 := Extract("zebra", cfg1)
	r2 := Extract("zebra", cfg2)

	if len(r1.NonEntityTokens) != 0 {
		t.Errorf("cfg1 should filter zebra; got %v", r1.NonEntityTokens)
	}
	if len(r2.NonEntityTokens) != 1 || r2.NonEntityTokens[0] != "zebra" {
		t.Errorf("cfg2 should NOT filter zebra (isolated); got %v", r2.NonEntityTokens)
	}
}

func TestExtract_PreservesOrderInResults(t *testing.T) {
	t.Parallel()
	cfg := predictionGoatConfig()
	got := Extract("USA Portugal England world cup", cfg)
	want := []string{"USA", "Portugal", "England"}
	if !reflect.DeepEqual(got.Entities, want) {
		t.Errorf("Entities = %v, want %v (order matters)", got.Entities, want)
	}
}

func nilIfEmpty[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	return s
}
