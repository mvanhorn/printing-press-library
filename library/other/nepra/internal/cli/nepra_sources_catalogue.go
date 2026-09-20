// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-sources-catalogue.json.

package cli

import (
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// The document catalogue behind `sources`.
//
// NEPRA cannot be enumerated. robots.txt advertises two sitemaps and both are
// HTTP 404 with a nine-byte body; the one directory that holds the workbooks
// answers 403 with no autoindex; and the three index pages that do exist are
// incomplete in BOTH directions — Main.htm links five of the seven reachable
// generation years, and two reachable surfaces it never mentions. So the only
// honest way to turn this site into fetchable arguments is a table of URLs that
// were each measured live, and that is what this file is.
//
// EVERY NUMBER HERE WAS MEASURED, and the measurement is cited beside it. The
// survey figures were re-verified live on 2026-09-10 before this table shipped:
// robots.txt 1,944 B, Main.htm 12,210 B, the PER index 57,982 B, the SIR index
// 57,876 B, gen FY2023-24 493,187 B, gen FY2017-18 426,282 B, the workbook
// shell 9,838 B, filelist.xml 270 B, tabstrip.htm 823 B, both SIR Data stubs
// 374 B at md5 37c27f4decfbad45c80078a997f13c19, the FCA sheet 79,843 B and its
// shell 9,612 B, the generation directory 403/9 B, FY2024-25 404/9 B and
// sitemap_1.xml 404/9 B. All reproduced to the byte.
//
// THREE RULES THIS FILE OBEYS AND THE TESTS ENFORCE:
//
//  1. A path is never CONSTRUCTED for a PER or a SIR PDF. NEPRA's filenames
//     carry upstream typos that are part of the URL ("Genenration", "DSICOs",
//     "(FFinal)") and six different directory conventions, so the paths are a
//     table, not a pattern. The 8 per rows are joined from
//     internal/nepraper.Sources() at runtime, and the 22 sir paths were
//     HARVESTED VERBATIM from the SIR index — TestSourcesSIRPathsAreHarvested
//     re-derives all 22 from the committed fixture and fails if any was typed
//     by hand.
//
//  2. An HTTP 200 is never evidence of data. The per-year workbook shell is 200
//     and byte-identical at 9,838 B for three different fiscal years, and SIR
//     Data 2025.htm is 200, 374 B, titled "SIR Data 2024" and frames FY2023-24.
//     A gen data row therefore carries a ByteFloor and must clear it before
//     --diff will call it reachable.
//
//  3. A missing measurement is null, never zero. BytesAsOf, HTTPStatusAsOf,
//     PagesAsOf and CreatorAsOf are pointers for exactly that reason: NEPRA has
//     real 0-byte-shaped facts (a /Creator that is measured ABSENT on the
//     FY2024-25 PER) and unmeasured ones, and the two must not collapse.

// sourcesAsOfDate is the survey date every unannotated baseline in this file
// was measured on.
const sourcesAsOfDate = "2026-09-08"

// sourcesReverifiedDate is when the baselines listed in this file's header
// comment were independently re-fetched and reproduced to the byte.
const sourcesReverifiedDate = "2026-09-10"

const (
	// sourcesGenByteFloor is the size a generation workbook payload must clear
	// before it may be called data. The smallest real year is 418,887 B; the
	// largest decoy is 9,838 B, so the floor sits between them with three
	// orders of magnitude of headroom.
	sourcesGenByteFloor = 400000
	// sourcesDecoyShellBytes is the workbook frameset shell, byte-identical
	// across 2017-18, 2020-21 and 2023-24.
	sourcesDecoyShellBytes = 9838
	// sourcesSIRDataStubBytes is the SIR Data 2024/2025 stub. One file, two
	// names, same md5.
	sourcesSIRDataStubBytes = 374
	// sourcesNotFoundBodyBytes is NEPRA's 404 body: the nine bytes "Not Found",
	// hex-verified 4e6f7420466f756e64. A 403 returns the same nine.
	sourcesNotFoundBodyBytes = 9
	// sourcesFetchByteCeiling caps --fetch at 64 MiB because the client buffers
	// the whole body (io.ReadAll) and base64-wraps it at ~1.33x on the way out.
	sourcesFetchByteCeiling = 64 << 20
	// The measured throughput window for nepra.org.pk, which is what makes the
	// ceiling a fact rather than a preference.
	sourcesThroughputMinBPS = 68768
	sourcesThroughputMaxBPS = 163650
)

// sourcesKinds is the closed set of --kind values. "events" is deliberately
// NOT one of them: the tariff-determination surfaces are owned by the events
// catalogue, which asserts a ROW floor per surface, and two catalogues claiming
// one URL under two fetch routes is the drift this command exists to prevent.
var sourcesKinds = []string{"gen", "per", "sir", "fca", "tariff"}

// sourceCaseTwin records the same path under the other extension casing.
// NEPRA's server is case-sensitive on the extension and the direction is not
// predictable: the June 2026 FCA determination is 200 as .pdf and 404 as .PDF,
// while the July 2026 file is the reverse. Site-wide the homepage links 154
// .pdf and 81 .PDF.
type sourceCaseTwin struct {
	URL            string `json:"url"`
	HTTPStatusAsOf *int   `json:"http_status_as_of"`
	BytesAsOf      *int   `json:"bytes_as_of"`
}

// sourceDoc is one catalogued document.
//
// Field naming is deliberately "_as_of": every number is what a measurement on
// AsOfDate returned, not a promise about now. --diff is how a caller finds out
// whether it still holds.
type sourceDoc struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
	// Path carries NEPRA's own encoding byte for byte. It is EMPTY only for a
	// row whose URL was never recorded.
	Path string `json:"path,omitempty"`
	// Coverage is the fiscal or calendar window the document reports on.
	Coverage string `json:"coverage,omitempty"`
	// State is one of: published, reachable, unavailable, forbidden, decoy,
	// indexed_unmeasured, reference.
	State string `json:"state"`
	// ContentKind is pdf, html-excel-sheet, html, xml, css, php or txt. It
	// decides whether --diff may probe the row at all: this client has no HEAD
	// and no Range, so a PDF's existence cannot be checked without downloading
	// it.
	ContentKind string `json:"content_kind,omitempty"`
	// BytesAsOf is the DECODED length as CONTENT-LENGTH reports it — what curl,
	// a HEAD request and internal/nepraper all record. nil means never
	// measured, which is a different fact from zero.
	BytesAsOf *int `json:"bytes_as_of"`
	// ClientBytesAsOf is the decoded length THIS CLIENT observes, which is a
	// different number for every text/html body on this site. See
	// sourcesCloudflareHTMLInjectionBytes. nil means this build has not
	// measured the row with its own client, and --diff then says so rather
	// than comparing against a baseline taken with a different instrument.
	ClientBytesAsOf *int `json:"client_bytes_as_of"`
	// ByteFloor is the size below which a 200 is NOT data. Zero asserts
	// nothing.
	ByteFloor int `json:"byte_floor,omitempty"`
	// HTTPStatusAsOf is set ONLY where a non-2xx was actually observed. It is
	// never populated on a success: the client returns the body without the
	// status code, so recording "200" would assert a value never seen.
	HTTPStatusAsOf   *int    `json:"http_status_as_of,omitempty"`
	BodyBytesAsOf    *int    `json:"body_bytes_as_of,omitempty"`
	PagesAsOf        *int    `json:"pages_as_of,omitempty"`
	CreatorAsOf      *string `json:"creator_as_of,omitempty"`
	ProducerAsOf     *string `json:"producer_as_of,omitempty"`
	LastModifiedAsOf string  `json:"last_modified_as_of,omitempty"`
	MD5AsOf          string  `json:"md5_as_of,omitempty"`
	Charset          string  `json:"charset,omitempty"`
	// TextLayer is present, absent or unmeasured. It is "unmeasured" for every
	// SIR row on purpose — see the note on sirRows.
	TextLayer string `json:"text_layer,omitempty"`
	// LowTextPagesAsOf is [] for a PDF that was extracted and had none, and
	// null for one that was never extracted.
	// No omitempty: an empty slice must reach the JSON as [] rather than
	// vanishing, or "extracted and found none" becomes indistinguishable from
	// "never extracted".
	LowTextPagesAsOf []int `json:"low_text_pages_as_of"`
	// Structural counts measured byte-safely on the surface. Zero means not
	// counted.
	TablesAsOf        int `json:"tables_as_of,omitempty"`
	TRAsOf            int `json:"tr_as_of,omitempty"`
	TDAsOf            int `json:"td_as_of,omitempty"`
	RowsExtractedAsOf int `json:"rows_extracted_as_of,omitempty"`
	HrefsAsOf         int `json:"hrefs_as_of,omitempty"`
	// FetchWith is the shipped command line that retrieves this document, or
	// empty when no shipped command can. FetchArg is the exact argument value.
	FetchWith string `json:"fetch_with,omitempty"`
	FetchArg  string `json:"fetch_arg,omitempty"`
	// FetchBlockedReason says WHY no shipped command reaches the document. It
	// is required whenever FetchWith is empty and the document is reachable,
	// because "no route" without a reason is indistinguishable from an
	// oversight.
	FetchBlockedReason string `json:"fetch_blocked_reason,omitempty"`
	// PathEncodable is false when the path cannot be placed in a URL verbatim.
	// A false row is refused by --fetch rather than guessed at.
	PathEncodable bool `json:"path_encodable"`
	// Fetchable is COMPUTED by sourcesDocs from sourcesValidateFetch, so the
	// catalogue can never advertise a --fetch route the command would refuse.
	// False always comes with FetchBlockedReason or FetchRefusal set.
	Fetchable bool `json:"fetchable"`
	// FetchRefusal is the exact refusal --fetch would return, so a caller does
	// not have to run the command to find out why.
	FetchRefusal string          `json:"fetch_refusal,omitempty"`
	Quirks       []string        `json:"quirks,omitempty"`
	CaseTwin     *sourceCaseTwin `json:"case_twin,omitempty"`
	// RowsOwnedBy names the command that asserts this surface's row floor,
	// when that is a different command.
	RowsOwnedBy string `json:"rows_owned_by,omitempty"`
	Role        string `json:"role,omitempty"`
	Note        string `json:"note,omitempty"`
	// Evidence cites the measurement. Every number in this table has one.
	Evidence string `json:"evidence,omitempty"`
	// SourceOfRecord names the package that owns the row, where one does.
	SourceOfRecord string `json:"source_of_record,omitempty"`
	AsOfDate       string `json:"as_of_date"`
	// Probe is filled in only by --diff.
	Probe *sourceProbe `json:"probe,omitempty"`
}

