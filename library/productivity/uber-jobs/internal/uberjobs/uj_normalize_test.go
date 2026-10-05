// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ujCountryISO3 is an independent table for the countries in the corpus,
// so the normalize test does not check CountryNameToISO3 against itself.
var ujCountryISO3 = map[string]string{
	"United Arab Emirates": "ARE", "Saudi Arabia": "SAU", "India": "IND", "Netherlands": "NLD",
	"United Kingdom": "GBR", "Malaysia": "MYS", "United States": "USA", "Chile": "CHL",
	"Germany": "DEU", "Canada": "CAN", "Philippines": "PHL", "Taiwan": "TWN", "Mexico": "MEX", "Brazil": "BRA",
}

var ujHTMLTag = regexp.MustCompile(`<[A-Za-z/!]`)

// TestNormalizeEveryFixtureRow checks the tracker contract on all 56 real
// rows: identity, dates, locations, stripped text, flags, and URLs.
func TestNormalizeEveryFixtureRow(t *testing.T) {
	floors := 0
	for _, raw := range ujCorpusRaw(t) {
		p := Normalize(raw, "")
		id := string(raw.ID)
		if p.ID != id || p.ID != strings.TrimSpace(string(raw.Reference)) {
			t.Errorf("%s: id = %q, want Id == Reference", id, p.ID)
		}
		if p.Title != strings.TrimSpace(string(raw.Title)) || p.Title == "" {
			t.Errorf("%s: title = %q", id, p.Title)
		}
		if ujS(p.PostedRaw) != string(raw.DisplayDate) {
			t.Errorf("%s: posted_raw = %s, want verbatim %s", id, ujS(p.PostedRaw), raw.DisplayDate)
		}
		if string(raw.DisplayDate) == DateFloor {
			floors++
			if p.PostedDate != nil || p.PostedOn != nil || !p.PostedDateIsFloor {
				t.Errorf("%s: floor row posted = %s/%s floor=%v, want null/null/true", id, ujS(p.PostedDate), ujS(p.PostedOn), p.PostedDateIsFloor)
			}
		} else {
			ts, err := time.Parse(time.RFC3339, string(raw.DisplayDate))
			if err != nil {
				t.Fatalf("%s: fixture date %q: %v", id, raw.DisplayDate, err)
			}
			if p.PostedDateIsFloor || ujS(p.PostedOn) != string(raw.DisplayDate)[:10] || ujS(p.PostedDate) != ts.Format("January 2, 2006") {
				t.Errorf("%s: posted = %s/%s floor=%v", id, ujS(p.PostedDate), ujS(p.PostedOn), p.PostedDateIsFloor)
			}
		}
		if len(p.Locations) != len(raw.Locations) || len(p.Locations) == 0 {
			t.Errorf("%s: %d locations, raw has %d", id, len(p.Locations), len(raw.Locations))
			continue
		}
		for i, l := range p.Locations {
			want := ujCountryISO3[string(raw.Locations[i].Country)]
			if want == "" {
				t.Fatalf("%s: fixture country %q missing from the test table", id, raw.Locations[i].Country)
			}
			if ujS(l.CountryCode) != want {
				t.Errorf("%s: location %d country_code = %s, want %s", id, i, ujS(l.CountryCode), want)
			}
			if l.Lat == nil || l.Lng == nil {
				t.Errorf("%s: location %d has no coordinates", id, i)
			}
		}
		if ujS(p.CountryCode) != ujCountryISO3[string(raw.Locations[0].Country)] || ujS(p.Country) != string(raw.Locations[0].Country) {
			t.Errorf("%s: country = %s %s, want the first location's", id, ujS(p.CountryCode), ujS(p.Country))
		}
		if p.NormalizedLocation == nil || !strings.HasSuffix(*p.NormalizedLocation, string(raw.Locations[0].Country)) {
			t.Errorf("%s: normalized_location = %s", id, ujS(p.NormalizedLocation))
		}
		if p.Description == nil {
			t.Errorf("%s: description is null", id)
		} else {
			if ujHTMLTag.MatchString(*p.Description) {
				t.Errorf("%s: description still has a tag: %.200q", id, *p.Description)
			}
			if strings.Contains(*p.Description, "Cleaned Document") {
				t.Errorf("%s: description kept the whole-document title", id)
			}
			if strings.Contains(*p.Description, "&nbsp;") || strings.Contains(*p.Description, "&amp;") {
				t.Errorf("%s: description has undecoded entities", id)
			}
		}
		wp := string(raw.WorkPattern)
		university := len(raw.Teams) > 0 && raw.Teams[0] == "University"
		switch {
		case wp == "Intern":
			if p.IsIntern == nil || !*p.IsIntern || p.UniversityJob == nil || !*p.UniversityJob {
				t.Errorf("%s: Intern row flags = %v %v, want true/true", id, p.IsIntern, p.UniversityJob)
			}
		case wp == "Direct NCG Hire":
			if p.IsIntern == nil || *p.IsIntern || p.UniversityJob == nil || !*p.UniversityJob {
				t.Errorf("%s: NCG row flags wrong", id)
			}
		case wp == "" && !university:
			if p.IsIntern != nil || p.UniversityJob != nil {
				t.Errorf("%s: empty WorkPattern must leave both flags null, got %v %v", id, p.IsIntern, p.UniversityJob)
			}
		default:
			if p.IsIntern == nil || *p.IsIntern || p.UniversityJob == nil || *p.UniversityJob != university {
				t.Errorf("%s: %q row flags = %v %v", id, wp, p.IsIntern, p.UniversityJob)
			}
		}
		if !strings.HasPrefix(p.JobPath, "/en/jobs/") || !strings.HasSuffix(p.JobPath, "/") || p.URL != "https://jobs.uber.com"+p.JobPath {
			t.Errorf("%s: job_path=%q url=%q", id, p.JobPath, p.URL)
		}
		if p.Employer != "uber" || p.Source != SourceSite || p.SalaryRanges == nil || p.Score == nil {
			t.Errorf("%s: employer=%q source=%q salary_ranges nil=%v score nil=%v", id, p.Employer, p.Source, p.SalaryRanges == nil, p.Score == nil)
		}
		if len(raw.Teams) == 0 {
			if p.JobCategory != nil || p.SubTeam != nil {
				t.Errorf("%s: no Teams but category/sub_team = %s/%s", id, ujS(p.JobCategory), ujS(p.SubTeam))
			}
		} else if ujS(p.JobCategory) != raw.Teams[0] || p.SubTeam == nil || raw.Teams[0]+" "+*p.SubTeam != string(raw.AdditionalText) {
			t.Errorf("%s: category/sub_team = %s/%s from %q", id, ujS(p.JobCategory), ujS(p.SubTeam), raw.AdditionalText)
		}
	}
	if floors != 4 {
		t.Errorf("floor rows = %d, want the 4 in the fixture", floors)
	}
}

