// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// TDD tests for computeRevenueByAxisScoped — date/event-scoped axis grouping
// via the order→ticket-ID→local-tickets-table join path — and for the
// (not applicable) bucket split applied to both the scoped and unscoped paths.
//
// All fixtures are synthetic (IETF example.com, fabricated IDs, no real names).
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// --- fixture helpers ---

// ticketJSON builds a tickets-table fixture payload with the fields read by
// the local ticket map: ticketType.name (for axis resolution) and total (cents,
// for revenue attribution). id must match the ID stored in the resources table.
func ticketJSON(id, typeName string, totalCents int64) string {
	type ticketTypeF struct {
		Name string `json:"name"`
	}
	type ticketF struct {
		ID         string      `json:"id"`
		Total      int64       `json:"total"`
		TicketType ticketTypeF `json:"ticketType"`
	}
	b, _ := json.Marshal(ticketF{
		ID:         id,
		Total:      totalCents,
		TicketType: ticketTypeF{Name: typeName},
	})
	return string(b)
}

// orderWithTicketIDs builds an orders fixture payload that includes only ticket
// IDs in the nested tickets array, matching the lean orderSelectionWithTickets
// which fetches tickets { id } only.
func orderWithTicketIDs(id, purchasedAt, eventID, eventName string, totalCents int64, ticketIDs []string) string {
	type ticketIDRef struct {
		ID string `json:"id"`
	}
	type eventF struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type orderF struct {
		ID          string        `json:"id"`
		PurchasedAt string        `json:"purchasedAt"`
		Total       int64         `json:"total"`
		Event       eventF        `json:"event"`
		Tickets     []ticketIDRef `json:"tickets"`
	}
	tks := make([]ticketIDRef, len(ticketIDs))
	for i, tid := range ticketIDs {
		tks[i] = ticketIDRef{ID: tid}
	}
	o := orderF{
		ID:          id,
		PurchasedAt: purchasedAt,
		Total:       totalCents,
		Event:       eventF{ID: eventID, Name: eventName},
		Tickets:     tks,
	}
	b, _ := json.Marshal(o)
	return string(b)
}

// adjustmentRef describes a single Order.adjustments entry for a fixture: the
// ticket ID the adjustment applies to and the promoter-side delta in cents
// (negative for a refund/reduction).
type adjustmentRef struct {
	ticketID      string
	promoterCents int64
}

// orderWithTicketsAndAdjustments builds an orders fixture payload with both
// nested ticket IDs and an adjustments array, matching the base orderSelection
// (which now captures adjustments) plus the enriched tickets { id }.
func orderWithTicketsAndAdjustments(id, purchasedAt, eventID, eventName string, totalCents int64, ticketIDs []string, adjustments []adjustmentRef) string {
	type ticketIDRef struct {
		ID string `json:"id"`
	}
	type feesChangeF struct {
		Category string `json:"category"`
		Dice     int64  `json:"dice"`
		Promoter int64  `json:"promoter"`
	}
	type ticketRefF struct {
		ID string `json:"id"`
	}
	type adjustmentF struct {
		FeesChange  feesChangeF `json:"feesChange"`
		ProcessedAt string      `json:"processedAt"`
		Reason      string      `json:"reason"`
		Ticket      ticketRefF  `json:"ticket"`
	}
	type eventF struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type orderF struct {
		ID          string        `json:"id"`
		PurchasedAt string        `json:"purchasedAt"`
		Total       int64         `json:"total"`
		Event       eventF        `json:"event"`
		Tickets     []ticketIDRef `json:"tickets"`
		Adjustments []adjustmentF `json:"adjustments"`
	}
	tks := make([]ticketIDRef, len(ticketIDs))
	for i, tid := range ticketIDs {
		tks[i] = ticketIDRef{ID: tid}
	}
	adjs := make([]adjustmentF, len(adjustments))
	for i, a := range adjustments {
		adjs[i] = adjustmentF{
			FeesChange:  feesChangeF{Category: "REFUND", Promoter: a.promoterCents},
			ProcessedAt: purchasedAt,
			Reason:      "synthetic adjustment",
			Ticket:      ticketRefF{ID: a.ticketID},
		}
	}
	o := orderF{
		ID:          id,
		PurchasedAt: purchasedAt,
		Total:       totalCents,
		Event:       eventF{ID: eventID, Name: eventName},
		Tickets:     tks,
		Adjustments: adjs,
	}
	b, _ := json.Marshal(o)
	return string(b)
}