func intp(v int) *int       { return &v }
func strp(v string) *string { return &v }

// sourceEnum is a would-be enumerator: something that could in principle list
// the site. Four of the six are dead, and recording that is the point.
type sourceEnum struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	// State is reachable or unavailable.
	State           string `json:"state"`
	BytesAsOf       *int   `json:"bytes_as_of"`
	ClientBytesAsOf *int   `json:"client_bytes_as_of"`
	HTTPStatusAsOf  *int   `json:"http_status_as_of,omitempty"`
	BodyBytesAsOf   *int   `json:"body_bytes_as_of,omitempty"`
	TRAsOf          int    `json:"tr_as_of,omitempty"`
	PDFRefsAsOf     int    `json:"pdf_refs_as_of,omitempty"`
	HrefsAsOf       int    `json:"hrefs_as_of,omitempty"`
	YearsLinked     int    `json:"years_linked_as_of,omitempty"`
	Role            string `json:"role,omitempty"`
	Note            string `json:"note,omitempty"`
	// Evidence cites the measurement behind every number on this row.
	Evidence string `json:"evidence,omitempty"`
	// DiffKinds are the catalogue kinds whose paths this index is comparable
	// with in the catalogue_only direction. Empty means the comparison is only
	// meaningful in the index_only direction — see gen-main-index.
	DiffKinds []string `json:"diff_kinds,omitempty"`
	// HrefFilter is "all" or "pdf": which hrefs on the page count as document
	// references. Recorded because the two indexes were measured on their PDF
	// refs and Main.htm on its complete href set.
	HrefFilter string       `json:"href_filter,omitempty"`
	Probe      *sourceProbe `json:"probe,omitempty"`
}

const (
	sourcesGenDir  = "/publications/State%20of%20Industry%20Reports/Detail%20of%20Generation"
	sourcesSIRDir  = "/publications/State%20of%20Industry%20Reports"
	sourcesPERPath = "/publications/Performance%20Reports.php"
	sourcesSIRPath = "/publications/State%20of%20Industry%20Reports.php"
	sourcesMainPat = sourcesGenDir + "/Main.htm"
)

