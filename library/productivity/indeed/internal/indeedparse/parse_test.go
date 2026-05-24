package indeedparse

import "testing"

// searchFixture mimics how Indeed embeds the job-card model in the raw SERP
// HTML: an inline script assigns the mosaic provider data, with the results
// array nested under mosaicProviderJobCardsModel. totalJobCount lives outside
// that object (as on the real page).
const searchFixture = `<!doctype html><html><head></head><body>
<script>
window.mosaic.providerData["mosaic-provider-jobcards"]={"metaData":{"isJpBundle":false,"mosaicProviderJobCardsModel":{"adSignature":"3049","results":[
{"jobkey":"abc123","title":"Senior Go Engineer","company":"Acme Corp","formattedLocation":"Austin, TX","remoteLocation":false,"jobTypes":["Full-time"],"formattedRelativeTime":"2 days ago","companyRating":4.2,"companyReviewCount":150,"snippet":"<ul><li>Build &amp; ship Go services</li></ul>","salarySnippet":{"text":"$140,000 - $180,000 a year","currency":"USD"},"extractedSalary":{"min":140000,"max":180000,"type":"YEARLY"}},
{"jobkey":"def456","title":"Backend Developer","company":"Globex","formattedLocation":"Remote","remoteLocation":true,"jobTypes":[],"formattedRelativeTime":"Just posted","snippet":"Work from home","salarySnippet":{"text":"$60 an hour","currency":"USD"}},
{"jobkey":"","title":"Should be skipped","company":"NoKey"}
]}}};
window.mosaic.providerData["other"]={"totalJobCount":4217,"foo":"bar"};
</script>
</body></html>`

func TestParseSearchResults(t *testing.T) {
	jobs, total, err := ParseSearchResults([]byte(searchFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 4217 {
		t.Errorf("totalJobCount = %d, want 4217", total)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2 (empty-key job must be skipped)", len(jobs))
	}
	j0 := jobs[0]
	if j0.Key != "abc123" || j0.Title != "Senior Go Engineer" || j0.Company != "Acme Corp" {
		t.Errorf("job0 basic fields wrong: %+v", j0)
	}
	if j0.SalaryMin != 140000 || j0.SalaryMax != 180000 || j0.SalaryPeriod != "year" {
		t.Errorf("job0 salary wrong: min=%v max=%v period=%q", j0.SalaryMin, j0.SalaryMax, j0.SalaryPeriod)
	}
	if j0.URL != "https://www.indeed.com/viewjob?jk=abc123" {
		t.Errorf("job0 url wrong: %q", j0.URL)
	}
	// Snippet HTML entities/tags should be cleaned.
	if want := "ship Go services"; !contains(j0.Snippet, want) {
		t.Errorf("job0 snippet not cleaned: %q", j0.Snippet)
	}
	j1 := jobs[1]
	if !j1.Remote {
		t.Errorf("job1 should be remote")
	}
	if j1.SalaryPeriod != "hour" || j1.SalaryMin != 60 {
		t.Errorf("job1 hourly salary parse wrong: min=%v period=%q", j1.SalaryMin, j1.SalaryPeriod)
	}
}

func TestParseSearchResultsBlocked(t *testing.T) {
	_, _, err := ParseSearchResults([]byte(`<html><head><title>Just a moment...</title></head><body>cf-mitigated</body></html>`))
	if err == nil {
		t.Fatal("expected error for blocked page")
	}
}

const detailFixture = `<!doctype html><html><body>
<script type="application/ld+json">
{"@context":"https://schema.org/","@type":"JobPosting","title":"Senior Go Engineer","description":"<p>We need a <b>Go</b> engineer to build services.</p>","datePosted":"2026-05-20","validThrough":"2026-06-20","employmentType":["FULL_TIME"],"jobLocationType":"TELECOMMUTE","hiringOrganization":{"@type":"Organization","name":"Acme Corp"},"jobLocation":{"@type":"Place","address":{"addressLocality":"Austin","addressRegion":"TX","addressCountry":"US"}}}
</script>
<script>window._initialData={"jobInfoWrapperModel":{"jobInfoModel":{"sanitizedJobDescription":"fallback body"}}};</script>
</body></html>`

func TestParseJobDetail(t *testing.T) {
	d, err := ParseJobDetail([]byte(detailFixture), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Title != "Senior Go Engineer" || d.Company != "Acme Corp" {
		t.Errorf("title/company wrong: %+v", d)
	}
	if d.EmploymentType != "FULL_TIME" {
		t.Errorf("employmentType = %q", d.EmploymentType)
	}
	if !d.Remote {
		t.Errorf("should be remote (TELECOMMUTE)")
	}
	if d.DatePosted != "2026-05-20" || d.ValidThrough != "2026-06-20" {
		t.Errorf("dates wrong: %+v", d)
	}
	if !contains(d.Description, "Go engineer to build services") {
		t.Errorf("description not cleaned from JSON-LD: %q", d.Description)
	}
	if d.URL != "https://www.indeed.com/viewjob?jk=abc123" {
		t.Errorf("url wrong: %q", d.URL)
	}
}

func TestParseSalary(t *testing.T) {
	cases := []struct {
		in     string
		min    float64
		max    float64
		period string
	}{
		{"$55 an hour", 55, 55, "hour"},
		{"$100,000 - $130,000 a year", 100000, 130000, "year"},
		{"Up to $90K a year", 90000, 90000, "year"},
		{"From $25.50 an hour", 25.50, 25.50, "hour"},
		// 3+ figures must not invert min/max (smallest..largest).
		{"$120,000 - $150,000 plus $10,000 bonus a year", 10000, 150000, "year"},
		{"Competitive", 0, 0, ""},
	}
	for _, c := range cases {
		min, max, period := ParseSalary(c.in)
		if min != c.min || max != c.max || period != c.period {
			t.Errorf("ParseSalary(%q) = (%v,%v,%q), want (%v,%v,%q)", c.in, min, max, period, c.min, c.max, c.period)
		}
	}
}

func TestExtractBracedObjectEscapes(t *testing.T) {
	s := `prefix "model":{"a":"he said \"hi\" }","b":{"c":1}} suffix`
	obj, ok := ExtractBracedObject(s, `"model":`)
	if !ok {
		t.Fatal("expected to find object")
	}
	want := `{"a":"he said \"hi\" }","b":{"c":1}}`
	if obj != want {
		t.Errorf("got %q want %q", obj, want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
