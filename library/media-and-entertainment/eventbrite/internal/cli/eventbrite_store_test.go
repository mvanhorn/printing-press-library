// Behavioral tests for the hand-authored Eventbrite store readers. Each test
// seeds a temp SQLite store with known Eventbrite-shaped fixtures and asserts
// the reader's exact output, since there is no live API token to integration
// test against. All emails use the RFC 2606 example.com domain so they can
// never resolve to a real mailbox.
package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eventbrite/internal/store"
)

const (
	buyerA = "Buyer.A@Example.com" // mixed-case on purpose: readers must lowercase + trim
	buyerB = "buyer.b@example.com"
)

// seedEBStore opens a fresh store in a temp dir and upserts the given fixtures
// (resource_type -> id -> raw JSON payload).
func seedEBStore(t *testing.T, fixtures map[string]map[string]string) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for rt, byID := range fixtures {
		for id, payload := range byID {
			if err := s.Upsert(rt, id, json.RawMessage(payload)); err != nil {
				t.Fatalf("upsert %s/%s: %v", rt, id, err)
			}
		}
	}
	return s
}

func TestReadEBOrders_DedupAndNullSafety(t *testing.T) {
	// Same order id under two sync paths (events_orders + organizations_orders)
	// must collapse to one row. A second order omits costs entirely (NULL gross)
	// and must still be returned, not silently dropped.
	s := seedEBStore(t, map[string]map[string]string{
		"events_orders": {
			"1001": `{"id":"1001","created":"2026-05-01T10:00:00Z","status":"placed","email":"` + buyerA + `","event_id":"E1","costs":{"gross":{"value":2500,"currency":"USD"}}}`,
		},
		"organizations_orders": {
			"1001": `{"id":"1001","created":"2026-05-01T10:00:00Z","status":"placed","email":"` + buyerA + `","event_id":"E1","costs":{"gross":{"value":2500,"currency":"USD"}}}`,
			"1002": `{"id":"1002","created":"2026-05-02T10:00:00Z","status":"refunded","email":"` + buyerB + `","event_id":"E1"}`,
		},
	})
	orders, err := readEBOrders(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("readEBOrders: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("want 2 deduped orders, got %d: %+v", len(orders), orders)
	}
	byID := map[string]ebOrder{}
	for _, o := range orders {
		byID[o.ID] = o
	}
	if got := byID["1001"]; got.GrossMinor != 2500 || got.Email != "buyer.a@example.com" {
		t.Errorf("order 1001: gross/email wrong: %+v", got)
	}
	if got := byID["1002"]; got.GrossMinor != 0 || got.Status != "refunded" {
		t.Errorf("order 1002 (NULL costs) should survive with gross 0: %+v", got)
	}
}

func TestReadEBAttendees_CheckinAndNameFallback(t *testing.T) {
	s := seedEBStore(t, map[string]map[string]string{
		"events_attendees": {
			"A1": `{"id":"A1","checked_in":true,"status":"Attending","profile":{"email":"` + buyerA + `","name":"Ada Lovelace"},"event_id":"E1","ticket_class_name":"VIP","costs":{"gross":{"value":5000}}}`,
			"A2": `{"id":"A2","checked_in":false,"status":"Attending","profile":{"email":"` + buyerB + `","first_name":"Grace","last_name":"Hopper"},"event_id":"E1"}`,
		},
	})
	att, err := readEBAttendees(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("readEBAttendees: %v", err)
	}
	if len(att) != 2 {
		t.Fatalf("want 2 attendees, got %d", len(att))
	}
	byID := map[string]ebAttendee{}
	for _, a := range att {
		byID[a.ID] = a
	}
	if got := byID["A1"]; !got.CheckedIn || got.Name != "Ada Lovelace" || got.Email != "buyer.a@example.com" {
		t.Errorf("A1 wrong: %+v", got)
	}
	if got := byID["A2"]; got.CheckedIn || got.Name != "Grace Hopper" {
		t.Errorf("A2 should derive name from first+last and not be checked in: %+v", got)
	}
}

func TestReadEBEvents_NestedNameExtraction(t *testing.T) {
	s := seedEBStore(t, map[string]map[string]string{
		"organizations_events": {
			"E1": `{"id":"E1","name":{"text":"Summer Showcase"},"start":{"utc":"2026-07-01T19:00:00Z"},"status":"live","currency":"USD","organization_id":"ORG9","capacity":500}`,
		},
	})
	events, err := readEBEvents(context.Background(), s.DB())
	if err != nil {
		t.Fatalf("readEBEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Name != "Summer Showcase" || e.OrgID != "ORG9" || e.Capacity != 500 || e.Status != "live" {
		t.Errorf("event extraction wrong: %+v", e)
	}
}
