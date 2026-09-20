// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// EVERY NUMBER ASSERTED IN THIS FILE WAS MEASURED BY RUNNING CODE, not copied
// from a document. Where a document and the code disagreed, the code won and
// the disagreement is recorded in the test's own comment. Three of those
// happened while this file was written and all three are noted below:
// Main.htm's tenth href is a <link rel=File-List> rather than an <a>; the SIR
// index's per-year hrefs carry RAW spaces; and every text/html body is 361
// bytes larger through this client than through curl.

package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

func sourcesFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	if !strings.HasSuffix(name, ".gz") {
		return raw
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip %s: %v", name, err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress %s: %v", name, err)
	}
	return out
}

// --- catalogue integrity ------------------------------------------------

// TestSourcesCatalogueIntegrity guards the table itself. MEASURED: 63
// documents across five kinds.
func TestSourcesCatalogueIntegrity(t *testing.T) {
	docs := sourcesDocs()
	if len(docs) != 63 {
		t.Errorf("catalogued documents = %d, want 63", len(docs))
	}
	want := map[string]int{"gen": 18, "per": 8, "sir": 22, "fca": 4, "tariff": 11}
	got := map[string]int{}
	ids := map[string]bool{}
	for _, d := range docs {
		got[d.Kind]++
		if ids[d.ID] {
			t.Errorf("duplicate document id %q", d.ID)
		}
		ids[d.ID] = true
		if !strings.HasPrefix(d.ID, d.Kind+"-") {
			t.Errorf("%s id does not carry its kind prefix %q", d.ID, d.Kind+"-")
		}
		if d.State == "" || d.AsOfDate == "" {
			t.Errorf("%s is missing state or as_of_date: %+v", d.ID, d)
		}
		if d.Evidence == "" {
			t.Errorf("%s carries no evidence citation; every number here must cite its measurement", d.ID)
		}
		if d.Path != "" && !strings.HasPrefix(d.Path, "/") {
			t.Errorf("%s path is not rooted: %q", d.ID, d.Path)
		}
		if d.BytesAsOf != nil && *d.BytesAsOf < 0 {
			t.Errorf("%s has a negative byte baseline", d.ID)
		}
		// The honesty invariant that matters most: a row with no route must say
		// why, or the absence is indistinguishable from an oversight.
		if d.FetchWith == "" && !d.Fetchable && d.FetchBlockedReason == "" && d.FetchRefusal == "" {
			t.Errorf("%s has no fetch route and no reason", d.ID)
		}
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("kind %s = %d rows, want %d", k, got[k], n)
		}
	}
	if len(sourcesEnumerators) != 6 {
		t.Errorf("enumerators = %d, want 6 (robots + three dead sitemaps + two live indexes)",
			len(sourcesEnumerators))
	}
	// Four of the six candidate enumerators are dead. That ratio IS the
	// finding: there is no machine enumerator for this site.
	dead := 0
	for _, e := range sourcesEnumerators {
		if e.State == "unavailable" {
			dead++
		}
	}
	if dead != 3 {
		t.Errorf("dead enumerators = %d, want 3 (sitemap_1, sitemap_2, sitemap.xml)", dead)
	}
	if len(sourcesDeclaredGaps) == 0 {
		t.Error("no declared gaps; this command has holes and must say so")
	}
	for _, g := range sourcesDeclaredGaps {
		if g.Subject == "" || g.Verdict == "" || g.Reason == "" {
			t.Errorf("a declared gap is incomplete: %+v", g)
		}
	}
}

// TestSourcesCompletenessHoldsOnTheShippedCatalogue runs the in-code assertion.
// It must be silent on what ships; if it is not, the catalogue has drifted from
// nepraper and no output from this command can be trusted.
func TestSourcesCompletenessHoldsOnTheShippedCatalogue(t *testing.T) {
	if problems := sourcesCompleteness(); len(problems) != 0 {
		t.Fatalf("completeness assertion failed on the shipped catalogue:\n  %s",
			strings.Join(problems, "\n  "))
	}
}

// --- the per kind is a JOIN, not a copy ---------------------------------

// TestSourcesPerKindTracksNepraper is the test that makes "reference, not copy"
// enforceable. MUTATION-VERIFIED: appending a hand-written ninth per row to
// sourcesCatalogue makes this fail on the count, and changing any URL makes it
// fail on the identity check.
func TestSourcesPerKindTracksNepraper(t *testing.T) {
	recs := nepraper.Sources()
	rows := sourcesPerRows()
	if len(rows) != len(recs) {
		t.Fatalf("per rows = %d, want %d from nepraper.Sources()", len(rows), len(recs))
	}
	if len(recs) != 8 {
		t.Errorf("nepraper.Sources() = %d records, want 8", len(recs))
	}
	for i, r := range recs {
		if rows[i].Path != r.URLPath {
			t.Errorf("row %d path = %q, nepraper has %q", i, rows[i].Path, r.URLPath)
		}
		if got := nepraSourcesHost + rows[i].Path; got != r.URL() {
			t.Errorf("row %d url = %q, nepraper.URL() = %q", i, got, r.URL())
		}
		if rows[i].State != r.Availability.String() {
			t.Errorf("row %d state = %q, nepraper availability = %q",
				i, rows[i].State, r.Availability.String())
		}
		if rows[i].SourceOfRecord != "internal/nepraper.Sources()" {
			t.Errorf("row %d does not declare nepraper as its source of record", i)
		}
	}
	// Every catalogue row of kind per must come from that join, so a per row
	// hand-written into sourcesCatalogue is caught here too.
	for _, d := range sourcesCatalogue {
		if d.Kind == "per" {
			t.Errorf("per row %q is hand-written into sourcesCatalogue; the per kind must be joined "+
				"from nepraper at runtime", d.ID)
		}
	}
	// MEASURED: 7 published, 1 unavailable, and the unavailable one has no
	// bytes rather than zero bytes.
	published, unavailable := 0, 0
	for _, d := range rows {
		switch d.State {
		case "published":
			published++
			if d.BytesAsOf == nil {
				t.Errorf("%s is published but has no measured size", d.ID)
			}
		case "unavailable":
			unavailable++
			if d.BytesAsOf != nil {
				t.Errorf("%s is unavailable but carries a byte count of %d; UNAVAILABLE must never "+
					"read as a measurement", d.ID, *d.BytesAsOf)
			}
			if d.HTTPStatusAsOf == nil || *d.HTTPStatusAsOf != 404 {
				t.Errorf("%s is unavailable but records no 404", d.ID)
			}
		default:
			t.Errorf("%s has unexpected state %q for a per row", d.ID, d.State)
		}
	}
	if published != 7 || unavailable != 1 {
		t.Errorf("per states = %d published / %d unavailable, want 7 / 1", published, unavailable)
	}
}

// TestSourcesPerFetchArgIsAcceptableToReliability is the load-bearing one.
//
// MEASURED: exactly 5 of the 8 records are rooted at /Standards/ and so
// reachable through `reliability`; the other 3 are rooted at /M&E/ and must
// carry no fetch_arg at all.
//
// MUTATION-VERIFIED, and this is why the round-trip assertion exists: setting
// FY2022-23's fetch_arg to "M&E/PER/Distribution/PER%202022-23%20-%20DSICOs.pdf"
// PASSES validateEncodedSubPath — it is printable ASCII with no ".." — so only
// the "/Standards/" + arg == path check catches it. The sub-test below proves
// the validator alone is not enough.
func TestSourcesPerFetchArgIsAcceptableToReliability(t *testing.T) {
	rows := sourcesPerRows()
	withArg, blocked := 0, 0
	for _, d := range rows {
		if d.FetchArg == "" {
			blocked++
			if d.FetchBlockedReason == "" {
				t.Errorf("%s has no fetch_arg and no reason", d.ID)
			}
			if d.FetchWith != "" {
				t.Errorf("%s advertises %q with no argument", d.ID, d.FetchWith)
			}
			continue
		}
		withArg++
		if err := validateEncodedSubPath("path", d.FetchArg); err != nil {
			t.Errorf("%s emits a fetch_arg `reliability` would refuse: %v", d.ID, err)
		}
		if got := "/Standards/" + d.FetchArg; got != d.Path {
			t.Errorf("%s fetch_arg round-trips to %q but its measured path is %q", d.ID, got, d.Path)
		}
		if !strings.HasPrefix(d.Path, "/Standards/") {
			t.Errorf("%s carries a fetch_arg but is not rooted at /Standards/: %q", d.ID, d.Path)
		}
	}
	if withArg != 5 || blocked != 3 {
		t.Errorf("per rows with a fetch_arg = %d, without = %d; want 5 / 3", withArg, blocked)
	}

	t.Run("validatorAloneWouldAcceptTheMEPath", func(t *testing.T) {
		bad := "M&E/PER/Distribution/PER%202022-23%20-%20DSICOs.pdf"
		if err := validateEncodedSubPath("path", bad); err != nil {
			t.Fatalf("the premise of this guard has changed: validateEncodedSubPath now rejects %q (%v), "+
				"so the round-trip check is no longer the only thing standing between a caller and "+
				"/Standards/M&E/PER/...", bad, err)
		}
		if got := "/Standards/" + bad; got == "/M&E/PER/Distribution/PER%202022-23%20-%20DSICOs.pdf" {
			t.Fatal("the round-trip check cannot distinguish the two paths")
		}
	})
}

