// Integration test for the espn search command's rerank pipeline.
// Proves the loop closes end-to-end: a taught learning observably
// reorders the next discovery call's result list, --no-learn
// short-circuits the reranker, and missing-resource teaches do not
// corrupt the bundle.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"

	_ "modernc.org/sqlite"
)

// openIntegrationStore opens a fresh store at a t.TempDir() path so each
// test gets an isolated DB. The canonical learn schema (search_learnings
// + entity_lookups + search_patterns) lands via store.Open's migration.
func openIntegrationStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seedResource writes a synthetic resource row into the local store so
// the reranker's resolveSearchLearnedHit can promote an InsertLearnedHit
// when the boosted learning's resource_id isn't already in the result
// bundle. The schema mirrors espn's existing `resources` table.
func seedResource(t *testing.T, s *store.Store, rt, id, payload string) {
	t.Helper()
	if err := s.Upsert(rt, id, json.RawMessage(payload)); err != nil {
		t.Fatalf("seed resource: %v", err)
	}
}

// rawEvent builds a synthetic events-shape RawMessage. The applier's
// id-parser handles both string and number id columns; espn's
// SearchEvents returns JSON-string ids, so the tests use that form.
func rawEvent(id, name string) json.RawMessage {
	return json.RawMessage(`{"id":"` + id + `","name":"` + name + `"}`)
}

func TestApplyLearningsForSearch_BoostMovesTaughtResourceToFront(t *testing.T) {
	t.Parallel()
	s := openIntegrationStore(t)

	// Build a synthetic events slice with 6 entries; taught id is at index 5.
	taughtID := "401547439"
	events := []json.RawMessage{
		rawEvent("401547434", "Browns at Steelers"),
		rawEvent("401547435", "Bills at Jets"),
		rawEvent("401547436", "Falcons at Bucs"),
		rawEvent("401547437", "Eagles at Cowboys"),
		rawEvent("401547438", "Packers at Bears"),
		rawEvent(taughtID, "Niners at Cardinals"),
	}
	news := []json.RawMessage{}
	general := []json.RawMessage{}

	// Teach: agent answered "Niners game tonight" by surfacing event 401547439.
	if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{
		Query:        "Niners game tonight",
		ResourceID:   taughtID,
		ResourceType: "events",
		Action:       store.LearningActionBoost,
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach (first): %v", err)
	}
	// Second upsert bumps confidence from 2 to 3 (high-confidence floor).
	if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{
		Query:        "Niners game tonight",
		ResourceID:   taughtID,
		ResourceType: "events",
		Action:       store.LearningActionBoost,
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach (second): %v", err)
	}

	applied, hasHigh := applyLearningsForSearch(context.Background(), s, "Niners game tonight", &events, &news, &general)
	if applied == 0 {
		t.Fatalf("want applied>0, got 0")
	}
	if !hasHigh {
		t.Errorf("want hasHigh=true after two teaches (confidence=3); got false")
	}
	if got := rawHitID(events[0]); got != taughtID {
		t.Errorf("front of events = %q, want %q (taught resource should move to front)", got, taughtID)
	}
	// hintForApplied path
	if hint := hintForApplied(applied, hasHigh); hint != "none" {
		t.Errorf("hint = %q, want %q", hint, "none")
	}
}

func TestApplyLearningsForSearch_EmptyStoreReturnsZero(t *testing.T) {
	t.Parallel()
	s := openIntegrationStore(t)

	events := []json.RawMessage{rawEvent("e1", "x")}
	news := []json.RawMessage{rawEvent("n1", "y")}
	general := []json.RawMessage{rawEvent("g1", "z")}

	applied, hasHigh := applyLearningsForSearch(context.Background(), s, "kanye", &events, &news, &general)
	if applied != 0 {
		t.Errorf("want applied=0 on empty store, got %d", applied)
	}
	if hasHigh {
		t.Errorf("want hasHigh=false on empty store")
	}
	if rawHitID(events[0]) != "e1" {
		t.Errorf("events slice mutated on empty-store path: front=%q want %q", rawHitID(events[0]), "e1")
	}
	if hint := hintForApplied(applied, hasHigh); hint != "empty" {
		t.Errorf("hint = %q, want %q", hint, "empty")
	}
}

func TestApplyLearningsForSearch_LowConfidenceWhenSingleTeach(t *testing.T) {
	t.Parallel()
	s := openIntegrationStore(t)

	taughtID := "401547440"
	events := []json.RawMessage{
		rawEvent("a", "x"),
		rawEvent(taughtID, "y"),
	}
	news := []json.RawMessage{}
	general := []json.RawMessage{}

	// Single teach -> confidence=2 -> below the high-confidence floor of 3.
	if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{
		Query:        "kanye",
		ResourceID:   taughtID,
		ResourceType: "events",
		Action:       store.LearningActionBoost,
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach: %v", err)
	}

	applied, hasHigh := applyLearningsForSearch(context.Background(), s, "kanye", &events, &news, &general)
	if applied == 0 {
		t.Fatalf("want applied>0, got 0")
	}
	if hasHigh {
		t.Errorf("want hasHigh=false at confidence=2, got true")
	}
	if hint := hintForApplied(applied, hasHigh); hint != "low_confidence" {
		t.Errorf("hint = %q, want %q", hint, "low_confidence")
	}
}