// TestNormalizeKnownRows pins exact values for named rows so a formatting
// change in dates or locations shows up as a diff, not a pass.
func TestNormalizeKnownRows(t *testing.T) {
	by := ujCorpusByID(t)

	p := by["303232"]
	if ujS(p.PostedDate) != "October 5, 2026" || ujS(p.PostedOn) != "2026-10-05" {
		t.Errorf("303232 posted = %s / %s", ujS(p.PostedDate), ujS(p.PostedOn))
	}
	if ujS(p.Location) != "Amsterdam, Netherlands" || ujS(p.NormalizedLocation) != "Amsterdam, Noord-Holland, Netherlands" || ujS(p.CountryCode) != "NLD" {
		t.Errorf("303232 location = %s | %s | %s", ujS(p.Location), ujS(p.NormalizedLocation), ujS(p.CountryCode))
	}
	if l := p.Locations[0]; l.Lat == nil || *l.Lat != 52.3675734 || *l.Lng != 4.9041389 {
		t.Errorf("303232 lat/lng swapped or missing: %v %v (GeoJSON is [lng, lat])", l.Lat, l.Lng)
	}
	if p.JobCategory != nil || p.WorkPattern != nil || p.IsIntern != nil {
		t.Errorf("303232 has no Teams or WorkPattern; category/work_pattern/is_intern must be null")
	}

	p = by["300300"]
	if ujS(p.PostedDate) != "July 22, 2026" || p.ContractType != nil {
		t.Errorf("300300 posted = %s contract = %s, want July 22, 2026 and null contract", ujS(p.PostedDate), ujS(p.ContractType))
	}

	p = by["302016"]
	if len(p.Locations) != 2 || ujS(p.Locations[0].CountryCode) != "ARE" || ujS(p.Locations[1].CountryCode) != "SAU" || ujS(p.Locations[1].Country) != "Saudi Arabia" {
		t.Errorf("302016 locations = %+v", p.Locations)
	}
	if ujS(p.NormalizedLocation) != "United Arab Emirates" || ujS(p.SubTeam) != "Sales" || ujS(p.WorkPattern) != "Fixed Term" {
		t.Errorf("302016 normalized=%s sub_team=%s work_pattern=%s", ujS(p.NormalizedLocation), ujS(p.SubTeam), ujS(p.WorkPattern))
	}

	p = by["303139"]
	if ujS(p.SubTeam) != "Business & Sales" || p.IsIntern == nil || !*p.IsIntern {
		t.Errorf("303139 sub_team=%s is_intern=%v", ujS(p.SubTeam), p.IsIntern)
	}

	for _, id := range []string{"149574", "155579", "157568", "157054"} {
		p := by[id]
		if !p.PostedDateIsFloor || p.PostedDate != nil || p.PostedOn != nil || ujS(p.PostedRaw) != "2026-06-19T07:30:00Z" {
			t.Errorf("%s: floor fields = %v %s %s %s", id, p.PostedDateIsFloor, ujS(p.PostedDate), ujS(p.PostedOn), ujS(p.PostedRaw))
		}
	}

	p = by["302278"]
	if p.Description == nil || !strings.HasPrefix(*p.Description, "About the role and team") {
		t.Errorf("302278 whole-document description starts %.80q", ujS(p.Description))
	}
	if !strings.HasSuffix(ujS(p.Description), "this form.") {
		t.Errorf("302278 description lost its tail link text: %.80q", ujS(p.Description))
	}
}

