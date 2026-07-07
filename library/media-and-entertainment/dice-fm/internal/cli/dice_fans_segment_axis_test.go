// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// TDD tests for axis-filter flags on fans segment: --access-class, --sales-stage,
// --entry-window, --comp, --min-group-size. All fixtures are synthetic; no real
// tenant ticket-type names (IETF example.com addresses only).
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// --- fixture helpers ---

// ticketWithHolderAndType builds a tickets payload with holder identity and type name.
// Reuses ticketWithHolder from dice_analytics_new_test.go.

// seedSegmentCrosswalk seeds a full axis row: access_class + comp_flag + group_size.
// Other axes are left zero/empty.
func seedSegmentCrosswalk(t *testing.T, s *store.Store, ticketTypeName, accessClass string, compFlag bool, groupSize int) {
	t.Helper()
	cid := mintCanonicalID("ticket_type", ticketTypeName)
	if err := s.UpsertCrosswalk(store.CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice",
		SourceValue: ticketTypeName, CanonicalID: cid,
		Method: "regex", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert crosswalk %q: %v", ticketTypeName, err)
	}
	if err := s.UpsertTierAttributes(cid, store.TierAttributesRow{
		CanonicalID: cid, AccessClass: accessClass,
		CompFlag: compFlag, GroupSize: groupSize,
		ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert tier attrs %q: %v", ticketTypeName, err)
	}
}

// seedSegmentCrosswalkFull seeds a complete axis row with all five axes.
func seedSegmentCrosswalkFull(t *testing.T, s *store.Store, ticketTypeName, accessClass, salesStage, entryWindow string, compFlag bool, groupSize int) {
	t.Helper()
	cid := mintCanonicalID("ticket_type", ticketTypeName)
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
		CompFlag:          compFlag,
		GroupSize:         groupSize,
		ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert tier attrs full %q: %v", ticketTypeName, err)
	}
}

// --- tests ---

// TestFansSegmentAccessClass verifies --access-class filters fans by their
// ticket's resolved access_class axis, does NOT shrink total_spend/events_count.
func TestFansSegmentAccessClass(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			// fanA: 2 orders across 2 events (total_spend 130.00)
			"o-a1": order("o-a1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 8000, 0, 1, false, "", ""),
			"o-a2": order("o-a2", "2026-01-20T10:00:00Z", "evtB", "Show B", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			// fanB: 1 order (total_spend 30.00)
			"o-b1": order("o-b1", "2026-01-15T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"tickets": {
			// fanA holds a VIP ticket; fanB holds a GA ticket.
			"tk-a1": ticketWithHolder("tk-a1", fanA, "VIP Experience", "vip-tier", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "General Admission", "ga-tier", false),
		},
	})
	seedSegmentCrosswalk(t, s, "VIP Experience", "vip", false, 0)
	seedSegmentCrosswalk(t, s, "General Admission", "ga", false, 0)

	// --access-class vip: only fanA qualifies.
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{accessClass: "vip"})
	if err != nil {
		t.Fatalf("computeFansSegment access_class=vip: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 fan for access_class=vip, got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanA {
		t.Errorf("want fanA, got %q", rows[0].Email)
	}
	// total_spend must include ALL of fanA's orders (not shrunk to VIP orders only).
	if rows[0].TotalSpend != 130.00 {
		t.Errorf("total_spend = %v, want 130.00 (not shrunk)", rows[0].TotalSpend)
	}
	// events_count = 2 (both events).
	if rows[0].EventsCount != 2 {
		t.Errorf("events_count = %d, want 2 (not shrunk)", rows[0].EventsCount)
	}

	// --access-class ga: only fanB qualifies.
	gaRows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{accessClass: "ga"})
	if err != nil {
		t.Fatalf("computeFansSegment access_class=ga: %v", err)
	}
	if len(gaRows) != 1 || gaRows[0].Email != fanB {
		t.Errorf("access_class=ga: want only fanB, got %+v", gaRows)
	}
}

// TestFansSegmentCompFlag verifies --comp filters fans by their ticket's
// resolved comp_flag axis.
func TestFansSegmentCompFlag(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-02-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 0, 0, 1, false, "", ""),
			"o-b1": order("o-b1", "2026-02-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"tickets": {
			"tk-a1": ticketWithHolder("tk-a1", fanA, "Comp Ticket", "comp-tier", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "General Admission", "ga-tier", false),
		},
	})
	seedSegmentCrosswalk(t, s, "Comp Ticket", "", true, 0)
	seedSegmentCrosswalk(t, s, "General Admission", "ga", false, 0)

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{comp: true})
	if err != nil {
		t.Fatalf("computeFansSegment comp=true: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 comp fan, got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanA {
		t.Errorf("want fanA (comp holder), got %q", rows[0].Email)
	}
}

