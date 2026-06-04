// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Table-driven tests for the three new analytics commands: fans segment,
// fans profile, and revenue by-artist. Seeded-store style mirrors
// dice_transcend_test.go (plain t.Errorf/Fatalf, no testify).
package cli

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
)

// eventWithArtists builds an `events` store payload with artist and genre data.
func eventWithArtists(id, name string, artists []string, genres []string) string {
	type artist struct {
		Name string `json:"name"`
	}
	type ev struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Artists    []artist `json:"artists"`
		Genres     []string `json:"genres"`
		GenreTypes []string `json:"genreTypes"`
	}
	e := ev{ID: id, Name: name}
	for _, a := range artists {
		e.Artists = append(e.Artists, artist{Name: a})
	}
	e.Genres = genres
	b, _ := json.Marshal(e)
	return string(b)
}

// ticketWithHolder builds a `tickets` store payload with holder and type info.
func ticketWithHolder(id, holderEmail, ttName, tierName string, optIn bool) string {
	type holder struct {
		Email         string `json:"email"`
		OptInPartners bool   `json:"optInPartners"`
	}
	type tt struct {
		Name string `json:"name"`
	}
	type tier struct {
		Name string `json:"name"`
	}
	type tkt struct {
		ID         string `json:"id"`
		Holder     holder `json:"holder"`
		TicketType tt     `json:"ticketType"`
		PriceTier  tier   `json:"priceTier"`
	}
	t := tkt{
		ID:         id,
		Holder:     holder{Email: holderEmail, OptInPartners: optIn},
		TicketType: tt{Name: ttName},
		PriceTier:  tier{Name: tierName},
	}
	b, _ := json.Marshal(t)
	return string(b)
}

// --- fans segment tests ---

func TestFansSegmentNoFilter(t *testing.T) {
	// No filters → all fans with any order are returned.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{})
	if err != nil {
		t.Fatalf("computeFansSegment: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 fans, got %d: %+v", len(rows), rows)
	}
	// Sorted by total_spend desc: fanA (50.00) then fanB (30.00).
	if rows[0].Email != fanA || rows[0].TotalSpend != 50.00 {
		t.Errorf("rows[0] = %+v, want fanA 50.00", rows[0])
	}
	if rows[1].Email != fanB || rows[1].TotalSpend != 30.00 {
		t.Errorf("rows[1] = %+v, want fanB 30.00", rows[1])
	}
}

func TestFansSegmentMinEvents(t *testing.T) {
	// fanLoyal attends 2 events; fanCasual attends 1.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanLoyal, "Lo", "Yal", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-20T10:00:00Z", "evtB", "Show B", fanLoyal, "Lo", "Yal", 7000, 0, 1, false, "", ""),
			"o3": order("o3", "2026-01-15T10:00:00Z", "evtA", "Show A", fanCasual, "Ca", "Sual", 3000, 0, 1, false, "", ""),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{minEvents: 2})
	if err != nil {
		t.Fatalf("computeFansSegment minEvents: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 fan (minEvents=2), got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanLoyal {
		t.Errorf("email = %q, want %q", rows[0].Email, fanLoyal)
	}
	if rows[0].EventsCount != 2 {
		t.Errorf("events_count = %d, want 2", rows[0].EventsCount)
	}
}