// eventJSON builds an events fixture payload with a startDatetime.
func eventJSON(id, name, startDatetime string) string {
	e := storeEvent{ID: id, Name: name, StartDatetime: startDatetime}
	b, _ := json.Marshal(e)
	return string(b)
}

// seedCrosswalkAndTiers seeds the crosswalk + tier_attributes rows for a given
// ticketType name -> access_class mapping. Other axis columns are left NULL.
func seedCrosswalkAndTiers(t *testing.T, s *store.Store, ticketTypeName, accessClass string) {
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
		ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert tier attrs %q: %v", ticketTypeName, err)
	}
}

// seedCrosswalkNoTierAttr seeds a crosswalk row for the given ticket type but
// intentionally omits tier_attributes so the axis column will be NULL —
// testing the "(not applicable)" bucket.
func seedCrosswalkNoTierAttr(t *testing.T, s *store.Store, ticketTypeName string) {
	t.Helper()
	cid := mintCanonicalID("ticket_type", ticketTypeName)
	if err := s.UpsertCrosswalk(store.CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice",
		SourceValue: ticketTypeName, CanonicalID: cid,
		Method: "regex", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert crosswalk (no tier attr) %q: %v", ticketTypeName, err)
	}
	// Seed a tier_attributes row but leave access_class empty (NULL) to trigger
	// the "(not applicable)" bucket.
	if err := s.UpsertTierAttributes(cid, store.TierAttributesRow{
		CanonicalID: cid, AccessClass: "", // explicitly empty
		ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert tier attrs (empty access_class) %q: %v", ticketTypeName, err)
	}
}

// --- tests ---

// TestRevenueSummaryByAxisAcceptsFilters verifies the previous filter-rejection
// error is GONE — combining --by-axis with --event/--from/--to must succeed (no
// error from the router), routing to the scoped path. The store is empty so no
// revenue rows are expected, but the command must not return an error.
func TestRevenueSummaryByAxisAcceptsFilters(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"by-axis + event", []string{"revenue", "summary", "--by-axis", "access_class", "--event", "evt-123"}},
		{"by-axis + from", []string{"revenue", "summary", "--by-axis", "access_class", "--from", "2026-01-01"}},
		{"by-axis + to", []string{"revenue", "summary", "--by-axis", "access_class", "--to", "2026-12-31"}},
		{"by-axis + from + to", []string{"revenue", "summary", "--by-axis", "access_class", "--from", "2026-01-01", "--to", "2026-12-31"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Isolate from the operator's real default-path store: the --dry-run
			// flag is never actually passed in tc.args, so without a $HOME
			// override the command resolves openStoreForRead -> defaultDBPath ->
			// the real ~/.local/share store. A temp $HOME yields no data.db, so
			// openStoreForRead returns (nil, nil) and the command exercises the
			// scoped by-axis routing path without touching real data.
			t.Setenv("HOME", t.TempDir())
			flags := &rootFlags{dryRun: true} // dry-run: don't need a real store
			root := newRootCmd(flags)
			root.SetArgs(tc.args)
			err := root.Execute()
			if err != nil {
				t.Errorf("want no error for %v, got: %v", tc.args, err)
			}
		})
	}
}

