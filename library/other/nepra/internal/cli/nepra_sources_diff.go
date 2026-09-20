// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-sources-catalogue.json.

package cli

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
)

// --diff: re-measure the catalogue against the live site.
//
// TWO RULES GOVERN THIS FILE.
//
// FIRST: A PROBED 404 OR 403 IS THE PRODUCT, NOT AN ERROR. classifyAPIError
// (helpers.go:360) maps HTTP 404 to exit 3 and HTTP 403 to exit 4. Routing a
// probe through it would make `sources --diff` exit non-zero on a catalogue
// that is behaving exactly as documented — four of its rows are catalogued
// PRECISELY BECAUSE they 404, and one because it 403s. So probe failures are
// read off *client.APIError and recorded as data. Only --strict turns a DRIFT
// VERDICT into a non-zero exit, and only ever exit 1.
//
// SECOND: A PDF IS NEVER PROBED. internal/client has no Head() and no Range
// support, and ProbeGet (client.go:464) issues a full GET and throws the body
// away — so it can report a status but cannot measure bytes, and it cannot
// check a PDF's existence without downloading it. The SIR corpus alone is
// 640,312,915 B at a measured 68,768-163,650 B/s, which is 65-155 minutes. So
// every content_kind=="pdf" row comes back verdict "not_probed" with the
// missing capability named. NEPRA serves accept-ranges: bytes on every PDF, so
// a client Head() or a one-byte Range accessor would make this leg cheap and
// honest; that is a declared gap, not an approximation.

// sourceProbe is what one live re-measurement found.
type sourceProbe struct {
	// Verdict is unchanged | grew | shrank | appeared | disappeared |
	// still_unreachable | decoy | not_probed.
	Verdict string `json:"verdict"`
	// BytesNow is the decoded length on a successful fetch, nil otherwise.
	BytesNow *int `json:"bytes_now,omitempty"`
	// DeltaBytes is BytesNow - BytesAsOf, and is nil when either side is
	// unmeasured. A nil delta is not a zero delta.
	DeltaBytes *int `json:"delta_bytes,omitempty"`
	// HTTPStatus is present ONLY on a non-2xx, which is the only case where
	// this client reports one.
	HTTPStatus *int `json:"http_status,omitempty"`
	// BodyBytes is the error body's length, and is present only when the body
	// came back under the client's 4,096-byte cap, where the length is exact.
	BodyBytes *int `json:"body_bytes,omitempty"`
	// BaselineUsed is "client" | "content-length" | "none": WHICH baseline the
	// delta was computed against. It matters because the two differ by a
	// measured 361 bytes on every text/html body, so a delta without this
	// field cannot be interpreted.
	BaselineUsed  string `json:"baseline_used,omitempty"`
	BaselineBytes *int   `json:"baseline_bytes,omitempty"`
	// BaselineNote is set when the only available baseline was taken with a
	// DIFFERENT instrument, so a delta of exactly +361 on an HTML body means
	// "different Accept header", not "the document changed".
	BaselineNote    string `json:"baseline_note,omitempty"`
	NotProbedReason string `json:"not_probed_reason,omitempty"`
	// Drift marks a verdict the caller should look at. It is computed, not
	// asserted, so --strict and the human summary cannot disagree.
	Drift bool `json:"drift"`
}

// sourcesProbeObs is one observation, split out from the transport so the
// verdict table is testable without a network.
type sourcesProbeObs struct {
	// Status is 0 on the client's success path, where no status is reported.
	Status int
	// Bytes is the decoded body length on success.
	Bytes int
	// BodyBytes is the error-body length. BodyExact says whether the client's
	// 4,096-byte truncation cap left it exact.
	BodyBytes int
	BodyExact bool
	// TransportErr is a network/transport failure: no status, no body, and no
	// verdict is possible.
	TransportErr string
}

func (o sourcesProbeObs) ok() bool {
	return o.TransportErr == "" && (o.Status == 0 || (o.Status >= 200 && o.Status < 300))
}

