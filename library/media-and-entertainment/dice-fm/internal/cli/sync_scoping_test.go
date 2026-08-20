// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

func TestBuildWhereEvents(t *testing.T) {
	tests := []struct {
		name string
		f    syncFilters
		want map[string]any
	}{
		{
			name: "startDatetime window",
			f:    syncFilters{EventsFrom: "2026-01-01T00:00:00Z", EventsTo: "2027-01-01T00:00:00Z", EventsDateField: "startDatetime"},
			want: map[string]any{"startDatetime": map[string]any{"gte": "2026-01-01T00:00:00Z", "lt": "2027-01-01T00:00:00Z"}},
		},
		{
			name: "from only",
			f:    syncFilters{EventsFrom: "2026-01-01T00:00:00Z", EventsDateField: "startDatetime"},
			want: map[string]any{"startDatetime": map[string]any{"gte": "2026-01-01T00:00:00Z"}},
		},
		{
			name: "to only",
			f:    syncFilters{EventsTo: "2027-01-01T00:00:00Z", EventsDateField: "startDatetime"},
			want: map[string]any{"startDatetime": map[string]any{"lt": "2027-01-01T00:00:00Z"}},
		},
		{
			name: "onSaleDatetime field",
			f:    syncFilters{EventsFrom: "2026-01-01T00:00:00Z", EventsDateField: "onSaleDatetime"},
			want: map[string]any{"onSaleDatetime": map[string]any{"gte": "2026-01-01T00:00:00Z"}},
		},
		{
			name: "updatedAt field",
			f:    syncFilters{EventsFrom: "2026-01-01T00:00:00Z", EventsDateField: "updatedAt"},
			want: map[string]any{"updatedAt": map[string]any{"gte": "2026-01-01T00:00:00Z"}},
		},
		{
			name: "sinceTS back-compat",
			f:    syncFilters{SinceTS: "2026-05-01T00:00:00Z"},
			want: map[string]any{"updatedAt": map[string]any{"gte": "2026-05-01T00:00:00Z"}},
		},
		{
			name: "no filters",
			f:    syncFilters{},
			want: nil,
		},
		{
			name: "events window wins over sinceTS",
			f:    syncFilters{SinceTS: "2026-05-01T00:00:00Z", EventsFrom: "2026-01-01T00:00:00Z", EventsDateField: "startDatetime"},
			want: map[string]any{"startDatetime": map[string]any{"gte": "2026-01-01T00:00:00Z"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildWhere("events", tt.f)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildWhere(events, %+v) = %v, want %v", tt.f, got, tt.want)
			}
		})
	}
}

func TestBuildWhereTicketsOrders(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		f        syncFilters
		want     map[string]any
	}{
		{
			name:     "orders by event ids",
			resource: "orders",
			f:        syncFilters{EventIDs: []string{"evt-1", "evt-2"}},
			want:     map[string]any{"eventId": map[string]any{"in": []string{"evt-1", "evt-2"}}},
		},
		{
			name:     "tickets by event ids",
			resource: "tickets",
			f:        syncFilters{EventIDs: []string{"evt-1", "evt-2"}},
			want:     map[string]any{"eventId": map[string]any{"in": []string{"evt-1", "evt-2"}}},
		},
		{
			name:     "orders since only",
			resource: "orders",
			f:        syncFilters{SinceTS: "2026-05-01T00:00:00Z"},
			want:     map[string]any{"purchasedAt": map[string]any{"gte": "2026-05-01T00:00:00Z"}},
		},
		{
			name:     "tickets since only is nil (no date field)",
			resource: "tickets",
			f:        syncFilters{SinceTS: "2026-05-01T00:00:00Z"},
			want:     nil,
		},
		{
			name:     "orders prefer eventId over sinceTS",
			resource: "orders",
			f:        syncFilters{SinceTS: "2026-05-01T00:00:00Z", EventIDs: []string{"evt-9"}},
			want:     map[string]any{"eventId": map[string]any{"in": []string{"evt-9"}}},
		},
		{
			name:     "orders no filters nil",
			resource: "orders",
			f:        syncFilters{},
			want:     nil,
		},
		{
			name:     "tickets no filters nil",
			resource: "tickets",
			f:        syncFilters{},
			want:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildWhere(tt.resource, tt.f)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildWhere(%s, %+v) = %v, want %v", tt.resource, tt.f, got, tt.want)
			}
		})
	}
}

func TestBuildWhereReturnsTransfers(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		f        syncFilters
		want     map[string]any
	}{
		{
			name:     "returns gte back-compat",
			resource: "returns",
			f:        syncFilters{SinceTS: "2026-05-01T00:00:00Z"},
			want:     map[string]any{"returnedAt": map[string]any{"gte": "2026-05-01T00:00:00Z"}},
		},
		{
			name:     "returns no since nil",
			resource: "returns",
			f:        syncFilters{},
			want:     nil,
		},
		{
			name:     "transfers gte back-compat",
			resource: "transfers",
			f:        syncFilters{SinceTS: "2026-05-01T00:00:00Z"},
			want:     map[string]any{"transferredAt": map[string]any{"gte": "2026-05-01T00:00:00Z"}},
		},
		{
			name:     "transfers no since nil",
			resource: "transfers",
			f:        syncFilters{},
			want:     nil,
		},
		{
			name:     "unknown resource nil",
			resource: "genres",
			f:        syncFilters{SinceTS: "2026-05-01T00:00:00Z"},
			want:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildWhere(tt.resource, tt.f)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildWhere(%s, %+v) = %v, want %v", tt.resource, tt.f, got, tt.want)
			}
		})
	}
}

func TestParseFlexibleDate(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "date only", in: "2026-01-01", want: "2026-01-01T00:00:00Z"},
		{name: "rfc3339 passthrough", in: "2026-01-01T12:30:00Z", want: "2026-01-01T12:30:00Z"},
		{name: "rfc3339 with offset", in: "2026-01-01T12:30:00+02:00", want: "2026-01-01T12:30:00+02:00"},
		{name: "empty", in: "", want: ""},
		{name: "bad", in: "not-a-date", wantErr: true},
		{name: "bad month", in: "2026-13-01", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFlexibleDate(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseFlexibleDate(%q) err = nil, want error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFlexibleDate(%q) unexpected err: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("parseFlexibleDate(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEventsDateFieldValidation(t *testing.T) {
	for _, field := range []string{"startDatetime", "onSaleDatetime", "updatedAt"} {
		if err := validateEventsDateField(field); err != nil {
			t.Fatalf("validateEventsDateField(%q) = %v, want nil", field, err)
		}
	}
	if err := validateEventsDateField("bogus"); err == nil {
		t.Fatalf("validateEventsDateField(\"bogus\") = nil, want error")
	}
}
