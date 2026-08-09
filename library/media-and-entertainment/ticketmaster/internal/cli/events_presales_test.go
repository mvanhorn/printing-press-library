// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFlattenPresalesClassifiesAndFilters(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	event := ticketmasterEvent{ID: "event-1", Name: "The Example"}
	event.Dates.Start.LocalDate = "2026-08-01"
	event.Sales.Presales = []ticketmasterPresale{
		{Name: "Open", URL: "https://example.com/open", StartDateTime: "2026-07-24T10:00:00Z", EndDateTime: "2026-07-25T10:00:00Z"},
		{Name: "Soon", URL: "https://example.com/soon", StartDateTime: "2026-07-25T00:00:00Z", EndDateTime: "2026-07-26T00:00:00Z"},
		{Name: "Later", URL: "https://example.com/later", StartDateTime: "2026-07-30T00:00:00Z", EndDateTime: "2026-07-31T00:00:00Z"},
		{Name: "Ended", URL: "https://example.com/ended", StartDateTime: "2026-07-20T00:00:00Z", EndDateTime: "2026-07-21T00:00:00Z"},
		{Name: "Undated", URL: "https://example.com/undated"},
	}

	all := flattenPresales([]ticketmasterEvent{event}, now, "all", 0)
	if len(all) != 5 {
		t.Fatalf("len(all) = %d, want 5", len(all))
	}
	if all[0].Status != "ended" || all[1].Status != "open" || all[2].Status != "upcoming" || all[4].Status != "unknown" {
		t.Fatalf("unexpected statuses/order: %#v", all)
	}
	soon := flattenPresales([]ticketmasterEvent{event}, now, "upcoming", 24)
	if len(soon) != 1 || soon[0].Presale != "Soon" || soon[0].HoursToOpen == nil || *soon[0].HoursToOpen != 12 {
		t.Fatalf("unexpected upcoming result: %#v", soon)
	}
}

func TestSelectCheckoutPresalePrefersOpenThenUpcoming(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	presales := []ticketmasterPresale{
		{Name: "Future", URL: "https://example.com/future", StartDateTime: "2026-07-25T00:00:00Z"},
		{Name: "Artist Presale", URL: "https://example.com/open", StartDateTime: "2026-07-24T00:00:00Z", EndDateTime: "2026-07-25T00:00:00Z"},
	}
	selected, status := selectCheckoutPresale(presales, "", now)
	if selected.Name != "Artist Presale" || status != "open" {
		t.Fatalf("selected %q (%s), want open Artist Presale", selected.Name, status)
	}
	selected, status = selectCheckoutPresale(presales, "future", now)
	if selected.Name != "Future" || status != "upcoming" {
		t.Fatalf("selected %q (%s), want upcoming Future", selected.Name, status)
	}
}

func TestValidateCheckoutURL(t *testing.T) {
	for _, raw := range []string{
		"", "http://example.com", "javascript:alert(1)",
		"https://user:pass@example.com", "https://example.com/event/123",
		"https://ticketmaster.evil.example/event/123",
		"https://not-ticketmaster.com/event/123",
	} {
		if err := validateCheckoutURL(raw); err == nil {
			t.Fatalf("validateCheckoutURL(%q) unexpectedly passed", raw)
		}
	}
	for _, raw := range []string{
		"https://www.ticketmaster.com/event/123",
		"https://ticketmaster.co.uk/event/123",
		"https://tickets.universe.com/event/123",
		"https://www.frontgatetickets.com/event/123",
	} {
		if err := validateCheckoutURL(raw); err != nil {
			t.Fatalf("valid URL %q failed: %v", raw, err)
		}
	}
}

func TestEventsPresalesQueriesRealPresaleRange(t *testing.T) {
	var received http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events" {
			t.Fatalf("path = %q, want /events", r.URL.Path)
		}
		if got := r.URL.Query().Get("preSaleDateTime"); !strings.Contains(got, ",") {
			t.Fatalf("preSaleDateTime = %q, want start,end range", got)
		}
		if _, present := r.URL.Query()["city"]; present {
			t.Fatal("empty city parameter must not be sent")
		}
		if got := r.URL.Query().Get("keyword"); got != "Example" {
			t.Fatalf("keyword = %q, want Example", got)
		}
		received = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"_embedded":{"events":[{"id":"e1","name":"Example","url":"https://www.ticketmaster.com/e1","dates":{"start":{"localDate":"2026-08-01"}},"sales":{"presales":[{"name":"Artist Presale","startDateTime":"2099-01-01T00:00:00Z","url":"https://www.ticketmaster.com/e1"}]}}]}}`))
	}))
	defer server.Close()

	t.Setenv("TICKETMASTER_BASE_URL", server.URL)
	t.Setenv("TICKETMASTER_API_KEY", "test-key")
	t.Setenv("PRINTING_PRESS_HOME", t.TempDir())
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"events", "presales", "--keyword", "Example", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	var got []presaleResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if len(got) != 1 || got[0].Presale != "Artist Presale" || got[0].Status != "upcoming" {
		t.Fatalf("unexpected output: %#v", got)
	}
	if received == nil {
		t.Fatal("mock server did not receive request")
	}
}