// TestNormalizeSalaryFromFixture covers the three salary outcomes in the
// real corpus: equal multi-city ranges, different ranges, and a non-US
// sentence that must stay unparsed.
func TestNormalizeSalaryFromFixture(t *testing.T) {
	by := ujCorpusByID(t)

	p := by["302500"]
	if len(p.SalaryRanges) != 3 || p.SalaryMin == nil || *p.SalaryMin != 171000 || *p.SalaryMax != 190000 || ujS(p.SalaryCurrency) != "USD" || ujS(p.SalaryPeriod) != "year" {
		t.Errorf("302500 salary = %+v min=%v max=%v cur=%s per=%s", p.SalaryRanges, p.SalaryMin, p.SalaryMax, ujS(p.SalaryCurrency), ujS(p.SalaryPeriod))
	}
	wantLoc := []string{"San Francisco, CA", "Seattle, WA", "Sunnyvale, CA"}
	for i, r := range p.SalaryRanges {
		if i < len(wantLoc) && ujS(r.Location) != wantLoc[i] {
			t.Errorf("302500 range %d location = %s, want %s", i, ujS(r.Location), wantLoc[i])
		}
	}

	p = by["303121"]
	if len(p.SalaryRanges) != 3 || p.SalaryMin != nil || p.SalaryMax != nil || p.SalaryCurrency != nil || p.SalaryPeriod != nil {
		t.Errorf("303121 different ranges must keep scalars null: ranges=%d min=%v", len(p.SalaryRanges), p.SalaryMin)
	}
	if len(p.SalaryRanges) == 3 && (p.SalaryRanges[1].Min != 145000 || ujS(p.SalaryRanges[1].Location) != "San Francisco, CA") {
		t.Errorf("303121 SF range = %+v", p.SalaryRanges[1])
	}

	p = by["300313"]
	if len(p.SalaryRanges) != 0 || p.SalaryMin != nil || p.SalaryText == nil || !strings.Contains(*p.SalaryText, "CAD $24.76 per hour") {
		t.Errorf("300313 CAD text: ranges=%d min=%v text=%.80q", len(p.SalaryRanges), p.SalaryMin, ujS(p.SalaryText))
	}

	p = by["303232"]
	if p.SalaryText != nil || len(p.SalaryRanges) != 0 || p.SalaryMin != nil {
		t.Errorf("303232 empty salary: text=%s ranges=%d", ujS(p.SalaryText), len(p.SalaryRanges))
	}
}

