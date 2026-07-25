// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the amazon-jobs shared helpers.

package cli

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func ptrBool(b bool) *bool { return &b }

func TestBuildSearchValues(t *testing.T) {
	tests := []struct {
		name                              string
		query, country, state, city, sort string
		limit, offset                     int
		wantQuery, wantSort, wantLimit    string
		wantCityKey                       string
		wantCityPresent                   bool
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

func TestClientFiltersMatches(t *testing.T) {
	sde := Job{JobCategory: "Software Development", BusinessCategory: "aws", JobScheduleType: "Full-Time", IsManager: ptrBool(false), IsIntern: nil}
	mgr := Job{JobCategory: "Software Development", IsManager: ptrBool(true)}
	intern := Job{JobCategory: "Software Development", IsIntern: ptrBool(true)}

	tests := []struct {
		name    string
		job     Job
		filters clientFilters
		want    bool
	}{
		{name: "category substring match", job: sde, filters: clientFilters{category: "software"}, want: true},
		{name: "category business match", job: sde, filters: clientFilters{category: "aws"}, want: true},
		{name: "category no match", job: sde, filters: clientFilters{category: "hardware"}, want: false},
		{name: "schedule match", job: sde, filters: clientFilters{schedule: "Full-Time"}, want: true},
		{name: "schedule mismatch", job: sde, filters: clientFilters{schedule: "Part-Time"}, want: false},
		{name: "manager=false keeps non-manager", job: sde, filters: clientFilters{manager: ptrBool(false)}, want: true},
		{name: "manager=false excludes manager", job: mgr, filters: clientFilters{manager: ptrBool(false)}, want: false},
		{name: "manager=true keeps manager", job: mgr, filters: clientFilters{manager: ptrBool(true)}, want: true},
		{name: "null is_manager treated as non-manager", job: Job{IsManager: nil}, filters: clientFilters{manager: ptrBool(false)}, want: true},
		{name: "intern=true keeps intern", job: intern, filters: clientFilters{intern: ptrBool(true)}, want: true},
		{name: "intern=false excludes intern", job: intern, filters: clientFilters{intern: ptrBool(false)}, want: false},
		{name: "no filters matches all", job: sde, filters: clientFilters{}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filters.matches(tt.job); got != tt.want {
				t.Errorf("clientFilters.matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientFiltersActive(t *testing.T) {
	if (clientFilters{}).active() {
		t.Error("no filters should be inactive")
	}
	if !(clientFilters{category: "software"}).active() {
		t.Error("category should count as a filter")
	}
	if !(clientFilters{manager: ptrBool(false)}).active() {
		t.Error("manager filter should count")
	}
	if !(clientFilters{postedCutoff: time.Now()}).active() {
		t.Error("posted-within cutoff should count")
	}
	if !(clientFilters{descContains: regexp.MustCompile("x")}).active() {
		t.Error("description filter should count")
	}
}

// TestClientFiltersPostedWithin locks the inclusive-by-date contract: the
// cutoff day itself is inside the window, the day before it is not, and a row
// with no usable posted_date is excluded rather than passed through.
func TestClientFiltersPostedWithin(t *testing.T) {
	now := time.Date(2026, time.July, 25, 14, 0, 0, 0, time.Local)
	f := clientFilters{postedCutoff: postedWithinCutoff(now, 7*24*time.Hour)}

	tests := []struct {
		name   string
		posted string
		want   bool
	}{
		{name: "today", posted: "July 25, 2026", want: true},
		{name: "cutoff day is inclusive", posted: "July 18, 2026", want: true},
		{name: "day before cutoff excluded", posted: "July 17, 2026", want: false},
		{name: "months old excluded", posted: "March 9, 2026", want: false},
		{name: "empty posted_date excluded", posted: "", want: false},
		{name: "garbage posted_date excluded", posted: "not a date", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.matches(Job{PostedDate: tt.posted}); got != tt.want {
				t.Errorf("matches(posted=%q) = %v, want %v", tt.posted, got, tt.want)
			}
		})
	}

	// The double-space form ("July  2, 2026") is ~19% of live rows. Verify it
	// is admitted by a window that contains it — a parse failure here would
	// silently drop every req posted on the 1st through the 9th of a month.
	t.Run("double-spaced single-digit day is admitted", func(t *testing.T) {
		early := clientFilters{postedCutoff: postedWithinCutoff(
			time.Date(2026, time.July, 8, 9, 0, 0, 0, time.Local), 7*24*time.Hour)}
		if !early.matches(Job{PostedDate: "July  2, 2026"}) {
			t.Error("double-spaced posted_date inside the window must match")
		}
		if early.matches(Job{PostedDate: "June 30, 2026"}) {
			t.Error("date before the cutoff must not match")
		}
	})
}

// TestClientFiltersDescription covers the real motivating queries: sponsorship
// language, relocation language (which cuts both ways), and HTML-wrapped text.
func TestClientFiltersDescription(t *testing.T) {
	job := Job{
		Description:             "Join the team.<br/>We build <b>logistics</b> software.",
		BasicQualifications:     "Must be authorized to work in Singapore without sponsorship.",
		PreferredQualifications: "Relocation assistance is NOT provided.",
	}

	tests := []struct {
		name               string
		contains, excludes string
		want               bool
	}{
		{name: "matches across HTML tags", contains: "logistics software", want: true},
		{name: "case insensitive", contains: "MUST BE AUTHORIZED", want: true},
		{name: "no match", contains: "quantum cryptography", want: false},
		{name: "regex alternation", contains: "sponsorship|visa", want: true},
		{name: "excludes sponsorship language", excludes: "without sponsorship", want: false},
		{name: "exclude that does not fire", excludes: "quantum", want: true},
		{name: "contains and excludes combine", contains: "logistics", excludes: "NOT provided", want: false},
		{name: "literal fallback for invalid regex", contains: "C++", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f clientFilters
			var err error
			if f.descContains, err = compileDescriptionPattern(tt.contains); err != nil {
				t.Fatalf("compile contains %q: %v", tt.contains, err)
			}
			if f.descExcludes, err = compileDescriptionPattern(tt.excludes); err != nil {
				t.Fatalf("compile excludes %q: %v", tt.excludes, err)
			}
			if got := f.matches(job); got != tt.want {
				t.Errorf("matches = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCompileDescriptionPatternLiteralFallback pins the decision that an
// unparseable pattern matches literally instead of erroring — "C++" is a real
// query, not a user mistake.
func TestCompileDescriptionPatternLiteralFallback(t *testing.T) {
	re, err := compileDescriptionPattern("C++")
	if err != nil {
		t.Fatalf("expected literal fallback, got error: %v", err)
	}
	if !re.MatchString("experience with c++ and rust") {
		t.Error("literal fallback should match the raw text")
	}
	if nilRe, err := compileDescriptionPattern("   "); err != nil || nilRe != nil {
		t.Errorf("blank pattern should compile to nil, got (%v, %v)", nilRe, err)
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