// sourcesVerdict is the whole verdict table, as a pure function.
//
// The byte-floor branch comes FIRST and that ordering is load-bearing: without
// it a 200 at 9,838 bytes on a generation row reads as "reachable", which is
// exactly the mistake the shell decoy is designed to cause. A row carrying a
// floor must clear it before any byte comparison is allowed to run.
func sourcesVerdict(row sourceDoc, obs sourcesProbeObs) sourceProbe {
	if obs.TransportErr != "" {
		return sourceProbe{
			Verdict: "not_probed",
			NotProbedReason: "transport failure, so nothing was observed: " + obs.TransportErr +
				". This is not evidence the document is gone.",
		}
	}
	if !obs.ok() {
		p := sourceProbe{HTTPStatus: intp(obs.Status)}
		if obs.BodyExact {
			p.BodyBytes = intp(obs.BodyBytes)
		}
		if row.HTTPStatusAsOf != nil {
			p.Verdict = "still_unreachable"
			// A row catalogued as 404 that now answers 403, or vice versa, is a
			// change worth seeing even though both are "unreachable".
			p.Drift = *row.HTTPStatusAsOf != obs.Status
			return p
		}
		p.Verdict = "disappeared"
		p.Drift = true
		return p
	}

	bytesNow := obs.Bytes
	p := sourceProbe{BytesNow: &bytesNow}
	baseline, which := sourcesBaselineFor(row)
	p.BaselineUsed = which
	p.BaselineBytes = baseline
	p.BaselineNote = sourcesBaselineNote(row, which)

	if row.ByteFloor > 0 && bytesNow < row.ByteFloor {
		// The floor is checked BEFORE any byte comparison and it is absolute:
		// a 200 under the floor is a decoy whatever the baseline says. Without
		// this ordering, 9,838 bytes on a generation row reads as "reachable",
		// which is exactly the mistake the shell decoy is built to cause.
		p.Verdict = "decoy"
		p.Drift = true
		if baseline != nil {
			d := bytesNow - *baseline
			p.DeltaBytes = &d
		}
		return p
	}
	if row.State == "decoy" {
		p.Verdict = "decoy"
		if baseline != nil {
			d := bytesNow - *baseline
			p.DeltaBytes = &d
			p.Drift = d != 0
		}
		return p
	}
	if baseline == nil {
		// Either nothing was ever measured, or the row was catalogued as
		// unreachable and is now serving. Both are "appeared" and both are
		// drift.
		p.Verdict = "appeared"
		p.Drift = true
		return p
	}
	d := bytesNow - *baseline
	p.DeltaBytes = &d
	switch {
	case d == 0:
		p.Verdict = "unchanged"
	case d > 0:
		p.Verdict = "grew"
		p.Drift = true
	default:
		p.Verdict = "shrank"
		p.Drift = true
	}
	return p
}

// sourcesBaselineFor picks the baseline a live probe is comparable with, and
// names it.
//
// A byte count is a property of the REQUEST as much as of the document here:
// every text/html body arrives 361 bytes larger through this client than
// through curl, measured unanimously across 16 rows. So a probe made by this
// client is compared against what this client measured, and only falls back to
// the Content-Length baseline when there is nothing else — where the fallback
// is named in the output so a +361 is readable as an instrument difference
// rather than a change to the document.
// sourcesBaselineNote warns when a comparison crosses instruments.
func sourcesBaselineNote(row sourceDoc, which string) string {
	if which != "content-length" || !sourcesHTMLContentKinds[row.ContentKind] {
		return ""
	}
	return "compared against a Content-Length baseline taken with a different client. Every text/html " +
		"body on this site arrives " + itoa(sourcesCloudflareHTMLInjectionBytes) +
		" bytes larger through this client (Cloudflare appends its email-decode script when the request " +
		"Accept asks for HTML), so a delta of exactly +" + itoa(sourcesCloudflareHTMLInjectionBytes) +
		" here is an instrument difference and not a change to the document."
}