// TestNormalizeJSONNeverNull: arrays are [] and nullable scalars are
// present as null, so a tracker never sees a missing key.
func TestNormalizeJSONNeverNull(t *testing.T) {
	p := Normalize(RawPosting{ID: "1", Title: "x"}, "")
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if mustJSON(m["salary_ranges"]) != "[]" || mustJSON(m["locations"]) != "[]" {
		t.Errorf("salary_ranges=%v locations=%v, want []", m["salary_ranges"], m["locations"])
	}
	for _, k := range ContractFields {
		if _, ok := m[k]; !ok {
			t.Errorf("contract key %q missing from JSON", k)
		}
	}
	if len(m) != len(ContractFields) {
		t.Errorf("JSON has %d keys, ContractFields lists %d", len(m), len(ContractFields))
	}
	for _, k := range []string{"posted_date", "job_category", "country_code", "description", "salary_min", "score"} {
		if v, ok := m[k]; !ok || v != nil {
			t.Errorf("%s = %v (present=%v), want explicit null", k, v, ok)
		}
	}
}

// TestNormalizeLenientDecode: ids may arrive as numbers, the score as a
// numeric string, and salary values as null or strings.
func TestNormalizeLenientDecode(t *testing.T) {
	raw := `{"Id":303232,"Reference":303232,"Title":"  Trimmed Title  ","@search.score":"1.5",
		"DisplayDate":"2026-10-05T09:15:59Z","Teams":["Engineer"],"AdditionalText":"Engineer Software Engineering",
		"Remote":null,"Salary":{"MinValue":null,"MaxValue":"120000","Currency":"USD","Period":"year","Description":null},
		"Locations":[{"Country":"Germany","CountryCode":"DE"},{"Country":"Atlantis","CountryCode":null}],
		"Urls":[{"Url":"/en/jobs/303232","IsDefault":false},{"Url":"/en/jobs/303232/default","IsDefault":true}]}`
	var r RawPosting
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("lenient decode failed: %v", err)
	}
	p := Normalize(r, "http://127.0.0.1:1/")
	if p.ID != "303232" || p.Title != "Trimmed Title" {
		t.Errorf("id/title = %q %q", p.ID, p.Title)
	}
	if p.Score == nil || *p.Score != 1.5 {
		t.Errorf("score = %v, want 1.5 from a numeric string", p.Score)
	}
	if p.SalaryMin != nil || p.SalaryMax == nil || *p.SalaryMax != 120000 {
		t.Errorf("salary min=%v max=%v, want null and 120000", p.SalaryMin, p.SalaryMax)
	}
	if ujS(p.Locations[0].CountryCode) != "DEU" {
		t.Errorf("ISO2 country code DE -> %s, want DEU", ujS(p.Locations[0].CountryCode))
	}
	if p.Locations[1].CountryCode != nil {
		t.Errorf("unknown country name got code %s, want null", ujS(p.Locations[1].CountryCode))
	}
	if p.JobPath != "/en/jobs/303232/default/" || p.URL != "http://127.0.0.1:1/en/jobs/303232/default/" {
		t.Errorf("job_path=%q url=%q, want the default Url with a trailing slash", p.JobPath, p.URL)
	}
	if p.Remote != nil {
		t.Errorf("remote = %v, want null", *p.Remote)
	}
}

