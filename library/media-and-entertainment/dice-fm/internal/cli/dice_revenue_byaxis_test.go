// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for computeRevenueByAxis — normalized tier axis grouping with raw
// fallback. All fixtures are synthetic; no real tenant ticket-type names.
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// TestRevenueSummaryByAxisWithFiltersSucceeds verifies that --by-axis combined
// with --event, --from, or --to is now accepted (routes to the scoped path) and
// does NOT return an error. The previous rejection behavior was removed when
// computeRevenueByAxisScoped was added to support date/event-scoped axis views.
func TestRevenueSummaryByAxisWithFiltersSucceeds(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"by-axis + event", []string{"revenue", "summary", "--by-axis", "access_class", "--event", "evt-123"}},
		{"by-axis + from", []string{"revenue", "summary", "--by-axis", "access_class", "--from", "2026-01-01"}},
		{"by-axis + to", []string{"revenue", "summary", "--by-axis", "access_class", "--to", "2026-12-31"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Isolate from the operator's real default-path store. The command
			// resolves its store via openStoreForRead -> defaultDBPath ->
			// os.UserHomeDir(); without this override it opens
			// ~/.local/share/dice-fm-pp-cli/data.db (which under -race made the
			// suite scan a real ~70k-ticket store and time out). Pointing $HOME
			// at a temp dir means openStoreForRead sees no data.db and returns
			// (nil, nil) gracefully — the command still exercises the scoped
			// by-axis routing path and must not error.
			t.Setenv("HOME", t.TempDir())
			flags := &rootFlags{dryRun: true}
			root := newRootCmd(flags)
			var outBuf, errBuf bytes.Buffer
			root.SetOut(&outBuf)
			root.SetErr(&errBuf)
			root.SetArgs(tc.args)
			err := root.Execute()
			if err != nil {
				t.Errorf("want no error when --by-axis combined with filter flag, got: %v", err)
			}
		})
	}
}

// TestRevenueByAxisFallsBackWhenUnnormalized verifies that computeRevenueByAxis
// returns Normalized=false and does not error when the entity_crosswalk table
// has no rows for entity_type='ticket_type' (normalize has not been run yet).
func TestRevenueByAxisFallsBackWhenUnnormalized(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {"t1": `{"id":"t1","ticketType":{"name":"General Admission","price":3000}}`},
	})
	// No normalize run yet -> by-axis must not error; result flags normalized=false.
	res, err := computeRevenueByAxis(context.Background(), s.DB(), "access_class")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Normalized {
		t.Errorf("want normalized=false (normalization not run)")
	}
	// Fallback must still return rows grouped by raw ticketType.name.
	if len(res.Rows) == 0 {
		t.Errorf("want at least one fallback row when tickets exist")
	}
	// Warning must be non-empty on fallback.
	if res.Warning == "" {
		t.Errorf("want a warning message when falling back to raw grouping")
	}
}

// TestLoadTicketTypeCrosswalkUnsupportedAxis verifies that calling
// loadTicketTypeCrosswalk with an unrecognized axis name returns a descriptive
// error naming the bad axis, rather than panicking or executing malformed SQL.
func TestLoadTicketTypeCrosswalkUnsupportedAxis(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})
	_, err := loadTicketTypeCrosswalk(context.Background(), s.DB(), "bogus")
	if err == nil {
		t.Fatalf("want error for unsupported axis %q, got nil", "bogus")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error %q should mention the unsupported axis name", err.Error())
	}
}

// TestGroupTicketRevenueByAxisUnsupportedAxis verifies that calling
// groupTicketRevenueByAxis with an unrecognized axis name returns a descriptive
// error naming the bad axis, rather than constructing malformed SQL.
func TestGroupTicketRevenueByAxisUnsupportedAxis(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})
	_, err := groupTicketRevenueByAxis(context.Background(), s.DB(), "bogus")
	if err == nil {
		t.Fatalf("want error for unsupported axis %q, got nil", "bogus")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error %q should mention the unsupported axis name", err.Error())
	}
}

// TestRevenueByAxisNormalized seeds crosswalk + tier_attributes rows (as if
// normalize has been run) and verifies that computeRevenueByAxis groups by the
// requested axis with Normalized=true.
func TestRevenueByAxisNormalized(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		// Revenue --by-axis sums paid ticket $.total (not list price); set total
		// == price here so the per-bucket revenue assertions below are unchanged.
		"tickets": {
			"t1": `{"id":"t1","total":2500,"ticketType":{"name":"General Admission","price":2500}}`,
			"t2": `{"id":"t2","total":7500,"ticketType":{"name":"VIP Experience","price":7500}}`,
			"t3": `{"id":"t3","total":2500,"ticketType":{"name":"General Admission","price":2500}}`,
		},
	})

	// Mint canonical IDs + seed crosswalk + tier_attributes to simulate a
	// completed normalize run. GA -> access_class="ga"; VIP -> access_class="vip".
	gaID := mintCanonicalID("ticket_type", "general admission")
	vipID := mintCanonicalID("ticket_type", "vip experience")

	if err := s.UpsertCrosswalk(store.CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice",
		SourceValue: "General Admission", CanonicalID: gaID,
		Method: "regex", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert crosswalk GA: %v", err)
	}
	if err := s.UpsertCrosswalk(store.CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice",
		SourceValue: "VIP Experience", CanonicalID: vipID,
		Method: "regex", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert crosswalk VIP: %v", err)
	}
	if err := s.UpsertTierAttributes(gaID, store.TierAttributesRow{
		CanonicalID: gaID, AccessClass: "ga",
		ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert tier attrs GA: %v", err)
	}
	if err := s.UpsertTierAttributes(vipID, store.TierAttributesRow{
		CanonicalID: vipID, AccessClass: "vip",
		ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert tier attrs VIP: %v", err)
	}

	res, err := computeRevenueByAxis(context.Background(), s.DB(), "access_class")
	if err != nil {
		t.Fatalf("computeRevenueByAxis: %v", err)
	}
	if !res.Normalized {
		t.Errorf("want normalized=true after seeding crosswalk + tier_attributes")
	}
	if res.Warning != "" {
		t.Errorf("want no warning when normalized, got: %s", res.Warning)
	}
	if len(res.Rows) < 2 {
		t.Fatalf("want >=2 axis rows (ga + vip), got %d: %+v", len(res.Rows), res.Rows)
	}
	byAxis := map[string]revenueByAxisRow{}
	for _, r := range res.Rows {
		byAxis[r.AxisValue] = r
	}
	// GA: 2 tickets x 25.00 = 50.00
	ga, ok := byAxis["ga"]
	if !ok {
		t.Fatalf("no 'ga' row in result: %+v", byAxis)
	}
	if ga.TicketCount != 2 {
		t.Errorf("ga ticket_count = %d, want 2", ga.TicketCount)
	}
	if ga.TotalRevenue != 50.00 {
		t.Errorf("ga total_revenue = %v, want 50.00", ga.TotalRevenue)
	}
	// VIP: 1 ticket x 75.00 = 75.00
	vip, ok := byAxis["vip"]
	if !ok {
		t.Fatalf("no 'vip' row in result: %+v", byAxis)
	}
	if vip.TicketCount != 1 {
		t.Errorf("vip ticket_count = %d, want 1", vip.TicketCount)
	}
	if vip.TotalRevenue != 75.00 {
		t.Errorf("vip total_revenue = %v, want 75.00", vip.TotalRevenue)
	}
}