func sourcesBaselineFor(row sourceDoc) (*int, string) {
	if row.ClientBytesAsOf != nil {
		return row.ClientBytesAsOf, "client"
	}
	if row.BytesAsOf != nil {
		return row.BytesAsOf, "content-length"
	}
	return nil, "none"
}

// sourcesCheapContentKinds are the content kinds --diff is allowed to fetch.
// Everything here is HTML-family and gzips to a few KB on the wire.
var sourcesCheapContentKinds = map[string]bool{
	"html-excel-sheet": true,
	"html":             true,
	"xml":              true,
	"css":              true,
	"php":              true,
	"txt":              true,
}

// sourcesNotProbedReason says why a row was left alone, or "" when it may be
// probed. Every refusal names the missing capability or the owning command
// rather than going quiet.
func sourcesNotProbedReason(row sourceDoc) string {
	if row.Path == "" {
		return "no URL was ever recorded for this document, so there is nothing to probe"
	}
	if row.State == "reference" {
		owner := row.RowsOwnedBy
		if owner == "" {
			owner = "events"
		}
		return "this surface's contents are asserted by `" + owner + "`, not here. Probing it from two " +
			"catalogues would create two claims about one URL, which is the drift this command exists " +
			"to prevent."
	}
	if row.ContentKind == "pdf" {
		return "pdf: this client has no HEAD and no Range, and ProbeGet issues a full GET and discards " +
			"the body, so existence cannot be checked without downloading the document. Use --fetch for " +
			"one row under the " + itoa(sourcesFetchByteCeiling>>20) + " MiB ceiling."
	}
	if !sourcesCheapContentKinds[row.ContentKind] {
		return "content kind " + quoteArg(row.ContentKind) + " has no cheap existence check in this client"
	}
	if !row.PathEncodable {
		return "the recorded path is not encodable verbatim, and this build will not guess an encoding"
	}
	return ""
}

// sourcesFetcher is the one client capability this leg needs. It exists so the
// probe loop is drivable in a test without a live site.
type sourcesFetcher interface {
	GetWithHeadersNoCache(ctx context.Context, path string, params map[string]string, headers map[string]string) ([]byte, error)
}

// sourcesClientFetcher adapts *client.Client, whose GetWithHeadersNoCache
// returns json.RawMessage.
type sourcesClientFetcher struct{ c *client.Client }

func (f sourcesClientFetcher) GetWithHeadersNoCache(ctx context.Context, path string, params, headers map[string]string) ([]byte, error) {
	body, err := f.c.GetWithHeadersNoCache(ctx, path, params, headers)
	return body, err
}

// sourcesObserve makes ONE probe request and reduces the outcome to an
// observation. It NEVER returns an error: an HTTP failure is the product.
//
// The HTML response header is mandatory. Without it client.go:1266-1269 runs
// summarizeHTMLDocument over a successful HTML body and converts it into an
// error, which would turn every one of these probes into a failure.
func sourcesObserve(ctx context.Context, f sourcesFetcher, path string) sourcesProbeObs {
	body, err := f.GetWithHeadersNoCache(ctx, path, nil,
		map[string]string{client.HTMLResponseHeader: "true"})
	if err == nil {
		return sourcesProbeObs{Bytes: len(body)}
	}
	var ae *client.APIError
	if errors.As(err, &ae) {
		n := len(ae.Body)
		// The client truncates an error body at maxErrorBodyBytes (4,096). Under
		// that cap the length is exact; at or above it, it is a floor and must
		// not be reported as a measurement.
		return sourcesProbeObs{
			Status:    ae.StatusCode,
			BodyBytes: n,
			BodyExact: n < 4096,
		}
	}
	return sourcesProbeObs{TransportErr: err.Error()}
}

