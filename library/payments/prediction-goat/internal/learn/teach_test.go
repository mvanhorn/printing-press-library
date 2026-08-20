// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package learn_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// openValidateStore opens a fresh store at a temp path. Mirrors
// openRecallStore in recall_test.go so the validator tests have the
// same isolated-DB setup the recall tests use.
func openValidateStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "validate.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seedValidateResource is the same shape as recall_test.seedResource
// duplicated here to avoid pulling the recall_test helpers into a
// shared file -- the tests evolve independently and the duplication is
// trivial.
func seedValidateResource(t *testing.T, s *store.Store, resourceType, id string, payload map[string]any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := s.Upsert(resourceType, id, data); err != nil {
		t.Fatalf("upsert resource (%s/%s): %v", resourceType, id, err)
	}
}

// TestValidateResourceShape_ParentEventWithMatchingChild is the
// flagship USA case from the U6 plan: a teach against the parent
// ticker for a query carrying the USA entity should surface a
// parent_event_when_child_exists warning naming the matching child.
func TestValidateResourceShape_ParentEventWithMatchingChild(t *testing.T) {
	s := openValidateStore(t)

	// Parent event resource.
	seedValidateResource(t, s, "kalshi_events", "KXMENWORLDCUP-26", map[string]any{
		"title":         "2026 Men's World Cup Winner",
		"event_ticker":  "KXMENWORLDCUP-26",
		"series_ticker": "KXMENWORLDCUP",
	})
	// Child market with USA subtitle.
	seedValidateResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-US", map[string]any{
		"title":         "FIFA Men's World Cup 2026 Winner",
		"yes_sub_title": "USA",
		"ticker":        "KXMENWORLDCUP-26-US",
		"event_ticker":  "KXMENWORLDCUP-26",
	})

	got := learn.ValidateResourceShape(
		context.Background(), s.DB(),
		"odds USA wins world cup",
		"KXMENWORLDCUP-26",
		"kalshi_events",
	)
	if len(got) == 0 {
		t.Fatalf("want parent_event_when_child_exists warning; got none")
	}
	var found *learn.Warning
	for i := range got {
		if got[i].Code == learn.WarningParentEventWhenChildExists {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("want %s warning; got %+v", learn.WarningParentEventWhenChildExists, got)
	}
	if found.Suggested != "KXMENWORLDCUP-26-US" {
		t.Errorf("want suggested=KXMENWORLDCUP-26-US, got %q", found.Suggested)
	}
	if found.Resource != "KXMENWORLDCUP-26" {
		t.Errorf("want resource=KXMENWORLDCUP-26 (the thing taught), got %q", found.Resource)
	}
}

// TestValidateResourceShape_ParentEventWithoutMatchingChild covers the
// "no entity match in child markets" case: the parent IS the right
// target for this query family, so no warning fires.
func TestValidateResourceShape_ParentEventWithoutMatchingChild(t *testing.T) {
	s := openValidateStore(t)

	seedValidateResource(t, s, "kalshi_events", "KXMENWORLDCUP-26", map[string]any{
		"title":        "2026 Men's World Cup Winner",
		"event_ticker": "KXMENWORLDCUP-26",
	})
	// Only Portugal child -- USA query has no matching child.
	seedValidateResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-PT", map[string]any{
		"yes_sub_title": "Portugal",
		"ticker":        "KXMENWORLDCUP-26-PT",
		"event_ticker":  "KXMENWORLDCUP-26",
	})

	got := learn.ValidateResourceShape(
		context.Background(), s.DB(),
		"odds USA wins world cup",
		"KXMENWORLDCUP-26",
		"kalshi_events",
	)
	for _, w := range got {
		if w.Code == learn.WarningParentEventWhenChildExists {
			t.Errorf("did not expect parent_event_when_child_exists; got %+v", w)
		}
	}
}

// TestValidateResourceShape_ChildTickerNotWarned ensures that teaching
// against a child ticker (KXMENWORLDCUP-26-US) does NOT fire the
// parent-vs-child warning. The whole point of the warning is to
// redirect parent teaches to children; a child teach is the desired
// shape.
func TestValidateResourceShape_ChildTickerNotWarned(t *testing.T) {
	s := openValidateStore(t)

	seedValidateResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-US", map[string]any{
		"yes_sub_title": "USA",
		"ticker":        "KXMENWORLDCUP-26-US",
		"event_ticker":  "KXMENWORLDCUP-26",
	})

	got := learn.ValidateResourceShape(
		context.Background(), s.DB(),
		"odds USA wins world cup",
		"KXMENWORLDCUP-26-US",
		"kalshi_markets",
	)
	for _, w := range got {
		if w.Code == learn.WarningParentEventWhenChildExists {
			t.Errorf("child-ticker teach should not fire parent warning; got %+v", w)
		}
		if w.Code == "no_entity_overlap" {
			t.Errorf("child-ticker teach with matching entity should not fire no_entity_overlap; got %+v", w)
		}
	}
}

// TestValidateResourceShape_QueryWithoutEntities covers the
// categorical-pattern teach case: a query like "rate cuts" with no
// extracted entities should never produce a no_entity_overlap warning
// even when the resource carries entities.
func TestValidateResourceShape_QueryWithoutEntities(t *testing.T) {
	s := openValidateStore(t)

	seedValidateResource(t, s, "kalshi_markets", "KXFEDCUT-26-FOMC", map[string]any{
		"title":         "Fed Rate Decision",
		"yes_sub_title": "Cut",
		"ticker":        "KXFEDCUT-26-FOMC",
	})

	got := learn.ValidateResourceShape(
		context.Background(), s.DB(),
		"rate cuts this year",
		"KXFEDCUT-26-FOMC",
		"kalshi_markets",
	)
	for _, w := range got {
		if w.Code == "no_entity_overlap" {
			t.Errorf("query without entities should not fire no_entity_overlap; got %+v", w)
		}
	}
}