func TestFansSegmentOptedIn(t *testing.T) {
	// fanGB opted in; fanOut did not.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanGB, "Geo", "Brit", 5000, 0, 1, true, "London", "GB"),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtA", "Show A", fanOut, "Op", "Out", 3000, 0, 1, false, "", ""),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{optedIn: true})
	if err != nil {
		t.Fatalf("computeFansSegment optedIn: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 opted-in fan, got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanGB || !rows[0].OptedIn {
		t.Errorf("row = %+v, want fanGB opted_in true", rows[0])
	}
}

func TestFansSegmentMinQty(t *testing.T) {
	// fanA placed an order qty=3; fanB placed qty=1.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 9000, 0, 3, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{minQty: 2})
	if err != nil {
		t.Fatalf("computeFansSegment minQty: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 fan (minQty=2), got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanA {
		t.Errorf("email = %q, want fanA", rows[0].Email)
	}
}

// An order with an API-omitted quantity (0) counts as one ticket, so it must not
// be silently dropped by --min-qty 1 — mirrors the qty<=0→1 fallback in the
// other analytics commands.
func TestFansSegmentMinQtyZeroQuantityFallback(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 4000, 0, 0, false, "", ""),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{minQty: 1})
	if err != nil {
		t.Fatalf("computeFansSegment minQty zero-fallback: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Fatalf("want fanA included (qty 0 → 1 ticket >= min-qty 1), got %+v", rows)
	}
}

// --min-qty qualifies a fan but must NOT reduce their total_spend/events_count
// to only the qualifying orders — all of the fan's orders count.
func TestFansSegmentMinQtyAggregatesAllOrders(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			// fanA: a qty=3 order ($100, evtA) qualifies for --min-qty 2, plus a
			// qty=1 order ($40, evtB). Both must count toward spend + events.
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 10000, 0, 3, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtB", "Show B", fanA, "Ann", "A", 4000, 0, 1, false, "", ""),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{minQty: 2})
	if err != nil {
		t.Fatalf("computeFansSegment: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Fatalf("want fanA qualified by min-qty, got %+v", rows)
	}
	if rows[0].TotalSpend != 140.00 {
		t.Errorf("total_spend = %v, want 140.00 (both orders, not just the qty>=2 one)", rows[0].TotalSpend)
	}
	if rows[0].EventsCount != 2 {
		t.Errorf("events_count = %d, want 2 (both events, not just the qty>=2 one)", rows[0].EventsCount)
	}
}

// --opted-in qualifies a fan but must NOT reduce their total_spend/events_count
// to only the orders where optInPartners was set — all of the fan's orders count.
func TestFansSegmentOptedInAggregatesAllOrders(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			// fanGB: an opted-in order ($100, evtA) qualifies for --opted-in, plus
			// an order where opt-in was not set ($40, evtB). Both must count.
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanGB, "Geo", "Brit", 10000, 0, 1, true, "London", "GB"),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtB", "Show B", fanGB, "Geo", "Brit", 4000, 0, 1, false, "", ""),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{optedIn: true})
	if err != nil {
		t.Fatalf("computeFansSegment: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanGB {
		t.Fatalf("want fanGB qualified by opted-in, got %+v", rows)
	}
	if !rows[0].OptedIn {
		t.Errorf("opted_in = false, want true")
	}
	if rows[0].TotalSpend != 140.00 {
		t.Errorf("total_spend = %v, want 140.00 (both orders, not just the opted-in one)", rows[0].TotalSpend)
	}
	if rows[0].EventsCount != 2 {
		t.Errorf("events_count = %d, want 2 (both events, not just the opted-in one)", rows[0].EventsCount)
	}
}

func TestFansSegmentTicketType(t *testing.T) {
	// fanA has a "VIP" ticket; fanB has a "GA" ticket.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 15000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 5000, 0, 1, false, "", ""),
		},
		"tickets": {
			"t1": ticketWithHolder("t1", fanA, "VIP Early Bird", "Tier 1", false),
			"t2": ticketWithHolder("t2", fanB, "GA Standard", "Tier 1", false),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{ticketType: "vip"})
	if err != nil {
		t.Fatalf("computeFansSegment ticketType: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 fan (ticketType=vip), got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanA {
		t.Errorf("email = %q, want fanA", rows[0].Email)
	}
}

func TestFansSegmentTier(t *testing.T) {
	// fanA has an "Early Bird" tier; fanB has "General".
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"tickets": {
			"t1": ticketWithHolder("t1", fanA, "GA", "Early Bird", false),
			"t2": ticketWithHolder("t2", fanB, "GA", "General", false),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{tier: "early"})
	if err != nil {
		t.Fatalf("computeFansSegment tier: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 fan (tier=early), got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanA {
		t.Errorf("email = %q, want fanA", rows[0].Email)
	}
}

func TestFansSegmentGenre(t *testing.T) {
	// evtA has genre "dj:electronic"; evtB has "rock".
	// fanA buys evtA; fanB buys evtB.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtB", "Show B", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"events": {
			"evtA": eventWithArtists("evtA", "Show A", nil, []string{"dj:electronic"}),
			"evtB": eventWithArtists("evtB", "Show B", nil, []string{"rock:indie"}),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{genre: "electronic"})
	if err != nil {
		t.Fatalf("computeFansSegment genre: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 fan (genre=electronic), got %d: %+v", len(rows), rows)
	}
	if rows[0].Email != fanA {
		t.Errorf("email = %q, want fanA", rows[0].Email)
	}
}

func TestFansSegmentEventName(t *testing.T) {
	// fanA buys "Jazz Night"; fanB buys "Rock Show". Filter on "jazz".
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Jazz Night", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtB", "Rock Show", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{eventName: "jazz"})
	if err != nil {
		t.Fatalf("computeFansSegment eventName: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Fatalf("want fanA only, got %+v", rows)
	}
}

// --event-name qualifies a fan but must NOT reduce their total_spend/events_count
// to only the matching-event orders — all of the fan's orders count.
func TestFansSegmentEventNameAggregatesAllOrders(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			// fanA: a "Jazz Night" order ($100, evtA) qualifies for --event-name
			// jazz, plus a "Rock Show" order ($40, evtB). Both must count.
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Jazz Night", fanA, "Ann", "A", 10000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtB", "Rock Show", fanA, "Ann", "A", 4000, 0, 1, false, "", ""),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{eventName: "jazz"})
	if err != nil {
		t.Fatalf("computeFansSegment: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Fatalf("want fanA qualified by event-name, got %+v", rows)
	}
	if rows[0].TotalSpend != 140.00 {
		t.Errorf("total_spend = %v, want 140.00 (both orders, not just the jazz one)", rows[0].TotalSpend)
	}
	if rows[0].EventsCount != 2 {
		t.Errorf("events_count = %d, want 2 (both events, not just the jazz one)", rows[0].EventsCount)
	}
}

// --genre qualifies a fan but must NOT reduce their total_spend/events_count to
// only the matching-genre orders — all of the fan's orders count.
func TestFansSegmentGenreAggregatesAllOrders(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			// fanA: an electronic-genre order ($100, evtA) qualifies for --genre
			// electronic, plus a rock-genre order ($40, evtB). Both must count.
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 10000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtB", "Show B", fanA, "Ann", "A", 4000, 0, 1, false, "", ""),
		},
		"events": {
			"evtA": eventWithArtists("evtA", "Show A", nil, []string{"dj:electronic"}),
			"evtB": eventWithArtists("evtB", "Show B", nil, []string{"rock:indie"}),
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{genre: "electronic"})
	if err != nil {
		t.Fatalf("computeFansSegment: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Fatalf("want fanA qualified by genre, got %+v", rows)
	}
	if rows[0].TotalSpend != 140.00 {
		t.Errorf("total_spend = %v, want 140.00 (both orders, not just the electronic one)", rows[0].TotalSpend)
	}
	if rows[0].EventsCount != 2 {
		t.Errorf("events_count = %d, want 2 (both events, not just the electronic one)", rows[0].EventsCount)
	}
}

func TestFansSegmentShowDateWindow(t *testing.T) {
	// evtA show on 2026-05-10; evtB show on 2026-06-20.
	// Filter from=2026-05-01 to=2026-05-31 keeps only evtA orders.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-04-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-04-02T10:00:00Z", "evtB", "Show B", fanB, "Bob", "B", 3000, 0, 1, false, "", ""),
		},
		"events": {
			"evtA": `{"id":"evtA","name":"Show A","startDatetime":"2026-05-10T20:00:00Z"}`,
			"evtB": `{"id":"evtB","name":"Show B","startDatetime":"2026-06-20T20:00:00Z"}`,
		},
	})
	rows, err := computeFansSegment(context.Background(), s.DB(), segmentFilters{fromDate: "2026-05-01", toDate: "2026-05-31"})
	if err != nil {
		t.Fatalf("computeFansSegment dateWindow: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != fanA {
		t.Fatalf("want only fanA in window, got %+v", rows)
	}
}