// TestSourcesTrailingSpaceSurvives pins the one byte of URL that decides
// whether FY2020-21 exists. MEASURED LIVE on 2026-09-10: this exact path
// returned HTTP 200 and 2,277,838 bytes through `sources --fetch`.
func TestSourcesTrailingSpaceSurvives(t *testing.T) {
	const wantArg = "2022/NEPRA%20PER%202021%20Distribution%20Companies%20.pdf"
	const wantPath = "/Standards/" + wantArg
	var row sourceDoc
	for _, d := range sourcesPerRows() {
		if d.ID == "per-fy2020-21" {
			row = d
		}
	}
	if row.ID == "" {
		t.Fatal("per-fy2020-21 is not in the catalogue")
	}
	if row.Path != wantPath {
		t.Errorf("path = %q, want %q byte for byte", row.Path, wantPath)
	}
	if row.FetchArg != wantArg {
		t.Errorf("fetch_arg = %q, want %q byte for byte", row.FetchArg, wantArg)
	}
	if !strings.HasSuffix(row.Path, "%20.pdf") || !strings.HasSuffix(row.FetchArg, "%20.pdf") {
		t.Error("the trailing encoded space is gone; the same URL without it is HTTP 404 with a 9-byte body")
	}
	// The whole row must be reachable through the shipped command.
	if !strings.Contains(row.FetchWith, wantArg) {
		t.Errorf("fetch_with = %q does not carry the byte-exact argument", row.FetchWith)
	}
}

// --- the href harvester -------------------------------------------------

// TestSourcesHrefHarvesterPreservesRawTrailingSpace is the justification for
// nepra_sources_href.go existing at all.
//
// It is a SYNTHETIC fragment on purpose. MEASURED on the committed PER index
// fixture: NEPRA currently pre-encodes that trailing space as %20 in its
// markup, so the shipped href readers survive that one URL by luck. This guard
// covers the raw form, which is one upstream edit away and which TrimSpace
// silently converts into a 404.
//
// MUTATION-VERIFIED: replacing sourcesHarvestHrefs with html_extract.go's
// extractHTMLLink fails this test, because that reader TrimSpaces the attribute
// and then round-trips it through url.Parse.
func TestSourcesHrefHarvesterPreservesRawTrailingSpace(t *testing.T) {
	frag := []byte(`<html><body>` +
		`<a href="NEPRA PER 2021 Distribution Companies .pdf">raw trailing space</a>` +
		`<a href="pre%20encoded%20already%20.pdf">already encoded</a>` +
		`</body></html>`)
	hrefs, err := sourcesHarvestHrefs(frag, "/Standards/2022/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if len(hrefs) != 2 {
		t.Fatalf("harvested %d hrefs, want 2", len(hrefs))
	}

	raw := hrefs[0]
	if raw.HrefRaw != "NEPRA PER 2021 Distribution Companies .pdf" {
		t.Errorf("HrefRaw = %q; the raw attribute value was altered", raw.HrefRaw)
	}
	if !strings.HasSuffix(raw.HrefRaw, " .pdf") {
		t.Error("the RAW trailing space was trimmed out of HrefRaw")
	}
	want := "/Standards/2022/NEPRA%20PER%202021%20Distribution%20Companies%20.pdf"
	if raw.PathEncoded != want {
		t.Errorf("PathEncoded = %q, want %q", raw.PathEncoded, want)
	}
	if !raw.PathEncodable {
		t.Error("a resolvable href was reported as not encodable")
	}

	// An already-encoded href must pass through untouched, never double-encoded
	// into %2520.
	enc := hrefs[1]
	if strings.Contains(enc.PathEncoded, "%25") {
		t.Errorf("PathEncoded = %q: an already-encoded href was encoded again", enc.PathEncoded)
	}
	if enc.PathEncoded != "/Standards/2022/pre%20encoded%20already%20.pdf" {
		t.Errorf("PathEncoded = %q", enc.PathEncoded)
	}
}

// TestSourcesHrefHarvesterRefusesHalfEncoded covers the refusals and the
// lexical resolver.
func TestSourcesHrefHarvesterRefusesHalfEncoded(t *testing.T) {
	for _, tc := range []struct {
		name, href, base string
		wantPath         string
		wantEncodable    bool
		wantSkipped      bool
		wantOffHost      bool
	}{
		{name: "bare percent", href: "a%.pdf", base: "/p/i.php", wantEncodable: false},
		{name: "non-hex escape", href: "a%zz.pdf", base: "/p/i.php", wantEncodable: false},
		{name: "truncated escape", href: "a%2", base: "/p/i.php", wantEncodable: false},
		{name: "high byte", href: "caf\xe9.pdf", base: "/p/i.php", wantEncodable: false},
		{name: "already encoded", href: "a%20b.pdf", base: "/p/i.php",
			wantPath: "/p/a%20b.pdf", wantEncodable: true},
		{name: "one dotdot", href: "../M&E/x.pdf", base: "/publications/i.php",
			wantPath: "/M&E/x.pdf", wantEncodable: true},
		{name: "two dotdot", href: "../../a/x.pdf", base: "/p/q/i.php",
			wantPath: "/a/x.pdf", wantEncodable: true},
		{name: "dotdot past root", href: "../../../../x.pdf", base: "/p/i.php",
			wantPath: "/x.pdf", wantEncodable: true},
		{name: "dot slash", href: "./x.pdf", base: "/p/i.php",
			wantPath: "/p/x.pdf", wantEncodable: true},
		{name: "rooted", href: "/a/b.pdf", base: "/p/i.php",
			wantPath: "/a/b.pdf", wantEncodable: true},
		{name: "same host absolute", href: "https://nepra.org.pk/a%20b.pdf", base: "/p/i.php",
			wantPath: "/a%20b.pdf", wantEncodable: true},
		{name: "off host", href: "https://example.org/x.pdf", base: "/p/i.php",
			wantEncodable: false, wantOffHost: true},
		{name: "fragment only", href: "#top", base: "/p/i.php", wantSkipped: true},
		{name: "javascript", href: "javascript:void(0)", base: "/p/i.php", wantSkipped: true},
		{name: "empty", href: "   ", base: "/p/i.php", wantSkipped: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, ok := sourcesResolveHref(tc.href, sourcesHrefBaseDir(tc.base))
			if tc.wantSkipped {
				if ok {
					t.Fatalf("href %q was kept as a document reference", tc.href)
				}
				return
			}
			if !ok {
				t.Fatalf("href %q was skipped", tc.href)
			}
			if h.HrefRaw != tc.href {
				t.Errorf("HrefRaw = %q, want the verbatim %q", h.HrefRaw, tc.href)
			}
			if h.OffHost != tc.wantOffHost {
				t.Errorf("OffHost = %v, want %v", h.OffHost, tc.wantOffHost)
			}
			if h.PathEncodable != tc.wantEncodable {
				t.Errorf("PathEncodable = %v, want %v (reason %q)",
					h.PathEncodable, tc.wantEncodable, h.NotEncodableReason)
			}
			if !tc.wantEncodable {
				if h.PathEncoded != "" {
					t.Errorf("a non-encodable href still produced a path %q", h.PathEncoded)
				}
				if h.NotEncodableReason == "" {
					t.Error("a refusal with no reason")
				}
				return
			}
			if h.PathEncoded != tc.wantPath {
				t.Errorf("PathEncoded = %q, want %q", h.PathEncoded, tc.wantPath)
			}
		})
	}
}

