// Copyright 2026 Dave Morin and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Search parsing/output tests. The fixture at testdata/search_kanye_west.html
// is a trimmed live capture of /search/d3results.jsp (query "kanye west",
// fetched 2026-07-04); see the comment at the top of that file.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func loadSearchFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/search_kanye_west.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

// newRenderTestCmd builds a bare cobra command with captured stdout/stderr for
// exercising renderSearchResults without HTTP, per the novel-scaffold-test
// buffer-capture shape.
func newRenderTestCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := &cobra.Command{Use: "test"}
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	return cmd, &out, &errBuf
}

// Happy path: the fixture carries exactly four story items (publication
// anchor + headline anchor + iinf date each). The parser must emit exactly
// those four records with the right source domain, decoded headline, link,
// and ISO date — and nothing from the nav header, the publication cites, the
// prevnext pagination block, or the post-results /r2/ sponsor item.
func TestParseSearchResults_Fixture(t *testing.T) {
	results, warn := parseSearchResults(loadSearchFixture(t))
	if warn {
		t.Errorf("fixture has iinf blocks; anchorsWithoutDates should be false")
	}
	if len(results) != 4 {
		var got []string
		for _, r := range results {
			got = append(got, r.Headline)
		}
		t.Fatalf("want 4 story records, got %d:\n%s", len(results), strings.Join(got, "\n"))
	}

	want := []struct {
		source       string
		headlinePart string
		link         string
		date         string
	}{
		{"fortune.com", "Slash, which provides banking services", "https://fortune.com/article/slash-vertical-banking-nea-fintech-tech-funding-performance-marketing/", "2025-05-22"},
		{"404media.co", "Instagram hasn't scrubbed", "https://www.404media.co/kanyes-nazi-song-is-all-over-instagram/", "2025-05-13"},
		{"wsj.com", "Twitter/X reinstates Kanye West's account", "https://www.wsj.com/articles/x-formerly-known-as-twitter-reinstates-kanye-wests-account-7c8c70d2?mod=djemalertNEWS", "2023-07-29"},
		{"theverge.com", "Parler has ~50,000 DAUs", "https://www.theverge.com/2022/10/18/23410816/kanye-parler-acquisition-business-history-free-speech-spam", "2022-10-18"},
	}
	for i, w := range want {
		r := results[i]
		if r.Num != i+1 {
			t.Errorf("record %d: Num = %d, want %d", i, r.Num, i+1)
		}
		if r.Source != w.source {
			t.Errorf("record %d: Source = %q, want %q", i, r.Source, w.source)
		}
		if !strings.Contains(r.Headline, w.headlinePart) {
			t.Errorf("record %d: Headline %q does not contain %q", i, r.Headline, w.headlinePart)
		}
		if r.Link != w.link {
			t.Errorf("record %d: Link = %q, want %q", i, r.Link, w.link)
		}
		if r.Date != w.date {
			t.Errorf("record %d: Date = %q, want %q", i, r.Date, w.date)
		}
	}
}

// Noise filtering: bare publication anchors ("Wall Street Journal",
// "Fortune", …) precede each headline on the page and must never surface as
// result records, and nothing after the prevnext div (pagination links, /r2/
// sponsor promos) is a result.
func TestParseSearchResults_NoPublicationOrPostPrevnextNoise(t *testing.T) {
	results, _ := parseSearchResults(loadSearchFixture(t))
	pubNames := map[string]bool{
		"Fortune": true, "404 Media": true, "Wall Street Journal": true,
		"The Verge": true, "Leaderboard": true, "Newsletter": true, "Soxton": true,
	}
	for _, r := range results {
		if pubNames[r.Headline] {
			t.Errorf("publication/nav anchor leaked as a result record: %q", r.Headline)
		}
		if strings.Contains(r.Link, "/r2/") {
			t.Errorf("sponsor /r2/ promo leaked as a result record: %q", r.Link)
		}
		if strings.Contains(r.Link, "search/d3results.jsp") {
			t.Errorf("prevnext pagination link leaked as a result record: %q", r.Link)
		}
		if strings.Contains(r.Headline, "law for startups") {
			t.Errorf("post-prevnext sponsor headline leaked as a result record: %q", r.Headline)
		}
	}
}