// sourcesEnumerators is every candidate enumerator for this site, alive or
// dead. Four of six are dead, which is the finding: there is no machine
// enumerator for nepra.org.pk.
var sourcesEnumerators = []sourceEnum{
	{
		ID: "enum-robots", Path: "/robots.txt", State: "reachable",
		BytesAsOf: intp(1944), ClientBytesAsOf: intp(1944), Role: "governance",
		Note: "Content-Signal: search=yes,ai-train=no,use=reference; Allow: / for the wildcard agent, " +
			"plus a named-bot Disallow block and an Article 4 (EU 2019/790) reservation. " +
			"Reference use is permitted, training is not.",
		Evidence: "probe-evidence L1325-1330; re-verified live 1,944 B on " + sourcesReverifiedDate,
	},
	{
		ID: "enum-sitemap-1", Path: "/sitemap_1.xml", State: "unavailable",
		BytesAsOf: nil, HTTPStatusAsOf: intp(404), BodyBytesAsOf: intp(sourcesNotFoundBodyBytes),
		Role: "enumerator-dead",
		Note: "declared by robots.txt and 404. The body is literally 'Not Found' " +
			"(hex-verified 4e6f7420466f756e64). There is no machine enumerator for this site.",
		Evidence: "probe-evidence L1346; re-verified live 404/9 B on " + sourcesReverifiedDate,
	},
	{
		ID: "enum-sitemap-2", Path: "/sitemap_2.xml", State: "unavailable",
		BytesAsOf: nil, HTTPStatusAsOf: intp(404), BodyBytesAsOf: intp(sourcesNotFoundBodyBytes),
		Role: "enumerator-dead", Note: "declared by robots.txt and 404, same nine-byte body.",
		Evidence: "probe-evidence L1346",
	},
	{
		ID: "enum-sitemap", Path: "/sitemap.xml", State: "unavailable",
		BytesAsOf: nil, HTTPStatusAsOf: intp(404), BodyBytesAsOf: intp(sourcesNotFoundBodyBytes),
		Role: "enumerator-dead", Note: "the conventional location is 404 too.",
		Evidence: "probe-evidence L1346",
	},
	{
		ID: "enum-per-index", Path: sourcesPERPath, State: "reachable",
		BytesAsOf: intp(57982), ClientBytesAsOf: intp(58343),
		TRAsOf: 40, PDFRefsAsOf: 42, HrefsAsOf: 42,
		Role:       "the ONLY enumerator for the PER family: /M&E/PER/Distribution/ itself returns 403",
		DiffKinds:  []string{"per"},
		HrefFilter: "pdf",
		Note: "42 pdf refs, 41 of them distinct (PER 2022-23 - DSICOs is linked twice), and one linked " +
			"entry (PER DISCOs 2023-24.pdf) 404s. Five of its hrefs carry RAW spaces and one carries a " +
			"pre-encoded trailing %20 before .pdf.",
		Evidence: "probe-evidence L1286, L1338; re-verified live 57,982 B / 42 pdf refs on " + sourcesReverifiedDate,
	},
	{
		ID: "enum-sir-index", Path: sourcesSIRPath, State: "reachable",
		BytesAsOf: intp(57876), ClientBytesAsOf: intp(58237),
		PDFRefsAsOf: 46, HrefsAsOf: 46, YearsLinked: 22,
		Role:       "the enumerator for the SIR family; every one of the 22 sir rows' paths was harvested from it",
		DiffKinds:  []string{"sir"},
		HrefFilter: "pdf",
		Note: "46 pdf refs = 22 years linked TWICE each, plus two unrelated PDFs. Every SIR href carries " +
			"RAW spaces, so a harvester that does not encode them produces a path that 404s.",
		Evidence: "probe-evidence L1268-1282, L1337; re-verified live 57,876 B / 46 pdf refs / years 2004..2025 on " + sourcesReverifiedDate,
	},
}

// sourcesGenYearsReachable is the seven fiscal years whose workbook payload is
// real data, with the decoded byte count measured for each. The token carries
// NO "FY" prefix: the same path with FY2021-22 is a 404 with a nine-byte body.
var sourcesGenYearsReachable = []struct {
	FY    string
	Bytes int
}{
	{"2017-18", 426282},
	{"2018-19", 418887},
	{"2019-20", 421591},
	{"2020-21", 455640},
	{"2021-22", 516219},
	{"2022-23", 490461},
	{"2023-24", 493187},
}

// sourcesGenYears404 is the year tokens that were probed and are NOT published.
// They are catalogued because "probed and absent" is a fact a caller can use,
// and an omission is not.
var sourcesGenYears404 = []string{"2015-16", "2016-17", "2024-25", "2025-26"}

// sourcesGenSheetPath builds the workbook payload path for a fiscal year.
//
// This is the ONE place in this file where a path is built from a token rather
// than harvested, and it is safe for exactly one reason: the template is the
// shipped resource path (resource_paths.go "generation"), the seven tokens that
// resolve were each measured, and the four that do not are catalogued as 404s.
// Nothing here guesses at an unprobed year — sourcesGenYears404 exists so the
// series' end is recorded rather than inferred.
func sourcesGenSheetPath(fy string) string {
	return sourcesGenDir + "/List%20of%20Companies%20Genenration%20wise%20" + fy + "_files/sheet001.htm"
}

// sourcesGenShellPath builds the top-level per-year .htm, which is the decoy.
func sourcesGenShellPath(fy string) string {
	return sourcesGenDir + "/List%20of%20Companies%20Genenration%20wise%20" + fy + ".htm"
}

// sourcesSIRYearsMeasured is every SIR year whose Content-Length was actually
// read off the wire. The 2016, 2017 and 2018 figures are MINE, measured on
// 2026-09-10; the survey recorded those three in mixed units (97.6 "MB" is
// decimal, 7.2 and 26.5 are MiB) and never wrote their byte counts down.
//
// These ten sum to exactly 640,312,915 B — the corpus figure the survey
// reported — which is the cross-check that the three I measured are the three
// it meant. TestSourcesSIRCorpusReconciles pins it.
var sourcesSIRYearsMeasured = map[int]struct {
	Bytes        int
	LastModified string
	AsOf         string
}{
	2016: {97607884, "Tue, 25 Feb 2020", sourcesReverifiedDate},
	2017: {7538922, "Thu, 17 Jun 2021", sourcesReverifiedDate},
	2018: {27755828, "Tue, 25 Feb 2020", sourcesReverifiedDate},
	2019: {106428020, "Thu, 14 May 2020", sourcesAsOfDate},
	2020: {42616315, "Tue, 20 Oct 2020", sourcesAsOfDate},
	2021: {7595614, "Thu, 30 Sep 2021", sourcesAsOfDate},
	2022: {6140818, "Fri, 30 Sep 2022", sourcesAsOfDate},
	2023: {3425763, "Fri, 02 Feb 2024", sourcesAsOfDate},
	2024: {9350361, "Tue, 31 Dec 2024", sourcesAsOfDate},
	2025: {331853390, "Fri, 30 Jan 2026", sourcesAsOfDate},
}

// sourcesSIRCorpusBytesAsOf is the measured corpus, and it is a SUM OF TEN
// MEASUREMENTS, not a reported figure: TestSourcesSIRCorpusReconciles recomputes
// it from the rows. The other twelve years contribute nothing to it because
// nobody has measured them.
const sourcesSIRCorpusBytesAsOf = 640312915

// sourcesDISCOs is the eleven ex-WAPDA distribution companies whose tariff
// pages are catalogued as references.
var sourcesDISCOs = []string{
	"FESCO", "GEPCO", "HAZECO", "HESCO", "IESCO",
	"LESCO", "MEPCO", "PESCO", "QESCO", "SEPCO", "TESCO",
}

// sourcesCatalogue is the 55 non-per rows. The per kind is joined from
// nepraper at runtime by sourcesPerRows — see sourcesDocs.
var sourcesCatalogue = buildSourcesCatalogue()

