// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
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

	rows, err := computeRevenue(context.Background(), s.DB(), "", "", "")
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

	// --from/--to filter by SHOW date (event startDatetime), not purchase date.
	// Seed events so evtA's show is in-window and evtB's is out; the window keeps
	// all of evtA's orders (regardless of when they were purchased) and drops evtB.
	sd := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-02-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 20000, 2000, 2, false, "", ""),
			"o2": order("o2", "2026-02-02T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 10000, 1000, 1, false, "", ""),
			"o3": order("o3", "2026-02-03T10:00:00Z", "evtB", "Show B", fanC, "Cat", "C", 5000, 250, 1, false, "", ""),
		},
		"events": {
			"evtA": `{"id":"evtA","name":"Show A","startDatetime":"2026-05-16T20:00:00Z"}`,
			"evtB": `{"id":"evtB","name":"Show B","startDatetime":"2026-06-20T20:00:00Z"}`,
		},
	})
	windowed, err := computeRevenue(context.Background(), sd.DB(), "", "2026-05-01", "2026-05-31")
	if err != nil {
		t.Fatalf("computeRevenue (show-date window): %v", err)
	}
	if len(windowed) != 1 || windowed[0].EventID != "evtA" {
		t.Fatalf("show-date window = %+v, want only evtA (show 2026-05-16)", windowed)
	}
	if windowed[0].OrdersCount != 2 || windowed[0].Gross != 300.00 {
		t.Errorf("windowed evtA = %+v, want all 2 orders, gross 300", windowed[0])
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
	// Event A: 10 orders x1 ticket = 10 tickets, 2 returns -> 0.2 (flagged).
	// Event B: 10 orders x1 ticket, 0 returns -> 0.0 (not flagged).
	// Event C: 1 order x5 tickets = 5 tickets, 2 returns -> 0.4 (flagged).
	//   Regression: dividing by order count would give 2/1 = 2.0; the rate must
	//   divide by tickets sold, so 2/5 = 0.4.
	orders := map[string]string{}
	for i := 0; i < 10; i++ {
		idA := "a" + string(rune('0'+i))
		idB := "b" + string(rune('0'+i))
		orders[idA] = order(idA, "2026-01-10T10:00:00Z", "evtA", "Show A", "fa"+idA+"@example.com", "F", "A", 5000, 0, 1, false, "", "")
		orders[idB] = order(idB, "2026-01-10T10:00:00Z", "evtB", "Show B", "fb"+idB+"@example.com", "F", "B", 5000, 0, 1, false, "", "")
	}
	orders["c0"] = order("c0", "2026-01-10T10:00:00Z", "evtC", "Show C", "fc@example.com", "F", "C", 25000, 0, 5, false, "", "")
	retFor := func(id, eventID, eventName string) string {
		return `{"id":"` + id + `","ticketId":"t-` + id + `","order":{"id":"ord-` + id + `","event":{"id":"` + eventID + `","name":"` + eventName + `"}}}`
	}
	s := seedStore(t, map[string]map[string]string{
		"orders": orders,
		"returns": {
			"r1": retFor("r1", "evtA", "Show A"),
			"r2": retFor("r2", "evtA", "Show A"),
			"r3": retFor("r3", "evtC", "Show C"),
			"r4": retFor("r4", "evtC", "Show C"),
		},
	})

	rows, err := computeReturnsAnomalies(context.Background(), s.DB(), 0.05, "", "")
	if err != nil {
		t.Fatalf("computeReturnsAnomalies: %v", err)
	}
	// evtC (0.4) and evtA (0.2) flagged; evtB (0.0) not. Sorted by rate desc.
	if len(rows) != 2 {
		t.Fatalf("want 2 flagged events, got %d: %+v", len(rows), rows)
	}
	byEvent := map[string]returnsAnomalyRow{}
	for _, r := range rows {
		byEvent[r.EventID] = r
	}
	c := byEvent["evtC"]
	if c.OrdersCount != 1 || c.TicketsSold != 5 || c.ReturnsCount != 2 || c.ReturnRate != 0.4 {
		t.Errorf("evtC = %+v, want orders 1 / tickets 5 / returns 2 / rate 0.4", c)
	}
	a := byEvent["evtA"]
	if a.OrdersCount != 10 || a.TicketsSold != 10 || a.ReturnsCount != 2 || a.ReturnRate != 0.2 {
		t.Errorf("evtA = %+v, want orders 10 / tickets 10 / returns 2 / rate 0.2", a)
	}
	if rows[0].EventID != "evtC" {
		t.Errorf("rows[0] = %s, want evtC first (highest rate)", rows[0].EventID)
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

// eventFixture builds an `events` store payload with a capacity and state.
func eventFixture(id, name, state string, capacity int64) string {
	e := storeEvent{ID: id, Name: name, State: state, TotalAllocQty: capacity}
	b, _ := json.Marshal(e)
	return string(b)
}

// ticketFixture builds a `tickets` store payload at a named price tier.
func ticketFixture(id, tierID, tierName string, tierPrice int64) string {
	t := storeTicket{ID: id}
	t.PriceTier.ID = tierID
	t.PriceTier.Name = tierName
	t.PriceTier.Price = tierPrice
	b, _ := json.Marshal(t)
	return string(b)
}

// returnFixture builds a `returns` store payload referencing a ticket.
func returnFixture(id, ticketID string) string {
	b, _ := json.Marshal(map[string]string{"id": id, "ticketId": ticketID})
	return string(b)
}

func TestDiceCapacity(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			// evtA: live, capacity 100. evtB: live, capacity 50.
			// evtC: cancelled — must be excluded from the headroom rollup.
			"evtA": eventFixture("evtA", "Show A", "live", 100),
			"evtB": eventFixture("evtB", "Show B", "on-sale", 50),
			"evtC": eventFixture("evtC", "Show C", "cancelled", 200),
		},
		"orders": {
			// evtA: 60 sold (40 + 20) -> 60% sold. evtB: 50 sold -> 100% sold.
			// evtC: 10 sold but cancelled, must not appear.
			"o1": order("o1", "2026-02-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 0, 0, 40, false, "", ""),
			"o2": order("o2", "2026-02-02T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 0, 0, 20, false, "", ""),
			"o3": order("o3", "2026-02-03T10:00:00Z", "evtB", "Show B", fanC, "Cat", "C", 0, 0, 50, false, "", ""),
			"o4": order("o4", "2026-02-04T10:00:00Z", "evtC", "Show C", fanA, "Ann", "A", 0, 0, 10, false, "", ""),
		},
	})

	rows, err := computeCapacity(context.Background(), s.DB(), "")
	if err != nil {
		t.Fatalf("computeCapacity: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 live rows, got %d: %+v", len(rows), rows)
	}
	// Sorted by pct_sold desc -> evtB (100%) first.
	if rows[0].EventID != "evtB" || rows[0].Sold != 50 || rows[0].Capacity != 50 || rows[0].Remaining != 0 || rows[0].PctSold != 100 {
		t.Errorf("row[0] = %+v, want evtB 50/50 rem 0 pct 100", rows[0])
	}
	if rows[1].EventID != "evtA" || rows[1].Sold != 60 || rows[1].Capacity != 100 || rows[1].Remaining != 40 || rows[1].PctSold != 60 {
		t.Errorf("row[1] = %+v, want evtA 60/100 rem 40 pct 60", rows[1])
	}

	// --event filter keeps only the requested event.
	filtered, err := computeCapacity(context.Background(), s.DB(), "evtA")
	if err != nil {
		t.Fatalf("computeCapacity filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].EventID != "evtA" {
		t.Errorf("filtered = %+v, want single evtA row", filtered)
	}
}

func TestDiceTierPerformance(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			// evtA: 3 Early Bird, 1 General. evtB: 1 General. t6 is returned.
			"t1": ticketFixture("t1", "tier-eb", "Early Bird", 2500),
			"t2": ticketFixture("t2", "tier-eb", "Early Bird", 2500),
			"t3": ticketFixture("t3", "tier-eb", "Early Bird", 2500),
			"t4": ticketFixture("t4", "tier-gen", "General", 3500),
			"t5": ticketFixture("t5", "tier-gen", "General", 3500),
			"t6": ticketFixture("t6", "tier-eb", "Early Bird", 2500),
		},
		"returns": {
			// t6 returned -> excluded from redemptions and the denominator.
			"r1": returnFixture("r1", "t6"),
		},
	})

	rows, err := computeTierPerformance(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("computeTierPerformance: %v", err)
	}
	// Global per-tier rollup (tickets carry no synced event reference):
	// tier-eb = t1,t2,t3 = 3 (t6 returned, excluded); tier-gen = t4,t5 = 2.
	// Total non-returned redemptions = 5.
	if len(rows) != 2 {
		t.Fatalf("want 2 tier rows, got %d: %+v", len(rows), rows)
	}
	top := rows[0]
	if top.TierID != "tier-eb" || top.Redemptions != 3 {
		t.Errorf("row[0] = %+v, want Early Bird redemptions 3", top)
	}
	if top.Price != 25 {
		t.Errorf("Early Bird price = %v, want 25 (2500 cents)", top.Price)
	}
	// Share of total: 3/5 = 0.6.
	if top.RedemptionRate != 0.6 {
		t.Errorf("Early Bird redemption_rate = %v, want 0.6", top.RedemptionRate)
	}
	// Returned ticket t6 must not inflate the count, and tier-gen = 2/5 = 0.4.
	byTier := map[string]tierPerformanceRow{}
	for _, r := range rows {
		byTier[r.TierID] = r
	}
	if byTier["tier-eb"].Redemptions != 3 {
		t.Errorf("Early Bird redemptions = %d, want 3 (returned t6 excluded)", byTier["tier-eb"].Redemptions)
	}
	if byTier["tier-gen"].Redemptions != 2 || byTier["tier-gen"].RedemptionRate != 0.4 {
		t.Errorf("General tier = %+v, want redemptions 2 / rate 0.4", byTier["tier-gen"])
	}
}
