// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestEventsCandyCrush(t *testing.T) {
	f := newS2AFake(t)
	rows := s2aJSONArray(t, f.mustOK("events", "553834731", "--json").stdout)
	if len(rows) != 1 {
		t.Fatalf("events = %d, want 1", len(rows))
	}
	e := rows[0]
	want := map[string]any{
		"eventId": "6805077554", "kind": "MAJOR UPDATE", "startDate": "2026-10-12T07:00:00Z",
		"endDate": "2026-11-07T23:00:00Z", "badge": "available", "status": "live", "storefront": "us",
	}
	for k, w := range want {
		if e[k] != w {
			t.Fatalf("event %s = %v, want %v (row %v)", k, e[k], w, e)
		}
	}
	if title, _ := e["title"].(string); !strings.Contains(title, "Spooky") {
		t.Fatalf("title = %q", title)
	}
	stored, err := store.EventsHistory(context.Background(), f.openStore(), "553834731", "us")
	if err != nil || len(stored) != 1 || stored[0].EventID != "6805077554" {
		t.Fatalf("stored events = %+v (%v)", stored, err)
	}

	soda := s2aJSONArray(t, f.mustOK("events", "850417475", "--json").stdout)
	if len(soda) != 2 {
		t.Fatalf("soda events = %d, want 2", len(soda))
	}
}

func TestEventStatusFollowsTheClock(t *testing.T) {
	cases := []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "upcoming"},
		{time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), "live"},
		{time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), "ended"},
	}
	for _, tc := range cases {
		if got := s2aEventStatus("2026-10-12T07:00:00Z", "2026-11-07T23:00:00Z", tc.now); got != tc.want {
			t.Errorf("status at %s = %s, want %s", tc.now, got, tc.want)
		}
	}
	if got := s2aEventStatus("", "", time.Now()); got != "unknown" {
		t.Fatalf("no dates = %s, want unknown", got)
	}
}

func TestEventsNoneIsEmptyWithNote(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("events", "324684580", "--json")
	if strings.TrimSpace(res.stdout) != "[]" {
		t.Fatalf("Spotify has no events: want [], got %s", res.stdout)
	}
	if !strings.Contains(res.stderr, "note: no in-app events are listed") {
		t.Fatalf("missing note: %q", res.stderr)
	}
	if res := f.run("events", "1", "--json"); res.code != 3 {
		t.Fatalf("events 1 exit = %d, want 3", res.code)
	}
}