func buildSourcesCatalogue() []sourceDoc {
	var out []sourceDoc
	out = append(out, sourcesGenRows()...)
	out = append(out, sourcesSIRRows()...)
	out = append(out, sourcesFCARows()...)
	out = append(out, sourcesTariffRows()...)
	return out
}

// sourcesGenRows is the 18 generation rows: seven reachable years, four probed
// 404s, the index, the 403 directory, the 200 decoy shell, the two workbook
// enumerators that DO work, and the two 374-byte SIR Data stubs.
func sourcesGenRows() []sourceDoc {
	out := make([]sourceDoc, 0, 18)
	for _, y := range sourcesGenYearsReachable {
		bytes := y.Bytes
		out = append(out, sourceDoc{
			ID: "gen-" + y.FY, Kind: "gen",
			Label:       "Detail of Generation workbook FY" + y.FY,
			Path:        sourcesGenSheetPath(y.FY),
			Coverage:    "FY" + y.FY,
			State:       "reachable",
			ContentKind: "html-excel-sheet",
			BytesAsOf:   &bytes,
			ByteFloor:   sourcesGenByteFloor,
			Charset:     "windows-1252",
			FetchWith:   "generation year " + y.FY,
			FetchArg:    y.FY,
			Quirks: []string{
				"upstream-typo-Genenration",
				"no-http-charset",
				"fy-prefix-404s",
			},
			PathEncodable: true,
			Role:          "data",
			Note: "the HTTP header declares no charset; only the in-document <meta> does. A naive UTF-8 " +
				"read of this file reports ZERO <tr> and ZERO <td>.",
			Evidence: "probe-evidence L918-935 (the byte table); FY2023-24 and FY2017-18 re-verified live on " + sourcesReverifiedDate,
			AsOfDate: sourcesAsOfDate,
		})
	}
	for _, fy := range sourcesGenYears404 {
		note := "probed and not published."
		if fy == "2024-25" {
			note = "four naming variants tested plus SIR Data 2026.htm and home25.htm; all 404. " +
				"The Detail-of-Generation series definitively ends at FY2023-24."
		}
		out = append(out, sourceDoc{
			ID: "gen-" + fy, Kind: "gen",
			Label:          "Detail of Generation workbook FY" + fy + " (not published)",
			Path:           sourcesGenSheetPath(fy),
			Coverage:       "FY" + fy,
			State:          "unavailable",
			ContentKind:    "html-excel-sheet",
			BytesAsOf:      nil,
			HTTPStatusAsOf: intp(404),
			BodyBytesAsOf:  intp(sourcesNotFoundBodyBytes),
			PathEncodable:  true,
			FetchBlockedReason: "NEPRA does not publish this fiscal year at this path. UNAVAILABLE is not " +
				"the same fact as a workbook that exists and reports nothing.",
			Role:     "data",
			Note:     note,
			Evidence: "probe-evidence L918-935, L1050, L1323; FY2024-25 re-verified live 404/9 B on " + sourcesReverifiedDate,
			AsOfDate: sourcesAsOfDate,
		})
	}
	out = append(out,
		sourceDoc{
			ID: "gen-main-index", Kind: "gen",
			Label: "Detail of Generation index (Main.htm)",
			Path:  sourcesMainPat, State: "reachable", ContentKind: "html",
			BytesAsOf: intp(12210), HrefsAsOf: 10, PathEncodable: true,
			Role: "enumerator-partial",
			Note: "INCOMPLETE IN BOTH DIRECTIONS, and must never be presented as the enumerator. Its " +
				"complete href set is exactly 10 entries: five generation years FY2017-18..FY2021-22, " +
				"Main_files/filelist.xml and four surfaces. It does NOT link FY2022-23, FY2023-24, " +
				"SIR Data 2024.htm or SIR Data 2025.htm — all of which are reachable. It also links the " +
				"workbook SHELLS, not the sheet001 payloads this catalogue holds, so a path-set diff " +
				"against it is only meaningful in the index_only direction.",
			FetchBlockedReason: "an index page is not a document; use `generation index`, or read the " +
				"catalogue this command prints.",
			Evidence: "probe-evidence L274-275, L1119, L1342; re-verified live 12,210 B / exactly 10 hrefs on " + sourcesReverifiedDate,
			AsOfDate: sourcesAsOfDate,
		},
		sourceDoc{
			ID: "gen-dir-listing", Kind: "gen",
			Label: "Detail of Generation directory",
			Path:  sourcesGenDir + "/", State: "forbidden", ContentKind: "html",
			BytesAsOf: nil, HTTPStatusAsOf: intp(403), BodyBytesAsOf: intp(sourcesNotFoundBodyBytes),
			PathEncodable: true, Role: "enumerator-dead",
			Note:               "no autoindex, so enumeration can never be a directory crawl.",
			FetchBlockedReason: "403. There is nothing behind this path to fetch.",
			Evidence:           "probe-evidence L961, L1054, L1098; re-verified live 403/9 B on " + sourcesReverifiedDate,
			AsOfDate:           sourcesAsOfDate,
		},
		sourceDoc{
			ID: "gen-shell-2023-24", Kind: "gen",
			Label: "Detail of Generation workbook shell FY2023-24",
			Path:  sourcesGenShellPath("2023-24"), State: "decoy", ContentKind: "html",
			BytesAsOf: intp(sourcesDecoyShellBytes), PathEncodable: true, Role: "decoy",
			Coverage: "FY2023-24",
			Quirks:   []string{"http-200-is-not-data", "byte-identical-across-years"},
			Note: "HTTP 200 and BYTE-IDENTICAL 9,838 B for 2017-18, 2020-21 and 2023-24. A " +
				"<frameset rows=\"*,18\"> shell, not data: fetching the obvious top-level .htm per year " +
				"yields three identical files under three 200s. sheet002.htm and sheet003.htm are both " +
				"404, so each workbook has exactly one sheet.",
			FetchBlockedReason: "this is a frameset, not data. Use `generation year 2023-24`, which " +
				"fetches the sheet001.htm payload.",
			Evidence: "probe-evidence L961, L1053; re-verified live 9,838 B on " + sourcesReverifiedDate,
			AsOfDate: sourcesAsOfDate,
		},
		sourceDoc{
			ID: "gen-filelist-2023-24", Kind: "gen",
			Label: "Excel manifest for the FY2023-24 workbook",
			Path:  sourcesGenDir + "/List%20of%20Companies%20Genenration%20wise%202023-24_files/filelist.xml",
			State: "reachable", ContentKind: "xml",
			BytesAsOf: intp(270), PathEncodable: true, Role: "enumerator-working",
			Note: "the one enumerator on this site that works, and it works per workbook rather than per " +
				"site: it names MainFile, stylesheet.css, tabstrip.htm, sheet001.htm and itself.",
			FetchBlockedReason: "no shipped command fetches the Excel manifest; it is catalogued because " +
				"it is the only reliable enumerator NEPRA publishes.",
			Evidence: "probe-evidence L952-961, L1052; re-verified live 270 B on " + sourcesReverifiedDate,
			AsOfDate: sourcesAsOfDate,
		},
		sourceDoc{
			ID: "gen-tabstrip-2023-24", Kind: "gen",
			Label: "Excel tab strip for the FY2023-24 workbook",
			Path:  sourcesGenDir + "/List%20of%20Companies%20Genenration%20wise%202023-24_files/tabstrip.htm",
			State: "reachable", ContentKind: "html",
			BytesAsOf: intp(823), PathEncodable: true, Charset: "windows-1252",
			Role: "enumerator-working",
			Note: "declares charset=windows-1252 and Microsoft Excel 15, and names the sheets: one tab " +
				"labelled 2023-24 linking sheet001.htm. This is where the encoding fact is documented " +
				"upstream, since the HTTP header never carries it.",
			FetchBlockedReason: "no shipped command fetches the tab strip.",
			Evidence:           "probe-evidence L952-961, L1082; re-verified live 823 B on " + sourcesReverifiedDate,
			AsOfDate:           sourcesAsOfDate,
		},
		sourceDoc{
			ID: "gen-sir-data-2024", Kind: "gen",
			Label: "SIR Data 2024 stub",
			Path:  sourcesGenDir + "/SIR%20Data%202024.htm", State: "decoy", ContentKind: "html",
			BytesAsOf: intp(sourcesSIRDataStubBytes), MD5AsOf: "37c27f4decfbad45c80078a997f13c19",
			PathEncodable: true, Role: "decoy", Coverage: "FY2023-24",
			Quirks: []string{"http-200-is-not-data", "unlinked-from-index"},
			Note: "374 B, titled 'SIR Data 2024', frames FY2023-24. Its header frame home24.htm (200, " +
				"669 B) reads 'Data pertaining to State of Industry Report 2023-24'. NOT linked from " +
				"Main.htm — the index is incomplete in BOTH directions.",
			FetchBlockedReason: "a 374-byte frameset carrying no data.",
			Evidence:           "probe-evidence L240-270, L322, L1323; re-verified live 374 B / md5 match on " + sourcesReverifiedDate,
			AsOfDate:           sourcesAsOfDate,
		},
		sourceDoc{
			ID: "gen-sir-data-2025", Kind: "gen",
			Label: "SIR Data 2025 stub",
			Path:  sourcesGenDir + "/SIR%20Data%202025.htm", State: "decoy", ContentKind: "html",
			BytesAsOf: intp(sourcesSIRDataStubBytes), MD5AsOf: "37c27f4decfbad45c80078a997f13c19",
			PathEncodable: true, Role: "decoy", Coverage: "FY2023-24",
			Quirks: []string{"http-200-is-not-data", "byte-identical-to-2024-stub", "unlinked-from-index"},
			Note: "BYTE-IDENTICAL to SIR Data 2024.htm — same 374 bytes, same md5 " +
				"37c27f4decfbad45c80078a997f13c19 — titled 'SIR Data 2024' and framing FY2023-24. It is " +
				"one file under two names. Treating it as a 2025 surface silently duplicates FY2023-24 " +
				"as FY2024-25, so these two rows must never be merged or de-duplicated.",
			FetchBlockedReason: "a 374-byte frameset carrying no data, and not a 2025 surface.",
			Evidence:           "probe-evidence L240-270, L1345; re-verified live 374 B / md5 match on " + sourcesReverifiedDate,
			AsOfDate:           sourcesAsOfDate,
		},
	)
	return out
}