// TestNormalizeFallbacks: Reference stands in for a missing Id, and a row
// with no Urls gets the canonical /en/jobs/<id>/ path.
func TestNormalizeFallbacks(t *testing.T) {
	p := Normalize(RawPosting{Reference: " 42 ", Title: "T"}, "")
	if p.ID != "42" || p.JobPath != "/en/jobs/42/" || p.URL != DefaultBaseURL+"/en/jobs/42/" {
		t.Errorf("fallback id/path = %q %q %q", p.ID, p.JobPath, p.URL)
	}
	p = Normalize(RawPosting{ID: "7", URLs: []RawURL{{URL: " /en/jobs/7 "}}}, "")
	if p.JobPath != "/en/jobs/7/" {
		t.Errorf("non-default url path = %q, want trailing slash added", p.JobPath)
	}
	p = Normalize(RawPosting{ID: "8", Teams: []string{"Sales"}, AdditionalText: "Marketing Brand"}, "")
	if p.SubTeam != nil {
		t.Errorf("AdditionalText not prefixed by the team gave sub_team %s, want null", ujS(p.SubTeam))
	}
	p = Normalize(RawPosting{ID: "9", Teams: []string{"University"}}, "")
	if p.IsIntern == nil || *p.IsIntern || p.UniversityJob == nil || !*p.UniversityJob {
		t.Errorf("University team with empty WorkPattern: flags = %v %v, want false/true", p.IsIntern, p.UniversityJob)
	}
}

// TestNormalizeSiteSalaryWins: when the site sends numeric MinValue or
// MaxValue, the text ranges never overwrite them.
func TestNormalizeSiteSalaryWins(t *testing.T) {
	r := RawPosting{ID: "1", Salary: &RawSalary{
		MinValue:    LooseFloat{Value: 1, Set: true},
		Currency:    "EUR",
		Description: "The base salary range for this role is USD $171,000 per year - USD $190,000 per year",
	}}
	p := Normalize(r, "")
	if p.SalaryMin == nil || *p.SalaryMin != 1 || p.SalaryMax != nil || ujS(p.SalaryCurrency) != "EUR" {
		t.Errorf("site values overwritten: min=%v max=%v cur=%s", p.SalaryMin, p.SalaryMax, ujS(p.SalaryCurrency))
	}
	if len(p.SalaryRanges) != 1 {
		t.Errorf("ranges = %d, want the parsed text range kept alongside", len(p.SalaryRanges))
	}
}

// TestLooseScalars covers the lenient decoders directly.
func TestLooseScalars(t *testing.T) {
	var s struct {
		A, B, C, D, E LooseString
	}
	if err := json.Unmarshal([]byte(`{"A":"x","B":12,"C":null,"D":true,"E":1.5e3}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.A != "x" || s.B != "12" || s.C != "" || s.D != "true" || s.E != "1.5e3" {
		t.Errorf("LooseString = %+v", s)
	}
	var f struct {
		A, B, C, D, E LooseFloat
	}
	if err := json.Unmarshal([]byte(`{"A":12.5,"B":"7","C":null,"D":"","E":"n/a"}`), &f); err != nil {
		t.Fatal(err)
	}
	if !f.A.Set || f.A.Value != 12.5 || !f.B.Set || f.B.Value != 7 {
		t.Errorf("numeric LooseFloat = %+v %+v", f.A, f.B)
	}
	if f.C.Set || f.D.Set || f.E.Set {
		t.Errorf("null, empty, and junk must stay unset: %+v %+v %+v", f.C, f.D, f.E)
	}
	var bad LooseString
	if err := bad.UnmarshalJSON([]byte(`"unterminated`)); err == nil {
		t.Errorf("malformed JSON string decoded without error")
	}
}
