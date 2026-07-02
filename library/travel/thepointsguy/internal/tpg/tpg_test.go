// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
package tpg

import "testing"

func TestClassifyValuationType(t *testing.T) {
	cases := []struct{ heading, want string }{
		{"What are airline points and miles worth?", "airline"},
		{"What are hotel points worth?", "hotel"},
		{"What are credit card points and miles worth?", "transferable"},
		{"What are Bilt Points worth?", "transferable"},
		{"Something else entirely", "other"},
	}
	for _, tc := range cases {
		if got := classifyValuationType(tc.heading); got != tc.want {
			t.Errorf("classifyValuationType(%q) = %q, want %q", tc.heading, got, tc.want)
		}
	}
}

func TestStripTags(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<td>American Airlines AAdvantage</td>", "American Airlines AAdvantage"},
		{"Miles&amp;Smiles", "Miles&Smiles"},
		{"<p>a</p><p>b</p>", "a b"},
		{"  spaced \n out  ", "spaced out"},
	}
	for _, tc := range cases {
		if got := stripTags(tc.in); got != tc.want {
			t.Errorf("stripTags(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseRSSDate(t *testing.T) {
	ok := []string{
		"2026-07-02T16:00:16.000Z",
		"Wed, 02 Jul 2026 16:00:16 +0000",
		"2026-07-02T16:00:16Z",
	}
	for _, s := range ok {
		if _, err := parseRSSDate(s); err != nil {
			t.Errorf("parseRSSDate(%q) unexpected error: %v", s, err)
		}
	}
	if _, err := parseRSSDate("not a date"); err == nil {
		t.Errorf("parseRSSDate(garbage) expected error, got nil")
	}
}

func TestFirstTag(t *testing.T) {
	block := `<item><title><![CDATA[Hello &amp; welcome]]></title><link>https://x/y</link></item>`
	if got := firstTag(block, "title"); got != "Hello & welcome" {
		t.Errorf("firstTag title = %q", got)
	}
	if got := firstTag(block, "link"); got != "https://x/y" {
		t.Errorf("firstTag link = %q", got)
	}
	if got := firstTag(block, "missing"); got != "" {
		t.Errorf("firstTag missing = %q, want empty", got)
	}
}

func TestValuationsParsing(t *testing.T) {
	// Minimal fixture mirroring the live table shape: heading + table with a
	// "valuation" header column, plus footnote/markup noise in the cents cell.
	html := `
<h2>What are airline points and miles worth?</h2>
<table>
<tr><th>Program</th><th>July 2026 valuation (cents)</th><th>Latest news</th></tr>
<tr><td>American Airlines AAdvantage</td><td>1.6<sup>*</sup></td><td>News here</td></tr>
<tr><td>United MileagePlus</td><td>1.35</td><td></td></tr>
</table>
<h2>What are hotel points worth?</h2>
<table>
<tr><th>Program</th><th>July 2026 valuation (cents)</th></tr>
<tr><td>World of Hyatt</td><td>1.7</td></tr>
</table>
<h2>Unrelated card rewards</h2>
<table>
<tr><th>Tier</th><th>Description</th></tr>
<tr><td>4X</td><td>Earn points</td></tr>
</table>`
	vals, month := parseValuationsHTML(html, "https://thepointsguy.com/loyalty-programs/monthly-valuations/")
	if month != "July 2026" {
		t.Errorf("month = %q, want July 2026", month)
	}
	if len(vals) != 3 {
		t.Fatalf("got %d valuations, want 3: %+v", len(vals), vals)
	}
	byName := map[string]Valuation{}
	for _, v := range vals {
		byName[v.Program] = v
	}
	if v := byName["American Airlines AAdvantage"]; v.CentsPerPoint != 1.6 || v.Type != "airline" {
		t.Errorf("AAdvantage = %+v, want 1.6 airline (footnote should be stripped)", v)
	}
	if v := byName["World of Hyatt"]; v.CentsPerPoint != 1.7 || v.Type != "hotel" {
		t.Errorf("Hyatt = %+v, want 1.7 hotel", v)
	}
	if _, ok := byName["4X"]; ok {
		t.Errorf("non-valuation table row leaked into results")
	}
}