// sourcesSIRRows is the 22-year State of Industry Report series.
//
// EVERY PATH HERE WAS HARVESTED FROM THE INDEX, not templated: the SIR index
// links all 22 years, twice each, with RAW spaces in the href, and
// TestSourcesSIRPathsAreHarvested re-derives all 22 from the committed fixture.
// Ten years have a measured Content-Length; the other twelve are on the index's
// run and nothing more, which is what state indexed_unmeasured means. Probing
// them into existence would cost a multi-megabyte download each.
//
// TextLayer is "unmeasured" on all 22 rows, deliberately. The claim that
// "sir2023.pdf extracts 0 bytes" names a file that appears nowhere in the probe
// evidence; what was measured is 'State of Industry Report 2023.pdf' at
// 3,425,763 B, and its text layer was never read. Reporting absent here would
// be asserting someone else's unsourced number.
func sourcesSIRRows() []sourceDoc {
	out := make([]sourceDoc, 0, 22)
	for year := 2004; year <= 2025; year++ {
		row := sourceDoc{
			ID: "sir-" + itoa(year), Kind: "sir",
			Label:         "State of Industry Report " + itoa(year),
			Path:          sourcesSIRDir + "/State%20of%20Industry%20Report%20" + itoa(year) + ".pdf",
			Coverage:      itoa(year),
			ContentKind:   "pdf",
			TextLayer:     "unmeasured",
			PathEncodable: true,
			AsOfDate:      sourcesAsOfDate,
			FetchWith:     "",
			FetchBlockedReason: "the shipped `sir <year>` command builds " +
				"/publications/State%20of%20Industry%20Reports/sir<year>.pdf, which matches NO path this " +
				"catalogue measured; that route is broken for every year. The measured form is the path " +
				"in this row.",
			Evidence: "path harvested verbatim from the SIR index (testdata/index-sir.php.gz), " +
				"re-verified live on " + sourcesReverifiedDate,
		}
		if m, ok := sourcesSIRYearsMeasured[year]; ok {
			b := m.Bytes
			row.State = "reachable"
			row.BytesAsOf = &b
			row.LastModifiedAsOf = m.LastModified
			row.AsOfDate = m.AsOf
			row.Evidence = "Content-Length measured by HEAD; probe-evidence L1268-1282" +
				sourcesSIRExtraEvidence(year)
			if b > sourcesFetchByteCeiling {
				row.FetchBlockedReason = commaBytes(b) + " B at the measured " +
					commaBytes(sourcesThroughputMinBPS) + "-" + commaBytes(sourcesThroughputMaxBPS) +
					" B/s is " + sourcesETAWindow(b) + ", and the client buffers the whole body then " +
					"base64-wraps it. Above the " + itoa(sourcesFetchByteCeiling>>20) + " MiB --fetch ceiling."
			}
		} else {
			row.State = "indexed_unmeasured"
			row.BytesAsOf = nil
			row.Note = "on the index's 22-year run 2004..2025 but never probed; 12 of 22 years are in " +
				"this state. That is the absence of a MEASUREMENT, not a claim about reachability, and " +
				"not a claim that the report is missing."
			row.Evidence = "linked twice from the SIR index (46 pdf refs = 22 years x 2 + 2 unrelated); " +
				"no status and no size were ever measured"
		}
		out = append(out, row)
	}
	return out
}

