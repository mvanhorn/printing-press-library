// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for the novel transcendence commands. Each test seeds a
// temp SQLite store with known fixtures and asserts the compute helper's exact
// output, since there is no live API token to integration-test against.
package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// Synthetic fan-identity emails used to seed the local store. All use the
// IETF-reserved example.com domain (RFC 2606) so they can never resolve to a
// real mailbox; the distinct local-parts double as human-readable role labels
// (loyal vs. casual buyer, opted-in GB vs. US fan, etc.). Declared once here
// so the fixture values aren't repeated as bare literals across every test.
const (
	fanA      = "a@example.com"
	fanB      = "b@example.com"
	fanC      = "c@example.com"
	fanLoyal  = "loyal@example.com"
	fanCasual = "casual@example.com"
	fanMid    = "mid@example.com"
	fanHigh   = "high@example.com"
	fanLow    = "low@example.com"
	fanGB     = "gb@example.com"
	fanUS     = "us@example.com"
	fanOut    = "out@example.com"
)

// seedStore opens a fresh store in a temp dir and upserts the given fixtures.
// fixtures maps resource_type -> id -> JSON payload.
func seedStore(t *testing.T, fixtures map[string]map[string]string) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for resourceType, byID := range fixtures {
		for id, payload := range byID {
			if err := s.Upsert(resourceType, id, json.RawMessage(payload)); err != nil {
				t.Fatalf("upsert %s/%s: %v", resourceType, id, err)
			}
		}
	}
	return s
}

// order builds an orders fixture JSON payload.
func order(id, purchasedAt, eventID, eventName, email, first, last string, total, diceComm, quantity int, optIn bool, city, country string) string {
	o := storeOrder{ID: id, PurchasedAt: purchasedAt, Quantity: quantity, Total: int64(total), DiceComm: int64(diceComm), IPCity: city, IPCountry: country}
	o.Fan.Email = email
	o.Fan.FirstName = first
	o.Fan.LastName = last
	o.Fan.OptInPartners = optIn
	o.Event.ID = eventID
	o.Event.Name = eventName
	b, _ := json.Marshal(o)
	return string(b)
}

func TestDiceRevenueSummary(t *testing.T) {
	// Event A: two orders totalling 30000 cents gross, 3000 cents dice fees.
	// Event B: one order, 5000 cents gross, 250 cents dice fees.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-02-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 20000, 2000, 2, false, "", ""),
			"o2": order("o2", "2026-02-02T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 10000, 1000, 1, false, "", ""),
			"o3": order("o3", "2026-02-03T10:00:00Z", "evtB", "Show B", fanC, "Cat", "C", 5000, 250, 1, false, "", ""),
		},
	})

	rows, err := computeRevenue(context.Background(), s.DB(), "", "")
	if err != nil {
		t.Fatalf("computeRevenue: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d: %+v", len(rows), rows)
	}
	// Sorted by gross desc -> Event A first.
	a := rows[0]
	if a.EventID != "evtA" || a.EventName != "Show A" {
		t.Errorf("row[0] = %+v, want evtA/Show A", a)
	}
	if a.Gross != 300.00 {
		t.Errorf("evtA gross = %v, want 300.00", a.Gross)
	}
	if a.DiceFees != 30.00 {
		t.Errorf("evtA dice_fees = %v, want 30.00", a.DiceFees)
	}
	if a.Net != 270.00 {
		t.Errorf("evtA net = %v, want 270.00", a.Net)
	}
	if a.OrdersCount != 2 {
		t.Errorf("evtA orders_count = %d, want 2", a.OrdersCount)
	}
	b := rows[1]
	if b.EventID != "evtB" || b.Gross != 50.00 || b.Net != 47.50 || b.OrdersCount != 1 {
		t.Errorf("row[1] = %+v, want evtB gross 50 net 47.50 orders 1", b)
	}

	// --from filter: only orders on/after 2026-02-02 -> drops o1.
	filtered, err := computeRevenue(context.Background(), s.DB(), "", "2026-02-02")
	if err != nil {
		t.Fatalf("computeRevenue (from): %v", err)
	}
	var aFiltered *revenueRow
	for i := range filtered {
		if filtered[i].EventID == "evtA" {
			aFiltered = &filtered[i]
		}
	}
	if aFiltered == nil {
		t.Fatalf("evtA missing from filtered result: %+v", filtered)
	}
	if aFiltered.OrdersCount != 1 || aFiltered.Gross != 100.00 {
		t.Errorf("filtered evtA = %+v, want orders 1 gross 100", *aFiltered)
	}
}

