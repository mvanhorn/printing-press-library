// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package jbo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceSeatDisplaysRemainConservative(t *testing.T) {
	for _, tc := range []struct {
		raw, status  string
		exact, lower any
	}{
		{"2 seats left", "available", nil, 2},
		{"1 seat left", "available", nil, 1},
		{"0 seats left", "sold_out", 0, nil},
		{"999999999999999999999999 seats left", "unknown", nil, nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			a := AvailabilityOf(tc.raw)
			if a.Status != tc.status || a.Exact != tc.exact || a.LowerBound != tc.lower {
				t.Fatalf("availability = %+v", a)
			}
		})
	}
	if got := PartyEvidence(AvailabilityOf("2 seats left"), 3, 0)["status"]; got != "unknown" {
		t.Fatalf("a larger party was ruled out from a lower bound: %v", got)
	}
}

func TestHighlightedNoticesDoNotDiscardBookableInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "CourseSearch") {
			fmt.Fprint(w, `<div id="122001600010"><a href="/en/Detail/12200160001/0/1/2/">Select</a></div>`)
			return
		}
		fmt.Fprint(w, `<div class="text_strong">Past announcement: 1/1/2025 and 1/2/2025</div>
			<div class="text_strong">Bookable Days of Operation<br>10/2/2026 ～ 12/2/2026</div>
			<input id="SelectDate" value="10/10/2026">
			<div data-route="0001" data-coursecd="12200160001" data-updownflg="0" data-depdate="20261010" data-deptime="2325" data-arrdate="20261011" data-arrtime="0705">2 seats left</div>`)
	}))
	defer server.Close()
	c, _ := New("en", 0)
	c.Base = server.URL
	out, rows, _, err := c.Services(context.Background(), "12200160001", 0, "2026-10-10")
	if err != nil || len(rows) != 1 || out["status"] != "inventory_reported" {
		t.Fatalf("valid requested-day inventory discarded: %v, %v", out, err)
	}
	window := out["sales_window"].([]string)
	if len(window) != 2 || window[0] != "2026-10-02" || window[1] != "2026-12-02" {
		t.Fatalf("unrelated highlighted date became the sale window: %v", window)
	}
}
