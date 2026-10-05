// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"testing"
)

// TestParseSalaryRanges covers the one parsed sentence shape and the
// shapes that must stay unparsed (numbers are never guessed).
func TestParseSalaryRanges(t *testing.T) {
	type rng struct {
		loc      string
		cur      string
		min, max float64
		per      string
	}
	cases := []struct {
		name string
		in   string
		want []rng
	}{
		{"single city", "For Seattle, WA-based roles: The base salary range for this role is USD $171,000 per year - USD $190,000 per year.",
			[]rng{{"Seattle, WA", "USD", 171000, 190000, "year"}}},
		{"no location prefix", "The base salary range for this role is USD $50 per hour - USD $60.50 per hour",
			[]rng{{"<nil>", "USD", 50, 60.5, "hour"}}},
		{"en dash and no dollar sign", "The base salary range for this role is USD 90,000 per year – USD 100,000 per year",
			[]rng{{"<nil>", "USD", 90000, 100000, "year"}}},
		{"lowercase currency normalized", "the base salary range for this role is usd $1 per month - usd $2 per month",
			[]rng{{"<nil>", "USD", 1, 2, "month"}}},
		{"multi-city", "For New York City, NY-based roles: The base salary range for this role is USD $153,000 per year - USD $170,000 per year.\n\nFor San Francisco, CA-based roles: The base salary range for this role is USD $145,000 per year - USD $160,000 per year.",
			[]rng{{"New York City, NY", "USD", 153000, 170000, "year"}, {"San Francisco, CA", "USD", 145000, 160000, "year"}}},
		{"currency mismatch skipped", "The base salary range for this role is USD $1 per year - CAD $2 per year", nil},
		{"period mismatch skipped", "The base salary range for this role is USD $1 per year - USD $2 per hour", nil},
		{"mismatch skipped, good kept", "For Austin, TX-based roles: The base salary range for this role is USD $1 per year - EUR $2 per year. For Boston, MA-based roles: The base salary range for this role is USD $3 per year - USD $4 per year.",
			[]rng{{"Boston, MA", "USD", 3, 4, "year"}}},
		{"canadian hourly shape", "For Canada-based roles: The base hourly rate for this role is CAD $24.76 per hour - CAD $25.97 per hour.", nil},
		{"OTE shape", "For Dallas, TX-based roles: The total annualized on-target earnings (OTE) for this position is USD $90,833-$95,417.", nil},
		{"empty", "", nil},
	}
	for _, tc := range cases {
		got := ParseSalaryRanges(tc.in)
		if got == nil {
			t.Errorf("%s: ParseSalaryRanges returned nil, want a non-nil slice", tc.name)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("%s: %d ranges %+v, want %d", tc.name, len(got), got, len(tc.want))
			continue
		}
		for i, w := range tc.want {
			g := got[i]
			if ujS(g.Location) != w.loc || g.Currency != w.cur || g.Min != w.min || g.Max != w.max || g.Period != w.per {
				t.Errorf("%s: range %d = {%s %s %v %v %s}, want %+v", tc.name, i, ujS(g.Location), g.Currency, g.Min, g.Max, g.Period, w)
			}
		}
	}
}