func TestDiceFansRepeat(t *testing.T) {
	// Loyal fan: 2 distinct events. Casual fan: 1 event (two orders, same event).
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanLoyal, "Lo", "Yal", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-20T10:00:00Z", "evtB", "Show B", fanLoyal, "Lo", "Yal", 7000, 0, 1, false, "", ""),
			"o3": order("o3", "2026-01-15T10:00:00Z", "evtA", "Show A", fanCasual, "Ca", "Sual", 3000, 0, 1, false, "", ""),
			"o4": order("o4", "2026-01-16T10:00:00Z", "evtA", "Show A", fanCasual, "Ca", "Sual", 4000, 0, 1, false, "", ""),
		},
	})

	rows, err := computeFansRepeat(context.Background(), s.DB(), 2, "")
	if err != nil {
		t.Fatalf("computeFansRepeat: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 repeat fan, got %d: %+v", len(rows), rows)
	}
	r := rows[0]
	if r.Email != fanLoyal {
		t.Errorf("email = %q, want %q", r.Email, fanLoyal)
	}
	if r.EventsCount != 2 {
		t.Errorf("events_count = %d, want 2", r.EventsCount)
	}
	if r.TotalSpend != 120.00 {
		t.Errorf("total_spend = %v, want 120.00", r.TotalSpend)
	}
	if r.Name != "Lo Yal" {
		t.Errorf("name = %q, want 'Lo Yal'", r.Name)
	}
	if len(r.EventIDs) != 2 {
		t.Errorf("event_ids = %v, want 2 entries", r.EventIDs)
	}
}

func TestDiceFansTop(t *testing.T) {
	// Three fans with distinct totals; expect descending order.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanMid, "Mi", "D", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtA", "Show A", fanHigh, "Hi", "Gh", 9000, 0, 1, false, "", ""),
			"o3": order("o3", "2026-01-12T10:00:00Z", "evtA", "Show A", fanHigh, "Hi", "Gh", 1000, 0, 1, false, "", ""),
			"o4": order("o4", "2026-01-13T10:00:00Z", "evtA", "Show A", fanLow, "Lo", "W", 2000, 0, 1, false, "", ""),
		},
	})

	rows, err := computeFansTop(context.Background(), s.DB(), "", 20)
	if err != nil {
		t.Fatalf("computeFansTop: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 fans, got %d: %+v", len(rows), rows)
	}
	// high: 9000+1000=10000 (100.00), mid: 5000 (50.00), low: 2000 (20.00)
	wantEmails := []string{fanHigh, fanMid, fanLow}
	wantTotals := []float64{100.00, 50.00, 20.00}
	for i := range rows {
		if rows[i].Email != wantEmails[i] {
			t.Errorf("rows[%d].Email = %q, want %q", i, rows[i].Email, wantEmails[i])
		}
		if rows[i].TotalSpend != wantTotals[i] {
			t.Errorf("rows[%d].TotalSpend = %v, want %v", i, rows[i].TotalSpend, wantTotals[i])
		}
	}
	if rows[0].OrdersCount != 2 {
		t.Errorf("high orders_count = %d, want 2", rows[0].OrdersCount)
	}

	// --n 1 limits to the top spender.
	limited, err := computeFansTop(context.Background(), s.DB(), "", 1)
	if err != nil {
		t.Fatalf("computeFansTop (n=1): %v", err)
	}
	if len(limited) != 1 || limited[0].Email != fanHigh {
		t.Errorf("n=1 result = %+v, want only %s", limited, fanHigh)
	}
}

func TestDiceFansOptin(t *testing.T) {
	// One opted-in GB/London fan, one opted-in US fan, one opted-out fan.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanGB, "Geo", "Brit", 5000, 0, 1, true, "London", "GB"),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtA", "Show A", fanUS, "Uma", "Sam", 5000, 0, 1, true, "Austin", "US"),
			"o3": order("o3", "2026-01-12T10:00:00Z", "evtA", "Show A", fanOut, "Op", "Tout", 5000, 0, 1, false, "London", "GB"),
		},
	})

	// No geo filter: only the two opted-in fans appear (opted-out excluded).
	all, err := computeFansOptin(context.Background(), s.DB(), "", "", "")
	if err != nil {
		t.Fatalf("computeFansOptin: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 opted-in fans, got %d: %+v", len(all), all)
	}
	for _, r := range all {
		if r.Email == fanOut {
			t.Errorf("opted-out fan leaked into result: %+v", r)
		}
	}

	// Country filter (case-insensitive) narrows to GB.
	gb, err := computeFansOptin(context.Background(), s.DB(), "", "gb", "")
	if err != nil {
		t.Fatalf("computeFansOptin (country): %v", err)
	}
	if len(gb) != 1 || gb[0].Email != fanGB {
		t.Errorf("country=gb result = %+v, want only %s", gb, fanGB)
	}
	if gb[0].City != "London" || gb[0].Country != "GB" || gb[0].FirstName != "Geo" {
		t.Errorf("gb row geography = %+v, want London/GB/Geo", gb[0])
	}

	// City substring filter (case-insensitive) also narrows to London.
	lon, err := computeFansOptin(context.Background(), s.DB(), "", "", "lond")
	if err != nil {
		t.Fatalf("computeFansOptin (city): %v", err)
	}
	if len(lon) != 1 || lon[0].Email != fanGB {
		t.Errorf("city=lond result = %+v, want only %s", lon, fanGB)
	}
}