// TestComputeRevenueByAxisScopedDateWindow verifies that only orders whose
// event's startDatetime falls in the requested window are included. Tickets are
// seeded in the local tickets table; orders carry only ticket IDs.
func TestComputeRevenueByAxisScopedDateWindow(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-jan": eventJSON("evt-jan", "Jan Show", "2026-01-15T20:00:00Z"),
			"evt-mar": eventJSON("evt-mar", "Mar Show", "2026-03-20T20:00:00Z"),
		},
		"orders": {
			"ord-jan": orderWithTicketIDs("ord-jan", "2026-01-14T10:00:00Z", "evt-jan", "Jan Show", 5000,
				[]string{"tk-jan-1", "tk-jan-2"}),
			"ord-mar": orderWithTicketIDs("ord-mar", "2026-03-19T10:00:00Z", "evt-mar", "Mar Show", 7500,
				[]string{"tk-mar-1"}),
		},
		"tickets": {
			"tk-jan-1": ticketJSON("tk-jan-1", "General Admission", 2500),
			"tk-jan-2": ticketJSON("tk-jan-2", "General Admission", 2500),
			"tk-mar-1": ticketJSON("tk-mar-1", "VIP Experience", 7500),
		},
	})
	seedCrosswalkAndTiers(t, s, "General Admission", "ga")
	seedCrosswalkAndTiers(t, s, "VIP Experience", "vip")

	// Only January: from=2026-01-01 to=2026-01-31 — only evt-jan qualifies.
	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "", "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	if !res.Normalized {
		t.Errorf("want normalized=true")
	}
	byAxis := axisRowsByValue(res.Rows)
	// ga: 2 tickets x 25.00 = 50.00
	ga, ok := byAxis["ga"]
	if !ok {
		t.Fatalf("want 'ga' row, keys: %v", axisKeys(byAxis))
	}
	if ga.TicketCount != 2 {
		t.Errorf("ga ticket_count = %d, want 2", ga.TicketCount)
	}
	if ga.TotalRevenue != 50.00 {
		t.Errorf("ga total_revenue = %v, want 50.00", ga.TotalRevenue)
	}
	// vip must NOT appear (Mar Show is outside the Jan window).
	if _, found := byAxis["vip"]; found {
		t.Errorf("vip row should NOT appear for a Jan-only filter")
	}
}

// TestComputeRevenueByAxisScopedEventFilter verifies that the --event filter
// restricts to a single event's orders. Tickets are seeded in the local tickets
// table; orders carry only ticket IDs.
func TestComputeRevenueByAxisScopedEventFilter(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-a": eventJSON("evt-a", "Event A", "2026-02-10T20:00:00Z"),
			"evt-b": eventJSON("evt-b", "Event B", "2026-02-20T20:00:00Z"),
		},
		"orders": {
			"ord-a": orderWithTicketIDs("ord-a", "2026-02-09T10:00:00Z", "evt-a", "Event A", 3000,
				[]string{"tk-a-1"}),
			"ord-b": orderWithTicketIDs("ord-b", "2026-02-19T10:00:00Z", "evt-b", "Event B", 9000,
				[]string{"tk-b-1"}),
		},
		"tickets": {
			"tk-a-1": ticketJSON("tk-a-1", "General Admission", 3000),
			"tk-b-1": ticketJSON("tk-b-1", "VIP Experience", 9000),
		},
	})
	seedCrosswalkAndTiers(t, s, "General Admission", "ga")
	seedCrosswalkAndTiers(t, s, "VIP Experience", "vip")

	// Filter to evt-a only.
	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "evt-a", "", "")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	byAxis := axisRowsByValue(res.Rows)
	ga, ok := byAxis["ga"]
	if !ok {
		t.Fatalf("want 'ga' row for evt-a filter, keys: %v", axisKeys(byAxis))
	}
	if ga.TicketCount != 1 {
		t.Errorf("ga ticket_count = %d, want 1", ga.TicketCount)
	}
	if ga.TotalRevenue != 30.00 {
		t.Errorf("ga total_revenue = %v, want 30.00", ga.TotalRevenue)
	}
	if _, found := byAxis["vip"]; found {
		t.Errorf("vip row should NOT appear for evt-a-only filter")
	}
}