// TestFansSegmentMinGroupSize verifies --min-group-size filters fans by their
// ticket's resolved group_size axis (>= N).
func TestFansSegmentMinGroupSize(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-03-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 8000, 0, 1, false, "", ""),
			"o-b1": order("o-b1", "2026-03-02T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 6000, 0, 1, false, "", ""),
			"o-c1": order("o-c1", "2026-03-03T10:00:00Z", "evtA", "Show A", fanC, "Cat", "C", 4000, 0, 1, false, "", ""),
		},
		"tickets": {
			// fanA: group of 6; fanB: group of 4; fanC: no group size
			"tk-a1": ticketWithHolder("tk-a1", fanA, "Table For Six", "table-tier", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "Table For Four", "table-tier", false),
			"tk-c1": ticketWithHolder("tk-c1", fanC, "General Admission", "ga-tier", false),
		},
	})
	seedSegmentCrosswalk(t, s, "Table For Six", "vip", false, 6)
	seedSegmentCrosswalk(t, s, "Table For Four", "vip", false, 4)
	seedSegmentCrosswalk(t, s, "General Admission", "ga", false, 0)

	// --min-group-size 5: only fanA (group 6) qualifies.
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{minGroupSize: 5})
	if err != nil {
		t.Fatalf("computeFansSegment minGroupSize=5: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Errorf("minGroupSize=5: want only fanA, got %+v", rows)
	}

	// --min-group-size 4: fanA (6) and fanB (4) qualify; fanC (0) does not.
	rows4, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{minGroupSize: 4})
	if err != nil {
		t.Fatalf("computeFansSegment minGroupSize=4: %v", err)
	}
	if len(rows4) != 2 {
		t.Fatalf("minGroupSize=4: want 2 fans, got %d: %+v", len(rows4), rows4)
	}
}

// TestFansSegmentSalesStage verifies --sales-stage filters by the resolved
// sales_stage axis.
func TestFansSegmentSalesStage(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-04-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 2500, 0, 1, false, "", ""),
			"o-b1": order("o-b1", "2026-04-02T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 5000, 0, 1, false, "", ""),
		},
		"tickets": {
			"tk-a1": ticketWithHolder("tk-a1", fanA, "Early Bird", "eb-tier", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "General Admission", "ga-tier", false),
		},
	})
	seedSegmentCrosswalkFull(t, s, "Early Bird", "ga", "early_bird", "", false, 0)
	seedSegmentCrosswalkFull(t, s, "General Admission", "ga", "", "", false, 0)

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{salesStage: "early_bird"})
	if err != nil {
		t.Fatalf("computeFansSegment salesStage=early_bird: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Errorf("salesStage=early_bird: want only fanA, got %+v", rows)
	}
}

// TestFansSegmentEntryWindow verifies --entry-window filters by the resolved
// entry_window_type axis.
func TestFansSegmentEntryWindow(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-05-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 3000, 0, 1, false, "", ""),
			"o-b1": order("o-b1", "2026-05-02T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"tickets": {
			"tk-a1": ticketWithHolder("tk-a1", fanA, "Deadline Entry", "dt-tier", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "Anytime Entry", "any-tier", false),
		},
	})
	seedSegmentCrosswalkFull(t, s, "Deadline Entry", "ga", "", "deadline", false, 0)
	seedSegmentCrosswalkFull(t, s, "Anytime Entry", "ga", "", "anytime", false, 0)

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{entryWindow: "deadline"})
	if err != nil {
		t.Fatalf("computeFansSegment entryWindow=deadline: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Errorf("entryWindow=deadline: want only fanA, got %+v", rows)
	}
}

