package cli

import (
	"strings"
	"testing"
)

// TestBackfillHint_ReferencesSkill ensures the hint string points users at
// the canonical skill invocation. Downstream UX (README, doctor, add) all
// reuse this string, so regressing the wording is a single-source change.
func TestBackfillHint_ReferencesSkill(t *testing.T) {
	hint := backfillHint()
	if !strings.Contains(hint, "/pp-instacart backfill") {
		t.Errorf("hint should name the skill invocation; got %q", hint)
	}
	if !strings.Contains(hint, "order history") {
		t.Errorf("hint should mention what gets populated; got %q", hint)
	}
}

func TestHistoryIsEmpty_FreshStore(t *testing.T) {
	app := newTestApp(t)
	if !historyIsEmpty(app) {
		t.Fatal("fresh store should be reported empty")
	}
}

func TestHistoryIsEmpty_WithOrder(t *testing.T) {
	app := newTestApp(t)
	// Seeding a purchased_item is the cheapest way to make the store
	// non-empty without wiring a full order through the upsert path.
	seedPurchase(t, app.Store, "qfc", "items_42-111", "Test Item", 3, true)
	if historyIsEmpty(app) {
		t.Fatal("store with a purchased_item should not be reported empty")
	}
}