func TestDiceReturnsAnomalies(t *testing.T) {
	// Event A: 10 orders, 2 returns -> 0.2 rate (flagged).
	// Event B: 10 orders, 0 returns -> 0.0 rate (not flagged).
	orders := map[string]string{}
	for i := 0; i < 10; i++ {
		idA := "a" + string(rune('0'+i))
		idB := "b" + string(rune('0'+i))
		orders[idA] = order(idA, "2026-01-10T10:00:00Z", "evtA", "Show A", "fa"+idA+"@example.com", "F", "A", 5000, 0, 1, false, "", "")
		orders[idB] = order(idB, "2026-01-10T10:00:00Z", "evtB", "Show B", "fb"+idB+"@example.com", "F", "B", 5000, 0, 1, false, "", "")
	}
	ret := func(id string) string {
		return `{"id":"` + id + `","ticketId":"t-` + id + `","order":{"id":"ord-` + id + `","event":{"id":"evtA","name":"Show A"}}}`
	}
	s := seedStore(t, map[string]map[string]string{
		"orders": orders,
		"returns": {
			"r1": ret("r1"),
			"r2": ret("r2"),
		},
	})

	rows, err := computeReturnsAnomalies(context.Background(), s.DB(), 0.05)
	if err != nil {
		t.Fatalf("computeReturnsAnomalies: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 flagged event, got %d: %+v", len(rows), rows)
	}
	r := rows[0]
	if r.EventID != "evtA" || r.EventName != "Show A" {
		t.Errorf("flagged event = %+v, want evtA/Show A", r)
	}
	if r.OrdersCount != 10 || r.ReturnsCount != 2 {
		t.Errorf("counts = %d orders / %d returns, want 10/2", r.OrdersCount, r.ReturnsCount)
	}
	if r.ReturnRate != 0.2 {
		t.Errorf("return_rate = %v, want 0.2", r.ReturnRate)
	}
}

func TestDiceVelocityShow(t *testing.T) {
	// Orders across two days: day1 sells 3 (2+1), day2 sells 5.
	// onSaleDatetime = day1 00:00 so day1 bucket offset is 0.
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evtA": `{"id":"evtA","name":"Show A","onSaleDatetime":"2026-03-01T00:00:00Z"}`,
		},
		"orders": {
			"o1": order("o1", "2026-03-01T09:00:00Z", "evtA", "Show A", fanA, "A", "A", 1000, 0, 2, false, "", ""),
			"o2": order("o2", "2026-03-01T18:00:00Z", "evtA", "Show A", fanB, "B", "B", 1000, 0, 1, false, "", ""),
			"o3": order("o3", "2026-03-02T12:00:00Z", "evtA", "Show A", fanC, "C", "C", 1000, 0, 5, false, "", ""),
		},
	})

	rows, err := computeVelocity(context.Background(), s.DB(), "evtA", "day")
	if err != nil {
		t.Fatalf("computeVelocity: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 day buckets, got %d: %+v", len(rows), rows)
	}
	// Chronological order.
	if rows[0].Bucket != "2026-03-01" || rows[1].Bucket != "2026-03-02" {
		t.Errorf("buckets out of order: %+v", rows)
	}
	if rows[0].PeriodSold != 3 {
		t.Errorf("day1 period_sold = %d, want 3", rows[0].PeriodSold)
	}
	if rows[1].PeriodSold != 5 {
		t.Errorf("day2 period_sold = %d, want 5", rows[1].PeriodSold)
	}
	// Cumulative is monotonic and final equals total (3 + 5 = 8).
	if rows[0].CumulativeSold != 3 {
		t.Errorf("day1 cumulative = %d, want 3", rows[0].CumulativeSold)
	}
	if rows[1].CumulativeSold != 8 {
		t.Errorf("day2 cumulative = %d, want 8 (total)", rows[1].CumulativeSold)
	}
	if rows[1].CumulativeSold < rows[0].CumulativeSold {
		t.Errorf("cumulative not monotonic: %d then %d", rows[0].CumulativeSold, rows[1].CumulativeSold)
	}
	// onSale = day1 00:00, day1 bucket start = day1 00:00 -> offset 0;
	// day2 bucket start = +24h -> offset 24.
	if rows[0].HourOffset != 0 {
		t.Errorf("day1 hour_offset = %d, want 0", rows[0].HourOffset)
	}
	if rows[1].HourOffset != 24 {
		t.Errorf("day2 hour_offset = %d, want 24", rows[1].HourOffset)
	}
}
