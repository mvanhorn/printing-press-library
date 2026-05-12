// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package crawl

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/corpus"
)

func TestCleanText(t *testing.T) {
	in := `<html><head><title>X</title><style>body{color:red}</style></head>
<body><script>var a=1</script><p>Hello&nbsp;&amp; <b>world</b></p><div>line two</div><!-- a comment --></body></html>`
	got := CleanText(in)
	if strings.Contains(got, "color:red") || strings.Contains(got, "var a=1") || strings.Contains(got, "a comment") {
		t.Errorf("CleanText kept script/style/comment content: %q", got)
	}
	if !strings.Contains(got, "Hello & world") {
		t.Errorf("CleanText: expected decoded entities; got %q", got)
	}
	if !strings.Contains(got, "line two") {
		t.Errorf("CleanText: expected block content; got %q", got)
	}
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Errorf("CleanText left tags: %q", got)
	}
}

func TestTitle(t *testing.T) {
	if got := Title(`<html><head><title> Ed's FAQ &amp; Stuff </title></head></html>`); got != "Ed's FAQ & Stuff" {
		t.Errorf("Title = %q; want \"Ed's FAQ & Stuff\"", got)
	}
	if got := Title(`<html><body>no title</body></html>`); got != "" {
		t.Errorf("Title = %q; want empty", got)
	}
}

func TestAbsLinksAndResolve(t *testing.T) {
	page := "https://www.seykota.com/tt/FAQ_Index/index.htm"
	html := `<a href="../2023/JAN/01-31/default.html">a</a>
<a href="../../tribe/risk/index.htm">b</a>
<a href="/tribe/TSP/index.htm">c</a>
<a href="https://www.seykota.com/x.html">d</a>
<a href="https://example.com/elsewhere">e</a>
<a href="#frag">f</a>
<a href="mailto:user@example.com">g</a>`
	got := AbsLinks(html, page)
	want := map[string]bool{
		"https://www.seykota.com/tt/2023/JAN/01-31/default.html": true,
		"https://www.seykota.com/tribe/risk/index.htm":           true,
		"https://www.seykota.com/tribe/TSP/index.htm":            true,
		"https://www.seykota.com/x.html":                         true,
	}
	if len(got) != len(want) {
		t.Errorf("AbsLinks = %v; want %d links", got, len(want))
	}
	for _, u := range got {
		if !want[u] {
			t.Errorf("unexpected link %q", u)
		}
	}
}

func TestFAQMonthLinks(t *testing.T) {
	idx := `https://www.seykota.com/tt/FAQ_Index/`
	html := `<a href="../2023/OCT/01-31/default.html">x</a>
<a href="../2010/Sep/15-30/default.html">y</a>
<a href="../mail/default.html">not a month</a>
<a href="../tribe/FAQ/2006_Apr/22/index.htm">old</a>`
	got := FAQMonthLinks(html, idx)
	if len(got) != 3 {
		t.Fatalf("FAQMonthLinks = %v; want 3", got)
	}
	for _, u := range got {
		if !reFAQMonthTT.MatchString(u) && !reFAQDayOld.MatchString(u) {
			t.Errorf("non-FAQ link slipped through: %q", u)
		}
	}
}

func TestParseFAQMonth(t *testing.T) {
	u := "https://www.seykota.com/tt/2007/Jul/01-31/default.html"
	html := `<html><head><title>Ed's FAQ Jul 01-31, 2007</title></head><body>
<p>Contributors</p><p>Dave Druz, Sam Q</p>
<p>Dear Ed, here is a question about heat and pyramiding.</p>
<p>Reply: think about your stops.</p></body></html>`
	d := ParseFAQMonth(u, html)
	if d.Source != corpus.SourceFAQ {
		t.Errorf("source = %q", d.Source)
	}
	if d.Year != "2007" || d.Month != "Jul" || d.Range != "01-31" {
		t.Errorf("year/month/range = %q/%q/%q", d.Year, d.Month, d.Range)
	}
	if d.MonthN != 7 {
		t.Errorf("monthN = %d; want 7", d.MonthN)
	}
	if d.ID != "tt/2007/Jul/01-31/default.html" {
		t.Errorf("id = %q", d.ID)
	}
	if !strings.Contains(d.Body, "heat and pyramiding") {
		t.Errorf("body missing content: %q", d.Body)
	}
	// contributors are best-effort; if parsed, Dave Druz should be in there
	if len(d.Contributors) > 0 {
		found := false
		for _, c := range d.Contributors {
			if strings.Contains(strings.ToLower(c), "druz") {
				found = true
			}
		}
		if !found {
			t.Logf("contributors parsed but Druz not found: %v (best-effort, not fatal)", d.Contributors)
		}
	}
}

