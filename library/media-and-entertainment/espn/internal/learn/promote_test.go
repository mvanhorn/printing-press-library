package learn

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/learn/entities"
)

// stubResolver returns canned canonicals for the keys present in m.
type stubResolver struct {
	m map[string][]string
}

func (s stubResolver) Resolve(token string) []string {
	return s.m[token]
}

func (s stubResolver) ResolveSet(tokens []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range tokens {
		for _, c := range s.Resolve(t) {
			out[c] = struct{}{}
		}
	}
	return out
}

func TestPromoteEntities_PromotesLowercaseAlias(t *testing.T) {
	t.Parallel()
	cfg := espnLikeConfig()
	normalized := Normalize("how are the mariners doing this year", cfg)
	// Capitalization-based extractor misses "mariners".
	if len(normalized.Entities) != 0 {
		t.Fatalf("baseline: extractor should not see 'mariners' as entity, got %v", normalized.Entities)
	}

	resolver := stubResolver{m: map[string][]string{
		"mariners": {"Seattle Mariners"},
	}}
	got := PromoteEntities(normalized, resolver)
	if !contains(got.Entities, "mariners") {
		t.Errorf("want 'mariners' promoted into Entities, got %v", got.Entities)
	}
	for _, tok := range gotTokens(got.NonEntityNormalized) {
		if tok == "mariners" {
			t.Errorf("non-entity tokens should no longer contain 'mariners'; got %v", got.NonEntityNormalized)
		}
	}
}

func TestPromoteEntities_NumericPrefixAlias(t *testing.T) {
	t.Parallel()
	cfg := espnLikeConfig()
	normalized := Normalize("49ers tonight", cfg)
	resolver := stubResolver{m: map[string][]string{
		"49ers": {"San Francisco 49ers"},
	}}
	got := PromoteEntities(normalized, resolver)
	if !contains(got.Entities, "49ers") {
		t.Errorf("want '49ers' promoted, got entities=%v non-entity=%q", got.Entities, got.NonEntityNormalized)
	}
}

func TestPromoteEntities_NoMatch_LeavesUnchanged(t *testing.T) {
	t.Parallel()
	cfg := espnLikeConfig()
	normalized := Normalize("hello world", cfg)
	resolver := stubResolver{m: map[string][]string{}}
	got := PromoteEntities(normalized, resolver)
	if !reflect.DeepEqual(got, normalized) {
		t.Errorf("no resolver hits should leave normalized unchanged; got %+v", got)
	}
}

func TestPromoteEntities_NilResolver_Identity(t *testing.T) {
	t.Parallel()
	cfg := espnLikeConfig()
	normalized := Normalize("how are the mariners doing", cfg)
	got := PromoteEntities(normalized, nil)
	if !reflect.DeepEqual(got, normalized) {
		t.Errorf("nil resolver should be identity; got %+v want %+v", got, normalized)
	}
}

func TestPromoteEntities_MultiEntity(t *testing.T) {
	t.Parallel()
	cfg := espnLikeConfig()
	normalized := Normalize("mariners vs mets tonight", cfg)
	resolver := stubResolver{m: map[string][]string{
		"mariners": {"Seattle Mariners"},
		"mets":     {"New York Mets"},
	}}
	got := PromoteEntities(normalized, resolver)
	if !contains(got.Entities, "mariners") || !contains(got.Entities, "mets") {
		t.Errorf("want both 'mariners' and 'mets' promoted, got %v", got.Entities)
	}
}

func TestPromoteEntities_PreservesExistingEntities(t *testing.T) {
	t.Parallel()
	cfg := espnLikeConfig()
	normalized := Normalize("how are the Mariners doing", cfg)
	// Capitalization rule already gives "Mariners".
	if !contains(normalized.Entities, "Mariners") {
		t.Fatalf("baseline: capitalized 'Mariners' should be entity; got %v", normalized.Entities)
	}
	resolver := stubResolver{m: map[string][]string{
		"mariners": {"Seattle Mariners"}, // would resolve but token is uppercase
	}}
	got := PromoteEntities(normalized, resolver)
	if !contains(got.Entities, "Mariners") {
		t.Errorf("Mariners (capitalized) should be preserved as entity; got %v", got.Entities)
	}
}

// Integration with the real CanonicalResolver against a seeded DB.
func TestPromoteEntities_WithCanonicalResolver(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "mlb_team", "Seattle Mariners",
		[]string{"Seattle Mariners", "Mariners", "SEA"})

	cfg := espnLikeConfig()
	normalized := Normalize("how are the mariners doing this year", cfg)
	resolver := NewCanonicalResolver(context.Background(), db)
	got := PromoteEntities(normalized, resolver)
	if !contains(got.Entities, "mariners") {
		t.Errorf("want 'mariners' promoted via real resolver, got entities=%v", got.Entities)
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func gotTokens(s string) []string {
	out := []string{}
	for _, t := range splitFields(s) {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// splitFields is a tiny helper to avoid importing strings just for
// strings.Fields in test helpers above.
func splitFields(s string) []string {
	// Mirror strings.Fields without the import in this test file.
	out := []string{}
	cur := []rune{}
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if len(cur) > 0 {
				out = append(out, string(cur))
				cur = cur[:0]
			}
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// Suppress unused-import lint if helper composition shrinks later.
var _ = entities.NewConfig
