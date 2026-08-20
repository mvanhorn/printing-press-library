// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests for the hand-authored DICE GraphQL query layer.
package cli

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

// relayConnectionRe matches a GraphQL field that opens a Relay connection
// (`<field>(<args>) { edges` or `<field> { edges`). DICE rejects any connection
// lacking a first/last pagination arg with "You must either supply :first or
// :last", so every connection the query emits — including ones nested inside a
// node selection — must carry one.
var relayConnectionRe = regexp.MustCompile(`(\w+)\s*(\([^)]*\))?\s*\{\s*edges\b`)

// Every connection rendered by buildConnectionQuery, at any depth, must declare
// first/last. Guards against the genres regression where the nested
// genreType.genres connection shipped without pagination args.
func TestBuiltConnectionQueriesBoundEveryConnection(t *testing.T) {
	for name, cs := range diceConnections {
		// Both pagination directions: forward (first/after) and the latest-only
		// backward variant (last/before).
		for _, latest := range []bool{false, true} {
			query := buildConnectionQuery(cs, latest)
			for _, m := range relayConnectionRe.FindAllStringSubmatch(query, -1) {
				field, args := m[1], m[2]
				if !strings.Contains(args, "first") && !strings.Contains(args, "last") {
					t.Errorf("connection query %q (latest=%v): field %q opens a Relay connection without a first/last pagination arg (args %q); DICE will reject it", name, latest, field, args)
				}
			}
		}
	}
}

// The latest-only variant must page backward (last) so it returns the newest
// records, not page 1 (oldest) of an oldest-first connection.
func TestLatestQueryPagesBackward(t *testing.T) {
	q := buildConnectionQuery(diceConnections["orders"], true)
	if !strings.Contains(q, "last: $last") || !strings.Contains(q, "before: $before") {
		t.Errorf("latest-only orders query should page backward via last/before; got:\n%s", q)
	}
	if strings.Contains(q, "first: $first") {
		t.Errorf("latest-only query must not use forward first/after pagination; got:\n%s", q)
	}
}

// TestEventSelectionIncludesNestedPoolsAndLinks confirms the event node
// selection captures the ticketPools and socialLinks nested entities, which the
// store keeps as raw node JSON for capacity views and a future ticketPool
// normalize entity sourced from events.ticketPools[*].name.
func TestEventSelectionIncludesNestedPoolsAndLinks(t *testing.T) {
	for _, want := range []string{
		"ticketPools { id name allocation }",
		"socialLinks { campaign default url }",
	} {
		if !strings.Contains(eventSelection, want) {
			t.Errorf("eventSelection must contain %q; got:\n%s", want, eventSelection)
		}
	}
}

// --- order-tickets enrichment opt-in tests ---

// TestLeanOrderSelectionHasNoTickets confirms the base orderSelection (used for
// default sync without --order-tickets) does NOT include the nested tickets
// block. This is the cost-guard: operators who don't use revenue --by-axis
// scoped should not pay for the heavier payload.
func TestLeanOrderSelectionHasNoTickets(t *testing.T) {
	if strings.Contains(orderSelection, "tickets {") {
		t.Errorf("lean orderSelection must not contain a nested tickets block; got:\n%s", orderSelection)
	}
}

// TestEnrichedOrderSelectionHasTickets confirms orderSelectionWithTickets
// includes the lean nested tickets block (id only) needed for revenue --by-axis
// scoped. Type name and totals come from the local tickets table, not from
// fields nested on the order, so ticketType/priceTier sub-fields must NOT be
// present in the order-level selection.
func TestEnrichedOrderSelectionHasTickets(t *testing.T) {
	if !strings.Contains(orderSelectionWithTickets, "tickets {") {
		t.Errorf("orderSelectionWithTickets must contain a nested tickets block; got:\n%s", orderSelectionWithTickets)
	}
	// The enriched selection fetches ticket IDs only — no nested type/tier fields
	// on the order, since those are joined from the local tickets table.
	for _, absent := range []string{"ticketType { name }", "priceTier { name }"} {
		if strings.Contains(orderSelectionWithTickets, absent) {
			t.Errorf("orderSelectionWithTickets must NOT contain %q (use local tickets table join instead); got:\n%s", absent, orderSelectionWithTickets)
		}
	}
}

