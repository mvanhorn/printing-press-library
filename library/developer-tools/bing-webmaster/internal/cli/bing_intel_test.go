// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseQueryRows(t *testing.T) {
	data := json.RawMessage(`[
		{"Query":"shoes","Impressions":100,"Clicks":10,"AvgImpressionPosition":4.5},
		{"Query":"boots","Impressions":"50","Clicks":"5","AvgClickPosition":"7"},
		{"Impressions":1}
	]`)
	rows := bParseQueryRows(data)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (row without query skipped)", len(rows))
	}
	if rows[0].Query != "shoes" || rows[0].Impressions != 100 || rows[0].Position != 4.5 {
		t.Errorf("row0 = %+v", rows[0])
	}
	// String-encoded numbers + AvgClickPosition fallback.
	if rows[1].Impressions != 50 || rows[1].Position != 7 {
		t.Errorf("row1 = %+v (string numbers / position fallback)", rows[1])
	}
}

func TestParseQueryRowsEmpty(t *testing.T) {
	if got := bParseQueryRows(json.RawMessage(`[]`)); len(got) != 0 {
		t.Errorf("empty array -> %d rows", len(got))
	}
	if got := bParseQueryRows(json.RawMessage(`null`)); len(got) != 0 {
		t.Errorf("null -> %d rows", len(got))
	}
}

func TestComputeReview(t *testing.T) {
	prior := []bQueryRow{{Query: "a", Impressions: 100, Clicks: 10, Position: 5}, {Query: "gone", Position: 9}}
	current := []bQueryRow{{Query: "a", Impressions: 150, Clicks: 12, Position: 3}, {Query: "new", Position: 8}}
	r := bComputeReview(prior, current, 7)
	if r.Summary.GainedCount != 1 || r.Gained[0] != "new" {
		t.Errorf("gained = %v", r.Gained)
	}
	if r.Summary.LostCount != 1 || r.Lost[0] != "gone" {
		t.Errorf("lost = %v", r.Lost)
	}
	if r.Summary.ImprovedCount != 1 { // "a" moved 5 -> 3 (improved)
		t.Errorf("improved = %d, want 1", r.Summary.ImprovedCount)
	}
	if len(r.Moved) != 1 || r.Moved[0].PositionDelta != -2 || r.Moved[0].ImpressionDelta != 50 {
		t.Errorf("moved = %+v", r.Moved)
	}
}

func TestComputeReviewEmpty(t *testing.T) {
	r := bComputeReview(nil, nil, 7)
	if r.Summary.GainedCount != 0 || r.Summary.LostCount != 0 || len(r.Moved) != 0 {
		t.Errorf("empty review not zero: %+v", r.Summary)
	}
}

func TestComputeDrift(t *testing.T) {
	prior := []bQueryRow{{Query: "up", Position: 10}, {Query: "down", Position: 2}, {Query: "flat", Position: 4}}
	current := []bQueryRow{{Query: "up", Position: 3}, {Query: "down", Position: 9}, {Query: "flat", Position: 4}}
	d := bComputeDrift(prior, current, 5, 7)
	if len(d.Climbers) != 1 || d.Climbers[0].Query != "up" || d.Climbers[0].PositionDelta != -7 {
		t.Errorf("climbers = %+v", d.Climbers)
	}
	if len(d.Droppers) != 1 || d.Droppers[0].Query != "down" || d.Droppers[0].PositionDelta != 7 {
		t.Errorf("droppers = %+v", d.Droppers)
	}
}

func TestComputeDriftTopLimit(t *testing.T) {
	prior := []bQueryRow{{Query: "a", Position: 10}, {Query: "b", Position: 10}, {Query: "c", Position: 10}}
	current := []bQueryRow{{Query: "a", Position: 1}, {Query: "b", Position: 2}, {Query: "c", Position: 3}}
	d := bComputeDrift(prior, current, 2, 7)
	if len(d.Climbers) != 2 {
		t.Errorf("top=2 should cap climbers at 2, got %d", len(d.Climbers))
	}
	if d.Climbers[0].Query != "a" { // biggest climb first
		t.Errorf("climbers[0] = %s, want a", d.Climbers[0].Query)
	}
}

func TestExtractLocs(t *testing.T) {
	xml := []byte(`<urlset><url><loc>https://x.com/a</loc></url><url><loc> https://x.com/b </loc></url></urlset>`)
	got := bExtractLocs(xml)
	if len(got) != 2 || got[0] != "https://x.com/a" || got[1] != "https://x.com/b" {
		t.Errorf("locs = %v", got)
	}
}

func TestExtractLocsCDATA(t *testing.T) {
	xml := []byte(`<url><loc><![CDATA[https://x.com/c]]></loc></url>`)
	got := bExtractLocs(xml)
	if len(got) != 1 || got[0] != "https://x.com/c" {
		t.Errorf("cdata loc = %v", got)
	}
}