func TestParseTSPSection(t *testing.T) {
	u := "https://www.seykota.com/tribe/TSP/EA/index.htm"
	html := `<html><head><title>EA Crossover</title></head><body>
<p>Updated October 26, 2005</p>
<p>The exponential crossover system: enter when the fast EMA crosses above the slow EMA.</p></body></html>`
	d := ParseTSPSection(u, html, 3)
	if d.Source != corpus.SourceTSP || d.Slug != "EA" {
		t.Errorf("source/slug = %q/%q", d.Source, d.Slug)
	}
	if d.Title != "EA Crossover" {
		t.Errorf("title = %q", d.Title)
	}
	if !strings.Contains(d.Updated, "October 26, 2005") {
		t.Errorf("updated = %q; want to contain 'October 26, 2005'", d.Updated)
	}
	if d.Ord != 3 {
		t.Errorf("ord = %d; want 3", d.Ord)
	}
	if !strings.Contains(d.Body, "exponential crossover") {
		t.Errorf("body missing content: %q", d.Body)
	}
	// "New Page 1" titles fall back to slug
	d2 := ParseTSPSection("https://www.seykota.com/tribe/TSP/SR/index.htm", `<html><head><title>New Page 1</title></head><body>support and resistance</body></html>`, 0)
	if d2.Title != "SR" {
		t.Errorf("expected slug fallback for 'New Page 1' title; got %q", d2.Title)
	}
}

func TestParseRiskEssayAndSectionWindow(t *testing.T) {
	u := "https://www.seykota.com/tribe/risk/index.htm"
	html := `<html><head><title>Risk Management</title></head><body>
<b>Risk Management</b><p>Intro about the possibility of loss.</p>
<b>The Coin Toss Example</b><p>A fair coin paying two to one.</p>
<b>The Kelly Formula</b><p>K equals W minus one minus W over R.</p>
<b>The Uncle Point</b><p>The equity level you would quit at.</p></body></html>`
	d := ParseRiskEssay(u, html)
	if d.Source != corpus.SourceRisk {
		t.Errorf("source = %q", d.Source)
	}
	if !strings.Contains(d.Body, "Kelly") {
		t.Errorf("body missing Kelly: %q", d.Body)
	}
	win, ok := RiskSectionWindow(d.Body, "The Kelly Formula")
	if !ok {
		t.Fatalf("section window not found")
	}
	if !strings.Contains(win, "W minus one minus W over R") {
		t.Errorf("section window missing the formula text: %q", win)
	}
	if strings.Contains(win, "equity level you would quit") {
		t.Errorf("section window leaked into the next section: %q", win)
	}
	// unknown section -> whole body, ok=false
	_, ok2 := RiskSectionWindow(d.Body, "Nonexistent Section")
	if ok2 {
		t.Errorf("expected ok=false for an unknown section")
	}
	if hs := RiskHeadings(); len(hs) < 5 {
		t.Errorf("RiskHeadings returned too few: %v", hs)
	}
}

func TestParseContributors(t *testing.T) {
	body := "Ed's FAQ Jul 01-31, 2007\nContributors\nDave Druz, Sam Q; Pat Lee\nDear Ed, my question is..."
	got := parseContributors(body)
	// best effort — but with this clean input it should pull at least one name
	if len(got) == 0 {
		t.Errorf("expected at least one contributor from clean input; got none")
	}
	for _, n := range got {
		if strings.ContainsAny(n, ".?!") || len(strings.Fields(n)) > 5 {
			t.Errorf("implausible contributor name: %q", n)
		}
	}
	// no "Contributors" word -> nil
	if parseContributors("just some prose with no contributor block at all") != nil {
		t.Errorf("expected nil when no Contributors block present")
	}
}