// Entity decoding: headlines carry &ldquo; &rdquo; (and elsewhere &amp;
// &mdash;) which must be decoded in output.
func TestParseSearchResults_EntityDecoding(t *testing.T) {
	results, _ := parseSearchResults(loadSearchFixture(t))
	found := false
	for _, r := range results {
		if strings.Contains(r.Headline, "“Heil Hitler”") {
			found = true
		}
		if strings.Contains(r.Headline, "&ldquo;") || strings.Contains(r.Headline, "&amp;") || strings.Contains(r.Headline, "&mdash;") {
			t.Errorf("undecoded HTML entity in headline: %q", r.Headline)
		}
	}
	if !found {
		t.Errorf("expected a headline containing decoded curly-quoted phrase; got %+v", results)
	}
}

// Degradation: a story whose iinf block has malformed date text is kept with
// date "" rather than dropped — staleness handling belongs to --days and
// downstream consumers, not the parser.
func TestParseSearchResults_MalformedDateKeepsRecord(t *testing.T) {
	page := `<div class="items">
<DIV CLASS="item">
<CITE><A HREF="https://www.example.com/">Example Wire</A>:</CITE>
<STRONG CLASS="L2"><A HREF="https://example.com/story-one">A perfectly plausible headline about technology</A></STRONG>
<div class="iinf">
<SPAN CLASS="idate">sometime recently</SPAN>
</div>
</DIV>
</div>
<div class="prevnext"></div>`
	results, warn := parseSearchResults(page)
	if warn {
		t.Errorf("iinf block present; anchorsWithoutDates should be false")
	}
	if len(results) != 1 {
		t.Fatalf("want 1 record, got %d: %+v", len(results), results)
	}
	if results[0].Date != "" {
		t.Errorf("malformed idate should yield Date \"\", got %q", results[0].Date)
	}
	if results[0].Headline != "A perfectly plausible headline about technology" {
		t.Errorf("wrong headline: %q", results[0].Headline)
	}
}

// Guard: candidate anchors with zero iinf blocks means Techmeme's markup
// shifted under the parser; the flag must fire so the command can warn on
// stderr instead of failing silently to zero results.
func TestParseSearchResults_AnchorsWithoutIinfSetsGuard(t *testing.T) {
	page := `<div class="items">
<STRONG><A HREF="https://example.com/story">A perfectly plausible headline about technology</A></STRONG>
</div>`
	results, warn := parseSearchResults(page)
	if len(results) != 0 {
		t.Errorf("no iinf blocks: want 0 records, got %d", len(results))
	}
	if !warn {
		t.Errorf("candidate anchors with zero iinf blocks must set the guard flag")
	}
}

// Guard, other direction: iinf date blocks with zero candidate anchors also
// means the markup shifted (the anchor shape changed under the parser). The
// flag must fire instead of silently yielding empty results; this also
// exercises the story == nil defensive branch in the pairing loop.
func TestParseSearchResults_DatesWithoutAnchorsSetsGuard(t *testing.T) {
	page := `<div class="items">
<DIV CLASS="item">
<div class="iinf">
<SPAN CLASS="idate">May 22, 2025, 11:35 AM</SPAN>
</div>
</DIV>
</div>`
	results, warn := parseSearchResults(page)
	if len(results) != 0 {
		t.Errorf("no candidate anchors: want 0 records, got %d: %+v", len(results), results)
	}
	if !warn {
		t.Errorf("iinf blocks with zero candidate anchors must set the guard flag")
	}
}

// A page with neither anchors nor iinf blocks (a genuine zero-hit results
// page) must not fire the guard.
func TestParseSearchResults_EmptyPageNoGuard(t *testing.T) {
	results, warn := parseSearchResults(`<html><body><H2>Results</H2><div class="prevnext"></div></body></html>`)
	if len(results) != 0 {
		t.Errorf("want 0 records, got %d", len(results))
	}
	if warn {
		t.Errorf("zero anchors must not fire the anchors-without-dates guard")
	}
}

