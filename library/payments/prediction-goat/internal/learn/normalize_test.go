// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package learn

import (
	"reflect"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/entities"
)

// TestNormalize_EnglandWorldCup pins the core fix this normalizer
// exists for: an "odds England wins the world cup" query must preserve
// "England" as an entity. Under the v3 lowercase-+-stopwords
// normalizer, the query collapsed to "england cup world" and a stored
// learning about Portugal scored Jaccard >= 0.6 against it, surfacing
// the wrong ticker on recall.
func TestNormalize_EnglandWorldCup(t *testing.T) {
	t.Parallel()
	got := Normalize("odds England wins the world cup", DefaultPredictionGoatConfig())
	want := NormalizedQuery{
		Original:            "odds England wins the world cup",
		Entities:            []string{"England"},
		Tickers:             nil,
		NonEntityNormalized: "cup world",
	}
	if !reflect.DeepEqual(got.Entities, want.Entities) {
		t.Errorf("Entities = %#v, want %#v", got.Entities, want.Entities)
	}
	if got.NonEntityNormalized != want.NonEntityNormalized {
		t.Errorf("NonEntityNormalized = %q, want %q", got.NonEntityNormalized, want.NonEntityNormalized)
	}
	if len(got.Tickers) != 0 {
		t.Errorf("Tickers = %#v, want empty", got.Tickers)
	}
	if got.Original != want.Original {
		t.Errorf("Original = %q, want %q", got.Original, want.Original)
	}
}

// TestNormalize_PortugalSameNonEntity verifies the entity-preserving
// shape: the Portugal and England versions of the same question share
// the same NonEntityNormalized (so non-entity Jaccard would tie) but
// have different Entities (so the entity-overlap match validator can
// reject the cross-entity match). This is the new contract recall in
// U3 relies on.
func TestNormalize_PortugalSameNonEntity(t *testing.T) {
	t.Parallel()
	cfg := DefaultPredictionGoatConfig()
	england := Normalize("odds England wins the world cup", cfg)
	portugal := Normalize("odds Portugal wins the world cup", cfg)

	if england.NonEntityNormalized != portugal.NonEntityNormalized {
		t.Fatalf("non-entity normalized forms differ:\n  england = %q\n  portugal = %q\n  want identical",
			england.NonEntityNormalized, portugal.NonEntityNormalized)
	}
	if reflect.DeepEqual(england.Entities, portugal.Entities) {
		t.Fatalf("entities should differ for england vs portugal queries, got identical = %#v", england.Entities)
	}
	if len(england.Entities) != 1 || england.Entities[0] != "England" {
		t.Errorf("england.Entities = %#v, want [England]", england.Entities)
	}
	if len(portugal.Entities) != 1 || portugal.Entities[0] != "Portugal" {
		t.Errorf("portugal.Entities = %#v, want [Portugal]", portugal.Entities)
	}
}

// TestNormalize_EmptyQuery handles the empty-input edge case: every
// field should be empty (not nil-string-vs-empty-string-distinct).
// Callers in the teach path will short-circuit on a fully-empty
// normalization, but Normalize itself must not panic or produce
// garbage on whitespace-only or empty input.
func TestNormalize_EmptyQuery(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "   ", "\n\t"} {
		got := Normalize(in, DefaultPredictionGoatConfig())
		if got.Original != in {
			t.Errorf("Original = %q, want %q", got.Original, in)
		}
		if len(got.Entities) != 0 {
			t.Errorf("Entities = %#v, want empty for input %q", got.Entities, in)
		}
		if len(got.Tickers) != 0 {
			t.Errorf("Tickers = %#v, want empty for input %q", got.Tickers, in)
		}
		if got.NonEntityNormalized != "" {
			t.Errorf("NonEntityNormalized = %q, want empty for input %q", got.NonEntityNormalized, in)
		}
	}
}

// TestNormalize_StopwordsOnly verifies that a query composed entirely
// of default stopwords collapses to empty non-entity normalized form
// without any false-entity classification. Sentence-initial "The" is
// the only token that could plausibly trip — it must drop, not be
// promoted to an entity by mid-sentence capitalization logic.
func TestNormalize_StopwordsOnly(t *testing.T) {
	t.Parallel()
	got := Normalize("the of for", DefaultPredictionGoatConfig())
	if got.NonEntityNormalized != "" {
		t.Errorf("NonEntityNormalized = %q, want empty", got.NonEntityNormalized)
	}
	if len(got.Entities) != 0 {
		t.Errorf("Entities = %#v, want empty", got.Entities)
	}
}

