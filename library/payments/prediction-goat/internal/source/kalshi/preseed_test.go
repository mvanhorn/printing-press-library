// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package kalshi_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// openPreseedTestStore mirrors the preseed test fixture setup —
// fresh DB per test, cleanup hook.
func openPreseedTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "kalshi-preseed.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seedKalshiResource writes a row into the resources table with the
// kalshi_{events,markets} resource_type and the supplied payload.
// Keeps each test's fixture obvious at the callsite.
func seedKalshiResource(t *testing.T, s *store.Store, resourceType, id string, payload map[string]any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := s.Upsert(resourceType, id, data); err != nil {
		t.Fatalf("upsert %s/%s: %v", resourceType, id, err)
	}
}

// TestKalshiScanForPreseed_WorldCupFixture exercises the dominant
// success path: one mutually_exclusive=true event with three
// well-shaped children. The scanner should emit 3 variants per child
// (9 rows) and entity strings should match the yes_sub_title field.
func TestKalshiScanForPreseed_WorldCupFixture(t *testing.T) {
	s := openPreseedTestStore(t)

	seedKalshiResource(t, s, "kalshi_events", "KXMENWORLDCUP-26", map[string]any{
		"event_ticker":       "KXMENWORLDCUP-26",
		"series_ticker":      "KXMENWORLDCUP",
		"title":              "2026 Men's World Cup Winner",
		"category":           "Sports",
		"mutually_exclusive": true,
	})
	for _, m := range []struct {
		ticker string
		sub    string
		title  string
	}{
		{"KXMENWORLDCUP-26-US", "USA", "Will the USA win the 2026 Men's World Cup?"},
		{"KXMENWORLDCUP-26-PT", "Portugal", "Will the Portugal win the 2026 Men's World Cup?"},
		{"KXMENWORLDCUP-26-EN", "England", "Will the England win the 2026 Men's World Cup?"},
	} {
		seedKalshiResource(t, s, "kalshi_markets", m.ticker, map[string]any{
			"ticker":        m.ticker,
			"event_ticker":  "KXMENWORLDCUP-26",
			"title":         m.title,
			"yes_sub_title": m.sub,
			"status":        "active",
		})
	}

	rows, err := kalshi.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	if got, want := len(rows), 9; got != want {
		t.Fatalf("rows = %d, want %d (3 variants x 3 children)", got, want)
	}

	// Group by resource_id and confirm each child got 3 variants
	// with matching entity strings.
	byResource := map[string][]learn.PreseedRow{}
	for _, r := range rows {
		byResource[r.ResourceID] = append(byResource[r.ResourceID], r)
	}
	for _, ticker := range []string{"KXMENWORLDCUP-26-US", "KXMENWORLDCUP-26-PT", "KXMENWORLDCUP-26-EN"} {
		group := byResource[ticker]
		if len(group) != 3 {
			t.Errorf("%s emitted %d rows, want 3 variants", ticker, len(group))
		}
		for _, r := range group {
			if r.Venue != kalshi.VenueKalshi {
				t.Errorf("%s venue = %q, want %q", ticker, r.Venue, kalshi.VenueKalshi)
			}
			if r.ResourceType != "kalshi_markets" {
				t.Errorf("%s resource_type = %q, want kalshi_markets", ticker, r.ResourceType)
			}
			if len(r.Entities) != 1 {
				t.Errorf("%s entities = %v, want length 1", ticker, r.Entities)
			}
		}
	}

	// Variant shape spot-check: the USA child should produce at
	// least the canonical "odds USA wins ..." form.
	usaRows := byResource["KXMENWORLDCUP-26-US"]
	foundCanonical := false
	for _, r := range usaRows {
		if r.QueryPattern == "odds USA wins 2026 Men's World Cup Winner" {
			foundCanonical = true
			break
		}
	}
	if !foundCanonical {
		patterns := make([]string, 0, len(usaRows))
		for _, r := range usaRows {
			patterns = append(patterns, r.QueryPattern)
		}
		t.Errorf("USA rows missing canonical pattern; got %v", patterns)
	}
}

// TestKalshiScanForPreseed_EmptySubTitleSkipped covers the guard that
// drops children with no entity to key on. A market whose yes_sub_title
// is empty cannot anchor a preseed pattern.
func TestKalshiScanForPreseed_EmptySubTitleSkipped(t *testing.T) {
	s := openPreseedTestStore(t)
	seedKalshiResource(t, s, "kalshi_events", "KX-EVT", map[string]any{
		"event_ticker":       "KX-EVT",
		"title":              "Some Event",
		"mutually_exclusive": true,
	})
	seedKalshiResource(t, s, "kalshi_markets", "KX-EVT-A", map[string]any{
		"ticker":        "KX-EVT-A",
		"event_ticker":  "KX-EVT",
		"title":         "Will the A win?",
		"yes_sub_title": "", // empty
	})

	rows, err := kalshi.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0 (empty yes_sub_title must skip)", len(rows))
	}
}