// TestValidateResourceShape_EntityMismatch covers the wrong-team teach
// anti-pattern. A USA query taught against a Portugal-tagged resource
// fires no_entity_overlap.
func TestValidateResourceShape_EntityMismatch(t *testing.T) {
	s := openValidateStore(t)

	seedValidateResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-PT", map[string]any{
		"title":         "FIFA Men's World Cup 2026 Winner",
		"yes_sub_title": "Portugal",
		"ticker":        "KXMENWORLDCUP-26-PT",
		"event_ticker":  "KXMENWORLDCUP-26",
	})

	got := learn.ValidateResourceShape(
		context.Background(), s.DB(),
		"odds USA wins world cup",
		"KXMENWORLDCUP-26-PT",
		"kalshi_markets",
	)
	var found *learn.Warning
	for i := range got {
		if got[i].Code == "no_entity_overlap" {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("want no_entity_overlap warning; got %+v", got)
	}
	if found.Resource != "KXMENWORLDCUP-26-PT" {
		t.Errorf("want resource=KXMENWORLDCUP-26-PT, got %q", found.Resource)
	}
	// Detail should reference both sets of entities so a human grepping
	// the log can see what went wrong.
	if !strings.Contains(found.Detail, "USA") {
		t.Errorf("detail should mention USA (query side); got %q", found.Detail)
	}
	if !strings.Contains(found.Detail, "Portugal") {
		t.Errorf("detail should mention Portugal (resource side); got %q", found.Detail)
	}
}

// TestValidateResourceShape_ResourceMissingFromStore covers the
// "we can't validate what we can't read" case. The teach still
// returns no warnings because the validator declines to fabricate a
// finding from a missing resource.
func TestValidateResourceShape_ResourceMissingFromStore(t *testing.T) {
	s := openValidateStore(t)

	got := learn.ValidateResourceShape(
		context.Background(), s.DB(),
		"odds USA wins world cup",
		"KXMENWORLDCUP-26-US", // not seeded
		"kalshi_markets",
	)
	for _, w := range got {
		if w.Code == "no_entity_overlap" {
			t.Errorf("missing resource should not fire no_entity_overlap; got %+v", w)
		}
	}
}

// TestValidateResourceShape_PolymarketResourceNoParentWarning
// documents the current gap: Polymarket resources don't have a clean
// parent-shape detector, so parent_event_when_child_exists doesn't
// fire on them. The TODO in teach.go references the Deferred section
// of the U6 plan.
func TestValidateResourceShape_PolymarketResourceNoParentWarning(t *testing.T) {
	s := openValidateStore(t)

	seedValidateResource(t, s, "events", "will-anyone-win-2026-world-cup", map[string]any{
		"title": "Who will win the 2026 World Cup?",
		"slug":  "will-anyone-win-2026-world-cup",
	})

	got := learn.ValidateResourceShape(
		context.Background(), s.DB(),
		"odds USA wins world cup",
		"will-anyone-win-2026-world-cup",
		"events",
	)
	for _, w := range got {
		if w.Code == learn.WarningParentEventWhenChildExists {
			t.Errorf("Polymarket parent detection is intentionally deferred; got warning %+v", w)
		}
	}
}

// TestValidateResourceShape_NilDB returns no warnings rather than
// panicking. The CLI hook should never pass a nil DB, but defensive
// behavior protects future callers that might.
func TestValidateResourceShape_NilDB(t *testing.T) {
	got := learn.ValidateResourceShape(
		context.Background(), nil,
		"odds USA wins world cup",
		"KXMENWORLDCUP-26",
		"kalshi_events",
	)
	if got != nil {
		t.Errorf("want nil warnings for nil db; got %+v", got)
	}
}

// TestValidateResourceShape_EmptyResourceID is a no-op. A teach with
// an empty resource_id should never have reached the validator (the
// upsert layer rejects it), but the validator returns nil
// defensively.
func TestValidateResourceShape_EmptyResourceID(t *testing.T) {
	s := openValidateStore(t)
	got := learn.ValidateResourceShape(
		context.Background(), s.DB(),
		"odds USA wins world cup",
		"",
		"kalshi_events",
	)
	if got != nil {
		t.Errorf("want nil warnings for empty resource_id; got %+v", got)
	}
}

// TestValidateResourceShape_ParentDetectedByShapeWhenTypeMissing
// covers the older-teach-row case where the caller didn't supply a
// resource_type. IsKalshiParentTicker on the ID should still fire the
// parent-detection branch.
func TestValidateResourceShape_ParentDetectedByShapeWhenTypeMissing(t *testing.T) {
	s := openValidateStore(t)

	seedValidateResource(t, s, "kalshi_markets", "KXMENWORLDCUP-26-US", map[string]any{
		"yes_sub_title": "USA",
		"ticker":        "KXMENWORLDCUP-26-US",
		"event_ticker":  "KXMENWORLDCUP-26",
	})

	got := learn.ValidateResourceShape(
		context.Background(), s.DB(),
		"odds USA wins world cup",
		"KXMENWORLDCUP-26",
		"", // no resource_type
	)
	var found *learn.Warning
	for i := range got {
		if got[i].Code == learn.WarningParentEventWhenChildExists {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("want parent_event_when_child_exists via shape detection; got %+v", got)
	}
	if found.Suggested != "KXMENWORLDCUP-26-US" {
		t.Errorf("want suggested=KXMENWORLDCUP-26-US, got %q", found.Suggested)
	}
}