// sourcesSIRExtraEvidence marks the three years this build measured itself.
func sourcesSIRExtraEvidence(year int) string {
	switch year {
	case 2016, 2017, 2018:
		return "; exact byte count measured on " + sourcesReverifiedDate +
			" (the survey recorded these three in mixed units and never wrote their bytes down)"
	default:
		return "; re-verified for 2025 on " + sourcesReverifiedDate
	}
}

// sourcesFCARows is the fuel-cost-adjustment surfaces: the frozen HTML table,
// its shell decoy, and the two 2026 determination PDFs that prove extension
// casing is server-enforced in both directions.
func sourcesFCARows() []sourceDoc {
	junePath := "/tariff/Tariff/Ex-WAPDA%20DISCOS/2026/TRF-100%20XWDISCOS%20FCA%20JUN%202026%2007-08-2026%2017536-54.pdf"
	julyPath := "/tariff/Tariff/Ex-WAPDA%20DISCOS/2026/TRF-100%20MFPA%20FOR%20THE%20MONTH%20OF%20JULY%202026%20EX-WAPDA%20DISCOS%2004-09-2026%2018342-60.PDF"
	return []sourceDoc{
		{
			ID: "fca-sheet-2018-2022", Kind: "fca",
			Label: "Consolidated FCA table (frozen)",
			Path:  sourcesGenDir + "/FCA%20(2018-2022)_files/sheet001.htm",
			State: "reachable", ContentKind: "html-excel-sheet",
			BytesAsOf: intp(79843), Charset: "windows-1252",
			Coverage:          "Jul-2018..Jun-2022 (48 consecutive months, zero gaps)",
			TablesAsOf:        1,
			TRAsOf:            53,
			TDAsOf:            572,
			RowsExtractedAsOf: 48,
			FetchWith:         "fca",
			PathEncodable:     true,
			Role:              "data",
			Quirks: []string{
				"frozen-since-jun-2022",
				"accounting-parenthesised-negatives",
				"upstream-typo-Forthnight",
				"doubled-space-in-header",
			},
			Note: "the ONLY NEPRA HTML surface carrying actual numeric tariff values. 49 parenthesised " +
				"negatives, which float() drops the sign of or throws on. Frozen for four years through " +
				"the largest tariff shock in Pakistan's history; the series continues only in scattered " +
				"scanned PDFs like the two rows below.",
			Evidence: "probe-evidence L1123-1180, L1306; extracted row count 48 pinned by " +
				"nepra_table_extract_test.go; re-verified live 79,843 B on " + sourcesReverifiedDate,
			AsOfDate: sourcesAsOfDate,
		},
		{
			ID: "fca-shell-2018-2022", Kind: "fca",
			Label: "FCA workbook shell",
			Path:  sourcesGenDir + "/FCA%20(2018-2022).htm",
			State: "decoy", ContentKind: "html",
			BytesAsOf: intp(9612), PathEncodable: true, Role: "decoy",
			TablesAsOf: 2, TRAsOf: 3, TDAsOf: 24,
			Quirks: []string{"http-200-is-not-data"},
			Note: "200 at 9,612 B of pure chrome: 3 <frameset>, 5 <frame>, 2 <table>, 3 <tr>, 24 <td>. " +
				"The spec originally pointed `fca` here and the command returned a 200 with no numbers. " +
				"Catalogued so the mistake cannot be made twice.",
			FetchBlockedReason: "this is the frameset. `fca` now reads the _files/sheet001.htm payload.",
			Evidence:           "probe-evidence L1123-1130; re-verified live 9,612 B on " + sourcesReverifiedDate,
			AsOfDate:           sourcesAsOfDate,
		},
		{
			ID: "fca-jun-2026-determination", Kind: "fca",
			Label: "FCA determination, June 2026 (ex-WAPDA DISCOs)",
			Path:  junePath,
			State: "reachable", ContentKind: "pdf",
			BytesAsOf: intp(793122), PagesAsOf: intp(13), TextLayer: "present",
			LastModifiedAsOf: "Fri, 07 Aug 2026", Coverage: "Jun-2026",
			PathEncodable: true,
			Quirks:        []string{"extension-case-sensitive"},
			CaseTwin: &sourceCaseTwin{
				URL:            nepraSourcesHost + strings.TrimSuffix(junePath, ".pdf") + ".PDF",
				HTTPStatusAsOf: intp(404),
				BytesAsOf:      nil,
			},
			FetchBlockedReason: "no shipped command fetches a determination PDF; `events` catalogues the " +
				"row that links it. Use `sources --fetch fca-jun-2026-determination`.",
			Note: "This exact path is 200 as .pdf and 404 as .PDF. The July 2026 file below REVERSES it. " +
				"Extension casing therefore cannot be normalised or guessed; site-wide the homepage " +
				"links 154 .pdf and 81 .PDF. Contents: CPPA-G requested Rs.1.2000/kWh, the Authority " +
				"allowed Rs.0.7503/kWh — the requested-vs-allowed structure NEPRA stopped tabulating in " +
				"June 2022.",
			Evidence: "probe-evidence L1544 (path verbatim), L1600, L1642; both casings re-verified live " +
				"on " + sourcesReverifiedDate + " (200/793,122 B as .pdf, 404 as .PDF)",
			AsOfDate: sourcesAsOfDate,
		},
		{
			ID: "fca-jul-2026-mfpa", Kind: "fca",
			Label: "MFPA determination, July 2026 (ex-WAPDA DISCOs)",
			Path:  julyPath,
			State: "reachable", ContentKind: "pdf",
			BytesAsOf: intp(630229), LastModifiedAsOf: "Fri, 04 Sep 2026",
			TextLayer: "unmeasured", Coverage: "Jul-2026",
			PathEncodable: true,
			Quirks:        []string{"extension-case-sensitive", "case-reversed-from-june"},
			CaseTwin: &sourceCaseTwin{
				URL:            nepraSourcesHost + strings.TrimSuffix(julyPath, ".PDF") + ".pdf",
				HTTPStatusAsOf: intp(404),
				BytesAsOf:      nil,
			},
			FetchBlockedReason: "no shipped command fetches a determination PDF. Use " +
				"`sources --fetch fca-jul-2026-mfpa`.",
			Note: "200 as .PDF and 404 as .pdf — the exact reverse of the June file. The survey recorded " +
				"the path (L1335) and the 630,229 B figure (L1600) in two different places and never " +
				"joined them; this build joined them by measuring this path directly.",
			Evidence: "path verbatim at probe-evidence L1335; the case reversal at L1600; the " +
				"path-to-size join measured on " + sourcesReverifiedDate + " (200, Content-Length 630,229)",
			AsOfDate: sourcesReverifiedDate,
		},
	}
}