// TestComputeRevenueByAxisScopedMixedOrder verifies per-ticket attribution for
// an order that contains tickets of different types (mixed order). Type names
// and totals come from the local tickets table; the order carries IDs only.
func TestComputeRevenueByAxisScopedMixedOrder(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-mix": eventJSON("evt-mix", "Mixed Show", "2026-04-05T20:00:00Z"),
		},
		"orders": {
			"ord-mix": orderWithTicketIDs("ord-mix", "2026-04-04T10:00:00Z", "evt-mix", "Mixed Show", 10000,
				[]string{"tk-mix-1", "tk-mix-2"}),
		},
		"tickets": {
			"tk-mix-1": ticketJSON("tk-mix-1", "General Admission", 2500),
			"tk-mix-2": ticketJSON("tk-mix-2", "VIP Experience", 7500),
		},
	})
	seedCrosswalkAndTiers(t, s, "General Admission", "ga")
	seedCrosswalkAndTiers(t, s, "VIP Experience", "vip")

	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "evt-mix", "", "")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	byAxis := axisRowsByValue(res.Rows)
	ga, ok := byAxis["ga"]
	if !ok {
		t.Fatalf("want 'ga' row in mixed-order result, keys: %v", axisKeys(byAxis))
	}
	if ga.TicketCount != 1 {
		t.Errorf("ga ticket_count = %d, want 1 (per-ticket attribution)", ga.TicketCount)
	}
	if ga.TotalRevenue != 25.00 {
		t.Errorf("ga total_revenue = %v, want 25.00 (from local tickets table)", ga.TotalRevenue)
	}
	vip, ok := byAxis["vip"]
	if !ok {
		t.Fatalf("want 'vip' row in mixed-order result, keys: %v", axisKeys(byAxis))
	}
	if vip.TicketCount != 1 {
		t.Errorf("vip ticket_count = %d, want 1", vip.TicketCount)
	}
	if vip.TotalRevenue != 75.00 {
		t.Errorf("vip total_revenue = %v, want 75.00 (from local tickets table)", vip.TotalRevenue)
	}
	// Total revenue must be sum of per-ticket totals — no revenue dropped.
	var totalRev float64
	for _, r := range res.Rows {
		totalRev += r.TotalRevenue
	}
	if totalRev != 100.00 {
		t.Errorf("total revenue across all buckets = %v, want 100.00 (no revenue dropped)", totalRev)
	}
}

// TestComputeRevenueByAxisScopedMissingLocalTicket verifies that a ticket ID
// referenced in an order but absent from the local tickets table is counted in
// "(unclassified)" with $0 revenue, and does not cause an error or double-count.
func TestComputeRevenueByAxisScopedMissingLocalTicket(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-miss": eventJSON("evt-miss", "Miss Show", "2026-04-10T20:00:00Z"),
		},
		"orders": {
			// Order references two ticket IDs; only tk-present is seeded locally.
			"ord-miss": orderWithTicketIDs("ord-miss", "2026-04-09T10:00:00Z", "evt-miss", "Miss Show", 5000,
				[]string{"tk-present", "tk-missing"}),
		},
		"tickets": {
			"tk-present": ticketJSON("tk-present", "General Admission", 3000),
			// "tk-missing" is intentionally absent from the tickets table.
		},
	})
	seedCrosswalkAndTiers(t, s, "General Admission", "ga")

	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "evt-miss", "", "")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	byAxis := axisRowsByValue(res.Rows)

	// The present ticket resolves to "ga" with its local total.
	ga, ok := byAxis["ga"]
	if !ok {
		t.Fatalf("want 'ga' row for the present ticket, keys: %v", axisKeys(byAxis))
	}
	if ga.TicketCount != 1 {
		t.Errorf("ga ticket_count = %d, want 1", ga.TicketCount)
	}
	if ga.TotalRevenue != 30.00 {
		t.Errorf("ga total_revenue = %v, want 30.00", ga.TotalRevenue)
	}

	// The missing ticket is counted in "(unclassified)" with $0 revenue.
	unc, ok := byAxis["(unclassified)"]
	if !ok {
		t.Fatalf("want '(unclassified)' row for the missing ticket, keys: %v", axisKeys(byAxis))
	}
	if unc.TicketCount != 1 {
		t.Errorf("(unclassified) ticket_count = %d, want 1", unc.TicketCount)
	}
	if unc.TotalRevenue != 0.00 {
		t.Errorf("(unclassified) total_revenue = %v, want 0.00 (no total for missing ticket)", unc.TotalRevenue)
	}
}