// sourcesProbeAll re-measures every cheap row plus the enumerators.
func sourcesProbeAll(ctx context.Context, f sourcesFetcher, docs []sourceDoc, enums []sourceEnum) ([]sourceDoc, []sourceEnum, int, int) {
	probed, drifted := 0, 0
	outDocs := make([]sourceDoc, len(docs))
	copy(outDocs, docs)
	for i := range outDocs {
		if reason := sourcesNotProbedReason(outDocs[i]); reason != "" {
			outDocs[i].Probe = &sourceProbe{Verdict: "not_probed", NotProbedReason: reason}
			continue
		}
		p := sourcesVerdict(outDocs[i], sourcesObserve(ctx, f, outDocs[i].Path))
		outDocs[i].Probe = &p
		probed++
		if p.Drift {
			drifted++
		}
	}
	outEnums := make([]sourceEnum, len(enums))
	copy(outEnums, enums)
	for i := range outEnums {
		row := sourceDoc{
			ID: outEnums[i].ID, State: outEnums[i].State, Path: outEnums[i].Path,
			ContentKind: "txt", BytesAsOf: outEnums[i].BytesAsOf,
			ClientBytesAsOf: outEnums[i].ClientBytesAsOf,
			HTTPStatusAsOf:  outEnums[i].HTTPStatusAsOf, PathEncodable: true,
		}
		p := sourcesVerdict(row, sourcesObserve(ctx, f, outEnums[i].Path))
		outEnums[i].Probe = &p
		probed++
		if p.Drift {
			drifted++
		}
	}
	return outDocs, outEnums, probed, drifted
}

// --- the enumerator leg -------------------------------------------------

// sourcesIndexPage is an index whose href set is diffed against the catalogue.
type sourcesIndexPage struct {
	ID   string
	Path string
	// HrefsAsOf is the MEASURED count of document hrefs under Filter.
	HrefsAsOf int
	// IndexOnlyAsOf is how many of those hrefs the catalogue did NOT hold when
	// this baseline was taken. It is not zero for any of the three pages, and
	// pretending it were would make every run report drift: the PER index links
	// 33 documents outside this build's scope (HSE, GENCO, NTDC and
	// transmission reports), the SIR index two unrelated PDFs, and Main.htm the
	// five workbook SHELLS whose payloads are what this catalogue holds. Drift
	// is a MOVE away from these numbers, which is what a genuinely new upstream
	// document looks like.
	IndexOnlyAsOf int
	// Filter is "all" or "pdf".
	Filter string
	// DiffKinds are the kinds whose catalogued paths this index is expected to
	// name. Empty disables the catalogue_only direction — see the Main.htm note.
	DiffKinds []string
	Note      string
}

// sourcesIndexPages is the three index pages that exist. There is no fourth:
// both sitemaps and /sitemap.xml are 404, and the workbook directory is 403.
var sourcesIndexPages = []sourcesIndexPage{
	{
		ID: "gen-main-index", Path: sourcesMainPat, HrefsAsOf: 10, IndexOnlyAsOf: 9, Filter: "all",
		DiffKinds: nil,
		Note: "Main.htm links workbook SHELLS while this catalogue holds the sheet001 PAYLOADS, so a " +
			"path-set diff is only meaningful in the index_only direction. Its 10 hrefs are five " +
			"generation years FY2017-18..FY2021-22, Main_files/filelist.xml and four surfaces; it names " +
			"neither FY2022-23, FY2023-24, nor either SIR Data stub, all of which are reachable.",
	},
	{
		ID: "enum-per-index", Path: sourcesPERPath, HrefsAsOf: 42, IndexOnlyAsOf: 33, Filter: "pdf",
		DiffKinds: []string{"per"},
		Note: "the ONLY enumerator for the PER family, because /M&E/PER/Distribution/ answers 403. It " +
			"links 42 pdf refs across six directory conventions, 41 of them distinct, and it links the " +
			"FY2023-24 report that 404s.",
	},
	{
		ID: "enum-sir-index", Path: sourcesSIRPath, HrefsAsOf: 46, IndexOnlyAsOf: 2, Filter: "pdf",
		DiffKinds: []string{"sir"},
		Note: "links all 22 years twice each with RAW spaces in the href, plus two unrelated PDFs. " +
			"Every sir row's path was harvested from here.",
	},
}

