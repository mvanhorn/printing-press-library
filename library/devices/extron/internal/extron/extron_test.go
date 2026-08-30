// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package extron

import (
	"testing"
)

const samplePage = `<html><body>
<p>Some intro text before any heading.</p>
<h2>Brochure (M - 2 files)</h2>
<table>
<tr><th>Description</th><th>Rev</th><th>Date</th><th>Size</th><th>Type</th></tr>
<tr>
<td data-label="none"><a id="ctl00_1_idFileUrl" href="/download/files/brochure/Matrix50SeriesBebro.pdf" target="download">Matrix 50</a></td>
<td data-label="none"><span class="d-sm-none">Rev</span><nobr>B</nobr></td>
<td data-label="none"><span class="d-sm-none">Date</span><nobr>Jun. 24, 2002</nobr></td>
<td data-label="none" class="text-md-right"><span class="d-sm-none">Size</span><nobr>22.8 KB</nobr></td>
<td data-label="none"><span class="d-sm-none">Type</span><nobr>PDF</nobr></td>
</tr>
<tr>
<td><a id="ctl00_2_idFileUrl" href="/download/files/userman/68-3006-50_B-12G_HD-SDI_101.pdf" target="download">12G HD-SDI Setup Guide</a></td>
<td><nobr>C</nobr></td><td><nobr>Mar. 1, 2024</nobr></td><td><nobr>2.4 MB</nobr></td><td><nobr>PDF</nobr></td>
</tr>
</table>
<h3>Manual (M - 1 files)</h3>
<table>
<tr>
<td><a id="ctl00_3_idFileUrl" href="/download/files/userman/MAVPlusUserGuide.pdf" target="download">MAV Plus Series User Guide</a></td>
<td><nobr>Rev E</nobr></td><td><nobr>Jan. 15, 2021</nobr></td><td><nobr>1.1 MB</nobr></td><td><nobr>PDF</nobr></td>
</tr>
</table>
<h3>Revit BIM files</h3>
<table>
<tr>
<td><a id="ctl00_4_idFileUrl" href="/download/files/bim/MAV88_Revit.zip" target="download">MAV 88 BIM Family</a></td>
<td><nobr></nobr></td><td><nobr>May 2, 2023</nobr></td><td><nobr>310 KB</nobr></td><td><nobr>ZIP</nobr></td>
</tr>
</table>
</body></html>`

func TestParseIndexRowsAndCategories(t *testing.T) {
	docs, err := ParseIndex([]byte(samplePage))
	if err != nil {
		t.Fatalf("ParseIndex() error = %v", err)
	}
	if len(docs) != 4 {
		t.Fatalf("ParseIndex() len = %d, want 4", len(docs))
	}

	got := docs[0]
	want := Doc{
		Title:    "Matrix 50",
		Category: "Brochure",
		Rev:      "B",
		Date:     "Jun. 24, 2002",
		Size:     "22.8 KB",
		Type:     "PDF",
		URL:      "/download/files/brochure/Matrix50SeriesBebro.pdf",
	}
	if got != want {
		t.Errorf("docs[0] = %+v, want %+v", got, want)
	}

	if got := docs[1]; got.Category != "Brochure" || got.Rev != "C" || got.Size != "2.4 MB" {
		t.Errorf("docs[1] = %+v, want Brochure/C/2.4 MB", got)
	}
	if got := docs[2]; got.Category != "Manual" || got.Rev != "Rev E" {
		t.Errorf("docs[2] = %+v, want Manual/Rev E", got)
	}
	if got := docs[3]; got.Category != "Revit BIM" || got.Rev != "" {
		t.Errorf("docs[3] = %+v, want Revit BIM with empty rev", got)
	}
}

func TestParseIndexDeduplicatesByURL(t *testing.T) {
	// Same URL twice (a doc listed under two tables) must appear once.
	dup := `<h2>Brochure (M - 1 files)</h2>` + samplePage
	docs, err := ParseIndex([]byte(dup))
	if err != nil {
		t.Fatalf("ParseIndex() error = %v", err)
	}
	seen := map[string]int{}
	for _, d := range docs {
		seen[d.URL]++
	}
	for url, n := range seen {
		if n > 1 {
			t.Errorf("url %s parsed %d times, want 1", url, n)
		}
	}
}

func TestParseIndexNoRows(t *testing.T) {
	if _, err := ParseIndex([]byte("<html><body>Nothing here</body></html>")); err == nil {
		t.Fatal("ParseIndex() on empty page = nil error, want error")
	}
}

func TestCategoryFromHeading(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Brochure (M - 95 files)", "Brochure"},
		{"Declaration of Conformity (M - 109 files)", "Declaration of Conformity"},
		{"Revit BIM files", "Revit BIM"},
		{"Manual (All - 1977 files)", "Manual"},
		{"Product Guide (M - 0 files)", "Product Guide"},
	}
	for _, c := range cases {
		if got := categoryFromHeading(c.in); got != c.want {
			t.Errorf("categoryFromHeading(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIndexURL(t *testing.T) {
	c := New()
	if got, want := c.IndexURL("M"), "https://www.extron.com/technology/literature.aspx?defaultLang=true&id=M&tabid=5"; got != want {
		t.Errorf("IndexURL(M) = %q, want %q", got, want)
	}
	if got := c.IndexURL(""); got != c.IndexURL("All") {
		t.Errorf("IndexURL('') = %q, want All variant %q", got, c.IndexURL("All"))
	}
}

func TestAbsoluteURL(t *testing.T) {
	c := New()
	if got, want := c.AbsoluteURL("/download/files/brochure/x.pdf"), "https://www.extron.com/download/files/brochure/x.pdf"; got != want {
		t.Errorf("AbsoluteURL = %q, want %q", got, want)
	}
	if got := c.AbsoluteURL("https://cdn.example.com/x.pdf"); got != "https://cdn.example.com/x.pdf" {
		t.Errorf("AbsoluteURL absolute = %q", got)
	}
}

func TestDefaultLetters(t *testing.T) {
	if len(DefaultLetters) != 27 {
		t.Fatalf("DefaultLetters len = %d, want 27", len(DefaultLetters))
	}
	if DefaultLetters[0] != "0" || DefaultLetters[1] != "A" || DefaultLetters[26] != "Z" {
		t.Errorf("DefaultLetters = %v", DefaultLetters)
	}
}
