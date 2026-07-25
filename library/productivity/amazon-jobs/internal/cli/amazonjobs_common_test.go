// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the amazon-jobs shared helpers.

package cli

import (
	"strings"
	"testing"
)

func ptrBool(b bool) *bool { return &b }

func TestBuildSearchValues(t *testing.T) {
	tests := []struct {
		name                                 string
		query, country, state, city, sort    string
		limit, offset                        int
		wantQuery, wantSort, wantLimit       string
		wantCityKey                          string
		wantCityPresent                      bool
	}{
		{
			name: "full filters", query: "sde", country: "USA", state: "Washington", city: "Seattle",
			sort: "relevant", limit: 25, offset: 50,
			wantQuery: "sde", wantSort: "relevant", wantLimit: "25",
			wantCityKey: "normalized_city_name[]", wantCityPresent: true,
		},
		{
			name: "defaults applied", query: "aws", limit: 0, offset: -5,
			wantQuery: "aws", wantSort: "recent", wantLimit: "20",
			wantCityKey: "normalized_city_name[]", wantCityPresent: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := buildSearchValues(tt.query, tt.country, tt.state, tt.city, tt.sort, tt.limit, tt.offset)
			if got := v.Get("base_query"); got != tt.wantQuery {
				t.Errorf("base_query = %q, want %q", got, tt.wantQuery)
			}
			if got := v.Get("sort"); got != tt.wantSort {
				t.Errorf("sort = %q, want %q", got, tt.wantSort)
			}
			if got := v.Get("result_limit"); got != tt.wantLimit {
				t.Errorf("result_limit = %q, want %q", got, tt.wantLimit)
			}
			_, present := v[tt.wantCityKey]
			if present != tt.wantCityPresent {
				t.Errorf("city key %q present = %v, want %v", tt.wantCityKey, present, tt.wantCityPresent)
			}
			// offset must never serialize negative.
			if got := v.Get("offset"); got == "-5" {
				t.Errorf("offset leaked negative value %q", got)
			}
		})
	}
}

func TestEffectiveBool(t *testing.T) {
	if effectiveBool(nil) {
		t.Error("nil should be false")
	}
	if effectiveBool(ptrBool(false)) {
		t.Error("*false should be false")
	}
	if !effectiveBool(ptrBool(true)) {
		t.Error("*true should be true")
	}
}

func TestBoolFlag(t *testing.T) {
	if boolFlag(false, true) != nil {
		t.Error("unchanged flag must yield nil")
	}
	p := boolFlag(true, false)
	if p == nil || *p != false {
		t.Errorf("changed flag must yield &false, got %v", p)
	}
}

func TestMatchesClientFilters(t *testing.T) {
	sde := Job{JobCategory: "Software Development", BusinessCategory: "aws", JobScheduleType: "Full-Time", IsManager: ptrBool(false), IsIntern: nil}
	mgr := Job{JobCategory: "Software Development", IsManager: ptrBool(true)}
	intern := Job{JobCategory: "Software Development", IsIntern: ptrBool(true)}

	tests := []struct {
		name                                        string
		job                                         Job
		category, schedule                          string
		wantIntern, wantManager, wantUniversity     *bool
		want                                        bool
	}{
		{name: "category substring match", job: sde, category: "software", want: true},
		{name: "category business match", job: sde, category: "aws", want: true},
		{name: "category no match", job: sde, category: "hardware", want: false},
		{name: "schedule match", job: sde, schedule: "Full-Time", want: true},
		{name: "schedule mismatch", job: sde, schedule: "Part-Time", want: false},
		{name: "manager=false keeps non-manager", job: sde, wantManager: ptrBool(false), want: true},
		{name: "manager=false excludes manager", job: mgr, wantManager: ptrBool(false), want: false},
		{name: "manager=true keeps manager", job: mgr, wantManager: ptrBool(true), want: true},
		{name: "null is_manager treated as non-manager", job: Job{IsManager: nil}, wantManager: ptrBool(false), want: true},
		{name: "intern=true keeps intern", job: intern, wantIntern: ptrBool(true), want: true},
		{name: "intern=false excludes intern", job: intern, wantIntern: ptrBool(false), want: false},
		{name: "no filters matches all", job: sde, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesClientFilters(tt.job, tt.category, tt.schedule, tt.wantIntern, tt.wantManager, tt.wantUniversity)
			if got != tt.want {
				t.Errorf("matchesClientFilters = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasClientFilters(t *testing.T) {
	if hasClientFilters("", "", nil, nil, nil) {
		t.Error("no filters should be false")
	}
	if !hasClientFilters("software", "", nil, nil, nil) {
		t.Error("category should count as a filter")
	}
	if !hasClientFilters("", "", nil, ptrBool(false), nil) {
		t.Error("manager filter should count")
	}
}

func TestCleanJobStripsHTML(t *testing.T) {
	j := Job{
		Title:               "SDE",
		Description:         "line one<br/>line two",
		BasicQualifications: "3+ years&#39; experience",
	}
	got := cleanJob(j)
	if got.Description == j.Description {
		t.Error("expected HTML <br/> to be stripped from description")
	}
	if wantContains := "experience"; !strings.Contains(got.BasicQualifications, wantContains) {
		t.Errorf("cleaned quals %q should still contain %q", got.BasicQualifications, wantContains)
	}
}
