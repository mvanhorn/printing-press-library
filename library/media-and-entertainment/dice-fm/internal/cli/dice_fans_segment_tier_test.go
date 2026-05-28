// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// TDD tests for the classifier-backed --tier filter on fans segment. --tier
// matches on EITHER the raw priceTier.name substring (back-compat) OR the
// normalized tier resolved via the classifier (canonical tier name substring,
// or access_class/sales_stage/entry_window value equality). All fixtures are
// synthetic; no real tenant ticket-type names.
package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// seedSegmentCrosswalkNamed seeds a crosswalk row PLUS the canonical_entity row
// so the canonical tier name is available to the --tier classifier path. Other
// axes are taken from the provided values.
func seedSegmentCrosswalkNamed(t *testing.T, s *store.Store, ticketTypeName, canonicalName, accessClass, salesStage, entryWindow string) {
	t.Helper()
	cid := mintCanonicalID("ticket_type", canonicalName)
	if err := s.UpsertCanonicalEntity("ticket_type", cid, canonicalName); err != nil {
		t.Fatalf("upsert canonical entity %q: %v", canonicalName, err)
	}
	if err := s.UpsertCrosswalk(store.CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice",
		SourceValue: ticketTypeName, CanonicalID: cid,
		Method: "regex", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert crosswalk %q: %v", ticketTypeName, err)
	}
	if err := s.UpsertTierAttributes(cid, store.TierAttributesRow{
		CanonicalID:       cid,
		AccessClass:       accessClass,
		SalesStage:        salesStage,
		EntryWindowType:   entryWindow,
		ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert tier attrs %q: %v", ticketTypeName, err)
	}
}

// TestFansSegmentTierClassifierBacked verifies --tier matches via the classifier
// axis path: a fan holding a ticket whose ticketType normalizes to access_class=vip
// matches --tier vip even though the raw priceTier.name does NOT contain "vip".
func TestFansSegmentTierClassifierBacked(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 8000, 0, 1, false, "", ""),
			"o-b1": order("o-b1", "2026-01-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"tickets": {
			// fanA's priceTier.name is "Front Section" (no "vip"), but the
			// ticketType "Backstage Pass" normalizes to access_class=vip.
			"tk-a1": ticketWithHolder("tk-a1", fanA, "Backstage Pass", "Front Section", false),
			// fanB holds plain GA: no "vip" anywhere.
			"tk-b1": ticketWithHolder("tk-b1", fanB, "General Admission", "Standard", false),
		},
	})
	seedSegmentCrosswalkNamed(t, s, "Backstage Pass", "Backstage Pass", "vip", "", "")
	seedSegmentCrosswalkNamed(t, s, "General Admission", "General Admission", "ga", "", "")

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{tier: "vip"})
	if err != nil {
		t.Fatalf("computeFansSegment tier=vip (classifier): %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("tier=vip (classifier): want 1 fan, got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanA {
		t.Errorf("tier=vip (classifier): want fanA (access_class=vip), got %q", rows[0].Email)
	}
}

// TestFansSegmentTierRawFallback verifies the raw-fallback path survives: a fan
// whose priceTier.name contains "vip" but whose ticketType has NO crosswalk row
// still matches --tier vip.
func TestFansSegmentTierRawFallback(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-02-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 8000, 0, 1, false, "", ""),
			"o-b1": order("o-b1", "2026-02-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"tickets": {
			// fanA's priceTier.name contains "VIP"; ticketType is NOT crosswalked.
			"tk-a1": ticketWithHolder("tk-a1", fanA, "Some Uncrosswalked Type", "VIP Lounge", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "Another Type", "Standard", false),
		},
	})
	// No crosswalk seeded at all — classifier path is inert, raw must still work.

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{tier: "vip"})
	if err != nil {
		t.Fatalf("computeFansSegment tier=vip (raw fallback): %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("tier=vip (raw fallback): want 1 fan, got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanA {
		t.Errorf("tier=vip (raw fallback): want fanA (raw priceTier.name 'VIP Lounge'), got %q", rows[0].Email)
	}
}

// TestFansSegmentTierCanonicalNameMatch verifies --tier matches when the want
// value is a substring of the canonical tier name (even when no axis value
// matches and the raw priceTier.name does not contain it).
func TestFansSegmentTierCanonicalNameMatch(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-03-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 8000, 0, 1, false, "", ""),
			"o-b1": order("o-b1", "2026-03-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"tickets": {
			// fanA's raw priceTier.name "Tier One" does NOT contain "premium";
			// the canonical tier name is "Premium Balcony". access_class is ga.
			"tk-a1": ticketWithHolder("tk-a1", fanA, "Balcony Type", "Tier One", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "Floor Type", "Tier Two", false),
		},
	})
	seedSegmentCrosswalkNamed(t, s, "Balcony Type", "Premium Balcony", "ga", "", "")
	seedSegmentCrosswalkNamed(t, s, "Floor Type", "Standard Floor", "ga", "", "")

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{tier: "premium"})
	if err != nil {
		t.Fatalf("computeFansSegment tier=premium (canonical name): %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("tier=premium (canonical name): want 1 fan, got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanA {
		t.Errorf("tier=premium (canonical name): want fanA (canonical 'Premium Balcony'), got %q", rows[0].Email)
	}
}

// TestFansSegmentTierClassifierNoMatch verifies a tier value that matches
// neither raw nor classifier returns no rows (no false positives).
func TestFansSegmentTierClassifierNoMatch(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-04-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 8000, 0, 1, false, "", ""),
		},
		"tickets": {
			"tk-a1": ticketWithHolder("tk-a1", fanA, "Backstage Pass", "Front Section", false),
		},
	})
	seedSegmentCrosswalkNamed(t, s, "Backstage Pass", "Backstage Pass", "vip", "", "")

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{tier: "earlybird"})
	if err != nil {
		t.Fatalf("computeFansSegment tier=earlybird (no match): %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("tier=earlybird (no match): want 0 rows, got %d: %+v", len(rows), rows)
	}
}

// TestFansSegmentTierAxisSalesStageMatch verifies --tier matches a normalized
// sales_stage value (axis equality), independent of access_class.
func TestFansSegmentTierAxisSalesStageMatch(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-05-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 2500, 0, 1, false, "", ""),
			"o-b1": order("o-b1", "2026-05-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 5000, 0, 1, false, "", ""),
		},
		"tickets": {
			// fanA: ticketType normalizes to sales_stage=early_bird; raw tier text
			// "Phase 1" does not contain "early_bird".
			"tk-a1": ticketWithHolder("tk-a1", fanA, "Phase One Ticket", "Phase 1", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "Standard Ticket", "Phase 2", false),
		},
	})
	seedSegmentCrosswalkNamed(t, s, "Phase One Ticket", "Phase One Ticket", "ga", "early_bird", "")
	seedSegmentCrosswalkNamed(t, s, "Standard Ticket", "Standard Ticket", "ga", "", "")

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{tier: "early_bird"})
	if err != nil {
		t.Fatalf("computeFansSegment tier=early_bird (sales_stage): %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Errorf("tier=early_bird (sales_stage): want only fanA, got %+v", rows)
	}
}