// TestComputeRevenueByAxisScopedFallbackUnnormalized verifies Normalized=false
// when no crosswalk rows exist — the scoped path must also fall back gracefully,
// reading type names and totals from the local tickets table.
func TestComputeRevenueByAxisScopedFallbackUnnormalized(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-fn": eventJSON("evt-fn", "Fallback Show", "2026-05-01T20:00:00Z"),
		},
		"orders": {
			"ord-fn": orderWithTicketIDs("ord-fn", "2026-04-30T10:00:00Z", "evt-fn", "Fallback Show", 5000,
				[]string{"tk-fn-1"}),
		},
		"tickets": {
			"tk-fn-1": ticketJSON("tk-fn-1", "General Admission", 5000),
		},
	})
	// No crosswalk/tier_attributes seeded.
	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "evt-fn", "", "")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	if res.Normalized {
		t.Errorf("want normalized=false when no crosswalk rows exist")
	}
	if res.Warning == "" {
		t.Errorf("want non-empty warning on fallback")
	}
	// Even on fallback, tickets are returned with raw name grouping from the
	// local tickets table.
	if len(res.Rows) == 0 {
		t.Errorf("want at least one fallback row")
	}
}

// TestComputeRevenueByAxisScopedWarnsWhenOrdersLackTickets verifies that when
// orders in scope were synced without --order-tickets (no nested ticket IDs),
// the scoped path returns a warning rather than an empty result with no
// explanation.
func TestComputeRevenueByAxisScopedWarnsWhenOrdersLackTickets(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-lean": eventJSON("evt-lean", "Lean Show", "2026-06-15T20:00:00Z"),
		},
		"orders": {
			// Order with no ticket IDs — synced without --order-tickets.
			"ord-lean": orderWithTicketIDs("ord-lean", "2026-06-14T10:00:00Z", "evt-lean", "Lean Show", 5000, nil),
		},
	})
	seedCrosswalkAndTiers(t, s, "General Admission", "ga") // crosswalk populated

	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "evt-lean", "", "")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	if res.Warning == "" || !strings.Contains(res.Warning, "order-tickets") {
		t.Errorf("want a warning mentioning --order-tickets, got %q", res.Warning)
	}
	if len(res.Rows) != 0 {
		t.Errorf("want no rows when orders in scope lack tickets, got %d", len(res.Rows))
	}
}

// --- Bucket split tests (not applicable vs unclassified) ---