func TestIsSitemapIndex(t *testing.T) {
	if !bIsSitemapIndex([]byte(`<?xml?><sitemapindex><sitemap><loc>x</loc></sitemap></sitemapindex>`)) {
		t.Error("should detect sitemapindex")
	}
	if bIsSitemapIndex([]byte(`<urlset><url><loc>x</loc></url></urlset>`)) {
		t.Error("urlset is not an index")
	}
}

func TestChunk(t *testing.T) {
	urls := make([]string, 1100)
	chunks := bChunk(urls, 500)
	if len(chunks) != 3 || len(chunks[0]) != 500 || len(chunks[2]) != 100 {
		t.Errorf("chunks: %d sizes %d/%d/%d", len(chunks), len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
}

func TestDedupeURLs(t *testing.T) {
	got := bDedupeURLs([]string{"a", " a ", "", "b", "a"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("dedupe = %v", got)
	}
}

func TestDetectQueryColumn(t *testing.T) {
	cases := []struct {
		header []string
		want   int
	}{
		{[]string{"Top queries", "Clicks", "Impressions"}, 0},
		{[]string{"Date", "Query", "Clicks"}, 1},
		{[]string{"Keyword", "Volume"}, 0},
		{[]string{"Clicks", "Impressions"}, -1},
	}
	for _, c := range cases {
		if got := bDetectQueryColumn(c.header); got != c.want {
			t.Errorf("DetectQueryColumn(%v) = %d, want %d", c.header, got, c.want)
		}
	}
}

func TestParseGSCQueries(t *testing.T) {
	csv := "Top queries,Clicks,Impressions\nshoes,10,100\nboots,5,50\nshoes,1,1\n"
	got, err := bParseGSCQueries(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 || got[0] != "shoes" || got[1] != "boots" {
		t.Errorf("gsc queries = %v (deduped)", got)
	}
}

func TestComputeGap(t *testing.T) {
	bing := []string{"shoes", "boots", "Sandals"}
	gsc := []string{"shoes", "sandals", "heels"}
	g := bComputeGap(bing, gsc)
	if g.Summary.BothCount != 2 { // shoes + sandals (case-insensitive)
		t.Errorf("both = %d, want 2", g.Summary.BothCount)
	}
	if g.Summary.BingOnlyCount != 1 || g.BingOnly[0] != "boots" {
		t.Errorf("bing-only = %v", g.BingOnly)
	}
	if g.Summary.GoogleOnlyCount != 1 || g.GoogleOnly[0] != "heels" {
		t.Errorf("google-only = %v", g.GoogleOnly)
	}
}

func TestDecodeCrawlIssues(t *testing.T) {
	if got := bDecodeCrawlIssues(0); len(got) != 1 || got[0] != "none" {
		t.Errorf("zero flags = %v", got)
	}
	got := bDecodeCrawlIssues(1 | 32) // 300-redirect + 404
	if len(got) != 2 {
		t.Errorf("two flags = %v", got)
	}
	// Unknown high bit reported, not dropped.
	un := bDecodeCrawlIssues(1 << 30)
	if len(un) != 1 || !strings.HasPrefix(un[0], "other(") {
		t.Errorf("unknown bit = %v", un)
	}
}

func TestParseTriageIssues(t *testing.T) {
	data := []byte(`[{"Url":"https://x.com/a","Issues":32},{"Url":"https://x.com/b","CrawlIssues":0}]`)
	got := parseTriageIssues(data)
	if len(got) != 2 {
		t.Fatalf("issues = %d", len(got))
	}
	if got[0].URL != "https://x.com/a" || len(got[0].Categories) == 0 {
		t.Errorf("issue0 = %+v", got[0])
	}
}

func TestParseFeedRows(t *testing.T) {
	data := json.RawMessage(`[{"Url":"https://x.com/sitemap.xml","UrlCount":100,"DiscoveredUrlCount":80,"IndexedUrlCount":60}]`)
	got := parseFeedRows(data)
	if len(got) != 1 || got[0].Submitted != 100 || got[0].Discovered != 80 || got[0].Indexed != 60 {
		t.Errorf("feed = %+v", got)
	}
}

func TestSumDailyStats(t *testing.T) {
	data := json.RawMessage(`[{"Impressions":10},{"Impressions":"20"},{"Impressions":5}]`)
	if got := sumDailyStats(data, "Impressions"); got != 35 {
		t.Errorf("sum = %v, want 35", got)
	}
	if got := sumDailyStats(json.RawMessage(`[]`), "Impressions"); got != 0 {
		t.Errorf("empty sum = %v", got)
	}
}