// --- fans profile tests ---

func TestFanProfileNotFound(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
		},
	})
	result, err := computeFanProfile(context.Background(), s.DB(), "nobody@example.com")
	if err != nil {
		t.Fatalf("computeFanProfile notFound: %v", err)
	}
	if result.Found {
		t.Errorf("expected found:false for unknown email, got %+v", result)
	}
	if result.OrdersCount != 0 || result.TotalSpend != 0 {
		t.Errorf("expected zero fields for not-found fan, got %+v", result)
	}
}

func TestFanProfileBasic(t *testing.T) {
	// fanA: 2 orders across 2 events; 1 paid, 1 free (total=0 -> not counted as paid event).
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-20T10:00:00Z", "evtB", "Show B", fanA, "Ann", "A", 0, 0, 1, false, "", ""),
		},
	})
	result, err := computeFanProfile(context.Background(), s.DB(), fanA)
	if err != nil {
		t.Fatalf("computeFanProfile: %v", err)
	}
	if !result.Found {
		t.Fatalf("expected found:true, got found:false")
	}
	if result.Email != fanA {
		t.Errorf("email = %q, want %q", result.Email, fanA)
	}
	if result.OrdersCount != 2 {
		t.Errorf("orders_count = %d, want 2", result.OrdersCount)
	}
	if result.TotalSpend != 50.00 {
		t.Errorf("total_spend = %v, want 50.00", result.TotalSpend)
	}
	if result.EventCountAll != 2 {
		t.Errorf("event_count_all = %d, want 2", result.EventCountAll)
	}
	// Only o1 has total > 0.
	if result.EventCountPaid != 1 {
		t.Errorf("event_count_paid = %d, want 1", result.EventCountPaid)
	}
	if result.FirstOrderDate != "2026-01-10T10:00:00Z" {
		t.Errorf("first_order_date = %q, want 2026-01-10T10:00:00Z", result.FirstOrderDate)
	}
	if result.LastOrderDate != "2026-01-20T10:00:00Z" {
		t.Errorf("last_order_date = %q, want 2026-01-20T10:00:00Z", result.LastOrderDate)
	}
	if result.FirstEvent != "Show A" {
		t.Errorf("first_event = %q, want Show A", result.FirstEvent)
	}
	if result.LastEvent != "Show B" {
		t.Errorf("last_event = %q, want Show B", result.LastEvent)
	}
	if len(result.EventsPurchased) != 2 {
		t.Errorf("events_purchased = %v, want 2 entries", result.EventsPurchased)
	}
	if result.Name != "Ann A" {
		t.Errorf("name = %q, want 'Ann A'", result.Name)
	}
}