// sourcesEnumDiff is one index page's href set against the catalogue.
type sourcesEnumDiff struct {
	Enumerator string `json:"enumerator"`
	Path       string `json:"path"`
	// State is probed | not_probed.
	State string `json:"state"`
	// HrefsNow is the count under the page's filter; HrefsAsOf the measured
	// baseline. HrefsNow is nil when the page could not be read.
	HrefsNow  *int `json:"hrefs_now"`
	HrefsAsOf int  `json:"hrefs_as_of"`
	// IndexOnly is an href the catalogue does not hold: a document NEPRA has
	// published and this build does not know about. It is the most valuable
	// output this command has, and it is NEVER an error.
	IndexOnly []harvestedHref `json:"index_only"`
	// IndexOnlyAsOf is the measured baseline for len(index_only).
	IndexOnlyAsOf int `json:"index_only_as_of"`
	// CatalogueOnly is a catalogued path the index no longer names.
	CatalogueOnly []string `json:"catalogue_only"`
	// NotEncodable is an href that cannot be turned into a request path without
	// guessing. Reported rather than repaired.
	NotEncodable []harvestedHref `json:"not_encodable,omitempty"`
	OffHost      int             `json:"off_host_hrefs,omitempty"`
	// Duplicates is how many of the page's document hrefs were repeats. The SIR
	// index links every year twice, so a raw href count is not a document
	// count.
	Duplicates int    `json:"duplicate_hrefs"`
	HTTPStatus *int   `json:"http_status,omitempty"`
	Error      string `json:"error,omitempty"`
	Drift      bool   `json:"drift"`
	Note       string `json:"note,omitempty"`
}

// sourcesRowIsIndexable reports whether an index page ought to name this row.
// A decoy is excluded because NEPRA's own index does not link either SIR Data
// stub, and a reference row is excluded because another catalogue owns it.
func sourcesRowIsIndexable(row sourceDoc) bool {
	switch row.State {
	case "published", "reachable", "indexed_unmeasured", "unavailable":
		return row.Path != ""
	}
	return false
}

// sourcesCataloguePathSet is every path this build knows about, across all
// kinds plus the enumerators. index_only is computed against this whole set so
// a document already catalogued under another kind is not reported as new.
func sourcesCataloguePathSet(docs []sourceDoc) map[string]bool {
	set := map[string]bool{}
	for _, d := range docs {
		if d.Path != "" {
			set[d.Path] = true
		}
	}
	for _, e := range sourcesEnumerators {
		set[e.Path] = true
	}
	for _, p := range sourcesIndexPages {
		set[p.Path] = true
	}
	return set
}