// TestSourcesHrefHarvesterReadsMainHtmVerbatim pins the index against the
// committed fixture.
//
// MEASURED, and it CORRECTS the survey: the recorded figure "exactly 10 hrefs"
// counts every href ATTRIBUTE on the page byte-safely, and only NINE of them
// are anchors. The tenth is
// `<link rel=File-List href="Main_files/filelist.xml">`, the Excel manifest
// pointer — which is a real document reference and the one working enumerator
// on this site, so the harvester reads that element too and the total is 10.
func TestSourcesHrefHarvesterReadsMainHtmVerbatim(t *testing.T) {
	hrefs, err := sourcesHarvestHrefs(sourcesFixture(t, "index-main.htm.gz"), sourcesMainPat)
	if err != nil {
		t.Fatal(err)
	}
	if len(hrefs) != 10 {
		t.Fatalf("Main.htm hrefs = %d, want 10", len(hrefs))
	}
	anchors, manifests := 0, 0
	years := map[string]bool{}
	for _, h := range hrefs {
		if !h.PathEncodable {
			t.Errorf("Main.htm href %q is not encodable: %s", h.HrefRaw, h.NotEncodableReason)
		}
		if strings.HasSuffix(h.HrefRaw, "filelist.xml") {
			manifests++
			continue
		}
		anchors++
		for _, fy := range []string{"2017-18", "2018-19", "2019-20", "2020-21", "2021-22"} {
			if strings.Contains(h.HrefRaw, "wise%20"+fy+".htm") {
				years[fy] = true
			}
		}
	}
	if anchors != 9 || manifests != 1 {
		t.Errorf("Main.htm = %d anchors + %d manifest links, want 9 + 1", anchors, manifests)
	}
	if len(years) != 5 {
		t.Errorf("Main.htm links %d of the 5 generation years it is known to name: %v", len(years), years)
	}
	// The index is INCOMPLETE IN BOTH DIRECTIONS, and that is the finding.
	for _, absent := range []string{"2022-23", "2023-24", "SIR%20Data"} {
		for _, h := range hrefs {
			if strings.Contains(h.HrefRaw, absent) {
				t.Errorf("Main.htm now links %q; the catalogue's note says it does not, so the note "+
					"needs re-measuring", absent)
			}
		}
	}
	// The &amp; entity IS decoded, because `&` is the real byte in the URL.
	// eventSurfaces stores /M&E/... the same way.
	foundAmp := false
	for _, h := range hrefs {
		if strings.Contains(h.HrefRaw, "XWD%20&%20KE") {
			foundAmp = true
			if strings.Contains(h.HrefRaw, "&amp;") {
				t.Error("the &amp; entity was not decoded; the real URL byte is &")
			}
		}
	}
	if !foundAmp {
		t.Error("the Quarterly href with its &-in-path was not found")
	}
}