func TestFanProfileVIPSpend(t *testing.T) {
	// fanA has a VIP ticket; total spend from orders should appear in vip_spend.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 20000, 0, 1, false, "", ""),
		},
		"tickets": {
			"t1": ticketWithHolder("t1", fanA, "VIP Lounge", "Tier 1", false),
		},
	})
	result, err := computeFanProfile(context.Background(), s.DB(), fanA)
	if err != nil {
		t.Fatalf("computeFanProfile VIP: %v", err)
	}
	if !result.Found {
		t.Fatalf("expected found:true")
	}
	// holderHasVIP = true because ticket type contains "VIP".
	if result.VIPSpend != 200.00 {
		t.Errorf("vip_spend = %v, want 200.00", result.VIPSpend)
	}
	// Ticket types list should include the VIP type name.
	if len(result.TicketTypes) == 0 {
		t.Errorf("ticket_types is empty, want at least VIP Lounge")
	}
	found := false
	for _, tt := range result.TicketTypes {
		if tt == "VIP Lounge" {
			found = true
		}
	}
	if !found {
		t.Errorf("ticket_types = %v, want to include 'VIP Lounge'", result.TicketTypes)
	}
}

// vip_spend is a fan-level signal: a fan who holds any VIP ticket has ALL of
// their order spend counted as vip_spend, not only the order tied to the VIP
// ticket. Locks the documented all-or-nothing semantic against a future
// regression that tries to attribute vip_spend per order.
func TestFanProfileVIPSpendCountsAllFanOrders(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 20000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-02-10T10:00:00Z", "evtB", "Show B", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
		},
		"tickets": {
			// One VIP ticket qualifies the fan; the GA ticket does not negate it.
			"t1": ticketWithHolder("t1", fanA, "VIP Lounge", "Tier 1", false),
			"t2": ticketWithHolder("t2", fanA, "GA Standard", "Tier 1", false),
		},
	})
	result, err := computeFanProfile(context.Background(), s.DB(), fanA)
	if err != nil {
		t.Fatalf("computeFanProfile: %v", err)
	}
	if !result.Found {
		t.Fatalf("expected found:true")
	}
	// $200 (o1) + $50 (o2) = $250, all attributed to vip_spend at the fan level.
	if result.VIPSpend != 250.00 {
		t.Errorf("vip_spend = %v, want 250.00 (all fan orders count; VIP is a fan-level signal)", result.VIPSpend)
	}
}

func TestFanProfileNoVIPSpend(t *testing.T) {
	// fanA has only GA tickets; vip_spend should be 0.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
		},
		"tickets": {
			"t1": ticketWithHolder("t1", fanA, "GA Standard", "Tier 1", false),
		},
	})
	result, err := computeFanProfile(context.Background(), s.DB(), fanA)
	if err != nil {
		t.Fatalf("computeFanProfile noVIP: %v", err)
	}
	if result.VIPSpend != 0 {
		t.Errorf("vip_spend = %v, want 0 for GA-only fan", result.VIPSpend)
	}
}