// TestBucketSplitNotApplicableVsUnclassifiedScoped verifies the two-bucket
// split on the scoped path: a ticket type in the crosswalk but with NULL axis
// value lands in "(not applicable)"; a ticket type with no crosswalk row lands
// in "(unclassified)". Type names and totals come from the local tickets table;
// orders carry IDs only. No revenue is dropped.
func TestBucketSplitNotApplicableVsUnclassifiedScoped(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-split": eventJSON("evt-split", "Split Show", "2026-06-10T20:00:00Z"),
		},
		"orders": {
			"ord-split": orderWithTicketIDs("ord-split", "2026-06-09T10:00:00Z", "evt-split", "Split Show", 9000,
				[]string{"tk-s-1", "tk-s-2", "tk-s-3"}),
		},
		"tickets": {
			// "GA" -> crosswalk row exists, access_class="ga"
			"tk-s-1": ticketJSON("tk-s-1", "General Admission", 3000),
			// "Staff Comp" -> crosswalk row exists but access_class=NULL -> (not applicable)
			"tk-s-2": ticketJSON("tk-s-2", "Staff Comp", 0),
			// "Mystery Tier" -> no crosswalk row -> (unclassified)
			"tk-s-3": ticketJSON("tk-s-3", "Mystery Tier", 6000),
		},
	})

	seedCrosswalkAndTiers(t, s, "General Admission", "ga")
	seedCrosswalkNoTierAttr(t, s, "Staff Comp")
	// "Mystery Tier" intentionally has no crosswalk row.

	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "evt-split", "", "")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	if !res.Normalized {
		t.Errorf("want normalized=true (GA and Staff Comp are in crosswalk)")
	}
	byAxis := axisRowsByValue(res.Rows)

	// "ga" bucket: 1 ticket x $30
	ga, ok := byAxis["ga"]
	if !ok {
		t.Fatalf("want 'ga' row, keys: %v", axisKeys(byAxis))
	}
	if ga.TicketCount != 1 || ga.TotalRevenue != 30.00 {
		t.Errorf("ga: count=%d revenue=%v, want count=1 revenue=30.00", ga.TicketCount, ga.TotalRevenue)
	}

	// "(not applicable)" bucket: Staff Comp was matched but has no access_class value.
	na, ok := byAxis["(not applicable)"]
	if !ok {
		t.Fatalf("want '(not applicable)' row, keys: %v", axisKeys(byAxis))
	}
	if na.TicketCount != 1 {
		t.Errorf("(not applicable) count = %d, want 1", na.TicketCount)
	}

	// "(unclassified)" bucket: Mystery Tier has no crosswalk row.
	unc, ok := byAxis["(unclassified)"]
	if !ok {
		t.Fatalf("want '(unclassified)' row, keys: %v", axisKeys(byAxis))
	}
	if unc.TicketCount != 1 || unc.TotalRevenue != 60.00 {
		t.Errorf("(unclassified): count=%d revenue=%v, want count=1 revenue=60.00", unc.TicketCount, unc.TotalRevenue)
	}

	// Total revenue: 30 + 0 + 60 = 90.00 — no revenue dropped.
	var totalRev float64
	for _, r := range res.Rows {
		totalRev += r.TotalRevenue
	}
	if totalRev != 90.00 {
		t.Errorf("total revenue = %v, want 90.00 (no revenue dropped)", totalRev)
	}
}

// TestBucketSplitNotApplicableVsUnclassifiedUnscoped verifies the same two-bucket
// split on the EXISTING unscoped path (computeRevenueByAxis via tickets table).
// This ensures the tickets-table SQL path was updated in lockstep.
func TestBucketSplitNotApplicableVsUnclassifiedUnscoped(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"t-ga":    `{"id":"t-ga","ticketType":{"name":"General Admission","price":3000}}`,
			"t-staff": `{"id":"t-staff","ticketType":{"name":"Staff Comp","price":0}}`,
			"t-myst":  `{"id":"t-myst","ticketType":{"name":"Mystery Tier","price":6000}}`,
		},
	})
	seedCrosswalkAndTiers(t, s, "General Admission", "ga")
	seedCrosswalkNoTierAttr(t, s, "Staff Comp")
	// "Mystery Tier" has no crosswalk row -> "(unclassified)".

	res, err := computeRevenueByAxis(context.Background(), s.DB(), "access_class")
	if err != nil {
		t.Fatalf("computeRevenueByAxis: %v", err)
	}
	if !res.Normalized {
		t.Errorf("want normalized=true (some crosswalk rows exist)")
	}
	byAxis := axisRowsByValue(res.Rows)

	if _, ok := byAxis["(not applicable)"]; !ok {
		t.Fatalf("want '(not applicable)' row in unscoped path, keys: %v", axisKeys(byAxis))
	}
	if _, ok := byAxis["(unclassified)"]; !ok {
		t.Fatalf("want '(unclassified)' row in unscoped path, keys: %v", axisKeys(byAxis))
	}
	if _, ok := byAxis["ga"]; !ok {
		t.Fatalf("want 'ga' row in unscoped path, keys: %v", axisKeys(byAxis))
	}
}

