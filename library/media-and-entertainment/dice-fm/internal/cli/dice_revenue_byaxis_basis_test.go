// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// TDD test for the revenue --by-axis monetary-basis unification: the unscoped
// path (computeRevenueByAxis -> groupTicketRevenueByAxis) must report the SAME
// money as the scoped path (computeRevenueByAxisScoped), i.e. paid ticket
// $.total, not list $.ticketType.price. Synthetic fixtures only.
package cli

import (
	"context"
	"encoding/json"
	"testing"
)

// ticketJSONWithPrice builds a tickets-table fixture carrying BOTH the list
// price (ticketType.price) and the paid total ($.total), set to DIFFERENT
// values so a path that sums the wrong field is detectable.
func ticketJSONWithPrice(id, typeName string, listPriceCents, paidTotalCents int64) string {
	type ticketTypeF struct {
		Name  string `json:"name"`
		Price int64  `json:"price"`
	}
	type ticketF struct {
		ID         string      `json:"id"`
		Total      int64       `json:"total"`
		TicketType ticketTypeF `json:"ticketType"`
	}
	b, _ := json.Marshal(ticketF{
		ID:         id,
		Total:      paidTotalCents,
		TicketType: ticketTypeF{Name: typeName, Price: listPriceCents},
	})
	return string(b)
}

// TestRevenueByAxisBasisMatchesScopedAndUnscoped seeds tickets whose list price
// differs from the paid total, then asserts the unscoped and scoped --by-axis
// paths return identical per-bucket total_revenue. Both must use paid total.
func TestRevenueByAxisBasisMatchesScopedAndUnscoped(t *testing.T) {
	// GA: list 3000c but paid 2500c (e.g. a discount); 2 tickets -> paid 50.00.
	// VIP: list 8000c but paid 7500c; 1 ticket -> paid 75.00.
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evt-1": eventJSON("evt-1", "Show 1", "2026-06-15T20:00:00Z"),
		},
		"tickets": {
			"tk-ga-1":  ticketJSONWithPrice("tk-ga-1", "General Admission", 3000, 2500),
			"tk-ga-2":  ticketJSONWithPrice("tk-ga-2", "General Admission", 3000, 2500),
			"tk-vip-1": ticketJSONWithPrice("tk-vip-1", "VIP Experience", 8000, 7500),
		},
		// Orders carry the ticket IDs so the scoped path can attribute them.
		"orders": {
			"ord-1": orderWithTicketIDs("ord-1", "2026-06-10T10:00:00Z", "evt-1", "Show 1", 12500,
				[]string{"tk-ga-1", "tk-ga-2", "tk-vip-1"}),
		},
	})
	seedCrosswalkAndTiers(t, s, "General Admission", "ga")
	seedCrosswalkAndTiers(t, s, "VIP Experience", "vip")

	ctx := context.Background()

	unscoped, err := computeRevenueByAxis(ctx, s.DB(), "access_class")
	if err != nil {
		t.Fatalf("computeRevenueByAxis: %v", err)
	}
	scoped, err := computeRevenueByAxisScoped(ctx, s.DB(), "access_class", "", "2026-01-01", "2026-12-31")
	if err != nil {
		t.Fatalf("computeRevenueByAxisScoped: %v", err)
	}

	toMap := func(rows []revenueByAxisRow) map[string]float64 {
		m := map[string]float64{}
		for _, r := range rows {
			m[r.AxisValue] = r.TotalRevenue
		}
		return m
	}
	u := toMap(unscoped.Rows)
	sc := toMap(scoped.Rows)

	// Paid-total basis: GA = 2 * 25.00 = 50.00; VIP = 75.00.
	if got := u["ga"]; got != 50.00 {
		t.Errorf("unscoped ga total_revenue = %v, want 50.00 (paid total, not list price)", got)
	}
	if got := u["vip"]; got != 75.00 {
		t.Errorf("unscoped vip total_revenue = %v, want 75.00 (paid total, not list price)", got)
	}
	// The two paths must agree per bucket.
	for _, axis := range []string{"ga", "vip"} {
		if u[axis] != sc[axis] {
			t.Errorf("axis %q: unscoped total_revenue %v != scoped %v (monetary basis diverges)", axis, u[axis], sc[axis])
		}
	}
}