func TestFanProfileOptedIn(t *testing.T) {
	// fanA opts in on second order; opted_in should be true.
	s := seedStore(t, map[string]map[string]string{
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-20T10:00:00Z", "evtB", "Show B", fanA, "Ann", "A", 7000, 0, 1, true, "", ""),
		},
	})
	result, err := computeFanProfile(context.Background(), s.DB(), fanA)
	if err != nil {
		t.Fatalf("computeFanProfile optedIn: %v", err)
	}
	if !result.OptedIn {
		t.Errorf("opted_in = false, want true (fan opted in on second order)")
	}
}

// --- revenue by-artist tests ---

func TestRevenueByArtistBasic(t *testing.T) {
	// evtA has two artists (ArtistX, ArtistY); evtB has one (ArtistZ).
	// By default both ArtistX and ArtistY get full evtA revenue attribution.
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evtA": eventWithArtists("evtA", "Show A", []string{"ArtistX", "ArtistY"}, nil),
			"evtB": eventWithArtists("evtB", "Show B", []string{"ArtistZ"}, nil),
		},
		"orders": {
			// evtA: 2 orders x 5000 cents each, 500 dice each.
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 500, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtA", "Show A", fanB, "Bob", "B", 5000, 500, 1, false, "", ""),
			// evtB: 1 order x 8000 cents, 800 dice.
			"o3": order("o3", "2026-01-12T10:00:00Z", "evtB", "Show B", fanC, "Cat", "C", 8000, 800, 1, false, "", ""),
		},
	})

	rows, err := computeRevenueByArtist(context.Background(), s.DB(), false, "", "")
	if err != nil {
		t.Fatalf("computeRevenueByArtist: %v", err)
	}
	// Expect 3 artist rows: ArtistX, ArtistY (each 100.00 gross from evtA), ArtistZ (80.00).
	if len(rows) != 3 {
		t.Fatalf("want 3 artist rows, got %d: %+v", len(rows), rows)
	}

	byArtist := map[string]artistRevenueRow{}
	for _, r := range rows {
		byArtist[r.Artist] = r
	}

	ax := byArtist["ArtistX"]
	if ax.Gross != 100.00 {
		t.Errorf("ArtistX gross = %v, want 100.00 (evtA double-count)", ax.Gross)
	}
	if ax.DiceFees != 10.00 {
		t.Errorf("ArtistX dice_fees = %v, want 10.00", ax.DiceFees)
	}
	if ax.Net != 90.00 {
		t.Errorf("ArtistX net = %v, want 90.00", ax.Net)
	}
	if ax.OrdersCount != 2 {
		t.Errorf("ArtistX orders_count = %d, want 2", ax.OrdersCount)
	}
	if ax.EventsCount != 1 {
		t.Errorf("ArtistX events_count = %d, want 1", ax.EventsCount)
	}

	ay := byArtist["ArtistY"]
	if ay.Gross != 100.00 {
		t.Errorf("ArtistY gross = %v, want 100.00 (same double-count as ArtistX)", ay.Gross)
	}

	az := byArtist["ArtistZ"]
	if az.Gross != 80.00 {
		t.Errorf("ArtistZ gross = %v, want 80.00", az.Gross)
	}
	if az.OrdersCount != 1 {
		t.Errorf("ArtistZ orders_count = %d, want 1", az.OrdersCount)
	}

	// Sorted by gross desc: ArtistX and ArtistY both 100.00; ArtistZ 80.00.
	// rows[0] and rows[1] should be X and Y (in alpha order), rows[2] ArtistZ.
	topGross := rows[0].Gross
	if topGross != 100.00 {
		t.Errorf("rows[0].gross = %v, want 100.00", topGross)
	}
	if rows[2].Artist != "ArtistZ" {
		t.Errorf("rows[2].artist = %q, want ArtistZ", rows[2].Artist)
	}
}

func TestRevenueByArtistHeadlinerOnly(t *testing.T) {
	// evtA: ArtistX (headliner), ArtistY (support). With --headliner-only,
	// only ArtistX gets attribution; ArtistY should not appear.
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evtA": eventWithArtists("evtA", "Show A", []string{"ArtistX", "ArtistY"}, nil),
		},
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 6000, 600, 1, false, "", ""),
		},
	})

	rows, err := computeRevenueByArtist(context.Background(), s.DB(), true, "", "")
	if err != nil {
		t.Fatalf("computeRevenueByArtist headliner: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row (headliner-only), got %d: %+v", len(rows), rows)
	}
	if rows[0].Artist != "ArtistX" {
		t.Errorf("artist = %q, want ArtistX", rows[0].Artist)
	}
	if rows[0].Gross != 60.00 {
		t.Errorf("gross = %v, want 60.00", rows[0].Gross)
	}
}