// TestComputeRevenueByAxisScopedNoOrders verifies that the scoped path returns
// empty rows (not an error) when no orders match the filter.
func TestComputeRevenueByAxisScopedNoOrders(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-empty": eventJSON("evt-empty", "Empty Show", "2026-07-01T20:00:00Z"),
		},
	})
	// No orders seeded.
	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "evt-empty", "", "")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	if len(res.Rows) != 0 {
		t.Errorf("want 0 rows when no orders match, got %d", len(res.Rows))
	}
}

// TestComputeRevenueByAxisScopedDateExclusion verifies that an order whose
// event falls outside the date window is excluded even if the filter nominally
// covers a wide range. Tickets are seeded in the local tickets table; orders
// carry only IDs.
func TestComputeRevenueByAxisScopedDateExclusion(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-in":  eventJSON("evt-in", "In Window Show", "2026-08-15T20:00:00Z"),
			"evt-out": eventJSON("evt-out", "Out of Window Show", "2026-09-15T20:00:00Z"),
		},
		"orders": {
			"ord-in": orderWithTicketIDs("ord-in", "2026-08-14T10:00:00Z", "evt-in", "In Window Show", 4000,
				[]string{"tk-in-1"}),
			"ord-out": orderWithTicketIDs("ord-out", "2026-09-14T10:00:00Z", "evt-out", "Out of Window Show", 8000,
				[]string{"tk-out-1"}),
		},
		"tickets": {
			"tk-in-1":  ticketJSON("tk-in-1", "General Admission", 4000),
			"tk-out-1": ticketJSON("tk-out-1", "General Admission", 8000),
		},
	})
	seedCrosswalkAndTiers(t, s, "General Admission", "ga")

	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "", "2026-08-01", "2026-08-31")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	byAxis := axisRowsByValue(res.Rows)
	ga, ok := byAxis["ga"]
	if !ok {
		t.Fatalf("want 'ga' row for in-window order, keys: %v", axisKeys(byAxis))
	}
	// Only the August order should count: 1 ticket x $40 (from local tickets table).
	if ga.TicketCount != 1 {
		t.Errorf("ga ticket_count = %d, want 1 (only in-window order)", ga.TicketCount)
	}
	if ga.TotalRevenue != 40.00 {
		t.Errorf("ga total_revenue = %v, want 40.00 (from local tickets table)", ga.TotalRevenue)
	}
}

// --- adjustment tests ---