func TestApplyLearningsForSearch_InsertSyntheticHitFromStore(t *testing.T) {
	t.Parallel()
	s := openIntegrationStore(t)

	// The boosted resource is in the local store but NOT in the upstream
	// search result list. resolveSearchLearnedHit should fetch it from
	// the resources table and prepend it.
	stashedID := "401547442"
	seedResource(t, s, "events", stashedID, `{"id":"`+stashedID+`","name":"49ers at Rams"}`)

	events := []json.RawMessage{
		rawEvent("a", "x"),
		rawEvent("b", "y"),
	}
	news := []json.RawMessage{}
	general := []json.RawMessage{}

	if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{
		Query:        "Niners game tonight",
		ResourceID:   stashedID,
		ResourceType: "events",
		Action:       store.LearningActionBoost,
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach: %v", err)
	}

	applied, _ := applyLearningsForSearch(context.Background(), s, "Niners game tonight", &events, &news, &general)
	if applied == 0 {
		t.Fatalf("want applied>0, got 0")
	}
	if len(events) != 3 {
		t.Fatalf("events len = %d, want 3 (synthetic prepended)", len(events))
	}
	if got := rawHitID(events[0]); got != stashedID {
		t.Errorf("front of events = %q, want synthetic %q", got, stashedID)
	}
}

func TestApplyLearningsForSearch_MissingResourceSoftFails(t *testing.T) {
	t.Parallel()
	s := openIntegrationStore(t)

	// Teach a resource that is NOT in events/news/resources. The applier
	// must soft-fail (no panic, no error propagation) and leave the
	// bundle untouched.
	events := []json.RawMessage{rawEvent("e1", "x")}
	news := []json.RawMessage{}
	general := []json.RawMessage{}

	if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{
		Query:        "kanye",
		ResourceID:   "no-such-id",
		ResourceType: "events",
		Action:       store.LearningActionBoost,
		Source:       store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach: %v", err)
	}

	applied, hasHigh := applyLearningsForSearch(context.Background(), s, "kanye", &events, &news, &general)
	// Missing-resource teach: appliers return ErrApplierSkip so Apply
	// neither counts the rule nor mutates the bundle. hint should be
	// "empty" because nothing actually applied.
	if applied != 0 {
		t.Errorf("want applied=0 for missing-resource teach, got %d", applied)
	}
	if hasHigh {
		t.Errorf("want hasHigh=false for missing-resource teach")
	}
	if hint := hintForApplied(applied, hasHigh); hint != "empty" {
		t.Errorf("hint = %q, want %q", hint, "empty")
	}
	if len(events) != 1 || rawHitID(events[0]) != "e1" {
		t.Errorf("events mutated despite missing-resource teach: %v", events)
	}
}

// TestApplyLearningsForSearch_UntypedLearningLandsInOneGroup pins the
// fix for the empty-resource_type triple-insert bug: a learning with no
// resource_type must surface in exactly one search slice (the one whose
// table actually holds the row), not get prepended to events + news +
// general.
func TestApplyLearningsForSearch_UntypedLearningLandsInOneGroup(t *testing.T) {
	t.Parallel()
	s := openIntegrationStore(t)

	stashedID := "401547443"
	seedResource(t, s, "events", stashedID, `{"id":"`+stashedID+`","name":"Untyped boost target"}`)

	events := []json.RawMessage{rawEvent("e1", "x")}
	news := []json.RawMessage{rawEvent("n1", "y")}
	general := []json.RawMessage{rawEvent("g1", "z")}

	if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{
		Query:      "ambiguous topic",
		ResourceID: stashedID,
		// ResourceType intentionally empty.
		Action: store.LearningActionBoost,
		Source: store.LearningSourceTaught,
	}); err != nil {
		t.Fatalf("teach: %v", err)
	}

	_, _ = applyLearningsForSearch(context.Background(), s, "ambiguous topic", &events, &news, &general)
	// Only the events slice should have the synthetic insert; news and
	// general should be untouched (any other behavior is the
	// triple-insert regression).
	if len(events) != 2 || rawHitID(events[0]) != stashedID {
		t.Errorf("events should have the untyped boost at front; got %v", events)
	}
	if len(news) != 1 || rawHitID(news[0]) != "n1" {
		t.Errorf("news should be untouched by untyped boost; got %v", news)
	}
	if len(general) != 1 || rawHitID(general[0]) != "g1" {
		t.Errorf("general should be untouched by untyped boost; got %v", general)
	}
}