func TestRevenueByArtistShowDateWindow(t *testing.T) {
	// evtA show 2026-05-10 (in window); evtB show 2026-06-20 (outside).
	// Only ArtistX (evtA headliner) should appear.
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evtA": func() string {
				type ev struct {
					ID            string `json:"id"`
					Name          string `json:"name"`
					StartDatetime string `json:"startDatetime"`
					Artists       []struct {
						Name string `json:"name"`
					} `json:"artists"`
				}
				e := ev{ID: "evtA", Name: "Show A", StartDatetime: "2026-05-10T20:00:00Z",
					Artists: []struct {
						Name string `json:"name"`
					}{{Name: "ArtistX"}}}
				b, _ := json.Marshal(e)
				return string(b)
			}(),
			"evtB": func() string {
				type ev struct {
					ID            string `json:"id"`
					Name          string `json:"name"`
					StartDatetime string `json:"startDatetime"`
					Artists       []struct {
						Name string `json:"name"`
					} `json:"artists"`
				}
				e := ev{ID: "evtB", Name: "Show B", StartDatetime: "2026-06-20T20:00:00Z",
					Artists: []struct {
						Name string `json:"name"`
					}{{Name: "ArtistY"}}}
				b, _ := json.Marshal(e)
				return string(b)
			}(),
		},
		"orders": {
			"o1": order("o1", "2026-04-01T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 5000, 500, 1, false, "", ""),
			"o2": order("o2", "2026-04-02T10:00:00Z", "evtB", "Show B", fanB, "Bob", "B", 8000, 800, 1, false, "", ""),
		},
	})

	rows, err := computeRevenueByArtist(context.Background(), s.DB(), false, "2026-05-01", "2026-05-31")
	if err != nil {
		t.Fatalf("computeRevenueByArtist dateWindow: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row in date window, got %d: %+v", len(rows), rows)
	}
	if rows[0].Artist != "ArtistX" {
		t.Errorf("artist = %q, want ArtistX", rows[0].Artist)
	}
	if rows[0].Gross != 50.00 {
		t.Errorf("gross = %v, want 50.00", rows[0].Gross)
	}
}

func TestRevenueByArtistNoArtist(t *testing.T) {
	// An event with no artists gets a "(no artist)" bucket.
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evtA": eventWithArtists("evtA", "Show A", nil, nil),
		},
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "Ann", "A", 4000, 400, 1, false, "", ""),
		},
	})
	rows, err := computeRevenueByArtist(context.Background(), s.DB(), false, "", "")
	if err != nil {
		t.Fatalf("computeRevenueByArtist noArtist: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].Artist != "(no artist)" {
		t.Errorf("artist = %q, want '(no artist)'", rows[0].Artist)
	}
}

func TestRevenueByArtistSortedByGrossDesc(t *testing.T) {
	// Three single-artist events with distinct gross; verify sort order.
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"evtA": eventWithArtists("evtA", "Show A", []string{"LowArtist"}, nil),
			"evtB": eventWithArtists("evtB", "Show B", []string{"HighArtist"}, nil),
			"evtC": eventWithArtists("evtC", "Show C", []string{"MidArtist"}, nil),
		},
		"orders": {
			"o1": order("o1", "2026-01-10T10:00:00Z", "evtA", "Show A", fanA, "A", "A", 1000, 0, 1, false, "", ""),
			"o2": order("o2", "2026-01-11T10:00:00Z", "evtB", "Show B", fanB, "B", "B", 9000, 0, 1, false, "", ""),
			"o3": order("o3", "2026-01-12T10:00:00Z", "evtC", "Show C", fanC, "C", "C", 5000, 0, 1, false, "", ""),
		},
	})
	rows, err := computeRevenueByArtist(context.Background(), s.DB(), false, "", "")
	if err != nil {
		t.Fatalf("computeRevenueByArtist sort: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	order := []string{rows[0].Artist, rows[1].Artist, rows[2].Artist}
	want := []string{"HighArtist", "MidArtist", "LowArtist"}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("rows[%d].artist = %q, want %q", i, order[i], want[i])
		}
	}
	_ = sort.Search // keep import happy
}
