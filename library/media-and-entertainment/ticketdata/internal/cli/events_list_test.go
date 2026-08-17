// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

const searchEventsFixture = `{
  "status": "success",
  "data": {
    "events": {
      "upcoming": [
        {
          "id": 1001,
          "title": "Seattle Seahawks vs Arizona Cardinals",
          "performer": "Seattle Seahawks",
          "performer_slug": "seattle-seahawks",
          "venue": "Lumen Field",
          "venue_slug": "lumen-field",
          "city": "Seattle",
          "state": "WA",
          "date": "Sat, 08/15/2026 05:00 PM",
          "category_type": "SPORT",
          "event_category_name": "NFL Football",
          "get_in_price": 250,
          "3day_price_change": {"percent": -5.5}
        },
        {
          "id": 1002,
          "title": "Lumen Field Tours",
          "performer": "Lumen Field",
          "performer_slug": "lumen-field",
          "venue": "Lumen Field",
          "venue_slug": "lumen-field",
          "city": "Seattle",
          "state": "WA",
          "date": "Sun, 08/16/2026 10:00 AM",
          "category_type": "SPORT",
          "event_category_name": "Stadium Tours",
          "get_in_price": 40,
          "3day_price_change": {"percent": 1.0}
        },
        {
          "id": 1003,
          "title": "Leagues Cup (If Necessary)",
          "performer": "Seattle Sounders FC",
          "performer_slug": "seattle-sounders-fc",
          "venue": "Lumen Field",
          "venue_slug": "lumen-field",
          "city": "Seattle",
          "state": "WA",
          "date": "Mon, 08/17/2026 07:30 PM",
          "category_type": "SPORT",
          "event_category_name": "Soccer",
          "get_in_price": "210",
          "3day_price_change": {"percent": "11.2"}
        }
      ],
      "past": [
        {
          "id": 9001,
          "title": "Seattle Brewfest",
          "performer": "Seattle Brewfest",
          "performer_slug": "seattle-brewfest",
          "venue": "Lumen Field",
          "venue_slug": "lumen-field",
          "city": "Seattle",
          "state": "WA",
          "date": "Sat, 06/13/2026 12:00 PM",
          "category_type": "THEATER",
          "event_category_name": "Festival",
          "get_in_price": 90,
          "3day_price_change": {"percent": 0}
        }
      ],
      "all": [
        {
          "id": 1001,
          "title": "Seattle Seahawks vs Arizona Cardinals",
          "performer": "Seattle Seahawks",
          "performer_slug": "seattle-seahawks",
          "venue": "Lumen Field",
          "venue_slug": "lumen-field",
          "city": "Seattle",
          "state": "WA",
          "date": "Sat, 08/15/2026 05:00 PM",
          "category_type": "SPORT",
          "event_category_name": "NFL Football",
          "get_in_price": 250,
          "3day_price_change": {"percent": -5.5}
        },
        {
          "id": 1002,
          "title": "Lumen Field Tours",
          "performer": "Lumen Field",
          "performer_slug": "lumen-field",
          "venue": "Lumen Field",
          "venue_slug": "lumen-field",
          "city": "Seattle",
          "state": "WA",
          "date": "Sun, 08/16/2026 10:00 AM",
          "category_type": "SPORT",
          "event_category_name": "Stadium Tours",
          "get_in_price": 40,
          "3day_price_change": {"percent": 1.0}
        },
        {
          "id": 1003,
          "title": "Leagues Cup (If Necessary)",
          "performer": "Seattle Sounders FC",
          "performer_slug": "seattle-sounders-fc",
          "venue": "Lumen Field",
          "venue_slug": "lumen-field",
          "city": "Seattle",
          "state": "WA",
          "date": "Mon, 08/17/2026 07:30 PM",
          "category_type": "SPORT",
          "event_category_name": "Soccer",
          "get_in_price": "210",
          "3day_price_change": {"percent": "11.2"}
        },
        {
          "id": 9001,
          "title": "Seattle Brewfest",
          "performer": "Seattle Brewfest",
          "performer_slug": "seattle-brewfest",
          "venue": "Lumen Field",
          "venue_slug": "lumen-field",
          "city": "Seattle",
          "state": "WA",
          "date": "Sat, 06/13/2026 12:00 PM",
          "category_type": "THEATER",
          "event_category_name": "Festival",
          "get_in_price": 90,
          "3day_price_change": {"percent": 0}
        }
      ]
    },
    "metadata": {
      "categorization": {
        "upcoming_count": 3,
        "past_count": 1,
        "total_matches": 4
      }
    }
  }
}`

func TestParseSearchEventsUpcomingMetaAndConditional(t *testing.T) {
	events, meta, err := parseSearchEvents(json.RawMessage(searchEventsFixture), "upcoming")
	if err != nil {
		t.Fatalf("parseSearchEvents error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("upcoming len = %d, want 3", len(events))
	}
	if meta.UpcomingCount != 3 || meta.PastCount != 1 || meta.TotalMatches != 4 {
		t.Fatalf("meta = %+v, want upcoming=3 past=1 total=4", meta)
	}
	if !events[2].Conditional {
		t.Fatalf("If Necessary row should be marked conditional: %+v", events[2])
	}
	if events[2].GetInPrice != 210 || events[2].ThreeDayChangePct != 11.2 {
		t.Fatalf("string numeric fields not decoded: %+v", events[2])
	}
}

func TestSearchEventsFiltersAndSorts(t *testing.T) {
	base, _, err := parseSearchEvents(json.RawMessage(searchEventsFixture), "upcoming")
	if err != nil {
		t.Fatalf("parseSearchEvents error = %v", err)
	}

	tests := []struct {
		name    string
		filters searchFilters
		sortBy  string
		wantIDs []string
	}{
		{
			name:    "games only excludes tours",
			filters: searchFilters{GamesOnly: true},
			sortBy:  "get_in",
			wantIDs: []string{"1001", "1003"},
		},
		{
			name:    "min get in bound",
			filters: searchFilters{MinGetIn: 200},
			sortBy:  "get_in",
			wantIDs: []string{"1001", "1003"},
		},
		{
			name:    "category contains",
			filters: searchFilters{Category: "soccer"},
			sortBy:  "get_in",
			wantIDs: []string{"1003"},
		},
		{
			name:    "movers uses absolute three day change",
			filters: searchFilters{},
			sortBy:  "movers",
			wantIDs: []string{"1003", "1001", "1002"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := append([]searchEvent(nil), base...)
			events = filterSearchEvents(events, tt.filters)
			sortSearchEvents(events, tt.sortBy)
			if got := searchEventIDs(events); !reflect.DeepEqual(got, tt.wantIDs) {
				t.Fatalf("ids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestParseSearchEventsAllTabIncludesPast(t *testing.T) {
	events, _, err := parseSearchEvents(json.RawMessage(searchEventsFixture), "all")
	if err != nil {
		t.Fatalf("parseSearchEvents all error = %v", err)
	}
	if got, want := len(events), 4; got != want {
		t.Fatalf("all len = %d, want %d", got, want)
	}
}

func searchEventIDs(events []searchEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID.String())
	}
	return ids
}