// sourcesDiffIndex reads one index page and diffs its href set.
// sourcesDiffIndex reads one index page and diffs its href set.
//
// The two directions use DIFFERENT universes, deliberately. index_only is
// computed against the WHOLE catalogue (all five kinds), because "NEPRA
// publishes a document this build does not know about" is a fact about the
// build, not about the caller's --kind filter — computing it against a filtered
// subset made Main.htm report the FCA shell as new upstream under --kind gen,
// which was wrong. catalogue_only is computed against the filtered rows of the
// kinds this index is expected to name.
func sourcesDiffIndex(ctx context.Context, f sourcesFetcher, page sourcesIndexPage, docs, universe []sourceDoc) sourcesEnumDiff {
	d := sourcesEnumDiff{
		Enumerator: page.ID, Path: page.Path, HrefsAsOf: page.HrefsAsOf,
		Note: page.Note,
		// json.Marshal renders a nil slice as null; these two fields are
		// answers, so they start as empty arrays.
		IndexOnly: []harvestedHref{}, CatalogueOnly: []string{},
	}
	body, err := f.GetWithHeadersNoCache(ctx, page.Path, nil,
		map[string]string{client.HTMLResponseHeader: "true"})
	if err != nil {
		d.State = "not_probed"
		var ae *client.APIError
		if errors.As(err, &ae) {
			d.HTTPStatus = intp(ae.StatusCode)
		}
		d.Error = err.Error()
		d.Drift = true
		return d
	}
	hrefs, herr := sourcesHarvestHrefs(body, page.Path)
	if herr != nil {
		d.State = "not_probed"
		d.Error = herr.Error()
		d.Drift = true
		return d
	}
	d.State = "probed"

	known := sourcesCataloguePathSet(universe)
	seen := map[string]bool{}
	count := 0
	for _, h := range hrefs {
		if page.Filter == "pdf" && !sourcesHrefIsPDF(h) {
			continue
		}
		count++
		if h.OffHost {
			d.OffHost++
			continue
		}
		if !h.PathEncodable {
			d.NotEncodable = append(d.NotEncodable, h)
			continue
		}
		if seen[h.PathEncoded] {
			d.Duplicates++
			continue
		}
		seen[h.PathEncoded] = true
		if !known[h.PathEncoded] {
			d.IndexOnly = append(d.IndexOnly, h)
		}
	}
	d.HrefsNow = &count

	for _, kind := range page.DiffKinds {
		for _, row := range docs {
			if row.Kind != kind || !sourcesRowIsIndexable(row) {
				continue
			}
			if !seen[row.Path] {
				d.CatalogueOnly = append(d.CatalogueOnly, row.Path)
			}
		}
	}
	sort.Strings(d.CatalogueOnly)
	sort.SliceStable(d.IndexOnly, func(i, j int) bool {
		return d.IndexOnly[i].PathEncoded < d.IndexOnly[j].PathEncoded
	})

	// Drift is a MOVE: the href count changed, the index dropped a catalogued
	// path, or the number of documents it names that this build does not hold
	// changed. A non-empty index_only is NOT itself a failure — it is the
	// command working — which is why it is compared against its measured
	// baseline rather than against zero.
	d.IndexOnlyAsOf = page.IndexOnlyAsOf
	d.Drift = count != page.HrefsAsOf ||
		len(d.CatalogueOnly) > 0 ||
		len(d.IndexOnly) != page.IndexOnlyAsOf
	return d
}

// sourcesDiffIndexes runs the enumerator leg over every index page.
func sourcesDiffIndexes(ctx context.Context, f sourcesFetcher, docs, universe []sourceDoc, kind string) []sourcesEnumDiff {
	out := make([]sourcesEnumDiff, 0, len(sourcesIndexPages))
	for _, p := range sourcesIndexPages {
		if kind != "" && !sourcesIndexPageCovers(p, kind) {
			continue
		}
		out = append(out, sourcesDiffIndex(ctx, f, p, docs, universe))
	}
	return out
}

// sourcesIndexPageCovers reports whether an index page is relevant to a --kind
// filter. Main.htm covers gen and fca because it links both.
func sourcesIndexPageCovers(p sourcesIndexPage, kind string) bool {
	if p.ID == "gen-main-index" {
		return kind == "gen" || kind == "fca"
	}
	for _, k := range p.DiffKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// sourcesDriftSummary is the one-line human verdict.
func sourcesDriftSummary(probed, drifted int, diffs []sourcesEnumDiff) string {
	var b strings.Builder
	b.WriteString(itoa(probed) + " rows probed, " + itoa(drifted) + " drifted")
	for _, d := range diffs {
		b.WriteString("; " + d.Enumerator + " ")
		if d.State != "probed" {
			b.WriteString("unread")
			continue
		}
		n := 0
		if d.HrefsNow != nil {
			n = *d.HrefsNow
		}
		b.WriteString(itoa(n) + "/" + itoa(d.HrefsAsOf) + " hrefs, " +
			itoa(len(d.IndexOnly)) + " new upstream, " + itoa(len(d.CatalogueOnly)) + " dropped")
	}
	return b.String()
}

// sourcesEnumDiffDrifted counts the index pages whose diff is drift.
func sourcesEnumDiffDrifted(diffs []sourcesEnumDiff) int {
	n := 0
	for _, d := range diffs {
		if d.Drift {
			n++
		}
	}
	return n
}
