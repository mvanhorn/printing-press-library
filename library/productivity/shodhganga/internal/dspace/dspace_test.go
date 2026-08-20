// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.

package dspace

import "testing"

func TestNormalizeID(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"305247", "305247", false},
		{"10603/305247", "305247", false},
		{"10603/305247/", "305247", false},
		{"http://hdl.handle.net/10603/305247", "305247", false},
		{"https://shodhganga.inflibnet.ac.in/handle/10603/312900", "312900", false},
		{"  638225  ", "638225", false},
		{"", "", true},
		{"not-a-handle", "", true},
	}
	for _, tt := range tests {
		got, err := NormalizeID(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("NormalizeID(%q): expected error, got %q", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeID(%q): unexpected error %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("NormalizeID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

const searchFixture = `
<div class="discovery-result-results">
<span>Results 1-3 of 137604</span>
<table>
<tr class="oddRowEvenCol"><td><a href="/handle/10603/305247">Analytical&#x20;description&#x20;of&#x20;black&#x20;holes&#x20;shadow</a></td></tr>
<tr class="evenRowEvenCol"><td><a href="/handle/10603/312900">Some&#x20;astro&#x20;particle&#x20;physics&#x20;aspects</a></td></tr>
<tr class="oddRowEvenCol"><td><a href="/handle/10603/638225">Study&#x20;of&#x20;Novae</a></td></tr>
</table>
</div>`

func TestParseSearch(t *testing.T) {
	res := ParseSearch([]byte(searchFixture), "https://shodhganga.inflibnet.ac.in")
	if res.Total != 137604 {
		t.Errorf("Total = %d, want 137604", res.Total)
	}
	if len(res.Hits) != 3 {
		t.Fatalf("got %d hits, want 3", len(res.Hits))
	}
	first := res.Hits[0]
	if first.ID != "305247" {
		t.Errorf("hit[0].ID = %q, want 305247", first.ID)
	}
	if first.Handle != "10603/305247" {
		t.Errorf("hit[0].Handle = %q, want 10603/305247", first.Handle)
	}
	if first.Title != "Analytical description of black holes shadow" {
		t.Errorf("hit[0].Title = %q (entities not decoded?)", first.Title)
	}
	if first.URL != "https://shodhganga.inflibnet.ac.in/handle/10603/305247" {
		t.Errorf("hit[0].URL = %q", first.URL)
	}
}

func TestParseSearchDedup(t *testing.T) {
	// A nav link to the same handle must not produce a duplicate hit.
	dup := `<a href="/handle/10603/1">nav</a>` + searchFixture + `<a href="/handle/10603/305247">Analytical description of black holes shadow</a>`
	res := ParseSearch([]byte(dup), "https://x")
	ids := map[string]int{}
	for _, h := range res.Hits {
		ids[h.ID]++
	}
	if ids["305247"] != 1 {
		t.Errorf("handle 305247 appeared %d times, want 1", ids["305247"])
	}
}

func TestParseSearchDecoratedAnchors(t *testing.T) {
	// DSpace themes may add class/data attributes to result links; the parser
	// must still match them, not silently return zero hits.
	h := `<span>Results 1-1 of 5</span>
<a href="/handle/10603/305247" class="list-group-item" data-x="1">Decorated Title</a>`
	res := ParseSearch([]byte(h), "https://x")
	if len(res.Hits) != 1 {
		t.Fatalf("got %d hits, want 1 (decorated anchor not matched)", len(res.Hits))
	}
	if res.Hits[0].ID != "305247" || res.Hits[0].Title != "Decorated Title" {
		t.Errorf("hit = %+v", res.Hits[0])
	}
}

const itemFixture = `<html><head>
<meta name="DC.title" content="Analytical description of black holes shadow" />
<meta name="DC.creator" content="Singh, Balendra Pratap" />
<meta name="DC.contributor" content="Ghosh, Sushant G" />
<meta name="DC.subject" content="Mathematical physics" />
<meta name="DC.subject" content="Physics" />
<meta name="DC.subject" content="Physics" />
<meta name="DC.publisher" content="Delhi" />
<meta name="DC.publisher" content="Jamia Millia Islamia University" />
<meta name="DC.publisher" content="Centre for Theoretical Physics" />
<meta name="DC.date" content="2019" />
<meta name="DC.language" content="English" />
<meta name="DC.type" content="Text" />
<meta name="DC.identifier" content="http://hdl.handle.net/10603/305247" scheme="DCTERMS.URI" />
<meta name="DCTERMS.abstract" content="newline" />
</head><body></body></html>`

func TestParseItem(t *testing.T) {
	th := ParseItem([]byte(itemFixture), "305247", "https://shodhganga.inflibnet.ac.in")
	if th.Title != "Analytical description of black holes shadow" {
		t.Errorf("Title = %q", th.Title)
	}
	if th.Researcher != "Singh, Balendra Pratap" {
		t.Errorf("Researcher = %q", th.Researcher)
	}
	if len(th.Guides) != 1 || th.Guides[0] != "Ghosh, Sushant G" {
		t.Errorf("Guides = %v", th.Guides)
	}
	if th.University != "Jamia Millia Islamia University" {
		t.Errorf("University = %q, want Jamia Millia Islamia University", th.University)
	}
	if th.Department != "Centre for Theoretical Physics" {
		t.Errorf("Department = %q", th.Department)
	}
	if th.Place != "Delhi" {
		t.Errorf("Place = %q, want Delhi", th.Place)
	}
	// DC.subject "Physics" appears twice; must dedupe to 2 keywords.
	if len(th.Keywords) != 2 {
		t.Errorf("Keywords = %v, want 2 deduped", th.Keywords)
	}
	if th.CompletedDate != "2019" {
		t.Errorf("CompletedDate = %q", th.CompletedDate)
	}
	if th.Language != "English" {
		t.Errorf("Language = %q", th.Language)
	}
	if th.Handle != "10603/305247" {
		t.Errorf("Handle = %q", th.Handle)
	}
	// Placeholder abstract must normalize away.
	if th.Abstract != "" {
		t.Errorf("Abstract = %q, want empty (placeholder)", th.Abstract)
	}
}

func TestNormalizeAbstract(t *testing.T) {
	cases := map[string]string{
		"newline":                            "",
		"Abstract Available\nnewline":        "",
		"":                                   "",
		"A real abstract about black holes.": "A real abstract about black holes.",
	}
	for in, want := range cases {
		if got := normalizeAbstract(clean(in)); got != want {
			t.Errorf("normalizeAbstract(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassifyPublishersSingle(t *testing.T) {
	th := &Thesis{}
	classifyPublishers(th, []string{"University of Calcutta"})
	if th.University != "University of Calcutta" {
		t.Errorf("University = %q", th.University)
	}
}
