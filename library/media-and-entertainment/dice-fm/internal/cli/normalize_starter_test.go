// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the embedded starter normalization config and the config-sourced
// classify path. All fixtures are synthetic — no real tenant names.
package cli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
)

// TestShippedStarterConfig verifies the embedded starter config declares the
// recommended entities with the expected shapes and ships with EMPTY rules.
func TestShippedStarterConfig(t *testing.T) {
	cfg, err := normalizecfg.Parse(starterConfigYAML)
	if err != nil {
		t.Fatalf("starter parse: %v", err)
	}
	want := map[string]string{"ticket_type": "attributes", "price_tier": "attributes", "venue": "attributes", "artist": "alias", "genre": "vocab", "ticket_pool": "alias"}
	for ent, shape := range want {
		if cfg.Entities[ent].Shape != shape {
			t.Errorf("starter %s shape=%q want %q", ent, cfg.Entities[ent].Shape, shape)
		}
		if len(cfg.Entities[ent].Rules) != 0 {
			t.Errorf("starter %s should ship with EMPTY rules", ent)
		}
	}
	// ticket_pool ships as an alias spine sourced from the event ticketPools array.
	pool, ok := cfg.Entities["ticket_pool"]
	if !ok {
		t.Fatal("starter missing ticket_pool entity")
	}
	if pool.Shape != normalizecfg.ShapeAlias {
		t.Errorf("ticket_pool shape=%q want %q", pool.Shape, normalizecfg.ShapeAlias)
	}
	if len(pool.Rules) != 0 {
		t.Errorf("ticket_pool should ship with EMPTY rules, got %v", pool.Rules)
	}
}

// TestClassifyTiersUsesStarterConfig proves the config-sourced classify path
// works end to end: with the embedded starter's empty rules, every distinct
// ticket-type name resolves unmatched, matching the prior hardcoded-Entity
// behavior.
func TestClassifyTiersUsesStarterConfig(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t1": `{"id":"t1","ticketType":{"name":"alpha label"}}`,
			"t2": `{"id":"t2","ticketType":{"name":"beta label"}}`,
		},
	})
	res, err := classifyTiers(context.Background(), s, classifyOpts{ClassifierVersion: 1})
	if err != nil {
		t.Fatalf("classifyTiers: %v", err)
	}
	// Two distinct names, empty rules → both unmatched, two canonical forms.
	if res.Unmatched != 2 {
		t.Errorf("want 2 unmatched with empty starter rules, got %d", res.Unmatched)
	}
	if res.CanonicalCount != 2 {
		t.Errorf("want 2 canonical forms, got %d", res.CanonicalCount)
	}
}