// TestNormalize_PolymarketSlug verifies a Polymarket slug lands in
// Tickers (not Entities, not NonEntityNormalized) when the
// prediction-goat config is in effect. Other content tokens land in
// NonEntityNormalized, lowercased and alphabetically sorted.
func TestNormalize_PolymarketSlug(t *testing.T) {
	t.Parallel()
	got := Normalize("look up will-portugal-win-the-2026-fifa-world-cup-912", DefaultPredictionGoatConfig())
	wantTickers := []string{"will-portugal-win-the-2026-fifa-world-cup-912"}
	if !reflect.DeepEqual(got.Tickers, wantTickers) {
		t.Errorf("Tickers = %#v, want %#v", got.Tickers, wantTickers)
	}
	if got.NonEntityNormalized != "look up" {
		t.Errorf("NonEntityNormalized = %q, want %q", got.NonEntityNormalized, "look up")
	}
	if len(got.Entities) != 0 {
		t.Errorf("Entities = %#v, want empty", got.Entities)
	}
}

// TestNormalize_KalshiTicker verifies Kalshi-shaped tickers
// (KX-prefixed, uppercase, optionally hyphenated) classify as tickers
// rather than as ALL-CAPS entities. The Kalshi pattern is registered
// ahead of the ALL-CAPS entity heuristic in extract precedence.
func TestNormalize_KalshiTicker(t *testing.T) {
	t.Parallel()
	got := Normalize("odds for KXMENWORLDCUP-26", DefaultPredictionGoatConfig())
	wantTickers := []string{"KXMENWORLDCUP-26"}
	if !reflect.DeepEqual(got.Tickers, wantTickers) {
		t.Errorf("Tickers = %#v, want %#v", got.Tickers, wantTickers)
	}
	if len(got.Entities) != 0 {
		t.Errorf("Entities = %#v, want empty (KX ticker should not be ALL-CAPS entity)", got.Entities)
	}
	if got.NonEntityNormalized != "" {
		t.Errorf("NonEntityNormalized = %q, want empty (odds+for are stopwords)", got.NonEntityNormalized)
	}
}

// TestNormalize_NilConfigFallback verifies the documented fallback:
// passing nil cfg gets a Config with default stopwords and no ticker
// patterns. The Polymarket slug then loses its ticker classification
// and lands in NonEntityNormalized as a plain content token (lowered,
// sorted with the other content tokens).
func TestNormalize_NilConfigFallback(t *testing.T) {
	t.Parallel()
	got := Normalize("look up will-portugal-win-the-2026-fifa-world-cup-912", nil)
	if len(got.Tickers) != 0 {
		t.Errorf("Tickers = %#v, want empty (nil config has no ticker patterns)", got.Tickers)
	}
	// Slug should now be a non-entity content token, sorted with "look" and "up".
	want := "look up will-portugal-win-the-2026-fifa-world-cup-912"
	if got.NonEntityNormalized != want {
		t.Errorf("NonEntityNormalized = %q, want %q", got.NonEntityNormalized, want)
	}
}

// TestNormalize_StableSort verifies the sort step is order-independent:
// "world cup england" and "england cup world" normalize to the same
// NonEntityNormalized. This is the property the Jaccard matcher
// downstream relies on for stable comparison.
func TestNormalize_StableSort(t *testing.T) {
	t.Parallel()
	cfg := DefaultPredictionGoatConfig()
	a := Normalize("world cup france", cfg)
	b := Normalize("france world cup", cfg)
	c := Normalize("cup france world", cfg)

	if a.NonEntityNormalized != b.NonEntityNormalized || b.NonEntityNormalized != c.NonEntityNormalized {
		t.Errorf("non-entity normalized forms should be order-independent:\n  a = %q\n  b = %q\n  c = %q",
			a.NonEntityNormalized, b.NonEntityNormalized, c.NonEntityNormalized)
	}
}

// TestDefaultPredictionGoatConfig_SharedInstance verifies the lazy-
// init pattern returns the same Config across calls. Without this,
// each call would rebuild the regex+stopwords map and downstream
// code that asks for the config in a hot loop (e.g. the v4->v5
// backfill walking every search_learnings row) would pay an
// unbounded allocation tax.
func TestDefaultPredictionGoatConfig_SharedInstance(t *testing.T) {
	t.Parallel()
	a := DefaultPredictionGoatConfig()
	b := DefaultPredictionGoatConfig()
	if a != b {
		t.Errorf("DefaultPredictionGoatConfig returned different instances; expected the lazy-init singleton")
	}
}

// TestNormalize_ExtractContract documents the boundary between this
// package and internal/learn/entities: Normalize wraps Extract with
// sorting + the storage-side struct shape. Anything classification-
// related must keep working when called via Normalize. This is a
// safety-net for refactors that swap out the extractor.
func TestNormalize_ExtractContract(t *testing.T) {
	t.Parallel()
	raw := entities.Extract("odds England wins the world cup", DefaultPredictionGoatConfig())
	got := Normalize("odds England wins the world cup", DefaultPredictionGoatConfig())

	if !reflect.DeepEqual(got.Entities, raw.Entities) {
		t.Errorf("Entities drift between Extract and Normalize:\n  extract = %#v\n  normalize = %#v",
			raw.Entities, got.Entities)
	}
	if !reflect.DeepEqual(got.Tickers, raw.Tickers) {
		t.Errorf("Tickers drift between Extract and Normalize:\n  extract = %#v\n  normalize = %#v",
			raw.Tickers, got.Tickers)
	}
}
