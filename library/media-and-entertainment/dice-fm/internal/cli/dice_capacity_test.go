// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for the `capacity pools` per-ticket-pool allocation view.
// Each test seeds a temp SQLite store with synthetic events whose JSON carries
// a ticketPools array, then asserts computeCapacityPools' exact output. There
// is no live API token, so all fixtures are local + synthetic.
package cli

import (
	"context"
	"encoding/json"
	"testing"
)

// eventWithPools builds an `events` store payload carrying a ticketPools
// allocation array and an event-level totalTicketAllocationQty.
func eventWithPools(id, name string, total int64, pools []storeTicketPool) string {
	e := storeEvent{
		ID:            id,
		Name:          name,
		TotalAllocQty: total,
		TicketPools:   pools,
	}
	b, _ := json.Marshal(e)
	return string(b)
}

func TestComputeCapacityPools(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evtA": eventWithPools("evtA", "Event One", 160, []storeTicketPool{
				{ID: "p1", Name: "Pool A", Allocation: 100},
				{ID: "p2", Name: "Pool B", Allocation: 50},
			}),
		},
	})

	rows, err := computeCapacityPools(context.Background(), s.DB(), "")
	if err != nil {
		t.Fatalf("computeCapacityPools: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d: %+v", len(rows), rows)
	}

	// Sorted by event ID then pool ID -> p1 before p2.
	r0, r1 := rows[0], rows[1]
	if r0.PoolID != "p1" || r0.PoolName != "Pool A" || r0.Allocation != 100 {
		t.Errorf("row[0] = %+v, want p1/Pool A/100", r0)
	}
	if r1.PoolID != "p2" || r1.PoolName != "Pool B" || r1.Allocation != 50 {
		t.Errorf("row[1] = %+v, want p2/Pool B/50", r1)
	}
	for _, r := range rows {
		if r.EventID != "evtA" || r.EventName != "Event One" {
			t.Errorf("row event = %s/%s, want evtA/Event One", r.EventID, r.EventName)
		}
		// Per-event pool-sum is 100 + 50 = 150, surfaced on every row.
		if r.PoolSum != 150 {
			t.Errorf("PoolSum = %d, want 150", r.PoolSum)
		}
		// Event total (totalTicketAllocationQty) is surfaced for comparison.
		if r.EventTotal != 160 {
			t.Errorf("EventTotal = %d, want 160", r.EventTotal)
		}
	}
}

func TestComputeCapacityPoolsEventFilter(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evtA": eventWithPools("evtA", "Event One", 160, []storeTicketPool{
				{ID: "p1", Name: "Pool A", Allocation: 100},
				{ID: "p2", Name: "Pool B", Allocation: 50},
			}),
			"evtB": eventWithPools("evtB", "Event Two", 80, []storeTicketPool{
				{ID: "p3", Name: "Pool C", Allocation: 80},
			}),
		},
	})

	rows, err := computeCapacityPools(context.Background(), s.DB(), "evtB")
	if err != nil {
		t.Fatalf("computeCapacityPools: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d: %+v", len(rows), rows)
	}
	r := rows[0]
	if r.EventID != "evtB" || r.PoolID != "p3" || r.Allocation != 80 {
		t.Errorf("row = %+v, want evtB/p3/80", r)
	}
	if r.PoolSum != 80 || r.EventTotal != 80 {
		t.Errorf("row = %+v, want PoolSum 80 / EventTotal 80", r)
	}
}

func TestComputeCapacityPoolsSkipsEventsWithoutPools(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			// No ticketPools -> contributes no rows.
			"evtA": eventWithPools("evtA", "Event One", 100, nil),
			"evtB": eventWithPools("evtB", "Event Two", 80, []storeTicketPool{
				{ID: "p3", Name: "Pool C", Allocation: 80},
			}),
		},
	})

	rows, err := computeCapacityPools(context.Background(), s.DB(), "")
	if err != nil {
		t.Fatalf("computeCapacityPools: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row (evtB only), got %d: %+v", len(rows), rows)
	}
	if rows[0].EventID != "evtB" {
		t.Errorf("row event = %s, want evtB", rows[0].EventID)
	}
}