// TestLoadNormalizeConfigFromOperatorMerge exercises the testable merge seam
// with a t.TempDir() operator path so it has no $HOME dependency. It covers the
// three documented paths: operator absent (starter alone), operator present with
// overrides (operator wins per entity / new entity present), and a malformed
// operator file (error).
func TestLoadNormalizeConfigFromOperatorMerge(t *testing.T) {
	t.Run("operator absent returns starter unchanged", func(t *testing.T) {
		// A path inside an empty temp dir that does not exist.
		opPath := filepath.Join(t.TempDir(), "missing-normalize.yaml")
		cfg, err := loadNormalizeConfigFrom(starterConfigYAML, opPath)
		if err != nil {
			t.Fatalf("loadNormalizeConfigFrom: %v", err)
		}
		// The shipped starter declares exactly six entities.
		if len(cfg.Entities) != 6 {
			t.Errorf("want 6 starter entities, got %d (%v)", len(cfg.Entities), cfg.Entities)
		}
		for _, ent := range []string{"ticket_type", "price_tier", "venue", "artist", "genre", "ticket_pool"} {
			if _, ok := cfg.Entities[ent]; !ok {
				t.Errorf("starter missing entity %q", ent)
			}
		}
		// Starter ships with EMPTY rules and an empty genre vocab.
		if g := cfg.Entities["genre"]; len(g.Vocab) != 0 {
			t.Errorf("starter genre vocab should be empty, got %v", g.Vocab)
		}
	})

	t.Run("operator override wins per entity and new entity present", func(t *testing.T) {
		opPath := filepath.Join(t.TempDir(), "normalize.yaml")
		operator := []byte(`version: 1
entities:
  genre:
    source: events.genres
    shape: vocab
    vocab: [house, techno]
  promoter:
    source: events.promoters[*].name
    shape: alias
`)
		if err := os.WriteFile(opPath, operator, 0o600); err != nil {
			t.Fatalf("write operator config: %v", err)
		}

		cfg, err := loadNormalizeConfigFrom(starterConfigYAML, opPath)
		if err != nil {
			t.Fatalf("loadNormalizeConfigFrom: %v", err)
		}

		// Operator's genre (with a real vocab) wins over the starter's empty one.
		gotVocab := append([]string(nil), cfg.Entities["genre"].Vocab...)
		sort.Strings(gotVocab)
		if want := []string{"house", "techno"}; !reflect.DeepEqual(gotVocab, want) {
			t.Errorf("genre vocab: got %v want %v", gotVocab, want)
		}

		// The new operator-only entity is present after the merge.
		promoter, ok := cfg.Entities["promoter"]
		if !ok {
			t.Fatalf("operator-added entity %q missing after merge", "promoter")
		}
		if promoter.Shape != normalizecfg.ShapeAlias {
			t.Errorf("promoter shape: got %q want %q", promoter.Shape, normalizecfg.ShapeAlias)
		}

		// Untouched starter entities survive the merge.
		if cfg.Entities["ticket_type"].Source != "tickets.ticketType.name" {
			t.Errorf("ticket_type source drifted after merge: %q", cfg.Entities["ticket_type"].Source)
		}
	})

	t.Run("malformed operator file returns error", func(t *testing.T) {
		opPath := filepath.Join(t.TempDir(), "normalize.yaml")
		// Unknown shape fails Parse validation (deliberately surfaced as error).
		bad := []byte(`version: 1
entities:
  genre:
    source: events.genres
    shape: not-a-real-shape
`)
		if err := os.WriteFile(opPath, bad, 0o600); err != nil {
			t.Fatalf("write malformed operator config: %v", err)
		}
		if _, err := loadNormalizeConfigFrom(starterConfigYAML, opPath); err == nil {
			t.Fatal("want error for malformed operator config, got nil")
		}
	})
}

// TestHardcodedFallbacksMatchStarter guards against silent drift between the
// hardcoded defensive fallbacks (hardcodedTierEntity / hardcodedVenueEntity) and
// the shipped starter config for the same entity names. The two are independent
// sources of truth; if the starter changes a source path, shape, or axis set,
// this test flags the unsynced fallback.
func TestHardcodedFallbacksMatchStarter(t *testing.T) {
	cfg, err := normalizecfg.Parse(starterConfigYAML)
	if err != nil {
		t.Fatalf("starter parse: %v", err)
	}

	cases := []struct {
		name       string
		entityName string
		hardcoded  normalizecfg.Entity
	}{
		{"ticket_type", "ticket_type", hardcodedTierEntity()},
		{"venue", "venue", hardcodedVenueEntity()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, ok := cfg.Entities[tc.entityName]
			if !ok {
				t.Fatalf("starter missing entity %q", tc.entityName)
			}
			got := tc.hardcoded
			if got.Source != want.Source {
				t.Errorf("source drift: hardcoded %q vs starter %q", got.Source, want.Source)
			}
			if got.Shape != want.Shape {
				t.Errorf("shape drift: hardcoded %q vs starter %q", got.Shape, want.Shape)
			}
			gotAttrs := append([]string(nil), got.Attributes...)
			wantAttrs := append([]string(nil), want.Attributes...)
			sort.Strings(gotAttrs)
			sort.Strings(wantAttrs)
			if !reflect.DeepEqual(gotAttrs, wantAttrs) {
				t.Errorf("attributes drift: hardcoded %v vs starter %v", gotAttrs, wantAttrs)
			}
		})
	}
}
