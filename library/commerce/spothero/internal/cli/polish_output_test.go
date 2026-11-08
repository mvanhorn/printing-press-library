// Copyright 2026 Amandeep Khurana and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/consumer"
)

// The reservation list runs newest start first and includes future bookings, so the
// first row is usually a trip that has not happened yet.
func TestLatestPastReservationSkipsUpcoming(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	list := []consumer.Reservation{
		{ID: "future", Starts: "2026-12-01T09:00:00Z"},
		{ID: "unparseable", Starts: "soon"},
		{ID: "past", Starts: "2026-11-01T09:00:00Z"},
		{ID: "older", Starts: "2026-10-01T09:00:00Z"},
	}
	got, ok := latestPastReservation(list, now)
	if !ok || got.ID != "past" {
		t.Fatalf("got %q, %v", got.ID, ok)
	}
	if _, ok := latestPastReservation(list[:1], now); ok {
		t.Fatal("only upcoming rows must report no past reservation")
	}
}

// Synced rows carry the name inside the nested facility object; stall_name is often empty.
func TestRankRegularsTakesTitleFromFacility(t *testing.T) {
	rows := []json.RawMessage{
		json.RawMessage(`{"facility_id":1,"stall_name":"","starts":"2026-01-01T00:00:00Z","facility":{"title":"Nested Garage"}}`),
		json.RawMessage(`{"facility_id":2,"facility_title":"Flat Garage","starts":"2026-01-01T00:00:00Z"}`),
		json.RawMessage(`{"facility_id":3,"stall_name":"Stall Garage","starts":"2026-01-01T00:00:00Z"}`),
	}
	want := map[int]string{1: "Nested Garage", 2: "Flat Garage", 3: "Stall Garage"}
	for _, g := range rankRegulars(rows) {
		if g.Title != want[g.FacilityID] {
			t.Errorf("facility %d: title %q, want %q", g.FacilityID, g.Title, want[g.FacilityID])
		}
	}
}

// --agent compaction must not hide why a check failed or why a refund lookup is missing.
func TestCompactionKeepsPreflightAndRefundableDetail(t *testing.T) {
	cases := []struct {
		name string
		v    any
		keep []string
		want []string
	}{
		{"preflight", map[string]any{"ready": false, "placed": false, "checks": []preflightCheck{{"max_total", false, "quote over cap"}}}, preflightCompactKeep, []string{"quote over cap", "max_total"}},
		{"refundable", map[string]any{"count": 1, "reservations": []map[string]any{{"reservation_id": "5", "error": "lookup failed"}, {"reservation_id": "6", "refund_as_credit": true, "refund_to_original_payment": false}}}, refundableCompactKeep, []string{"lookup failed", "refund_as_credit", "refund_to_original_payment"}},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		flags := &rootFlags{asJSON: true, compact: true}
		if err := printJSONFilteredKeep(&buf, tc.v, flags, tc.keep...); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		for _, w := range tc.want {
			if !strings.Contains(buf.String(), w) {
				t.Errorf("%s: compact output lost %q: %s", tc.name, w, buf.String())
			}
		}
	}
}