// TestFansSegmentCombinedAxisFilters verifies that when multiple axis filters
// are provided a fan must satisfy ALL of them (AND semantics).
func TestFansSegmentCombinedAxisFilters(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			// fanA: 3 events, holds VIP+comp ticket -> qualifies for both axes
			"o-a1": order("o-a1", "2026-06-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o-a2": order("o-a2", "2026-06-02T10:00:00Z", "evtB", "Show B", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o-a3": order("o-a3", "2026-06-03T10:00:00Z", "evtC", "Show C", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			// fanB: 1 event, holds VIP ticket but NOT comp
			"o-b1": order("o-b1", "2026-06-01T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 8000, 0, 1, false, "", ""),
		},
		"tickets": {
			"tk-a1": ticketWithHolder("tk-a1", fanA, "VIP Comp", "vip-tier", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "VIP Paid", "vip-tier", false),
		},
	})
	seedSegmentCrosswalk(t, s, "VIP Comp", "vip", true, 0)
	seedSegmentCrosswalk(t, s, "VIP Paid", "vip", false, 0)

	// --access-class vip + --comp: only fanA satisfies both.
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{accessClass: "vip", comp: true})
	if err != nil {
		t.Fatalf("computeFansSegment access_class=vip+comp: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Errorf("access_class=vip+comp: want only fanA, got %+v", rows)
	}

	// --access-class vip + --min-events 2: only fanA (3 events) satisfies both.
	rows2, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{accessClass: "vip", minEvents: 2})
	if err != nil {
		t.Fatalf("computeFansSegment access_class=vip+minEvents=2: %v", err)
	}
	if len(rows2) != 1 || rows2[0].Email != fanA {
		t.Errorf("access_class=vip+min_events=2: want only fanA (3 events), got %+v", rows2)
	}
}

// TestFansSegmentAxisNoopWhenNoCrosswalk verifies the raw-fallback path:
// when no entity_crosswalk rows exist, axis filters cannot be satisfied and
// computeFansSegment returns no rows (not an error), plus a stderr warning.
func TestFansSegmentAxisNoopWhenNoCrosswalk(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-07-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
		},
		"tickets": {
			"tk-a1": ticketWithHolder("tk-a1", fanA, "VIP Experience", "vip-tier", false),
		},
	})
	// No crosswalk seeded — normalization not run.

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{accessClass: "vip"})
	if err != nil {
		t.Fatalf("computeFansSegment no-crosswalk: unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("no-crosswalk: want 0 rows (axis filter cannot be satisfied), got %d: %+v", len(rows), rows)
	}
}

// TestFansSegmentAxisNoCrosswalkWarnOnCmd verifies the newFansSegmentCmd warns
// to stderr when axis flags are used but normalization has not been run.
func TestFansSegmentAxisNoCrosswalkWarnOnCmd(t *testing.T) {
	// Isolate from the operator's real default-path store. The --dry-run flag is
	// never actually passed in SetArgs below, so without a $HOME override the
	// command would open ~/.local/share/dice-fm-pp-cli/data.db. A temp $HOME
	// yields no data.db, so the store open is a graceful no-op and this stays a
	// pure flag-acceptance test.
	t.Setenv("HOME", t.TempDir())
	flags := &rootFlags{dryRun: true}
	root := newRootCmd(flags)
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"fans", "segment", "--access-class", "vip"})
	if err := root.Execute(); err != nil {
		t.Fatalf("fans segment --access-class vip (dry-run) unexpected error: %v", err)
	}
}

// TestFansSegmentAxisFlagsAppearInHelp verifies all five new axis flags are
// registered on the fans segment command.
func TestFansSegmentAxisFlagsAppearInHelp(t *testing.T) {
	flags := &rootFlags{}
	root := newRootCmd(flags)
	var outBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetArgs([]string{"fans", "segment", "--help"})
	_ = root.Execute()
	help := outBuf.String()
	for _, flag := range []string{"--access-class", "--sales-stage", "--entry-window", "--comp", "--min-group-size"} {
		if !strings.Contains(help, flag) {
			t.Errorf("flag %q not found in 'fans segment --help' output", flag)
		}
	}
}

// TestFansSegmentExistingFiltersUnchanged verifies that the existing raw
// --ticket-type and --tier substring filters still work after the axis changes.
func TestFansSegmentExistingFiltersUnchanged(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o-a1": order("o-a1", "2026-08-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o-b1": order("o-b1", "2026-08-02T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"tickets": {
			"tk-a1": ticketWithHolder("tk-a1", fanA, "VIP Experience", "vip-tier", false),
			"tk-b1": ticketWithHolder("tk-b1", fanB, "General Admission", "ga-tier", false),
		},
	})
	// No crosswalk seeded — raw filter must still work.

	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{ticketType: "vip"})
	if err != nil {
		t.Fatalf("computeFansSegment raw ticketType=vip: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Errorf("raw ticketType=vip: want only fanA, got %+v", rows)
	}

	rows2, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{tier: "early"})
	if err != nil {
		t.Fatalf("computeFansSegment raw tier=early (no match): %v", err)
	}
	if len(rows2) != 0 {
		t.Errorf("raw tier=early (no match): want 0 rows, got %d: %+v", len(rows2), rows2)
	}
}