// TestRevenueByAxisAdjustmentsAttributed verifies that a post-purchase
// adjustment is attributed to the same axis bucket as its ticket as a DISTINCT
// figure: AdjustmentCount / AdjustmentPromoter are populated, while the bucket's
// gross TotalRevenue is unchanged (the adjustment is NOT merged into gross).
func TestRevenueByAxisAdjustmentsAttributed(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-adj": eventJSON("evt-adj", "Adj Show", "2026-03-10T20:00:00Z"),
		},
		"orders": {
			// One VIP ticket priced 75.00; one adjustment of -5.00 (a refund)
			// on that ticket.
			"ord-adj": orderWithTicketsAndAdjustments(
				"ord-adj", "2026-03-09T10:00:00Z", "evt-adj", "Adj Show", 7500,
				[]string{"tk-adj-1"},
				[]adjustmentRef{{ticketID: "tk-adj-1", promoterCents: -500}},
			),
		},
		"tickets": {
			"tk-adj-1": ticketJSON("tk-adj-1", "VIP Experience", 7500),
		},
	})
	seedCrosswalkAndTiers(t, s, "VIP Experience", "vip")

	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "evt-adj", "", "")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}
	if !res.Normalized {
		t.Errorf("want normalized=true")
	}
	byAxis := axisRowsByValue(res.Rows)
	vip, ok := byAxis["vip"]
	if !ok {
		t.Fatalf("want 'vip' row, keys: %v", axisKeys(byAxis))
	}
	// Gross is UNCHANGED — the adjustment is not folded into gross.
	if vip.TotalRevenue != 75.00 {
		t.Errorf("vip total_revenue = %v, want 75.00 (gross unchanged by adjustment)", vip.TotalRevenue)
	}
	if vip.TicketCount != 1 {
		t.Errorf("vip ticket_count = %d, want 1", vip.TicketCount)
	}
	// Adjustment is a distinct figure on the same bucket.
	if vip.AdjustmentCount != 1 {
		t.Errorf("vip adjustment_count = %d, want 1", vip.AdjustmentCount)
	}
	if vip.AdjustmentPromoter != -5.00 {
		t.Errorf("vip adjustment_promoter = %v, want -5.00", vip.AdjustmentPromoter)
	}
	// Top-level totals reflect the single adjustment.
	if res.TotalAdjustmentCount != 1 {
		t.Errorf("total_adjustment_count = %d, want 1", res.TotalAdjustmentCount)
	}
	if res.TotalAdjustmentPromoter != -5.00 {
		t.Errorf("total_adjustment_promoter = %v, want -5.00", res.TotalAdjustmentPromoter)
	}
}

// TestAdjustmentParsing verifies an order JSON payload with an adjustments array
// unmarshals into storeOrder with the right promoter cents and ticket ID.
func TestAdjustmentParsing(t *testing.T) {
	raw := `{
		"id": "ord-1",
		"total": 5000,
		"adjustments": [
			{
				"feesChange": {"category": "REFUND", "dice": -50, "promoter": -500},
				"processedAt": "2026-03-09T10:00:00Z",
				"reason": "partial refund",
				"ticket": {"id": "tk-1"}
			}
		]
	}`
	var o storeOrder
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatalf("unmarshal storeOrder: %v", err)
	}
	if len(o.Adjustments) != 1 {
		t.Fatalf("len(Adjustments) = %d, want 1", len(o.Adjustments))
	}
	adj := o.Adjustments[0]
	if adj.FeesChange.Promoter != -500 {
		t.Errorf("FeesChange.Promoter = %d, want -500", adj.FeesChange.Promoter)
	}
	if adj.FeesChange.Dice != -50 {
		t.Errorf("FeesChange.Dice = %d, want -50", adj.FeesChange.Dice)
	}
	if adj.FeesChange.Category != "REFUND" {
		t.Errorf("FeesChange.Category = %q, want REFUND", adj.FeesChange.Category)
	}
	if adj.Ticket.ID != "tk-1" {
		t.Errorf("Ticket.ID = %q, want tk-1", adj.Ticket.ID)
	}
	if adj.Reason != "partial refund" {
		t.Errorf("Reason = %q, want 'partial refund'", adj.Reason)
	}
}

// --- helper utilities ---

func axisRowsByValue(rows []revenueByAxisRow) map[string]revenueByAxisRow {
	m := make(map[string]revenueByAxisRow, len(rows))
	for _, r := range rows {
		m[r.AxisValue] = r
	}
	return m
}

func axisKeys(m map[string]revenueByAxisRow) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