// Real zero-hit page shape (verified live 2026-07-04): nav anchors in the
// header, an empty results canvas with a "did not match any news items"
// paragraph, no items/prevnext divs, and a sponsor region after the canvas.
// This must parse as zero results WITHOUT firing the markup-shift guard —
// header and sponsor anchors are outside the results region.
func TestParseSearchResults_ZeroHitPageShapeNoGuard(t *testing.T) {
	page := `<DIV CLASS="head">
<A HREF="https://www.techmeme.com/lb" TITLE="Techmeme's top authors and sources">Leaderboard</A>
<A ID="froma1" HREF="/newsletter?from=tmd" TITLE="Sign up for Techmeme's newsletter">Newsletter</A>
</DIV>
<DIV CLASS="ed">
<DIV class="results">
<DIV class="resultscanvas">
<p>Your search - <span style="font-weight:bold;">zzqx nonexistent</span> - did not match any news items</p>
</DIV> <!-- resultscanvas -->
</DIV> <!-- results -->
<DIV class="sponsorscanvas">
<CITE><A HREF="/r2/www.soxton.ai_-HTDiPEpl.htm">Soxton</A>:</CITE>
<STRONG CLASS="L1"><A HREF="/r2/www.soxton.ai_-HTDiPEpl.htm">Fast, affordable law for startups</A></STRONG>
</DIV>`
	results, warn := parseSearchResults(page)
	if len(results) != 0 {
		t.Errorf("zero-hit page: want 0 records, got %d: %+v", len(results), results)
	}
	if warn {
		t.Errorf("genuine zero-hit page must not fire the anchors-without-dates guard")
	}
}

