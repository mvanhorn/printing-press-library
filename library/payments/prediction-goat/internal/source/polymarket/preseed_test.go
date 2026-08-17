// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package polymarket_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/polymarket"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

func openPreseedTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "poly-preseed.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seedPolymarketEvent writes a single event into the resources table.
// The event payload carries a nested markets array; the scanner reads
// those inline rather than fetching the secondary projection.
func seedPolymarketEvent(t *testing.T, s *store.Store, slug string, payload map[string]any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Use Upsert (generic resources insert) rather than UpsertEvents
	// because the typed events table requires every spec-tracked
	// column to be present; tests want minimal fixtures.
	if err := s.Upsert("events", slug, data); err != nil {
		t.Fatalf("upsert events/%s: %v", slug, err)
	}
}

func TestPolymarketScanForPreseed_NbaChampionshipFixture(t *testing.T) {
	s := openPreseedTestStore(t)

	seedPolymarketEvent(t, s, "nba-championship-2026", map[string]any{
		"slug":    "nba-championship-2026",
		"title":   "NBA Championship 2026",
		"negRisk": true,
		"markets": []any{
			map[string]any{"slug": "will-the-lakers-win-the-2026-nba-championship", "question": "Will the Lakers win the 2026 NBA Championship?"},
			map[string]any{"slug": "will-the-celtics-win-the-2026-nba-championship", "question": "Will the Celtics win the 2026 NBA Championship?"},
			map[string]any{"slug": "will-the-warriors-win-the-2026-nba-championship", "question": "Will the Warriors win the 2026 NBA Championship?"},
		},
	})

	rows, err := polymarket.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	if got, want := len(rows), 9; got != want {
		t.Fatalf("rows = %d, want %d (3 variants x 3 children)", got, want)
	}

	byResource := map[string][]learn.PreseedRow{}
	for _, r := range rows {
		byResource[r.ResourceID] = append(byResource[r.ResourceID], r)
	}
	for _, slug := range []string{
		"will-the-lakers-win-the-2026-nba-championship",
		"will-the-celtics-win-the-2026-nba-championship",
		"will-the-warriors-win-the-2026-nba-championship",
	} {
		group := byResource[slug]
		if len(group) != 3 {
			t.Errorf("%s emitted %d rows, want 3 variants", slug, len(group))
		}
		for _, r := range group {
			if r.Venue != polymarket.VenuePolymarket {
				t.Errorf("%s venue = %q, want %q", slug, r.Venue, polymarket.VenuePolymarket)
			}
			if r.ResourceType != "markets" {
				t.Errorf("%s resource_type = %q, want markets", slug, r.ResourceType)
			}
			if len(r.Entities) != 1 {
				t.Errorf("%s entities = %v, want length 1", slug, r.Entities)
			}
		}
	}

	// Spot-check the Lakers entity extraction.
	lakers := byResource["will-the-lakers-win-the-2026-nba-championship"]
	if len(lakers) == 0 || lakers[0].Entities[0] != "Lakers" {
		t.Errorf("Lakers entity extraction failed; got %+v", lakers)
	}
}

func TestPolymarketScanForPreseed_QuestionWithoutEntityShape_Skipped(t *testing.T) {
	s := openPreseedTestStore(t)

	seedPolymarketEvent(t, s, "fed-decision-q3", map[string]any{
		"slug":    "fed-decision-q3",
		"title":   "Fed Q3 Decision",
		"negRisk": true,
		"markets": []any{
			// Doesn't match "Will [the] X win/be/become ..." shape.
			map[string]any{"slug": "fed-rate-50bps", "question": "Federal Reserve cuts 50bps in September"},
			// Empty question — should be filtered out before regex.
			map[string]any{"slug": "fed-rate-25bps", "question": ""},
			// Matching shape.
			map[string]any{"slug": "will-the-fed-be-dovish", "question": "Will the Fed be dovish in Q3?"},
		},
	})

	rows, err := polymarket.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	// Only the third child should produce rows; expect at least one.
	if len(rows) == 0 {
		t.Fatalf("expected rows for the matching question; got 0")
	}
	for _, r := range rows {
		if r.ResourceID != "will-the-fed-be-dovish" {
			t.Errorf("unexpected resource_id %q in rows", r.ResourceID)
		}
	}
}

func TestPolymarketScanForPreseed_NonNegRiskEventSkipped(t *testing.T) {
	s := openPreseedTestStore(t)

	seedPolymarketEvent(t, s, "single-market-event", map[string]any{
		"slug":    "single-market-event",
		"title":   "Single Market",
		"negRisk": false,
		"markets": []any{
			map[string]any{"slug": "will-the-team-win", "question": "Will the Team win the cup?"},
		},
	})

	rows, err := polymarket.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0 (negRisk=false must skip)", len(rows))
	}
}

func TestPolymarketScanForPreseed_BoolStringShapeAccepted(t *testing.T) {
	s := openPreseedTestStore(t)

	seedPolymarketEvent(t, s, "string-shaped", map[string]any{
		"slug":    "string-shaped",
		"title":   "String Shaped",
		"negRisk": "true", // literal string instead of bool
		"markets": []any{
			map[string]any{"slug": "will-the-team-win-cup", "question": "Will the Team win the cup?"},
		},
	})

	rows, err := polymarket.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	if len(rows) == 0 {
		t.Errorf("scanner ignored string-shaped negRisk=true marker")
	}
}

func TestPolymarketScanForPreseed_EmptyTitleEventSkipped(t *testing.T) {
	s := openPreseedTestStore(t)

	seedPolymarketEvent(t, s, "no-title", map[string]any{
		"slug":    "no-title",
		"title":   "",
		"negRisk": true,
		"markets": []any{
			map[string]any{"slug": "will-the-team-win-something", "question": "Will the Team win something?"},
		},
	})

	rows, err := polymarket.ScanForPreseed(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("ScanForPreseed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0 (empty title must skip; no topic phrase to format)", len(rows))
	}
}
