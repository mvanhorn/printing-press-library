// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// TDD tests for computeRevenueByAxisScoped — date/event-scoped axis grouping
// via the order→tickets join path — and for the (not applicable) bucket split
// applied to both the scoped and unscoped paths.
//
// All fixtures are synthetic (IETF example.com, fabricated IDs, no real names).
package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// --- fixture helpers ---

// orderWithTickets builds an orders fixture payload that includes the nested
// tickets array required by the enriched orderSelection.
func orderWithTickets(id, purchasedAt, eventID, eventName string, totalCents int64, tickets []orderTicketFixture) string {
	type ticketTypeF struct {
		Name string `json:"name"`
	}
	type priceTierF struct {
		Name string `json:"name"`
	}
	type ticketF struct {
		ID         string      `json:"id"`
		Total      int64       `json:"total"`
		TicketType ticketTypeF `json:"ticketType"`
		PriceTier  priceTierF  `json:"priceTier"`
	}
	type eventF struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type orderF struct {
		ID          string    `json:"id"`
		PurchasedAt string    `json:"purchasedAt"`
		Total       int64     `json:"total"`
		Event       eventF    `json:"event"`
		Tickets     []ticketF `json:"tickets"`
	}
	tks := make([]ticketF, len(tickets))
	for i, tk := range tickets {
		tks[i] = ticketF{
			ID:         tk.ID,
			Total:      tk.TotalCents,
			TicketType: ticketTypeF{Name: tk.TicketTypeName},
			PriceTier:  priceTierF{Name: tk.PriceTierName},
		}
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

type orderTicketFixture struct {
	ID             string
	TotalCents     int64
	TicketTypeName string
	PriceTierName  string
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
// event's startDatetime falls in the requested window are included.
func TestComputeRevenueByAxisScopedDateWindow(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-jan": eventJSON("evt-jan", "Jan Show", "2026-01-15T20:00:00Z"),
			"evt-mar": eventJSON("evt-mar", "Mar Show", "2026-03-20T20:00:00Z"),
		},
		"orders": {
			"ord-jan": orderWithTickets("ord-jan", "2026-01-14T10:00:00Z", "evt-jan", "Jan Show", 5000,
				[]orderTicketFixture{
					{ID: "tk-jan-1", TotalCents: 2500, TicketTypeName: "General Admission", PriceTierName: "standard"},
					{ID: "tk-jan-2", TotalCents: 2500, TicketTypeName: "General Admission", PriceTierName: "standard"},
				}),
			"ord-mar": orderWithTickets("ord-mar", "2026-03-19T10:00:00Z", "evt-mar", "Mar Show", 7500,
				[]orderTicketFixture{
					{ID: "tk-mar-1", TotalCents: 7500, TicketTypeName: "VIP Experience", PriceTierName: "vip"},
				}),
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
// restricts to a single event's orders.
func TestComputeRevenueByAxisScopedEventFilter(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-a": eventJSON("evt-a", "Event A", "2026-02-10T20:00:00Z"),
			"evt-b": eventJSON("evt-b", "Event B", "2026-02-20T20:00:00Z"),
		},
		"orders": {
			"ord-a": orderWithTickets("ord-a", "2026-02-09T10:00:00Z", "evt-a", "Event A", 3000,
				[]orderTicketFixture{
					{ID: "tk-a-1", TotalCents: 3000, TicketTypeName: "General Admission", PriceTierName: "standard"},
				}),
			"ord-b": orderWithTickets("ord-b", "2026-02-19T10:00:00Z", "evt-b", "Event B", 9000,
				[]orderTicketFixture{
					{ID: "tk-b-1", TotalCents: 9000, TicketTypeName: "VIP Experience", PriceTierName: "vip"},
				}),
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
// an order that contains tickets of different types (mixed order).
func TestComputeRevenueByAxisScopedMixedOrder(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-mix": eventJSON("evt-mix", "Mixed Show", "2026-04-05T20:00:00Z"),
		},
		"orders": {
			"ord-mix": orderWithTickets("ord-mix", "2026-04-04T10:00:00Z", "evt-mix", "Mixed Show", 10000,
				[]orderTicketFixture{
					{ID: "tk-mix-1", TotalCents: 2500, TicketTypeName: "General Admission", PriceTierName: "standard"},
					{ID: "tk-mix-2", TotalCents: 7500, TicketTypeName: "VIP Experience", PriceTierName: "vip"},
				}),
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
		t.Errorf("ga total_revenue = %v, want 25.00", ga.TotalRevenue)
	}
	vip, ok := byAxis["vip"]
	if !ok {
		t.Fatalf("want 'vip' row in mixed-order result, keys: %v", axisKeys(byAxis))
	}
	if vip.TicketCount != 1 {
		t.Errorf("vip ticket_count = %d, want 1", vip.TicketCount)
	}
	if vip.TotalRevenue != 75.00 {
		t.Errorf("vip total_revenue = %v, want 75.00", vip.TotalRevenue)
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

// TestComputeRevenueByAxisScopedFallbackUnnormalized verifies Normalized=false
// when no crosswalk rows exist — the scoped path must also fall back gracefully.
func TestComputeRevenueByAxisScopedFallbackUnnormalized(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-fn": eventJSON("evt-fn", "Fallback Show", "2026-05-01T20:00:00Z"),
		},
		"orders": {
			"ord-fn": orderWithTickets("ord-fn", "2026-04-30T10:00:00Z", "evt-fn", "Fallback Show", 5000,
				[]orderTicketFixture{
					{ID: "tk-fn-1", TotalCents: 5000, TicketTypeName: "General Admission", PriceTierName: "standard"},
				}),
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
	// Even on fallback, tickets are returned with raw name grouping.
	if len(res.Rows) == 0 {
		t.Errorf("want at least one fallback row")
	}
}

// --- Bucket split tests (not applicable vs unclassified) ---

// TestBucketSplitNotApplicableVsUnclassifiedScoped verifies the two-bucket
// split on the scoped path: a ticket type in the crosswalk but with NULL axis
// value lands in "(not applicable)"; a ticket type with no crosswalk row lands
// in "(unclassified)". No revenue is dropped.
func TestBucketSplitNotApplicableVsUnclassifiedScoped(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-split": eventJSON("evt-split", "Split Show", "2026-06-10T20:00:00Z"),
		},
		"orders": {
			"ord-split": orderWithTickets("ord-split", "2026-06-09T10:00:00Z", "evt-split", "Split Show", 9000,
				[]orderTicketFixture{
					// "GA" -> crosswalk row exists, access_class="ga"
					{ID: "tk-s-1", TotalCents: 3000, TicketTypeName: "General Admission", PriceTierName: "std"},
					// "Staff" -> crosswalk row exists but access_class=NULL -> (not applicable)
					{ID: "tk-s-2", TotalCents: 0, TicketTypeName: "Staff Comp", PriceTierName: "comp"},
					// "Mystery" -> no crosswalk row -> (unclassified)
					{ID: "tk-s-3", TotalCents: 6000, TicketTypeName: "Mystery Tier", PriceTierName: "??"},
				}),
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
// covers a wide range.
func TestComputeRevenueByAxisScopedDateExclusion(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-in":  eventJSON("evt-in", "In Window Show", "2026-08-15T20:00:00Z"),
			"evt-out": eventJSON("evt-out", "Out of Window Show", "2026-09-15T20:00:00Z"),
		},
		"orders": {
			"ord-in": orderWithTickets("ord-in", "2026-08-14T10:00:00Z", "evt-in", "In Window Show", 4000,
				[]orderTicketFixture{
					{ID: "tk-in-1", TotalCents: 4000, TicketTypeName: "General Admission", PriceTierName: "std"},
				}),
			"ord-out": orderWithTickets("ord-out", "2026-09-14T10:00:00Z", "evt-out", "Out of Window Show", 8000,
				[]orderTicketFixture{
					{ID: "tk-out-1", TotalCents: 8000, TicketTypeName: "General Admission", PriceTierName: "std"},
				}),
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
	// Only the August order should count: 1 ticket x $40.
	if ga.TicketCount != 1 {
		t.Errorf("ga ticket_count = %d, want 1 (only in-window order)", ga.TicketCount)
	}
	if ga.TotalRevenue != 40.00 {
		t.Errorf("ga total_revenue = %v, want 40.00", ga.TotalRevenue)
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