// Date layouts: full and abbreviated month names, with and without the time
// suffix Techmeme appends, ISO output, empty string on failure.
func TestParseSearchDate(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"May 22, 2025, 11:35 AM", "2025-05-22"},
		{"Oct 18, 2022, 1:40 PM", "2022-10-18"},
		{"July 29, 2023, 8:10 PM", "2023-07-29"},
		{"January 2, 2026", "2026-01-02"},
		{"Jan 2, 2026", "2026-01-02"},
		{" Dec 1, 2022, 4:55 PM ", "2022-12-01"},
		{"sometime recently", ""},
		{"", ""},
		{"2025-05-22", ""},
	}
	for _, c := range cases {
		if got := parseSearchDate(c.in); got != c.want {
			t.Errorf("parseSearchDate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --days N drops out-of-window and undated records and renumbers survivors;
// days 0 disables filtering entirely (undated records retained).
func TestFilterSearchByDays(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	recent := now.AddDate(0, 0, -5).Format("2006-01-02")
	stale := now.AddDate(0, 0, -400).Format("2006-01-02")
	in := []searchResult{
		{Num: 1, Headline: "recent", Date: recent},
		{Num: 2, Headline: "stale", Date: stale},
		{Num: 3, Headline: "undated", Date: ""},
	}

	got := filterSearchByDays(in, 30, now)
	if len(got) != 1 || got[0].Headline != "recent" {
		t.Fatalf("days=30: want only the 5-day-old record, got %+v", got)
	}
	if got[0].Num != 1 {
		t.Errorf("days=30: surviving record should be renumbered to 1, got %d", got[0].Num)
	}

	if got := filterSearchByDays(in, 0, now); len(got) != 3 {
		t.Errorf("days=0: want all 3 records (no filtering), got %d", len(got))
	}
}

// Boundary pin: a record dated exactly N days before now must survive --days N
// regardless of now's time-of-day or zone. Record dates parse to midnight UTC,
// so a cutoff derived from the raw instant (carrying 23:59 and a non-UTC zone)
// would drop the boundary record; the filter must compare calendar dates in
// UTC instead.
func TestFilterSearchByDays_ExactBoundaryIncluded(t *testing.T) {
	zone := time.FixedZone("UTC-8", -8*60*60)
	now := time.Date(2026, 7, 4, 23, 59, 0, 0, zone)
	boundary := now.UTC().AddDate(0, 0, -30).Format("2006-01-02") // exactly 30 days ago

	in := []searchResult{
		{Num: 1, Headline: "boundary", Date: boundary},
	}
	got := filterSearchByDays(in, 30, now)
	if len(got) != 1 || got[0].Headline != "boundary" {
		t.Fatalf("record dated exactly 30 days before now must survive --days 30, got %+v", got)
	}
}

// Zero-hit contract, JSON mode: stdout is exactly a valid empty JSON array —
// parseable, len 0, no prose. This is the bug that broke piped agent
// consumers (JSON decode failed on `No results for ...`).
func TestRenderSearchResults_ZeroHitsJSONEmitsEmptyArray(t *testing.T) {
	cmd, out, errBuf := newRenderTestCmd()
	flags := &rootFlags{asJSON: true}

	if err := renderSearchResults(cmd, flags, "zzqx nonexistent", nil, false); err != nil {
		t.Fatalf("renderSearchResults error: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil {
		t.Fatalf("zero-hit JSON stdout is not a valid JSON array: %v\nstdout: %q", err, out.String())
	}
	if len(arr) != 0 {
		t.Errorf("want empty array, got %d elements", len(arr))
	}
	if strings.Contains(out.String(), "No results") {
		t.Errorf("prose leaked onto stdout in JSON mode: %q", out.String())
	}
	if strings.Contains(out.String(), "null") {
		t.Errorf("nil slice marshaled as null instead of []: %q", out.String())
	}
	_ = errBuf
}

// Zero-hit contract, human mode: the friendly prose line is retained.
func TestRenderSearchResults_ZeroHitsHumanKeepsProse(t *testing.T) {
	cmd, out, _ := newRenderTestCmd()
	flags := &rootFlags{}

	if err := renderSearchResults(cmd, flags, "zzqx nonexistent", nil, false); err != nil {
		t.Fatalf("renderSearchResults error: %v", err)
	}
	if !strings.Contains(out.String(), `No results for "zzqx nonexistent"`) {
		t.Errorf("human mode should keep the No results prose, got %q", out.String())
	}
}

// Populated JSON mode: every record carries the date key.
func TestRenderSearchResults_JSONRecordsCarryDate(t *testing.T) {
	cmd, out, _ := newRenderTestCmd()
	flags := &rootFlags{asJSON: true}
	results := []searchResult{
		{Num: 1, Source: "example.com", Headline: "A perfectly plausible headline", Link: "https://example.com/a", Date: "2026-07-01"},
		{Num: 2, Source: "example.org", Headline: "Another plausible headline here", Link: "https://example.org/b", Date: ""},
	}

	if err := renderSearchResults(cmd, flags, "q", results, false); err != nil {
		t.Fatalf("renderSearchResults error: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("want 2 records, got %d", len(arr))
	}
	for i, obj := range arr {
		if _, ok := obj["date"]; !ok {
			t.Errorf("record %d missing date key: %v", i, obj)
		}
	}
	if arr[0]["date"] != "2026-07-01" {
		t.Errorf("record 0 date = %v, want 2026-07-01", arr[0]["date"])
	}
}

// Flag interplay: --select and --compact flow through printOutputWithFlags on
// search output and still produce valid JSON.
func TestRenderSearchResults_SelectAndCompact(t *testing.T) {
	results := []searchResult{
		{Num: 1, Source: "example.com", Headline: "A perfectly plausible headline", Link: "https://example.com/a", Date: "2026-07-01"},
	}

	// --json --select headline,date
	cmd, out, _ := newRenderTestCmd()
	if err := renderSearchResults(cmd, &rootFlags{asJSON: true, selectFields: "headline,date"}, "q", results, false); err != nil {
		t.Fatalf("--select render error: %v", err)
	}
	var sel []map[string]any
	if err := json.Unmarshal(out.Bytes(), &sel); err != nil {
		t.Fatalf("--select output invalid JSON: %v\n%q", err, out.String())
	}
	if len(sel) != 1 {
		t.Fatalf("--select: want 1 record, got %d", len(sel))
	}
	if _, ok := sel[0]["headline"]; !ok {
		t.Errorf("--select headline,date lost the headline field: %v", sel[0])
	}
	if _, ok := sel[0]["date"]; !ok {
		t.Errorf("--select headline,date lost the date field: %v", sel[0])
	}
	if _, ok := sel[0]["link"]; ok {
		t.Errorf("--select headline,date should drop link: %v", sel[0])
	}

	// --json --compact (the shape --agent implies). The shared compaction
	// allow-list contains none of searchResult's keys, so without the
	// search-local select default every record would compact to {} — the
	// bug that made `search --agent` return [{}]. Assert the real field
	// values survive, not just that the array parses.
	cmd2, out2, _ := newRenderTestCmd()
	compactFlags := &rootFlags{asJSON: true, compact: true}
	if err := renderSearchResults(cmd2, compactFlags, "q", results, false); err != nil {
		t.Fatalf("--compact render error: %v", err)
	}
	var comp []map[string]any
	if err := json.Unmarshal(out2.Bytes(), &comp); err != nil {
		t.Fatalf("--compact output invalid JSON: %v\n%q", err, out2.String())
	}
	if len(comp) != 1 {
		t.Fatalf("--compact: want 1 record, got %d", len(comp))
	}
	if comp[0]["headline"] != "A perfectly plausible headline" {
		t.Errorf("--compact: headline = %v, want the populated headline (record compacted to {}?)", comp[0]["headline"])
	}
	if comp[0]["date"] != "2026-07-01" {
		t.Errorf("--compact: date = %v, want 2026-07-01", comp[0]["date"])
	}
	if comp[0]["link"] != "https://example.com/a" {
		t.Errorf("--compact: link = %v, want https://example.com/a", comp[0]["link"])
	}
	if compactFlags.selectFields != "" {
		t.Errorf("--compact: shared flags struct was mutated (selectFields = %q)", compactFlags.selectFields)
	}

	// Zero hits with --compact must still be a parseable empty array.
	cmd3, out3, _ := newRenderTestCmd()
	if err := renderSearchResults(cmd3, &rootFlags{asJSON: true, compact: true}, "q", nil, false); err != nil {
		t.Fatalf("--compact zero-hit render error: %v", err)
	}
	var compEmpty []map[string]any
	if err := json.Unmarshal(out3.Bytes(), &compEmpty); err != nil {
		t.Fatalf("--compact zero-hit output invalid JSON: %v\n%q", err, out3.String())
	}
	if len(compEmpty) != 0 {
		t.Errorf("--compact zero-hit: want empty array, got %d elements", len(compEmpty))
	}
}

// Guard surfacing: the anchors-without-iinf warning goes to stderr, and
// stdout stays a pure JSON array in JSON mode.
func TestRenderSearchResults_MarkupShiftWarningOnStderr(t *testing.T) {
	cmd, out, errBuf := newRenderTestCmd()
	flags := &rootFlags{asJSON: true}

	if err := renderSearchResults(cmd, flags, "q", nil, true); err != nil {
		t.Fatalf("renderSearchResults error: %v", err)
	}
	if !strings.Contains(errBuf.String(), "iinf") {
		t.Errorf("expected markup-shift warning on stderr, got %q", errBuf.String())
	}
	var arr []map[string]any
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil {
		t.Fatalf("stdout must stay a pure JSON array when warning fires: %v\n%q", err, out.String())
	}
}

// Human mode renders a DATE column: the header appears and each record's ISO
// date value lands in its row. Also pins the warn + human-mode combination:
// the markup-shift warning goes to stderr and the table still renders.
func TestRenderSearchResults_HumanModeShowsDateColumn(t *testing.T) {
	results := []searchResult{
		{Num: 1, Source: "example.com", Headline: "A perfectly plausible headline", Link: "https://example.com/a", Date: "2026-07-01"},
		{Num: 2, Source: "example.org", Headline: "Another plausible headline here", Link: "https://example.org/b", Date: "2025-12-31"},
	}

	cmd, out, errBuf := newRenderTestCmd()
	if err := renderSearchResults(cmd, &rootFlags{}, "q", results, true); err != nil {
		t.Fatalf("renderSearchResults error: %v", err)
	}
	table := out.String()
	if !strings.Contains(table, "DATE") {
		t.Errorf("human mode missing DATE header:\n%s", table)
	}
	for _, want := range []string{"2026-07-01", "2025-12-31"} {
		if !strings.Contains(table, want) {
			t.Errorf("human mode missing date value %q:\n%s", want, table)
		}
	}
	if !strings.Contains(errBuf.String(), "markup may have changed") {
		t.Errorf("expected markup-shift warning on stderr in human mode, got %q", errBuf.String())
	}
	if strings.Contains(table, "markup may have changed") {
		t.Errorf("warning leaked onto stdout: %q", table)
	}
}

// Wiring smoke test per the novel-scaffold-test shape: `search --help`
// resolves through the root command tree and renders usage including the new
// --days flag.
func TestSearchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"search", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search --help error = %v (command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "search", "--days"} {
		if !strings.Contains(help, want) {
			t.Fatalf("search --help missing %q in output:\n%s", want, help)
		}
	}
}

// A Techmeme story-permalink anchor (the "In context" link inside each iinf
// block) sits positionally after its own iinf match; if it survived the
// candidate filter it would pair with the NEXT story's date block and
// misattribute that story's link. The filter must key on the permalink href
// shape, so a renamed label ("View article") cannot defeat it.
func TestParseSearchResults_RenamedPermalinkStillExcluded(t *testing.T) {
	page := `<div class="items">
<A HREF="https://www.example-pub.com/">Example Publication</A>
<A HREF="https://example-pub.com/story-one">First story headline about the topic today</A>
<div class="iinf">
<span class="idate">May 22, 2025, 11:35 AM</span>
<span class="icontext"><a href="https://www.techmeme.com/250522/p26#a250522p26">View article</a></span>
</div>
<A HREF="https://www.other-pub.com/">Other Publication</A>
<A HREF="https://other-pub.com/story-two">Second story headline about the topic today</A>
<div class="iinf">
<span class="idate">May 13, 2025, 9:00 AM</span>
</div>
<div class="prevnext"></div>
</div>`
	results, warn := parseSearchResults(page)
	if warn {
		t.Errorf("fully paired page must not set the guard flag")
	}
	if len(results) != 2 {
		t.Fatalf("want 2 records, got %d: %+v", len(results), results)
	}
	if results[0].Link != "https://example-pub.com/story-one" {
		t.Errorf("story 1 link misattributed: %q", results[0].Link)
	}
	if results[1].Link != "https://other-pub.com/story-two" {
		t.Errorf("story 2 link misattributed (permalink leak): %q", results[1].Link)
	}
}

// Partial pairing failures must warn too: when some iinf blocks pair and
// others find no preceding candidate anchor, the caller sees incomplete data
// and needs the markup-shift signal.
func TestParseSearchResults_PartialPairingSetsGuard(t *testing.T) {
	page := `<div class="items">
<A HREF="https://example-pub.com/story-one">First story headline about the topic today</A>
<div class="iinf">
<span class="idate">May 22, 2025</span>
</div>
<div class="iinf">
<span class="idate">May 13, 2025</span>
</div>
</div>`
	results, warn := parseSearchResults(page)
	if len(results) != 1 {
		t.Fatalf("want 1 paired record, got %d: %+v", len(results), results)
	}
	if !warn {
		t.Errorf("an unpaired iinf block must set the guard flag (partial failure)")
	}
}