// TestApplySingleRange: scalars are set only when every range agrees, so a
// multi-city posting never reports one city's pay as the posting's pay.
func TestApplySingleRange(t *testing.T) {
	sf, ny := ujStrp("San Francisco, CA"), ujStrp("New York City, NY")

	p := Posting{}
	applySingleRange(&p)
	if p.SalaryMin != nil || p.SalaryCurrency != nil {
		t.Errorf("no ranges set scalars")
	}

	p = Posting{SalaryRanges: []SalaryRange{{Location: sf, Currency: "USD", Min: 1, Max: 2, Period: "year"}}}
	applySingleRange(&p)
	if p.SalaryMin == nil || *p.SalaryMin != 1 || *p.SalaryMax != 2 || ujS(p.SalaryCurrency) != "USD" || ujS(p.SalaryPeriod) != "year" {
		t.Errorf("single range: min=%v max=%v cur=%s per=%s", p.SalaryMin, p.SalaryMax, ujS(p.SalaryCurrency), ujS(p.SalaryPeriod))
	}

	p = Posting{SalaryRanges: []SalaryRange{{Location: sf, Currency: "USD", Min: 1, Max: 2, Period: "year"}, {Location: ny, Currency: "USD", Min: 1, Max: 2, Period: "year"}}}
	applySingleRange(&p)
	if p.SalaryMin == nil || *p.SalaryMin != 1 {
		t.Errorf("equal multi-city ranges must set scalars")
	}

	for name, second := range map[string]SalaryRange{
		"different min":      {Location: ny, Currency: "USD", Min: 5, Max: 2, Period: "year"},
		"different max":      {Location: ny, Currency: "USD", Min: 1, Max: 9, Period: "year"},
		"different currency": {Location: ny, Currency: "CAD", Min: 1, Max: 2, Period: "year"},
		"different period":   {Location: ny, Currency: "USD", Min: 1, Max: 2, Period: "hour"},
	} {
		p = Posting{SalaryRanges: []SalaryRange{{Location: sf, Currency: "USD", Min: 1, Max: 2, Period: "year"}, second}}
		applySingleRange(&p)
		if p.SalaryMin != nil || p.SalaryMax != nil || p.SalaryCurrency != nil || p.SalaryPeriod != nil {
			t.Errorf("%s: scalars set on disagreeing ranges", name)
		}
		if len(p.SalaryRanges) != 2 {
			t.Errorf("%s: ranges dropped", name)
		}
	}
}

// ujFacetCountryISO3 is the expected ISO3 for each of the 38 country names
// in testdata/facets.html, written out independently of countries.go.
var ujFacetCountryISO3 = map[string]string{
	"Argentina": "ARG", "Australia": "AUS", "Belgium": "BEL", "Brazil": "BRA", "Canada": "CAN",
	"Chile": "CHL", "Colombia": "COL", "Costa Rica": "CRI", "Denmark": "DNK", "Ecuador": "ECU",
	"Egypt": "EGY", "Finland": "FIN", "France": "FRA", "Germany": "DEU", "Hong Kong": "HKG",
	"India": "IND", "Ireland": "IRL", "Italy": "ITA", "Japan": "JPN", "Kenya": "KEN",
	"Korea, Republic of": "KOR", "Malaysia": "MYS", "Mexico": "MEX", "Netherlands": "NLD", "New Zealand": "NZL",
	"Panama": "PAN", "Philippines": "PHL", "Poland": "POL", "Portugal": "PRT", "Saudi Arabia": "SAU",
	"South Africa": "ZAF", "Spain": "ESP", "Sweden": "SWE", "Taiwan": "TWN", "Türkiye": "TUR",
	"United Arab Emirates": "ARE", "United Kingdom": "GBR", "United States": "USA",
}

// TestFacetCountriesAllMap: every country the site filters on maps to the
// right ISO3 and back to the site's exact spelling, so --country never
// sends a name the site ignores.
func TestFacetCountriesAllMap(t *testing.T) {
	f, err := ParseFacets(ujReadTestdata(t, "facets.html"))
	if err != nil {
		t.Fatalf("ParseFacets: %v", err)
	}
	if len(f.Countries) != 38 || len(f.UnmappedCountries) != 0 {
		t.Fatalf("countries=%d unmapped=%q, want 38 and none", len(f.Countries), f.UnmappedCountries)
	}
	for _, name := range f.Countries {
		want, ok := ujFacetCountryISO3[name]
		if !ok {
			t.Errorf("facet country %q not in the test table (fixture changed?)", name)
			continue
		}
		got, ok := CountryNameToISO3(name)
		if !ok || got != want {
			t.Errorf("CountryNameToISO3(%q) = %q %v, want %s", name, got, ok, want)
		}
		site, ok := SiteNameForISO3(want)
		if !ok || site != name {
			t.Errorf("SiteNameForISO3(%s) = %q, want the site spelling %q", want, site, name)
		}
		iso, siteName, ok := ResolveCountry(want)
		if !ok || iso != want || siteName != name {
			t.Errorf("ResolveCountry(%s) = %q %q %v, want %s %q", want, iso, siteName, ok, want, name)
		}
	}
	if got, _ := CountryNameToISO3("Saudi Arabia"); got != "SAU" {
		t.Errorf("Saudi Arabia -> %q, want SAU", got)
	}
}

