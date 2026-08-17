// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/cliutil"
)

func TestResolveCityVenues(t *testing.T) {
	tests := []struct {
		name        string
		city        string
		wantNil     bool
		wantContain string
	}{
		{name: "seattle", city: "seattle", wantContain: "lumen-field"},
		{name: "trim case", city: " Seattle ", wantContain: "lumen-field"},
		{name: "nyc alias", city: "nyc", wantContain: "madison-square-garden"},
		{name: "la alias", city: "la", wantContain: "sofi-stadium"},
		{name: "unknown", city: "nowhereville", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveCityVenues(tt.city)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("resolveCityVenues(%q) = %v, want nil", tt.city, got)
				}
				return
			}
			if !containsString(got, tt.wantContain) {
				t.Fatalf("resolveCityVenues(%q) = %v, want %q", tt.city, got, tt.wantContain)
			}
		})
	}
}

func TestAggregateCitySearchResultsDedupeFilterAndFailures(t *testing.T) {
	results := []cliutil.FanoutResult[[]searchEvent]{
		{
			Source: "lumen-field",
			Value: []searchEvent{
				{ID: json.Number("2001"), Title: "Seattle Seahawks vs Rams", VenueSlug: "lumen-field", CategoryType: "SPORT", EventCategoryName: "NFL Football", GetInPrice: 250, ThreeDayChangePct: 4},
				{ID: json.Number("2002"), Title: "Lumen Field Tours", VenueSlug: "lumen-field", CategoryType: "SPORT", EventCategoryName: "Stadium Tours", GetInPrice: 500, ThreeDayChangePct: 1},
			},
		},
		{
			Source: "alaska-airlines-arena",
			Value: []searchEvent{
				{ID: json.Number("2001"), Title: "Duplicate Seahawks Row", VenueSlug: "lumen-field", CategoryType: "SPORT", EventCategoryName: "NFL Football", GetInPrice: 250, ThreeDayChangePct: 4},
				{ID: json.Number("2003"), Title: "Washington Huskies Basketball", VenueSlug: "alaska-airlines-arena", CategoryType: "SPORT", EventCategoryName: "College Basketball", GetInPrice: 300, ThreeDayChangePct: -12},
				{ID: json.Number("2004"), Title: "Arena Concert", VenueSlug: "alaska-airlines-arena", CategoryType: "CONCERT", EventCategoryName: "Concert", GetInPrice: 350, ThreeDayChangePct: 20},
			},
		},
	}
	errs := []cliutil.FanoutError{{Source: "broken-venue", Err: errors.New("timeout")}}

	events, failures := aggregateCitySearchResults(results, errs)
	if got, want := len(events), 4; got != want {
		t.Fatalf("deduped events len = %d, want %d", got, want)
	}
	if got, want := failures, []cityFetchFailure{{Venue: "broken-venue", Error: "timeout"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("failures = %+v, want %+v", got, want)
	}

	events = filterSearchEvents(events, searchFilters{MinGetIn: 200, GamesOnly: true})
	sortSearchEvents(events, "get_in")
	if got, want := searchEventIDs(events), []string{"2003", "2001"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered get_in ids = %v, want %v", got, want)
	}
	sortSearchEvents(events, "movers")
	if got, want := searchEventIDs(events), []string{"2003", "2001"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("movers ids = %v, want %v", got, want)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