// sourcesTariffRows is the eleven per-DISCO tariff pages, as REFERENCES.
//
// These rows assert NOTHING about their contents and carry no byte or row
// floor, because none was ever measured per page: the survey recorded 3,682
// rows for the GROUP and the live rebuild measured 3,661 determination rows +
// 23 furniture skipped across all eleven. The floor therefore lives in the
// events catalogue at group level (eventsDISCORowsAsOf), and each row here
// points at the command that owns it.
//
// This is the one place where a sources path coincides with an eventSurfaces
// path, and it is why every one of these rows is state "reference".
// TestSourcesDoesNotShadowEventSurfaces enforces the invariant that matters: no
// row that ASSERTS anything may claim a URL the events catalogue already owns.
func sourcesTariffRows() []sourceDoc {
	out := make([]sourceDoc, 0, len(sourcesDISCOs))
	for _, d := range sourcesDISCOs {
		out = append(out, sourceDoc{
			ID: "tariff-" + strings.ToLower(d), Kind: "tariff",
			Label:         d + " tariff determinations",
			Path:          "/tariff/Distribution%20" + d + ".php",
			State:         "reference",
			ContentKind:   "php",
			BytesAsOf:     nil,
			PathEncodable: true,
			FetchWith:     "tariff " + d,
			FetchArg:      d,
			RowsOwnedBy:   "events --disco " + d,
			Role:          "reference",
			Note: "no per-page byte or row floor was ever measured for the eleven DISCO pages: the survey " +
				"recorded 3,682 rows for the GROUP, and the live rebuild measured 3,661 determination " +
				"rows + 23 furniture skipped across all eleven. This row asserts nothing and exists so " +
				"the surface is discoverable from one catalogue; `events --disco " + d + "` is what " +
				"asserts its rows.",
			Evidence: "eventsDISCORowsAsOf = " + itoa(eventsDISCORowsAsOf) +
				" (group level, nepra_events_catalogue.go); per-page counts at probe-evidence L1343",
			AsOfDate: sourcesAsOfDate,
		})
	}
	return out
}

const nepraSourcesHost = "https://nepra.org.pk"

// sourcesPerRows JOINS the Performance Evaluation Report kind from
// internal/nepraper rather than copying it.
//
// nepraper.Sources() is already the measured PER catalogue — fiscal year, the
// URL path with NEPRA's encoding preserved byte for byte, decoded bytes, page
// count, /Creator, the text-layer flag and the low-text pages — so copying it
// here would create a second table to drift against.
// TestSourcesPerKindTracksNepraper fails if these two ever disagree.
//
// The fetch_arg discipline is the load-bearing part. `reliability` substitutes
// its argument into /Standards/{path}, so only the five records rooted at
// /Standards/ can be reached through it. validateEncodedSubPath would happily
// ACCEPT "M&E/PER/Distribution/PER%202022-23%20-%20DSICOs.pdf" — it is printable
// ASCII with no ".." — and `reliability` would then build
// /Standards/M&E/PER/... and return a 404 indistinguishable from a document
// NEPRA never published. So the prefix test is what decides, not the validator.
func sourcesPerRows() []sourceDoc {
	const standardsPrefix = "/Standards/"
	recs := nepraper.Sources()
	out := make([]sourceDoc, 0, len(recs))
	for _, r := range recs {
		row := sourceDoc{
			ID: "per-" + strings.ToLower(r.FY), Kind: "per",
			Label:          "PER Distribution Companies " + r.FY,
			Path:           r.URLPath,
			Coverage:       r.FY,
			State:          r.Availability.String(),
			ContentKind:    "pdf",
			PathEncodable:  true,
			SourceOfRecord: "internal/nepraper.Sources()",
			AsOfDate:       sourcesAsOfDate,
			Note:           r.Note,
			Evidence: "internal/nepraper/registry.go, reproduced independently at " +
				"probe-evidence L409-419",
		}
		if r.Availability == nepraper.AvailabilityPublished {
			b := r.Bytes
			row.BytesAsOf = &b
			p := r.Pages
			row.PagesAsOf = &p
			// Creator "" on a PUBLISHED record is a measurement: the FY2024-25
			// report was re-processed by iLovePDF, so /Creator is absent. A
			// pointer keeps "measured absent" distinct from "never measured".
			row.CreatorAsOf = strp(r.Creator)
			if r.HasTextLayer {
				row.TextLayer = "present"
			} else {
				row.TextLayer = "unmeasured"
			}
			// [] means extracted and none found; null would mean never
			// extracted. nepraper returns nil for none, so normalise here.
			if r.LowTextPages == nil {
				row.LowTextPagesAsOf = []int{}
			} else {
				row.LowTextPagesAsOf = append([]int{}, r.LowTextPages...)
			}
		} else {
			row.BytesAsOf = nil
			row.HTTPStatusAsOf = intp(404)
			row.BodyBytesAsOf = intp(sourcesNotFoundBodyBytes)
			row.TextLayer = "unmeasured"
		}
		if strings.HasPrefix(r.URLPath, standardsPrefix) {
			arg := strings.TrimPrefix(r.URLPath, standardsPrefix)
			row.FetchWith = "reliability '" + arg + "'"
			row.FetchArg = arg
			if r.FY == "FY2020-21" {
				row.Quirks = append(row.Quirks, "trailing-encoded-space")
			}
			row.Quirks = append(row.Quirks, "filename-unconstructible")
		} else {
			row.FetchBlockedReason = "the `reliability` command substitutes into /Standards/{path}, so it " +
				"cannot reach a document rooted at /M&E/. Passing this path to it would build " +
				"/Standards/M&E/PER/... and return a 404 that is indistinguishable from a document NEPRA " +
				"never published. Use `sources --fetch " + row.ID + "`."
			row.Quirks = append(row.Quirks, "directory-moved-after-fy2021-22")
			if strings.Contains(r.URLPath, "DSICOs") {
				row.Quirks = append(row.Quirks, "upstream-typo-DSICOs")
			}
		}
		if r.Availability == nepraper.AvailabilityUnavailable {
			row.FetchWith = ""
			row.FetchArg = ""
			row.FetchBlockedReason = "linked from NEPRA's own Performance Reports index but 404s with a " +
				"nine-byte body. UNAVAILABLE is not the same fact as a report that exists and reports " +
				"nothing, and it is not a zero."
		}
		out = append(out, row)
	}
	return out
}

// sourcesCloudflareHTMLInjectionBytes is a MEASURED, UNANIMOUS 361.
//
// Every text/html body on nepra.org.pk comes back 361 bytes LARGER through this
// CLI's client than through curl, because Cloudflare appends its
// email-decode script to an HTML response whose request Accept asks for HTML,
// and this client's Accept does. MEASURED on 2026-09-10 across all 14 html and
// html-excel-sheet rows plus both .php indexes: the delta was +361 on 16 of 16,
// with no exceptions. It is +0 on robots.txt (text/plain) and +0 on
// filelist.xml (text/xml), which is what identifies the injection as
// HTML-specific rather than a site-wide change.
//
// It also corroborates a figure the survey already had and could not explain:
// "the same page is 74,431 vs 74,792 bytes depending on the request's Accept
// header". 74,792 - 74,431 = 361.
//
// THE CONSEQUENCE IS THE POINT: A BYTE BASELINE IS NOT PORTABLE BETWEEN
// CLIENTS. Comparing what this client receives against a Content-Length
// measured by curl reports "grew +361" on every HTML row forever, which would
// make --strict permanently red and hide any real change in the noise. So both
// baselines are recorded, --diff compares like with like, and every probe says
// which baseline it used. Neither number is adjusted to match the other:
// deriving one from the other by arithmetic would be inference, and both of
// these were measured.
const sourcesCloudflareHTMLInjectionBytes = 361