// TestResolveCountry covers every accepted input form and rejects junk.
func TestResolveCountry(t *testing.T) {
	cases := []struct {
		in, iso, site string
		ok            bool
	}{
		{"GBR", "GBR", "United Kingdom", true},
		{"gbr", "GBR", "United Kingdom", true},
		{"GB", "GBR", "United Kingdom", true},
		{"gb", "GBR", "United Kingdom", true},
		{"UK", "GBR", "United Kingdom", true},
		{"United Kingdom", "GBR", "United Kingdom", true},
		{"  united kingdom  ", "GBR", "United Kingdom", true},
		{"DEU", "DEU", "Germany", true},
		{"usa", "USA", "United States", true},
		{"US", "USA", "United States", true},
		{"United States of America", "USA", "United States", true},
		{"Türkiye", "TUR", "Türkiye", true},
		{"turkiye", "TUR", "Türkiye", true},
		{"Turkey", "TUR", "Türkiye", true},
		{"Côte d'Ivoire", "CIV", "Côte d'Ivoire", true},
		{"cote divoire", "CIV", "Côte d'Ivoire", true},
		{"Korea, Republic of", "KOR", "Korea, Republic of", true},
		{"south korea", "KOR", "Korea, Republic of", true},
		{"México", "MEX", "Mexico", true},
		{"UAE", "ARE", "United Arab Emirates", true},
		{"KSA", "SAU", "Saudi Arabia", true},
		{"sa", "SAU", "Saudi Arabia", true},
		{"XYZ", "", "", false},
		{"XX", "", "", false},
		{"Atlantis", "", "", false},
		{"G", "", "", false},
		{"", "", "", false},
		{"   ", "", "", false},
	}
	for _, tc := range cases {
		iso, site, ok := ResolveCountry(tc.in)
		if iso != tc.iso || site != tc.site || ok != tc.ok {
			t.Errorf("ResolveCountry(%q) = %q %q %v, want %q %q %v", tc.in, iso, site, ok, tc.iso, tc.site, tc.ok)
		}
	}
}

// TestToISO3 maps alpha-2 and alpha-3 and passes unknown codes through
// upper-cased so callers can report them.
func TestToISO3(t *testing.T) {
	for in, want := range map[string]string{"gb": "GBR", "GBR": "GBR", " us ": "USA", "de": "DEU", "SA": "SAU", "xx": "XX", "zzz": "ZZZ", "": ""} {
		if got := ToISO3(in); got != want {
			t.Errorf("ToISO3(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := CountryNameToISO3("Atlantis"); ok {
		t.Errorf("CountryNameToISO3 accepted Atlantis")
	}
	if _, ok := SiteNameForISO3("XXX"); ok {
		t.Errorf("SiteNameForISO3 accepted XXX")
	}
	if name, ok := SiteNameForISO3(" usa "); !ok || name != "United States" {
		t.Errorf("SiteNameForISO3(usa) = %q %v", name, ok)
	}
}

// TestCountryTableConsistent: no alias or code shadows another entry, which
// would silently remap a country in every filter.
func TestCountryTableConsistent(t *testing.T) {
	a2, a3 := map[string]string{}, map[string]string{}
	for _, c := range countryTable {
		if len(c.Alpha2) != 2 || len(c.Alpha3) != 3 {
			t.Errorf("%s: bad codes %q %q", c.Name, c.Alpha2, c.Alpha3)
		}
		if prev, dup := a2[c.Alpha2]; dup {
			t.Errorf("alpha-2 %s used by %s and %s", c.Alpha2, prev, c.Name)
		}
		if prev, dup := a3[c.Alpha3]; dup {
			t.Errorf("alpha-3 %s used by %s and %s", c.Alpha3, prev, c.Name)
		}
		a2[c.Alpha2], a3[c.Alpha3] = c.Name, c.Name
		if got, ok := CountryNameToISO3(c.Name); !ok || got != c.Alpha3 {
			t.Errorf("name %q -> %q, want %s", c.Name, got, c.Alpha3)
		}
		for _, alias := range c.Aliases {
			if got, ok := CountryNameToISO3(alias); !ok || got != c.Alpha3 {
				t.Errorf("alias %q of %s -> %q (shadowed by another entry?)", alias, c.Name, got)
			}
		}
	}
}