// TestSourcesSIRPathsAreHarvested proves no SIR path was typed by hand.
//
// MEASURED: the SIR index links all 22 years, TWICE each, and every one of
// those hrefs carries RAW SPACES rather than %20. So the harvester's raw-space
// encoding is not a theoretical nicety — without it, all 22 catalogue paths
// would 404. This test re-derives every sir row's path from the fixture.
func TestSourcesSIRPathsAreHarvested(t *testing.T) {
	hrefs, err := sourcesHarvestHrefs(sourcesFixture(t, "index-sir.php.gz"), sourcesSIRPath)
	if err != nil {
		t.Fatal(err)
	}
	harvested := map[string]int{}
	rawSpaced := 0
	for _, h := range hrefs {
		if !sourcesHrefIsPDF(h) || !h.PathEncodable {
			continue
		}
		harvested[h.PathEncoded]++
		if strings.Contains(h.HrefRaw, " ") {
			rawSpaced++
		}
	}
	if rawSpaced < 44 {
		t.Errorf("hrefs carrying a RAW space = %d, want at least 44 (22 years x 2); if this drops to "+
			"zero the raw-space encoding is no longer exercised by a real fixture", rawSpaced)
	}
	var missing []string
	for _, d := range sourcesDocs() {
		if d.Kind != "sir" {
			continue
		}
		if harvested[d.Path] == 0 {
			missing = append(missing, d.Path)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d sir paths are NOT on the index and were therefore not harvested from it:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
	// Every year is linked twice, which is why a raw href count is not a
	// document count.
	twice := 0
	for _, n := range harvested {
		if n == 2 {
			twice++
		}
	}
	if twice != 22 {
		t.Errorf("years linked exactly twice = %d, want 22", twice)
	}
}

// TestSourcesPerPathsAreOnTheIndex is the same proof for the PER family.
// MEASURED: all 8 of nepraper's paths are named by
// publications/Performance%20Reports.php, including the FY2023-24 one that
// 404s — which is exactly why that row is UNAVAILABLE rather than absent.
func TestSourcesPerPathsAreOnTheIndex(t *testing.T) {
	hrefs, err := sourcesHarvestHrefs(sourcesFixture(t, "index-performance-reports.php.gz"), sourcesPERPath)
	if err != nil {
		t.Fatal(err)
	}
	on := map[string]bool{}
	for _, h := range hrefs {
		if h.PathEncodable {
			on[h.PathEncoded] = true
		}
	}
	for _, d := range sourcesPerRows() {
		if !on[d.Path] {
			t.Errorf("%s path %q is not linked from the PER index; either the path was invented or the "+
				"index changed", d.ID, d.Path)
		}
	}
	// The trailing-space URL is on the index PRE-ENCODED. Recording that
	// settles an open question the survey left, and it is why the
	// raw-trailing-space guard above is synthetic.
	if !on["/Standards/2022/NEPRA%20PER%202021%20Distribution%20Companies%20.pdf"] {
		t.Error("the pre-encoded trailing-space path is no longer on the index")
	}
	rawSpace := 0
	for _, h := range hrefs {
		if sourcesHrefIsPDF(h) && strings.Contains(h.HrefRaw, " ") {
			rawSpace++
		}
	}
	if rawSpace == 0 {
		t.Error("no PER href carries a raw space any more; the measurement in this file's comment " +
			"(five of them do) needs revisiting")
	}
}

// --- catalogue-vs-events boundary ---------------------------------------

// TestSourcesDoesNotShadowEventSurfaces enforces the one-catalogue-per-claim
// rule.
//
// The eleven /tariff/Distribution%20<DISCO>.php paths appear in BOTH
// catalogues, and that is deliberate: `events` asserts their row floor and
// `sources` carries them as REFERENCE rows so the surface is discoverable from
// one place. What must never happen is two catalogues making two different
// claims about one URL, so every colliding row must be state "reference" and
// must name its owner.
//
// MUTATION-VERIFIED: changing any tariff row's state away from "reference" —
// or giving one a byte baseline — makes this fail.
func TestSourcesDoesNotShadowEventSurfaces(t *testing.T) {
	if shadowed := sourcesShadowedEventPaths(); len(shadowed) != 0 {
		t.Errorf("%d path(s) are claimed by both catalogues with an assertion attached: %v",
			len(shadowed), shadowed)
	}
	owned := map[string]bool{}
	for _, s := range eventSurfaces {
		owned[s.Path] = true
	}
	refs := 0
	for _, d := range sourcesDocs() {
		if d.Kind != "tariff" {
			continue
		}
		refs++
		if d.State != "reference" {
			t.Errorf("%s state = %q, want reference: a sources row that asserts anything about a "+
				"determination page collides with the events catalogue", d.ID, d.State)
		}
		if d.BytesAsOf != nil || d.ByteFloor != 0 {
			t.Errorf("%s carries a byte assertion; no per-page floor was ever measured for the DISCO "+
				"pages, only the group figure %d in the events catalogue", d.ID, eventsDISCORowsAsOf)
		}
		if d.RowsOwnedBy == "" {
			t.Errorf("%s does not name the command that owns its rows", d.ID)
		}
		if !owned[d.Path] {
			t.Errorf("%s points at %q, which the events catalogue does not own; a reference row that "+
				"references nothing is just an unasserted claim", d.ID, d.Path)
		}
		if sourcesNotProbedReason(d) == "" {
			t.Errorf("%s would be probed by --diff, which re-asserts a surface events owns", d.ID)
		}
		if err := sourcesValidateFetch(d); err == nil {
			t.Errorf("%s would be fetched by --fetch, which re-asserts a surface events owns", d.ID)
		}
	}
	if refs != 11 {
		t.Errorf("tariff reference rows = %d, want 11", refs)
	}
	// "events" must not be a --kind value.
	if err := sourcesValidateKind("events"); err == nil {
		t.Error("--kind events was accepted; the determination surfaces are not sources rows")
	} else if !strings.Contains(err.Error(), "events") {
		t.Errorf("the --kind events refusal does not explain itself: %v", err)
	}
}

// TestSourcesExtensionCaseTwinIsNotNormalised pins the case-sensitivity fact in
// both directions.
//
// MEASURED LIVE on 2026-09-10 with HEAD: the June 2026 file is 200 (793,122 B)
// as .pdf and 404 as .PDF; the July 2026 file is 200 (630,229 B) as .PDF and
// 404 as .pdf. The reversal is the point — casing cannot be normalised or
// guessed in either direction.
func TestSourcesExtensionCaseTwinIsNotNormalised(t *testing.T) {
	var june, july sourceDoc
	for _, d := range sourcesDocs() {
		switch d.ID {
		case "fca-jun-2026-determination":
			june = d
		case "fca-jul-2026-mfpa":
			july = d
		}
	}
	for _, tc := range []struct {
		row              sourceDoc
		wantSuffix       string
		wantTwinSuffix   string
		wantBytes        int
		wantTwinNotFound int
	}{
		{june, ".pdf", ".PDF", 793122, 404},
		{july, ".PDF", ".pdf", 630229, 404},
	} {
		if tc.row.ID == "" {
			t.Fatal("an FCA determination row is missing from the catalogue")
		}
		if !strings.HasSuffix(tc.row.Path, tc.wantSuffix) {
			t.Errorf("%s path %q does not end %q", tc.row.ID, tc.row.Path, tc.wantSuffix)
		}
		if tc.row.CaseTwin == nil {
			t.Fatalf("%s records no case twin, so the reversal is invisible", tc.row.ID)
		}
		if !strings.HasSuffix(tc.row.CaseTwin.URL, tc.wantTwinSuffix) {
			t.Errorf("%s twin %q does not end %q", tc.row.ID, tc.row.CaseTwin.URL, tc.wantTwinSuffix)
		}
		if tc.row.CaseTwin.HTTPStatusAsOf == nil || *tc.row.CaseTwin.HTTPStatusAsOf != tc.wantTwinNotFound {
			t.Errorf("%s twin status is not the measured %d", tc.row.ID, tc.wantTwinNotFound)
		}
		if tc.row.BytesAsOf == nil || *tc.row.BytesAsOf != tc.wantBytes {
			t.Errorf("%s bytes_as_of is not the measured %d", tc.row.ID, tc.wantBytes)
		}
		// The two URLs must differ ONLY in the extension casing.
		base := strings.TrimSuffix(nepraSourcesHost+tc.row.Path, tc.wantSuffix)
		if got := strings.TrimSuffix(tc.row.CaseTwin.URL, tc.wantTwinSuffix); got != base {
			t.Errorf("%s twin differs from the row by more than the extension:\n  %q\n  %q",
				tc.row.ID, base, got)
		}
		// And nothing in the catalogue may have re-cased a path.
		if strings.Contains(tc.row.Path, strings.ToUpper(tc.wantSuffix)) &&
			strings.Contains(tc.row.Path, strings.ToLower(tc.wantSuffix)) {
			t.Errorf("%s path carries both casings", tc.row.ID)
		}
	}
	// No path anywhere may be lower- or upper-cased wholesale.
	for _, d := range sourcesDocs() {
		if d.Path == "" {
			continue
		}
		if d.Path == strings.ToLower(d.Path) && strings.Contains(d.Path, "PER") {
			t.Errorf("%s path looks lowercased: %q", d.ID, d.Path)
		}
	}
}

// --- the verdict table --------------------------------------------------

// TestSourcesDecoyIsNeverReachable drives sourcesVerdict directly.
//
// MUTATION-VERIFIED: deleting the ByteFloor branch makes the 9,838-byte and
// 399,999-byte cases report "unchanged"/"shrank" instead of "decoy", which is
// exactly how a 200-OK frameset gets mistaken for a 493 KB workbook.
func TestSourcesDecoyIsNeverReachable(t *testing.T) {
	genRow := sourceDoc{
		ID: "gen-2023-24", Kind: "gen", State: "reachable", ContentKind: "html-excel-sheet",
		BytesAsOf: intp(493187), ClientBytesAsOf: intp(493548), ByteFloor: sourcesGenByteFloor,
	}
	stub := sourceDoc{
		ID: "gen-sir-data-2025", Kind: "gen", State: "decoy", ContentKind: "html",
		BytesAsOf: intp(374), ClientBytesAsOf: intp(735),
	}
	dead := sourceDoc{
		ID: "gen-2024-25", Kind: "gen", State: "unavailable", ContentKind: "html-excel-sheet",
		HTTPStatusAsOf: intp(404),
	}

	for _, tc := range []struct {
		name        string
		row         sourceDoc
		obs         sourcesProbeObs
		wantVerdict string
		wantDrift   bool
		wantBody    *int
	}{
		{"shell decoy under the floor", genRow, sourcesProbeObs{Status: 200, Bytes: sourcesDecoyShellBytes}, "decoy", true, nil},
		{"one byte under the floor", genRow, sourcesProbeObs{Status: 200, Bytes: sourcesGenByteFloor - 1}, "decoy", true, nil},
		{"real workbook", genRow, sourcesProbeObs{Status: 200, Bytes: 493548}, "unchanged", false, nil},
		{"workbook grew", genRow, sourcesProbeObs{Status: 200, Bytes: 493549}, "grew", true, nil},
		{"workbook shrank but above the floor", genRow, sourcesProbeObs{Status: 200, Bytes: 493547}, "shrank", true, nil},
		{"a catalogued decoy stays a decoy", stub, sourcesProbeObs{Status: 200, Bytes: 735}, "decoy", false, nil},
		{"a decoy that changed size", stub, sourcesProbeObs{Status: 200, Bytes: 900}, "decoy", true, nil},
		{"404 stays 404", dead, sourcesProbeObs{Status: 404, BodyBytes: 9, BodyExact: true}, "still_unreachable", false, intp(9)},
		{"404 became 403", dead, sourcesProbeObs{Status: 403, BodyBytes: 9, BodyExact: true}, "still_unreachable", true, intp(9)},
		{"404 started serving", dead, sourcesProbeObs{Status: 200, Bytes: 400001}, "appeared", true, nil},
		{"a live row went away", genRow, sourcesProbeObs{Status: 404, BodyBytes: 9, BodyExact: true}, "disappeared", true, intp(9)},
		{"transport failure is not evidence", genRow, sourcesProbeObs{TransportErr: "dial tcp: timeout"}, "not_probed", false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := sourcesVerdict(tc.row, tc.obs)
			if p.Verdict != tc.wantVerdict {
				t.Errorf("verdict = %q, want %q", p.Verdict, tc.wantVerdict)
			}
			if p.Drift != tc.wantDrift {
				t.Errorf("drift = %v, want %v", p.Drift, tc.wantDrift)
			}
			if tc.wantBody == nil && p.BodyBytes != nil {
				t.Errorf("body_bytes = %d, want absent", *p.BodyBytes)
			}
			if tc.wantBody != nil && (p.BodyBytes == nil || *p.BodyBytes != *tc.wantBody) {
				t.Errorf("body_bytes = %v, want %d", p.BodyBytes, *tc.wantBody)
			}
			// A success must never assert an HTTP status: this client does not
			// report one on the success path.
			if tc.obs.ok() && tc.obs.TransportErr == "" && p.HTTPStatus != nil {
				t.Errorf("http_status = %d was asserted on a successful probe", *p.HTTPStatus)
			}
		})
	}

	// A truncated error body is a floor, not a measurement, so it is not
	// reported at all.
	p := sourcesVerdict(dead, sourcesProbeObs{Status: 404, BodyBytes: 4096, BodyExact: false})
	if p.BodyBytes != nil {
		t.Errorf("a truncated error body was reported as an exact measurement: %d", *p.BodyBytes)
	}
}

// TestSourcesBaselineIsChosenNotMixed guards the comparison against the +361
// instrument difference.
func TestSourcesBaselineIsChosenNotMixed(t *testing.T) {
	both := sourceDoc{BytesAsOf: intp(100), ClientBytesAsOf: intp(461), State: "reachable"}
	p := sourcesVerdict(both, sourcesProbeObs{Status: 200, Bytes: 461})
	if p.Verdict != "unchanged" || p.BaselineUsed != "client" {
		t.Errorf("verdict/baseline = %q/%q, want unchanged/client: a probe made by this client must be "+
			"compared against what this client measured", p.Verdict, p.BaselineUsed)
	}
	only := sourceDoc{BytesAsOf: intp(100), State: "reachable"}
	p = sourcesVerdict(only, sourcesProbeObs{Status: 200, Bytes: 100})
	if p.BaselineUsed != "content-length" {
		t.Errorf("baseline = %q, want content-length as the named fallback", p.BaselineUsed)
	}
	none := sourceDoc{State: "reachable"}
	p = sourcesVerdict(none, sourcesProbeObs{Status: 200, Bytes: 1})
	if p.Verdict != "appeared" || p.BaselineUsed != "none" {
		t.Errorf("verdict/baseline = %q/%q, want appeared/none", p.Verdict, p.BaselineUsed)
	}
}

// TestSourcesClientBaselineOffsetIsMeasured pins the +361 Cloudflare injection.
//
// MEASURED on 2026-09-10 across every row this build fetched with its own
// client: the delta between what curl's Content-Length reports and what this
// client receives is EXACTLY +361 on every text/html body (16 of 16, no
// exceptions) and EXACTLY 0 on the two non-HTML bodies. Both sides of every
// comparison below were measured independently; neither was derived from the
// other by adding 361.
func TestSourcesClientBaselineOffsetIsMeasured(t *testing.T) {
	html, other := 0, 0
	for _, d := range sourcesDocs() {
		if d.BytesAsOf == nil || d.ClientBytesAsOf == nil {
			continue
		}
		delta := *d.ClientBytesAsOf - *d.BytesAsOf
		if sourcesHTMLContentKinds[d.ContentKind] {
			html++
			if delta != sourcesCloudflareHTMLInjectionBytes {
				t.Errorf("%s (%s) client-vs-content-length delta = %+d, want %+d",
					d.ID, d.ContentKind, delta, sourcesCloudflareHTMLInjectionBytes)
			}
			continue
		}
		other++
		if delta != 0 {
			t.Errorf("%s (%s) delta = %+d, want 0: the injection is HTML-specific, and a non-zero "+
				"delta here would mean it is not", d.ID, d.ContentKind, delta)
		}
	}
	if html != 14 {
		t.Errorf("html rows with both baselines = %d, want 14", html)
	}
	if other != 1 {
		t.Errorf("non-html rows with both baselines = %d, want 1 (filelist.xml)", other)
	}
	// The two live .php indexes carry the same offset.
	for _, e := range sourcesEnumerators {
		if e.BytesAsOf == nil || e.ClientBytesAsOf == nil {
			continue
		}
		delta := *e.ClientBytesAsOf - *e.BytesAsOf
		want := sourcesCloudflareHTMLInjectionBytes
		if e.ID == "enum-robots" {
			want = 0 // text/plain: no injection
		}
		if delta != want {
			t.Errorf("%s delta = %+d, want %+d", e.ID, delta, want)
		}
	}
	// And the survey's unexplained pair reduces to the same number.
	if 74792-74431 != sourcesCloudflareHTMLInjectionBytes {
		t.Error("the corroborating 74,431-vs-74,792 pair no longer reduces to the constant")
	}
}

// TestSourcesSIRCorpusReconciles recomputes the corpus from the rows.
//
// MEASURED: the ten SIR years with a byte count sum to EXACTLY 640,312,915 B,
// which is the corpus figure the survey reported. Three of those ten (2016,
// 2017, 2018) had no byte count anywhere and were measured by HEAD on
// 2026-09-10; that the total then lands on the reported figure to the byte is
// the cross-check that they are the right three.
func TestSourcesSIRCorpusReconciles(t *testing.T) {
	sum, measured, unmeasured := 0, 0, 0
	for _, d := range sourcesDocs() {
		if d.Kind != "sir" {
			continue
		}
		if d.BytesAsOf == nil {
			unmeasured++
			if d.State != "indexed_unmeasured" {
				t.Errorf("%s has no size but state %q", d.ID, d.State)
			}
			continue
		}
		measured++
		sum += *d.BytesAsOf
	}
	if measured != 10 || unmeasured != 12 {
		t.Errorf("sir years = %d measured / %d unmeasured, want 10 / 12", measured, unmeasured)
	}
	if sum != sourcesSIRCorpusBytesAsOf {
		t.Errorf("measured SIR corpus = %d B, want %d B", sum, sourcesSIRCorpusBytesAsOf)
	}
	// Every sir row ships text_layer: unmeasured. Nothing in this build has
	// read a SIR text layer.
	for _, d := range sourcesDocs() {
		if d.Kind == "sir" && d.TextLayer != "unmeasured" {
			t.Errorf("%s ships text_layer %q; no SIR text layer has ever been extracted, and reporting "+
				"'absent' would be asserting an unsourced claim", d.ID, d.TextLayer)
		}
	}
}

// TestSourcesNoAbsentValueReadsAsZero is the JSON-level honesty check. An
// unmeasured byte count must serialise as null, never as 0.
func TestSourcesNoAbsentValueReadsAsZero(t *testing.T) {
	docs := sourcesDocs()
	raw, err := json.Marshal(docs)
	if err != nil {
		t.Fatal(err)
	}
	var back []map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	nulls := 0
	for i, m := range back {
		v, present := m["bytes_as_of"]
		if !present {
			t.Fatalf("%s dropped bytes_as_of from its JSON; an omitted field is not the same as a "+
				"recorded absence", docs[i].ID)
		}
		if v == nil {
			nulls++
			continue
		}
		if f, ok := v.(float64); ok && f == 0 {
			t.Errorf("%s serialised bytes_as_of as 0; NEPRA has no zero-byte documents and an "+
				"unmeasured size must be null", docs[i].ID)
		}
	}
	// MEASURED: 29 of the 63 rows have no byte count (12 unmeasured SIR years,
	// 11 tariff references, 4 gen 404s, the 403 directory and the 404 PER).
	if nulls != 29 {
		t.Errorf("rows with bytes_as_of: null = %d, want 29", nulls)
	}
	// A measured-absent /Creator must survive as "" and not vanish.
	var iLovePDF sourceDoc
	for _, d := range docs {
		if d.ID == "per-fy2024-25" {
			iLovePDF = d
		}
	}
	if iLovePDF.CreatorAsOf == nil {
		t.Fatal("per-fy2024-25 lost its /Creator measurement; the report was re-processed by iLovePDF " +
			"so /Creator is measured ABSENT, which is not the same as never measured")
	}
	if *iLovePDF.CreatorAsOf != "" {
		t.Errorf("per-fy2024-25 creator = %q, want the measured empty string", *iLovePDF.CreatorAsOf)
	}

	// low_text_pages_as_of must distinguish "extracted, none found" ([]) from
	// "never extracted" (null). MEASURED from nepraper: FY2020-21 is [1],
	// FY2014-15 is [], and the unavailable FY2023-24 is null.
	wantLow := map[string]string{
		"per-fy2020-21": "[1]",
		"per-fy2014-15": "[]",
		"per-fy2023-24": "null",
		"gen-2023-24":   "null",
	}
	for i, m := range back {
		want, ok := wantLow[docs[i].ID]
		if !ok {
			continue
		}
		raw, err := json.Marshal(m["low_text_pages_as_of"])
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != want {
			t.Errorf("%s low_text_pages_as_of = %s, want %s", docs[i].ID, raw, want)
		}
	}
}

// --- --diff, driven end to end -----------------------------------------

// sourcesTestServer serves the committed fixtures and records every path it was
// asked for, so a test can assert what was NOT requested.
type sourcesTestServer struct {
	mu       sync.Mutex
	requests []string
	bodies   map[string]sourcesTestResponse
	fallback sourcesTestResponse
}

type sourcesTestResponse struct {
	Status      int
	Body        []byte
	ContentType string
}

func (s *sourcesTestServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// RequestURI keeps NEPRA's own encoding; r.URL.Path would decode it.
		p := r.URL.EscapedPath()
		s.mu.Lock()
		s.requests = append(s.requests, p)
		s.mu.Unlock()
		resp, ok := s.bodies[p]
		if !ok {
			resp = s.fallback
		}
		ct := resp.ContentType
		if ct == "" {
			ct = "text/html"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(resp.Status)
		_, _ = w.Write(resp.Body)
	}
}

func (s *sourcesTestServer) asked(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.requests {
		if r == path {
			return true
		}
	}
	return false
}

func (s *sourcesTestServer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

// runSources executes the real command through the root, against a base URL of
// the caller's choosing, and returns stdout, stderr and the typed exit code.
func runSources(t *testing.T, baseURL string, args ...string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	// An empty config file keeps a developer's own ~/.config out of the test.
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("base_url = \""+baseURL+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEPRA_CONFIG", cfgPath)
	t.Setenv("NEPRA_BASE_URL", baseURL)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	cmd := RootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	// rate-limit 0 disables the adaptive pacer. Against an httptest server
	// there is nothing to be polite to, and the default pacing made a single
	// --diff test take 14 seconds.
	cmd.SetArgs(append([]string{"sources"}, append(args, "--json", "--no-cache", "--rate-limit", "0")...))
	err := cmd.Execute()
	code := 0
	if err != nil {
		code = ExitCode(err)
	}
	return out.String(), errOut.String(), code
}

func sourcesDiffFixtureServer(t *testing.T) *sourcesTestServer {
	t.Helper()
	return &sourcesTestServer{
		bodies: map[string]sourcesTestResponse{
			sourcesMainPat: {Status: 200, Body: sourcesFixture(t, "index-main.htm.gz")},
			sourcesPERPath: {Status: 200, Body: sourcesFixture(t, "index-performance-reports.php.gz")},
			sourcesSIRPath: {Status: 200, Body: sourcesFixture(t, "index-sir.php.gz")},
		},
		fallback: sourcesTestResponse{Status: 404, Body: []byte("Not Found")},
	}
}

// TestSourcesDiffDoesNotClassifyProbeErrors is the test that keeps a probed 404
// from becoming the command's exit status.
//
// MUTATION-VERIFIED: routing the probe error through classifyAPIError turns
// this exit code into 3, and routing a 403 through it turns it into 4.
func TestSourcesDiffDoesNotClassifyProbeErrors(t *testing.T) {
	srv := sourcesDiffFixtureServer(t)
	// One catalogued row answers 200, everything else 404 — including rows the
	// catalogue says are reachable, which is the harshest version of this test.
	srv.bodies[sourcesGenSheetPath("2023-24")] = sourcesTestResponse{
		Status: 200, Body: bytes.Repeat([]byte("x"), 493548),
	}
	srv.bodies[sourcesGenDir+"/"] = sourcesTestResponse{Status: 403, Body: []byte("Forbidden")}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	out, _, code := runSources(t, ts.URL, "--diff")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0: a probed 404 or 403 is DATA, not a command failure", code)
	}

	var env struct {
		Meta struct {
			Probed  int `json:"probed"`
			Drifted int `json:"drifted"`
		} `json:"meta"`
		Results []struct {
			ID    string       `json:"id"`
			Probe *sourceProbe `json:"probe"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("stdout is not the owned envelope: %v\n%s", err, truncateForTest(out))
	}
	if len(env.Results) != 63 {
		t.Errorf("results = %d rows, want the whole catalogue (63) even under --diff", len(env.Results))
	}
	if env.Meta.Probed == 0 {
		t.Error("nothing was probed")
	}
	byID := map[string]*sourceProbe{}
	for _, r := range env.Results {
		byID[r.ID] = r.Probe
	}
	if p := byID["gen-2023-24"]; p == nil || p.Verdict != "unchanged" {
		t.Errorf("gen-2023-24 verdict = %v, want unchanged against the client baseline", p)
	}
	if p := byID["gen-2017-18"]; p == nil || p.Verdict != "disappeared" {
		t.Errorf("gen-2017-18 verdict = %v, want disappeared", p)
	} else if p.HTTPStatus == nil || *p.HTTPStatus != 404 {
		t.Errorf("gen-2017-18 recorded no 404: %+v", p)
	} else if p.BodyBytes == nil || *p.BodyBytes != 9 {
		t.Errorf("gen-2017-18 body_bytes = %v, want the nine bytes of 'Not Found'", p.BodyBytes)
	}
	if p := byID["gen-dir-listing"]; p == nil || p.Verdict != "still_unreachable" {
		t.Errorf("gen-dir-listing verdict = %v, want still_unreachable", p)
	} else if p.HTTPStatus == nil || *p.HTTPStatus != 403 {
		t.Errorf("gen-dir-listing did not record its 403: %+v", p)
	}

	// --strict is the ONLY thing that converts drift into a non-zero exit, and
	// it must be exit 1, not a typed API code.
	_, _, strictCode := runSources(t, ts.URL, "--diff", "--strict")
	if strictCode != 1 {
		t.Errorf("--strict exit code = %d, want 1 (drift, via a plain error)", strictCode)
	}
}

// TestSourcesDiffSkipsPDFs is what keeps `--diff` from becoming a 501 MB
// download. MEASURED: the shipped catalogue holds 35 pdf rows and --diff
// requests none of them.
func TestSourcesDiffSkipsPDFs(t *testing.T) {
	srv := sourcesDiffFixtureServer(t)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	out, _, code := runSources(t, ts.URL, "--diff")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var env struct {
		Results []struct {
			ID          string       `json:"id"`
			ContentKind string       `json:"content_kind"`
			Path        string       `json:"path"`
			Probe       *sourceProbe `json:"probe"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	pdfs := 0
	for _, r := range env.Results {
		if r.ContentKind != "pdf" {
			continue
		}
		pdfs++
		if r.Probe == nil || r.Probe.Verdict != "not_probed" {
			t.Errorf("%s (pdf) verdict = %v, want not_probed", r.ID, r.Probe)
			continue
		}
		if r.Probe.NotProbedReason == "" {
			t.Errorf("%s was skipped with no reason", r.ID)
		}
		if !strings.Contains(r.Probe.NotProbedReason, "HEAD") {
			t.Errorf("%s reason does not name the missing capability: %q", r.ID, r.Probe.NotProbedReason)
		}
		if r.Path != "" && srv.asked(r.Path) {
			t.Errorf("--diff requested the PDF at %q", r.Path)
		}
	}
	// MEASURED: 32 pdf rows = 8 per + 22 sir + the 2 FCA determinations.
	if pdfs != 32 {
		t.Errorf("pdf rows = %d, want 32 (8 per + 22 sir + 2 fca determinations)", pdfs)
	}
	for _, p := range srv.requests {
		if strings.HasSuffix(strings.ToLower(p), ".pdf") {
			t.Errorf("--diff requested a .pdf path: %q", p)
		}
	}
}

// TestSourcesDiffEnumeratorLeg pins the href diff against the three committed
// fixtures.
//
// MEASURED on the fixtures: Main.htm 10 hrefs / 9 index-only, the PER index 42
// pdf refs / 33 index-only / 1 duplicate, the SIR index 46 pdf refs / 2
// index-only / 22 duplicates, and catalogue_only EMPTY on all three. A
// non-empty index_only is the command working, not failing — the PER index
// genuinely names 33 documents outside this build's scope.
func TestSourcesDiffEnumeratorLeg(t *testing.T) {
	srv := sourcesDiffFixtureServer(t)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	out, _, code := runSources(t, ts.URL, "--diff")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var env struct {
		Meta struct {
			EnumeratorDiff []sourcesEnumDiff `json:"enumerator_diff"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	diffs := env.Meta.EnumeratorDiff
	if len(diffs) != 3 {
		t.Fatalf("enumerator diffs = %d, want 3 (there is no fourth index: both sitemaps and "+
			"/sitemap.xml are 404 and the workbook directory is 403)", len(diffs))
	}
	want := map[string]struct{ hrefs, indexOnly, dups int }{
		"gen-main-index": {10, 9, 0},
		"enum-per-index": {42, 33, 1},
		"enum-sir-index": {46, 2, 22},
	}
	for _, d := range diffs {
		w, ok := want[d.Enumerator]
		if !ok {
			t.Errorf("unexpected enumerator %q", d.Enumerator)
			continue
		}
		if d.State != "probed" {
			t.Errorf("%s state = %q (%s)", d.Enumerator, d.State, d.Error)
			continue
		}
		if d.HrefsNow == nil || *d.HrefsNow != w.hrefs {
			t.Errorf("%s hrefs_now = %v, want %d", d.Enumerator, d.HrefsNow, w.hrefs)
		}
		if d.HrefsAsOf != w.hrefs {
			t.Errorf("%s hrefs_as_of = %d, want %d", d.Enumerator, d.HrefsAsOf, w.hrefs)
		}
		if len(d.IndexOnly) != w.indexOnly {
			t.Errorf("%s index_only = %d, want %d", d.Enumerator, len(d.IndexOnly), w.indexOnly)
		}
		if d.IndexOnlyAsOf != w.indexOnly {
			t.Errorf("%s index_only_as_of = %d, want %d", d.Enumerator, d.IndexOnlyAsOf, w.indexOnly)
		}
		if d.Duplicates != w.dups {
			t.Errorf("%s duplicate_hrefs = %d, want %d", d.Enumerator, d.Duplicates, w.dups)
		}
		if len(d.CatalogueOnly) != 0 {
			t.Errorf("%s catalogue_only = %v, want empty: every catalogued path is still named",
				d.Enumerator, d.CatalogueOnly)
		}
		if d.Drift {
			t.Errorf("%s reports drift against its own baseline", d.Enumerator)
		}
		for _, h := range d.IndexOnly {
			if h.PathEncoded == "" || !h.PathEncodable {
				t.Errorf("%s index_only entry %q has no usable path", d.Enumerator, h.HrefRaw)
			}
			if !strings.HasPrefix(h.PathEncoded, "/") {
				t.Errorf("%s index_only path is not rooted: %q", d.Enumerator, h.PathEncoded)
			}
			if strings.Contains(h.PathEncoded, " ") {
				t.Errorf("%s index_only path carries a raw space: %q", d.Enumerator, h.PathEncoded)
			}
		}
	}
}

// TestSourcesEnumeratorDiffSurfacesANewDocument is the mutation of the fixture
// itself: adding one <a> must make it appear in index_only with a correct
// encoded path, and must move the drift verdict.
func TestSourcesEnumeratorDiffSurfacesANewDocument(t *testing.T) {
	fixture := sourcesFixture(t, "index-sir.php.gz")
	injected := bytes.Replace(fixture, []byte("</body>"),
		[]byte(`<a href="State of Industry Reports/State of Industry Report 2026.pdf">2026</a></body>`), 1)
	if bytes.Equal(fixture, injected) {
		t.Fatal("could not inject into the fixture")
	}
	srv := sourcesDiffFixtureServer(t)
	srv.bodies[sourcesSIRPath] = sourcesTestResponse{Status: 200, Body: injected}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	out, _, code := runSources(t, ts.URL, "--diff", "--kind", "sir")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0: a NEW upstream document is the product, not an error", code)
	}
	var env struct {
		Meta struct {
			EnumeratorDiff []sourcesEnumDiff `json:"enumerator_diff"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Meta.EnumeratorDiff) != 1 {
		t.Fatalf("--kind sir probed %d indexes, want 1", len(env.Meta.EnumeratorDiff))
	}
	d := env.Meta.EnumeratorDiff[0]
	if !d.Drift {
		t.Error("a new upstream document did not move the drift verdict")
	}
	found := ""
	for _, h := range d.IndexOnly {
		if strings.Contains(h.PathEncoded, "2026") {
			found = h.PathEncoded
		}
	}
	want := sourcesSIRDir + "/State%20of%20Industry%20Report%202026.pdf"
	if found != want {
		t.Errorf("the injected document surfaced as %q, want %q", found, want)
	}
	if len(d.IndexOnly) != d.IndexOnlyAsOf+1 {
		t.Errorf("index_only = %d against a baseline of %d; exactly one document was added",
			len(d.IndexOnly), d.IndexOnlyAsOf)
	}
	// --strict must catch it.
	_, _, strictCode := runSources(t, ts.URL, "--diff", "--kind", "sir", "--strict")
	if strictCode != 1 {
		t.Errorf("--strict exit = %d, want 1", strictCode)
	}
}

// --- --fetch ------------------------------------------------------------

// TestSourcesFetchRefusesOversize proves the ceiling is enforced BEFORE any
// request, and that the refusal carries the measurements that justify it.
func TestSourcesFetchRefusesOversize(t *testing.T) {
	srv := sourcesDiffFixtureServer(t)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	for _, tc := range []struct{ id, bytes string }{
		{"sir-2025", "331,853,390"},
		{"sir-2019", "106,428,020"},
		{"sir-2016", "97,607,884"},
	} {
		before := srv.count()
		_, errOut, code := runSources(t, ts.URL, "--fetch", tc.id)
		if code != 2 {
			t.Errorf("--fetch %s exit = %d, want 2", tc.id, code)
		}
		if srv.count() != before {
			t.Errorf("--fetch %s made %d request(s) before refusing", tc.id, srv.count()-before)
		}
		for _, want := range []string{tc.bytes, "68,768", "163,650"} {
			if !strings.Contains(errOut, want) {
				t.Errorf("--fetch %s refusal does not quote %s:\n%s", tc.id, want, errOut)
			}
		}
	}

	// An unmeasured year is refused for a DIFFERENT reason: the cost cannot be
	// bounded, which is a declared gap rather than a size limit.
	before := srv.count()
	_, errOut, code := runSources(t, ts.URL, "--fetch", "sir-2008")
	if code != 2 {
		t.Errorf("--fetch sir-2008 exit = %d, want 2", code)
	}
	if srv.count() != before {
		t.Error("--fetch sir-2008 made a request before refusing")
	}
	if !strings.Contains(errOut, "never been probed") {
		t.Errorf("the unmeasured-year refusal does not say why:\n%s", errOut)
	}
}

// TestSourcesFetchMeasuresPDF drives the measurement leg against a real PDF.
//
// The fixture is a hand-built 1,024-byte PDF whose page 1 carries one character
// and page 2 a real text layer. MEASURED by running nepraper.ExtractText over
// it: 2 pages, /Creator "Nitro Pro 8", /Producer "nepra-pp-cli test fixture",
// 68 characters, low-text pages [1].
//
// char_count is checked to be PRESENT and is deliberately not pinned against
// any survey figure — see the note on sourcesCharCountNote.
func TestSourcesFetchMeasuresPDF(t *testing.T) {
	pdf := sourcesFixture(t, "sources-fetch-fixture.pdf")
	srv := sourcesDiffFixtureServer(t)
	// per-fy2014-15 is a pdf row rooted at /Standards/, so serving the fixture
	// there exercises the real path resolution.
	target := "/Standards/PER%20DISCOs%20and%20KE%20for%202014-15.pdf"
	srv.bodies[target] = sourcesTestResponse{Status: 200, Body: pdf, ContentType: "application/pdf"}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	out, _, code := runSources(t, ts.URL, "--fetch", "per-fy2014-15")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, truncateForTest(out))
	}
	var env struct {
		Results []sourcesFetchResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("stdout is not the owned envelope: %v\n%s", err, truncateForTest(out))
	}
	if len(env.Results) != 1 {
		t.Fatalf("results = %d, want exactly 1", len(env.Results))
	}
	res := env.Results[0]
	if res.State != "measured" {
		t.Errorf("state = %q, want measured (%s)", res.State, res.RefusalReason)
	}
	m := res.Measured
	// bytes must be the DOCUMENT length, not the base64 envelope's.
	if m.Bytes != len(pdf) {
		t.Errorf("measured bytes = %d, want %d (the base64 envelope is ~1.33x, so this catches a "+
			"length taken before unwrapping)", m.Bytes, len(pdf))
	}
	if m.Pages == nil || *m.Pages != 2 {
		t.Errorf("pages = %v, want 2 from the PDF page tree", m.Pages)
	}
	if m.Creator == nil || *m.Creator != "Nitro Pro 8" {
		t.Errorf("creator = %v, want %q", m.Creator, "Nitro Pro 8")
	}
	if m.Producer == nil || *m.Producer != "nepra-pp-cli test fixture" {
		t.Errorf("producer = %v", m.Producer)
	}
	if m.CharCount == nil {
		t.Error("char_count was not reported")
	} else if *m.CharCount <= 0 {
		t.Errorf("char_count = %d", *m.CharCount)
	}
	if res.CharCountNote == "" || !strings.Contains(res.CharCountNote, "never asserted") {
		t.Errorf("char_count travelled without its note: %q", res.CharCountNote)
	}
	if len(m.LowTextPages) != 1 || m.LowTextPages[0] != 1 {
		t.Errorf("low_text_pages = %v, want [1]", m.LowTextPages)
	}
	if m.TextLayer != "present" {
		t.Errorf("text_layer = %q, want present", m.TextLayer)
	}
	// The baseline comparison must be computed and must DISAGREE here: the
	// fixture is 1,024 bytes against a real baseline of 1,022,593.
	if m.Matches == nil || m.Matches.Bytes == nil {
		t.Fatal("matches_baseline.bytes was not computed")
	}
	if *m.Matches.Bytes {
		t.Error("matches_baseline.bytes reported true for a 1,024-byte fixture against a 1,022,593-byte baseline")
	}
	if m.Matches.Pages == nil || *m.Matches.Pages {
		t.Error("matches_baseline.pages should be false: the fixture has 2 pages, the baseline 27")
	}
	if m.Matches.Creator == nil || !*m.Matches.Creator {
		t.Error("matches_baseline.creator should be TRUE: both are \"Nitro Pro 8\"")
	}
	// Provenance travels with the bytes.
	if res.Artifact.SHA256Content == "" || res.Artifact.SHA256Raw == "" {
		t.Error("the artifact carries no hashes")
	}
	if res.Artifact.Bytes != len(pdf) {
		t.Errorf("artifact bytes = %d, want the decoded %d", res.Artifact.Bytes, len(pdf))
	}
	if res.Artifact.Rows != 1 {
		t.Errorf("artifact rows = %d, want 1: a document is one artifact", res.Artifact.Rows)
	}

	// --strict turns the baseline disagreement into a non-zero exit.
	_, errOut, strictCode := runSources(t, ts.URL, "--fetch", "per-fy2014-15", "--strict")
	if strictCode == 0 {
		t.Error("--strict accepted a body that disagrees with its baseline")
	}
	if !strings.Contains(errOut, "BASELINE") {
		t.Errorf("no BASELINE line on stderr:\n%s", errOut)
	}
}

// TestSourcesFetchRefusesNonPDF covers the case NEPRA actually produces: a
// nine-byte text/html error body served under a path the catalogue calls a PDF.
// Those nine bytes must never reach the caller as document content.
func TestSourcesFetchRefusesNonPDF(t *testing.T) {
	srv := sourcesDiffFixtureServer(t)
	target := "/Standards/PER%20DISCOs%20and%20KE%20for%202014-15.pdf"
	srv.bodies[target] = sourcesTestResponse{
		Status: 200, Body: []byte("Not Found"), ContentType: "text/html",
	}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	out, errOut, code := runSources(t, ts.URL, "--fetch", "per-fy2014-15")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 without --strict", code)
	}
	var env struct {
		Results []sourcesFetchResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Results) != 1 {
		t.Fatalf("results = %d", len(env.Results))
	}
	res := env.Results[0]
	if res.State != "refused_not_pdf" {
		t.Errorf("state = %q, want refused_not_pdf", res.State)
	}
	if !strings.Contains(res.RefusalReason, "%PDF") {
		t.Errorf("the refusal does not name the missing header: %q", res.RefusalReason)
	}
	if res.Measured.Pages != nil || res.Measured.Creator != nil {
		t.Error("a refused body still produced PDF measurements")
	}
	if res.Measured.TextLayer != "unmeasured" {
		t.Errorf("text_layer = %q, want unmeasured on a refused body", res.Measured.TextLayer)
	}
	// The prefix is bounded and the body never appears as content.
	if len(res.BodyPrefix) > 32 {
		t.Errorf("body_prefix is too long to be a prefix: %q", res.BodyPrefix)
	}
	if !strings.Contains(errOut, "REFUSED") {
		t.Errorf("no REFUSED line on stderr:\n%s", errOut)
	}
	_, _, strictCode := runSources(t, ts.URL, "--fetch", "per-fy2014-15", "--strict")
	if strictCode == 0 {
		t.Error("--strict accepted a body that is not the document")
	}
}

// TestSourcesFetch404IsNotFound is the one place where an HTTP failure IS the
// command's exit status: --fetch names one document, so a 404 is a genuine
// not-found and gets the typed exit 3. --diff must never do this.
func TestSourcesFetch404IsNotFound(t *testing.T) {
	srv := sourcesDiffFixtureServer(t)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	_, _, code := runSources(t, ts.URL, "--fetch", "per-fy2018-19")
	if code != 3 {
		t.Errorf("exit = %d, want 3 (not found) for a named document that 404s", code)
	}
}

// --- refusals and shape -------------------------------------------------

// TestSourcesRefusals checks every usage error is exit 2, not 1, and that each
// one says what the caller may pass instead.
func TestSourcesRefusals(t *testing.T) {
	srv := sourcesDiffFixtureServer(t)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"unknown kind", []string{"--kind", "zzz"}, []string{"gen", "per", "sir", "fca", "tariff"}},
		{"kind events", []string{"--kind", "events"}, []string{"events"}},
		{"unknown id", []string{"--fetch", "nope"}, []string{"per-fy2018-19"}},
		{"diff and fetch", []string{"--diff", "--fetch", "per-fy2018-19"}, []string{"mutually exclusive"}},
		{"reference row", []string{"--fetch", "tariff-lesco"}, []string{"events --disco LESCO"}},
		{"positional arg", []string{"gen"}, []string{"no positional arguments"}},
		{"kind mismatch", []string{"--kind", "gen", "--fetch", "per-fy2018-19"}, []string{"kind"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := srv.count()
			_, errOut, code := runSources(t, ts.URL, tc.args...)
			if code != 2 {
				t.Errorf("exit = %d, want 2", code)
			}
			if srv.count() != before {
				t.Errorf("a usage error made %d request(s)", srv.count()-before)
			}
			for _, want := range tc.want {
				if !strings.Contains(errOut, want) {
					t.Errorf("refusal does not mention %q:\n%s", want, errOut)
				}
			}
		})
	}
}

