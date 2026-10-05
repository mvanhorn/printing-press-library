// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"reflect"
	"sort"
	"testing"
)

// TestParseFacetsFixture parses the real /en/jobs/ page: these lists are
// the only exact source of the site's filter spellings.
func TestParseFacetsFixture(t *testing.T) {
	f, err := ParseFacets(ujReadTestdata(t, "facets.html"))
	if err != nil {
		t.Fatalf("ParseFacets: %v", err)
	}
	if len(f.Countries) != 38 || len(f.Teams) != 18 || len(f.SubTeams) != 53 {
		t.Errorf("countries=%d teams=%d sub_teams=%d, want 38/18/53", len(f.Countries), len(f.Teams), len(f.SubTeams))
	}
	if !reflect.DeepEqual(f.ContractTypes, []string{"Full time"}) {
		t.Errorf("contract types = %q", f.ContractTypes)
	}
	if !reflect.DeepEqual(f.WorkPatterns, []string{"Direct NCG Hire", "Fixed Term", "Intern", "Regular"}) {
		t.Errorf("work patterns = %q", f.WorkPatterns)
	}
	if f.TotalJobs == nil || *f.TotalJobs != 584 {
		t.Errorf("total jobs = %v, want 584", f.TotalJobs)
	}
	if f.UnmappedCountries == nil || len(f.UnmappedCountries) != 0 {
		t.Errorf("unmapped = %v, want a non-nil empty list", f.UnmappedCountries)
	}
	if f.Teams[0] != "Administration" || f.Teams[17] != "University" || f.Countries[29] != "Saudi Arabia" {
		t.Errorf("list order/spelling changed: teams[0]=%q teams[17]=%q countries[29]=%q", f.Teams[0], f.Teams[17], f.Countries[29])
	}
	if len(f.TeamSubTeams) == 0 {
		t.Fatalf("team -> sub-team map is empty")
	}
	eng := f.TeamSubTeams["Engineer"]
	if !sort.StringsAreSorted(eng) || !ujContains(eng, "Software Engineering") || !ujContains(eng, "Information Security") {
		t.Errorf("Engineer sub-teams = %q, want sorted and including Software Engineering", eng)
	}
	team := map[string]bool{}
	for _, tm := range f.Teams {
		team[tm] = true
	}
	sub := map[string]bool{}
	for _, s := range f.SubTeams {
		sub[s] = true
	}
	for tm, subs := range f.TeamSubTeams {
		if !team[tm] {
			t.Errorf("mapping key %q is not a listed team", tm)
		}
		for _, s := range subs {
			if !sub[s] {
				t.Errorf("mapping %s -> %q is not a listed sub-team", tm, s)
			}
		}
	}
}

func ujContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// TestParseFacetsRejectsPagesWithoutLists: a challenge page or an unrelated
// page must be an error, never an empty facet set.
func TestParseFacetsRejectsPagesWithoutLists(t *testing.T) {
	for name, page := range map[string]string{
		"challenge":       "<!DOCTYPE html><html><head><title>Just a moment...</title></head><body><script>window._cf_chl_opt={}</script></body></html>",
		"home page":       "<html><head><title>Uber Careers</title></head><body>Find your role</body></html>",
		"countries only":  `<script>self.__next_f.push([1,"{\"countries\":[\"Germany\"]}"])</script>`,
		"teams only":      `<script>self.__next_f.push([1,"{\"teams\":[\"Sales\"]}"])</script>`,
		"empty lists":     `{"countries":[],"teams":[]}`,
		"unclosed list":   `{"countries":["Germany","teams":["Sales"`,
		"not a JSON list": `{"countries":[Germany],"teams":[Sales]}`,
		"empty":           "",
	} {
		f, err := ParseFacets([]byte(page))
		if err == nil || f != nil {
			t.Errorf("%s: f=%+v err=%v, want an error and no facets", name, f, err)
		}
	}
}

// TestParseFacetsEscapedPayload: the RSC payload escapes quotes inside a
// JS string; brackets inside values must not end a list early.
func TestParseFacetsEscapedPayload(t *testing.T) {
	page := `<script>self.__next_f.push([1,"{\"countries\":[\"Germany\",\"Atlantis\",\"Türkiye\"],\"teams\":[\"Sales [EMEA]\",\"Engineer\"],` +
		`\"subTeams\":[\"Software Engineering\"],\"teamToSubTeamMappings\":{\"Engineer\":[\"Software Engineering\",\"Cloud Engineering\"]},\"totalJobs\":12}"])</script>`
	f, err := ParseFacets([]byte(page))
	if err != nil {
		t.Fatalf("ParseFacets: %v", err)
	}
	if !reflect.DeepEqual(f.Countries, []string{"Germany", "Atlantis", "Türkiye"}) {
		t.Errorf("countries = %q", f.Countries)
	}
	if !reflect.DeepEqual(f.Teams, []string{"Sales [EMEA]", "Engineer"}) {
		t.Errorf("teams = %q, want the bracketed value kept whole", f.Teams)
	}
	if !reflect.DeepEqual(f.UnmappedCountries, []string{"Atlantis"}) {
		t.Errorf("unmapped = %q, want [Atlantis]", f.UnmappedCountries)
	}
	if !reflect.DeepEqual(f.TeamSubTeams["Engineer"], []string{"Cloud Engineering", "Software Engineering"}) {
		t.Errorf("mapping = %q, want sorted", f.TeamSubTeams["Engineer"])
	}
	if f.TotalJobs == nil || *f.TotalJobs != 12 {
		t.Errorf("total = %v, want 12", f.TotalJobs)
	}
	if f.ContractTypes == nil || len(f.ContractTypes) != 0 || f.WorkPatterns == nil {
		t.Errorf("absent lists must be empty, not nil: %v %v", f.ContractTypes, f.WorkPatterns)
	}
}

// TestParseFacetsOptionalParts: a page with only countries and teams parses,
// with an empty (non-nil) mapping and no total.
func TestParseFacetsOptionalParts(t *testing.T) {
	f, err := ParseFacets([]byte(`{"countries":["Germany"],"teams":["Sales"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if f.TeamSubTeams == nil || len(f.TeamSubTeams) != 0 || f.TotalJobs != nil || len(f.SubTeams) != 0 {
		t.Errorf("optional parts = %v %v %v", f.TeamSubTeams, f.TotalJobs, f.SubTeams)
	}
}

// TestMatchBracket: the scanner skips brackets and escaped quotes inside
// JSON strings and reports an unclosed bracket as -1.
func TestMatchBracket(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{`[1,2]`, 4},
		{`["a]b",[1]]`, 10},
		{`["a\"]",1]`, 9},
		{`{"k":{"x":"}"}}`, 14},
		{`[1,[2]`, -1},
	}
	for _, tc := range cases {
		open, closeCh := tc.s[0], byte(']')
		if open == '{' {
			closeCh = '}'
		}
		if got := matchBracket(tc.s, 0, open, closeCh); got != tc.want {
			t.Errorf("matchBracket(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
}