// TestKalshiScanForPreseed_MultiLegTitleSkipped covers the guard that
// drops children whose title is a comma-concatenated YES/NO leg
// shape — those markets don't carry a clean entity even when
// yes_sub_title is set, because the YES/NO leg combination is itself
// the discriminator.
func TestKalshiScanForPreseed_MultiLegTitleSkipped(t *testing.T) {
	s := openPreseedTestStore(t)
	seedKalshiResource(t, s, "kalshi_events", "KX-EVT", map[string]any{
		"event_ticker":       "KX-EVT",
		"title":              "Some Event",
		"mutually_exclusive": true,
	})
	seedKalshiResource(t, s, "kalshi_markets", "KX-EVT-X", map[string]any{
		"ticker":        "KX-EVT-X",
		"event_ticker":  "KX-EVT",
		// Canonical multi-leg shape: comma-joined YES/NO legs, no
		// space after the comma (matches the kalshiMultiLegTitleRE).
		"title":         "YES the A wins,YES the B wins",
		"yes_sub_title": "Combo",
	})

	rows, err := kalshi.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0 (multi-leg title must skip)", len(rows))
	}
}

// TestKalshiScanForPreseed_NonExclusiveEventSkipped covers the guard
// against scanning events that aren't multi-outcome winner-take-all.
// Events with mutually_exclusive=false (or missing) should not feed
// the preseed pipeline.
func TestKalshiScanForPreseed_NonExclusiveEventSkipped(t *testing.T) {
	s := openPreseedTestStore(t)
	seedKalshiResource(t, s, "kalshi_events", "KX-EVT-MULTI", map[string]any{
		"event_ticker":       "KX-EVT-MULTI",
		"title":              "Not Mutually Exclusive",
		"mutually_exclusive": false,
	})
	seedKalshiResource(t, s, "kalshi_markets", "KX-EVT-MULTI-A", map[string]any{
		"ticker":        "KX-EVT-MULTI-A",
		"event_ticker":  "KX-EVT-MULTI",
		"title":         "A market",
		"yes_sub_title": "USA",
	})

	rows, err := kalshi.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0 (mutually_exclusive=false must skip)", len(rows))
	}
}

// TestKalshiPreseedEndToEnd seeds a World Cup family, runs the
// driver via the (kalshi-registered) scanner, and confirms a recall
// for an unfamiliar entity ("USA") returns the right child ticker
// without any explicit teach. Mirrors the smoke path the plan calls
// out for U5 acceptance: drop DB -> seed -> preseed -> recall.
func TestKalshiPreseedEndToEnd(t *testing.T) {
	s := openPreseedTestStore(t)

	seedKalshiResource(t, s, "kalshi_events", "KXMENWORLDCUP-26", map[string]any{
		"event_ticker":       "KXMENWORLDCUP-26",
		"series_ticker":      "KXMENWORLDCUP",
		"title":              "2026 Men's World Cup Winner",
		"category":           "Sports",
		"mutually_exclusive": true,
	})
	for _, m := range []struct {
		ticker string
		sub    string
	}{
		{"KXMENWORLDCUP-26-US", "USA"},
		{"KXMENWORLDCUP-26-PT", "Portugal"},
		{"KXMENWORLDCUP-26-EN", "England"},
	} {
		seedKalshiResource(t, s, "kalshi_markets", m.ticker, map[string]any{
			"ticker":        m.ticker,
			"event_ticker":  "KXMENWORLDCUP-26",
			"title":         "Will the " + m.sub + " win the 2026 Men's World Cup?",
			"yes_sub_title": m.sub,
			"status":        "active",
		})
	}

	// Drive Run via the registered scanner. The kalshi package init
	// registers ScanForPreseed under PreseedResourceType so importing
	// this package is enough to make Run aware of it.
	count, err := learn.Run(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("learn.Run: %v", err)
	}
	if count == 0 {
		t.Fatalf("preseed inserted 0 rows; expected the World Cup family to produce learnings")
	}

	// Recall an unfamiliar phrasing for the USA child. The recall
	// path should match a preseed row and surface KXMENWORLDCUP-26-US.
	result, err := learn.Recall(context.Background(), s.DB(), "odds USA wins world cup", learn.Opts{})
	if err != nil {
		t.Fatalf("learn.Recall: %v", err)
	}
	if !result.Found {
		t.Fatalf("recall returned Found=false; want USA ticker via preseed. Result=%+v", result)
	}
	if len(result.Results) == 0 || result.Results[0].ResourceID != "KXMENWORLDCUP-26-US" {
		ids := make([]string, 0, len(result.Results))
		for _, h := range result.Results {
			ids = append(ids, h.ResourceID)
		}
		t.Errorf("top recall hit = %v, want KXMENWORLDCUP-26-US at position 0", ids)
	}
	if result.Results[0].Source != learn.SourcePreseed {
		t.Errorf("top hit source = %q, want %q", result.Results[0].Source, learn.SourcePreseed)
	}
}

// TestKalshiScanForPreseed_BoolStringShapeAccepted pins both the bool
// and the literal-string shape of mutually_exclusive. Kalshi has
// historically returned both depending on endpoint quirks.
func TestKalshiScanForPreseed_BoolStringShapeAccepted(t *testing.T) {
	s := openPreseedTestStore(t)
	// String-shaped truthy value.
	seedKalshiResource(t, s, "kalshi_events", "KX-EVT-STR", map[string]any{
		"event_ticker":       "KX-EVT-STR",
		"title":              "Some Event",
		"mutually_exclusive": "true",
	})
	seedKalshiResource(t, s, "kalshi_markets", "KX-EVT-STR-A", map[string]any{
		"ticker":        "KX-EVT-STR-A",
		"event_ticker":  "KX-EVT-STR",
		"title":         "Will the USA win?",
		"yes_sub_title": "USA",
	})

	rows, err := kalshi.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	if len(rows) == 0 {
		t.Errorf("scanner ignored the 'true' string-shaped mutually_exclusive marker")
	}
}