// TestSourcesCatalogueMakesNoRequest is the help-only-branch guarantee, and it
// is what makes `sources` safe to call before anything else.
func TestSourcesCatalogueMakesNoRequest(t *testing.T) {
	srv := sourcesDiffFixtureServer(t)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	for _, args := range [][]string{nil, {"--kind", "per"}, {"--kind", "gen"}, {"--strict"}} {
		out, _, code := runSources(t, ts.URL, args...)
		if code != 0 {
			t.Fatalf("sources %v exit = %d, want 0", args, code)
		}
		if srv.count() != 0 {
			t.Fatalf("sources %v made %d request(s); the catalogue must cost nothing", args, srv.count())
		}
		var env struct {
			Meta struct {
				Source       string       `json:"source"`
				Documents    int          `json:"documents"`
				Kinds        []string     `json:"kinds"`
				DeclaredGaps []sourcesGap `json:"declared_gaps"`
				SeeAlso      struct {
					Events struct {
						Surfaces int `json:"surfaces"`
					} `json:"events"`
					Scope struct {
						Limits int `json:"limits"`
					} `json:"scope"`
				} `json:"see_also"`
			} `json:"meta"`
			Results []sourceDoc `json:"results"`
		}
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("sources %v stdout is not the owned envelope: %v", args, err)
		}
		if env.Meta.Source != "catalogue" {
			t.Errorf("sources %v meta.source = %q, want catalogue", args, env.Meta.Source)
		}
		if len(env.Meta.Kinds) != 5 {
			t.Errorf("meta.kinds = %v, want the five", env.Meta.Kinds)
		}
		if len(env.Meta.DeclaredGaps) != len(sourcesDeclaredGaps) {
			t.Errorf("declared_gaps = %d, want %d", len(env.Meta.DeclaredGaps), len(sourcesDeclaredGaps))
		}
		// see_also counts are read from the OTHER catalogues at runtime, so
		// they can never drift from what they describe.
		if env.Meta.SeeAlso.Events.Surfaces != len(eventSurfaces) {
			t.Errorf("see_also.events.surfaces = %d, want len(eventSurfaces) = %d",
				env.Meta.SeeAlso.Events.Surfaces, len(eventSurfaces))
		}
		if env.Meta.SeeAlso.Scope.Limits != len(nepraScopeLimits) {
			t.Errorf("see_also.scope.limits = %d, want len(nepraScopeLimits) = %d",
				env.Meta.SeeAlso.Scope.Limits, len(nepraScopeLimits))
		}
		if env.Meta.Documents != len(env.Results) {
			t.Errorf("meta.documents = %d but %d rows were returned", env.Meta.Documents, len(env.Results))
		}
		for _, r := range env.Results {
			if r.Probe != nil {
				t.Errorf("%s carries a probe in catalogue mode; nothing was measured", r.ID)
			}
		}
	}
}