// sourcesClientBytesAsOf is what THIS CLIENT measured on 2026-09-10, per row.
//
// These are observations, not BytesAsOf + 361. The relationship between the two
// tables is what TestSourcesClientBaselineOffsetIsMeasured checks, and it can
// only check it because both sides were measured independently.
var sourcesClientBytesAsOf = map[string]int{
	// text/html and the Excel html-excel-sheet payloads: +361 each.
	"gen-2017-18":          426643,
	"gen-2018-19":          419248,
	"gen-2019-20":          421952,
	"gen-2020-21":          456001,
	"gen-2021-22":          516580,
	"gen-2022-23":          490822,
	"gen-2023-24":          493548,
	"gen-main-index":       12571,
	"gen-shell-2023-24":    10199,
	"gen-sir-data-2024":    735,
	"gen-sir-data-2025":    735,
	"gen-tabstrip-2023-24": 1184,
	"fca-sheet-2018-2022":  80204,
	"fca-shell-2018-2022":  9973,
	// text/xml: no HTML injection, so the two baselines agree exactly.
	"gen-filelist-2023-24": 270,
}

// sourcesHTMLContentKinds are the kinds Cloudflare injects into. xml and txt
// are deliberately absent: both were measured at +0.
var sourcesHTMLContentKinds = map[string]bool{
	"html":             true,
	"html-excel-sheet": true,
	"php":              true,
}

// sourcesDocs is the whole catalogue: the hand-authored 55 plus the per kind
// joined live from nepraper. Sorted by kind then id so output is stable.
func sourcesDocs() []sourceDoc {
	out := make([]sourceDoc, 0, len(sourcesCatalogue)+8)
	out = append(out, sourcesCatalogue...)
	out = append(out, sourcesPerRows()...)
	kindOrder := map[string]int{}
	for i, k := range sourcesKinds {
		kindOrder[k] = i
	}
	sort.SliceStable(out, func(i, j int) bool {
		if kindOrder[out[i].Kind] != kindOrder[out[j].Kind] {
			return kindOrder[out[i].Kind] < kindOrder[out[j].Kind]
		}
		return out[i].ID < out[j].ID
	})
	// Fetchability is computed, never typed in, so the catalogue cannot
	// advertise a route the command refuses. A row NEPRA does not currently
	// serve is not fetchable either, even though the request would be legal:
	// telling a caller to fetch a document catalogued as a 404 would be
	// advertising the 404.
	for i := range out {
		if b, ok := sourcesClientBytesAsOf[out[i].ID]; ok {
			v := b
			out[i].ClientBytesAsOf = &v
		}
		if err := sourcesValidateFetch(out[i]); err != nil {
			out[i].Fetchable = false
			out[i].FetchRefusal = err.Error()
			continue
		}
		switch out[i].State {
		case "published", "reachable":
			out[i].Fetchable = true
		default:
			out[i].Fetchable = false
			out[i].FetchRefusal = "state is " + out[i].State +
				", so a fetch would not return the document this row describes"
		}
	}
	return out
}

// sourcesDocsForKind filters the catalogue. An empty kind returns everything.
func sourcesDocsForKind(kind string) []sourceDoc {
	all := sourcesDocs()
	if kind == "" {
		return all
	}
	out := make([]sourceDoc, 0, len(all))
	for _, d := range all {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

// sourceDocByID resolves an id across the whole catalogue.
func sourceDocByID(id string) (sourceDoc, bool) {
	for _, d := range sourcesDocs() {
		if d.ID == id {
			return d, true
		}
	}
	return sourceDoc{}, false
}

// sourcesValidateKind refuses anything outside the closed set, naming it.
func sourcesValidateKind(kind string) error {
	if kind == "" {
		return nil
	}
	for _, k := range sourcesKinds {
		if k == kind {
			return nil
		}
	}
	extra := ""
	if strings.EqualFold(kind, "events") {
		extra = ". The tariff-determination surfaces are catalogued by `events`, which asserts a row " +
			"floor per surface; `sources` never re-lists them"
	}
	return &sourcesKindError{Kind: kind, Extra: extra}
}

type sourcesKindError struct {
	Kind  string
	Extra string
}

func (e *sourcesKindError) Error() string {
	return "unknown --kind " + quoteArg(e.Kind) + "; the five kinds are " +
		strings.Join(sourcesKinds, ", ") + e.Extra
}

// sourcesShadowedEventPaths returns every path where a sources row that
// ASSERTS something collides with a surface the events catalogue owns.
//
// A "reference" row is exempt by construction: it carries no bytes, no floor
// and no fetch route of its own, and it names the owning command in
// RowsOwnedBy. What must never happen is two catalogues making two different
// claims about one URL.
func sourcesShadowedEventPaths() []string {
	owned := map[string]bool{}
	for _, s := range eventSurfaces {
		owned[s.Path] = true
	}
	var out []string
	for _, d := range sourcesDocs() {
		if d.State == "reference" || d.Path == "" {
			continue
		}
		if owned[d.Path] {
			out = append(out, d.Path)
		}
	}
	sort.Strings(out)
	return out
}

// --- small formatting helpers, local to this command ---------------------
//
// These are deliberately local rather than added to helpers.go: helpers.go is
// generator-owned and a regen would revert an edit there.

func itoa(v int) string { return strconv.Itoa(v) }

// commaBytes groups a byte count with thousands separators, because the whole
// point of the --fetch refusal message is that a human reads 331,853,390 and
// stops.
func commaBytes(v int) string {
	s := strconv.Itoa(v)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func quoteArg(v string) string { return strconv.Quote(v) }

// sourcesETAWindow states how long a body of n bytes takes at NEPRA's measured
// throughput. It reports a RANGE because that is what was measured; a single
// number would be a guess dressed as a fact.
func sourcesETAWindow(n int) string {
	slow := n / sourcesThroughputMinBPS
	fast := n / sourcesThroughputMaxBPS
	return sourcesDuration(fast) + "-" + sourcesDuration(slow)
}

func sourcesDuration(seconds int) string {
	switch {
	case seconds < 60:
		return itoa(seconds) + "s"
	case seconds < 3600:
		return itoa(seconds/60) + "m"
	default:
		return itoa(seconds/3600) + "h" + itoa((seconds%3600)/60) + "m"
	}
}