// TestEnrichedOrderSelectionIsSupersetOfLean confirms the enriched selection
// is a strict superset: it includes everything in the lean selection plus more.
func TestEnrichedOrderSelectionIsSupersetOfLean(t *testing.T) {
	// Every field token in the lean selection must appear in the enriched one.
	for _, token := range []string{"purchasedAt", "quantity", "salesChannel", "fullPrice", "diceCommission", "ipCity", "ipCountry"} {
		if !strings.Contains(orderSelectionWithTickets, token) {
			t.Errorf("orderSelectionWithTickets must contain lean field %q; it appears to be a replacement rather than a superset", token)
		}
	}
}

// TestEffectiveConnectionSpecLeanOrders confirms that effectiveConnectionSpec
// returns the lean orderSelection when enrichOrders is false, so a default
// sync does not fetch nested tickets.
func TestEffectiveConnectionSpecLeanOrders(t *testing.T) {
	cs, ok := effectiveConnectionSpec("orders", false)
	if !ok {
		t.Fatal("effectiveConnectionSpec returned ok=false for 'orders'")
	}
	if strings.Contains(cs.selection, "tickets {") {
		t.Errorf("orders spec with enrichOrders=false must use lean selection (no tickets block); got:\n%s", cs.selection)
	}
}

// TestEffectiveConnectionSpecEnrichedOrders confirms that effectiveConnectionSpec
// returns the enriched orderSelectionWithTickets when enrichOrders is true,
// enabling the scoped revenue --by-axis path.
func TestEffectiveConnectionSpecEnrichedOrders(t *testing.T) {
	cs, ok := effectiveConnectionSpec("orders", true)
	if !ok {
		t.Fatal("effectiveConnectionSpec returned ok=false for 'orders' with enrichOrders=true")
	}
	if !strings.Contains(cs.selection, "tickets {") {
		t.Errorf("orders spec with enrichOrders=true must use enriched selection (with tickets block); got:\n%s", cs.selection)
	}
}

// TestEffectiveConnectionSpecOtherResourcesUnaffected confirms that
// enrichOrders=true has no effect on non-orders resources (the flag is
// orders-only; all other connections keep their default selection).
func TestEffectiveConnectionSpecOtherResourcesUnaffected(t *testing.T) {
	for _, resource := range []string{"events", "tickets", "returns", "transfers", "extras", "genres"} {
		lean, _ := effectiveConnectionSpec(resource, false)
		enriched, _ := effectiveConnectionSpec(resource, true)
		if lean.selection != enriched.selection {
			t.Errorf("resource %q: selection changed between enrichOrders=false and true; enrichOrders must only affect orders", resource)
		}
	}
}

// TestOrderQueryIncludesTicketsWhenEnriched builds the full GraphQL query string
// through buildConnectionQuery and verifies that the tickets sub-selection
// appears in the query when enrichOrders=true but not when false.
func TestOrderQueryIncludesTicketsWhenEnriched(t *testing.T) {
	lean, _ := effectiveConnectionSpec("orders", false)
	enriched, _ := effectiveConnectionSpec("orders", true)

	leanQuery := buildConnectionQuery(lean, false)
	enrichedQuery := buildConnectionQuery(enriched, false)

	if strings.Contains(leanQuery, "tickets {") {
		t.Errorf("order query without enrichOrders should not contain 'tickets {'; got:\n%s", leanQuery)
	}
	if !strings.Contains(enrichedQuery, "tickets {") {
		t.Errorf("order query with enrichOrders=true should contain 'tickets {'; got:\n%s", enrichedQuery)
	}
}

// TestComputeRevenueByAxisScopedNoTicketsInOrders verifies that
// computeRevenueByAxisScoped degrades gracefully when orders were synced
// without --order-tickets (Tickets slice is empty / no ticket IDs). It must
// return a warning and empty rows rather than panicking or erroring.
func TestComputeRevenueByAxisScopedNoTicketsInOrders(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-lean": eventJSON("evt-lean", "Lean Show", "2026-03-15T20:00:00Z"),
		},
		"orders": {
			// orderWithTicketIDs with a nil slice simulates a lean-synced order
			// (no --order-tickets flag was passed during sync).
			"ord-lean": orderWithTicketIDs("ord-lean", "2026-03-01T10:00:00Z", "evt-lean", "Lean Show", 5000, nil),
		},
	})
	seedCrosswalkAndTiers(t, s, "General Admission", "ga")

	res, err := computeRevenueByAxisScoped(context.Background(), s.DB(), "access_class", "evt-lean", "", "")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped must not error on lean-synced (no tickets) orders: %v", err)
	}
	// No ticket IDs in the order → warning emitted, empty rows returned.
	if len(res.Rows) != 0 {
		t.Errorf("want 0 rows when orders have no nested ticket IDs (lean sync), got %d: %v", len(res.Rows), res.Rows)
	}
}