// TestSourcesCommandShape guards the annotations the publish gate reads, and
// the dry-run contract.
func TestSourcesCommandShape(t *testing.T) {
	var cmd *cobra.Command
	for _, c := range RootCmd().Commands() {
		if c.Name() == "sources" {
			cmd = c
		}
	}
	if cmd == nil {
		t.Fatal("sources is not registered on the root command")
	}
	for k, want := range map[string]string{
		"mcp:read-only":       "true",
		"pp:happy-args":       "--kind=per",
		"pp:typed-exit-codes": "true",
	} {
		if got := cmd.Annotations[k]; got != want {
			t.Errorf("annotation %s = %q, want %q", k, got, want)
		}
	}
	if _, ok := cmd.Annotations["pp:novel-scaffold"]; ok {
		t.Error("the scaffold annotation is still set; this command is implemented")
	}
	for _, f := range []string{"kind", "diff", "fetch", "strict"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("--%s is not declared", f)
		}
	}
	// The happy-arg must be exercisable offline, which is what makes it a
	// happy arg the gate can run.
	srv := sourcesDiffFixtureServer(t)
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()
	if _, _, code := runSources(t, ts.URL, "--kind", "per"); code != 0 || srv.count() != 0 {
		t.Errorf("the happy arg exit = %d after %d request(s); it must work offline", code, srv.count())
	}
	// --dry-run short-circuits any mode that would make a request.
	out, _, code := runSources(t, ts.URL, "--diff", "--dry-run")
	if code != 0 {
		t.Errorf("--dry-run exit = %d", code)
	}
	if srv.count() != 0 {
		t.Errorf("--dry-run made %d request(s)", srv.count())
	}
	if !strings.Contains(out, "dry_run") {
		t.Errorf("--dry-run produced no sentinel:\n%s", truncateForTest(out))
	}
}

func truncateForTest(s string) string {
	if len(s) > 600 {
		return s[:600] + "..."
	}
	return s
}
