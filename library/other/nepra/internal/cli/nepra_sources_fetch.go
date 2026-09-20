// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-sources-catalogue.json.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// --fetch <id>: retrieve exactly one catalogued document and MEASURE it against
// the shipped baseline.
//
// This is what turns the catalogue from a claim into a checkable one. For a PDF
// it re-reads the page count off the page tree (file(1) reports "1 pages" for
// six of the seven PERs and is simply wrong), the /Creator and /Producer out of
// the info dictionary, and the text layer through nepraper.ExtractText.
//
// WHAT IT ASSERTS AND WHAT IT ONLY REPORTS. bytes, pages, creator and
// low_text_pages are compared against the baseline. char_count is REPORTED AND
// NEVER ASSERTED: nepraper.ExtractText does its own line grouping and gluing, so
// its character counts are not expected to equal the pymupdf figures the survey
// recorded (39,416-82,813), and nepraper.SourceRecord deliberately stores
// HasTextLayer and LowTextPages but NO character count — the package authors
// reached the same conclusion. Pinning one extractor's count as an equality
// against another's would manufacture a failure.

// sourcesFetchResult is the single-row payload --fetch returns.
type sourcesFetchResult struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	URL  string `json:"url"`
	// State is measured | refused_not_pdf.
	State    string             `json:"state"`
	Artifact nepraArtifact      `json:"artifact"`
	Measured sourcesMeasurement `json:"measured"`
	// CharCountNote travels with the payload so a consumer cannot mistake a
	// reported number for an asserted one.
	CharCountNote string `json:"char_count_note,omitempty"`
	// RefusalReason is set when the body was not the document it claimed to be.
	RefusalReason string `json:"refusal_reason,omitempty"`
	// BodyPrefix is the first 16 bytes of a refused body, quoted. The body
	// itself is NEVER emitted as document content: NEPRA answers a missing PDF
	// with a nine-byte HTML error page, and passing that on as a document is
	// how a 404 becomes a fact.
	BodyPrefix string `json:"body_prefix,omitempty"`
	Baseline   *int   `json:"baseline_bytes_as_of"`
}

// sourcesMeasurement is what this fetch actually observed.
type sourcesMeasurement struct {
	// Bytes is the DECODED document length: for a PDF, the length after the
	// base64 envelope is unwrapped, never the envelope's own size.
	Bytes int `json:"bytes"`
	// The PDF fields are nil for a non-PDF row rather than zero.
	Pages        *int    `json:"pages"`
	Creator      *string `json:"creator"`
	Producer     *string `json:"producer"`
	CharCount    *int    `json:"char_count"`
	LowTextPages []int   `json:"low_text_pages"`
	TextLayer    string  `json:"text_layer"`
	ContentType  string  `json:"content_type,omitempty"`
	// BaselineUsed names which baseline matches_baseline was computed against:
	// "client", "content-length" or "none". See sourcesBaselineFor.
	BaselineUsed string    `json:"baseline_used,omitempty"`
	BaselineNote string    `json:"baseline_note,omitempty"`
	Matches      *matchSet `json:"matches_baseline,omitempty"`
}

// matchSet reports agreement with the baseline. A nil field means the baseline
// held no value to compare, which is not the same as a mismatch.
type matchSet struct {
	Bytes        *bool `json:"bytes"`
	Pages        *bool `json:"pages"`
	Creator      *bool `json:"creator"`
	LowTextPages *bool `json:"low_text_pages"`
}

func boolp(v bool) *bool { return &v }

// sourcesFetchIDs lists the fetchable ids for a kind, for the refusal message.
func sourcesFetchIDs(kind string) []string {
	docs := sourcesDocsForKind(kind)
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.ID)
	}
	sort.Strings(out)
	return out
}

// sourcesValidateFetch is every refusal --fetch makes BEFORE any request. Each
// one names the measurement that justifies it, because a refusal without a
// reason is indistinguishable from a missing feature.
func sourcesValidateFetch(row sourceDoc) error {
	if row.Path == "" {
		return fmt.Errorf("%s has no recorded URL, so there is nothing to fetch. "+
			"Documents whose URL was never captured are declared by `doctor --scope`, not invented here", row.ID)
	}
	if row.State == "reference" {
		owner := row.RowsOwnedBy
		if owner == "" {
			owner = "events"
		}
		return fmt.Errorf("%s is a REFERENCE row: its contents are asserted by `%s`, which owns the row "+
			"floor for that surface. Fetching it from here would create two catalogues claiming one URL", row.ID, owner)
	}
	if !row.PathEncodable {
		return fmt.Errorf("%s carries a path this build cannot encode verbatim, and guessing an encoding "+
			"would produce a 404 indistinguishable from a document NEPRA never published", row.ID)
	}
	if row.State == "indexed_unmeasured" {
		return fmt.Errorf("%s is on an index's run and has never been probed, so its size is unknown and "+
			"the cost of fetching it cannot be bounded. The ten SIR years that WERE measured span "+
			"%s B to %s B at %s-%s B/s. This client has no HEAD and no Range, so there is no cheap way to "+
			"find out first; that is a declared gap, not a refusal to try",
			row.ID, commaBytes(3425763), commaBytes(331853390),
			commaBytes(sourcesThroughputMinBPS), commaBytes(sourcesThroughputMaxBPS))
	}
	if row.BytesAsOf != nil && *row.BytesAsOf > sourcesFetchByteCeiling {
		return fmt.Errorf("%s is %s B as of %s, above the %d MiB --fetch ceiling. At the measured "+
			"%s-%s B/s that is %s, and this client buffers the whole body then base64-wraps it. "+
			"Fetch it with a tool that streams",
			row.ID, commaBytes(*row.BytesAsOf), row.AsOfDate, sourcesFetchByteCeiling>>20,
			commaBytes(sourcesThroughputMinBPS), commaBytes(sourcesThroughputMaxBPS),
			sourcesETAWindow(*row.BytesAsOf))
	}
	return nil
}

// sourcesMeasureBody turns a fetched body into a measurement.
//
// raw is the DECODED document bytes — for a PDF, already unwrapped from the
// client's base64 envelope, so measured.bytes is the document's length and not
// the envelope's.
func sourcesMeasureBody(row sourceDoc, raw []byte, contentType string) (sourcesMeasurement, string, string) {
	// The baseline a fetch is compared against is chosen the same way --diff
	// chooses one, and for the same reason: an HTML body measured by curl and
	// the same body measured by this client differ by 361 bytes, so comparing
	// across instruments would report a mismatch on every html row forever.
	baseline, which := sourcesBaselineFor(row)
	m := sourcesMeasurement{
		Bytes:        len(raw),
		TextLayer:    "not-applicable",
		ContentType:  contentType,
		BaselineUsed: which,
		BaselineNote: sourcesBaselineNote(row, which),
	}
	if row.ContentKind != "pdf" {
		m.TextLayer = "not-applicable"
		m.Matches = &matchSet{Bytes: sourcesMatchInt(baseline, len(raw))}
		return m, "", ""
	}

	doc, err := nepraper.ExtractText(raw)
	if err != nil {
		if errors.Is(err, nepraper.ErrNotPDF) {
			// NEPRA answers a missing PDF with a nine-byte text/html body. It
			// must never reach the caller as document content.
			return sourcesMeasurement{
					Bytes: len(raw), TextLayer: "unmeasured", ContentType: contentType,
					BaselineUsed: which, Matches: &matchSet{Bytes: sourcesMatchInt(baseline, len(raw))},
				},
				"refused_not_pdf",
				"the body does not begin with a %PDF header, so it is not the document this row " +
					"describes. Its bytes are NOT reported as document content: " + err.Error()
		}
		return sourcesMeasurement{
				Bytes: len(raw), TextLayer: "unmeasured", ContentType: contentType,
				BaselineUsed: which, Matches: &matchSet{Bytes: sourcesMatchInt(baseline, len(raw))},
			},
			"extract_failed",
			"the bytes arrived but could not be read as a PDF: " + err.Error()
	}

	pages := doc.NumPages
	creator := doc.Creator
	producer := doc.Producer
	chars := doc.CharCount()
	low := doc.LowTextPages(20)
	if low == nil {
		low = []int{}
	}
	m.Pages = &pages
	m.Creator = &creator
	m.Producer = &producer
	m.CharCount = &chars
	m.LowTextPages = low
	if chars > 0 {
		m.TextLayer = "present"
	} else {
		// Zero extractable characters over a real page tree is an image-only
		// scan. Recorded as measured-absent, which is a stronger statement than
		// "unmeasured" and is only ever made here, after actually extracting.
		m.TextLayer = "absent"
	}
	m.Matches = &matchSet{
		Bytes:        sourcesMatchInt(baseline, len(raw)),
		Pages:        sourcesMatchInt(row.PagesAsOf, pages),
		Creator:      sourcesMatchStr(row.CreatorAsOf, creator),
		LowTextPages: sourcesMatchInts(row.LowTextPagesAsOf, low, row.PagesAsOf != nil),
	}
	return m, "", ""
}

func sourcesMatchInt(baseline *int, got int) *bool {
	if baseline == nil {
		return nil
	}
	return boolp(*baseline == got)
}

func sourcesMatchStr(baseline *string, got string) *bool {
	if baseline == nil {
		return nil
	}
	return boolp(*baseline == got)
}

// sourcesMatchInts compares the low-text page list. baselineKnown says whether
// the baseline was ever extracted at all: without it, an empty baseline list
// and an unextracted one look identical.
func sourcesMatchInts(baseline, got []int, baselineKnown bool) *bool {
	if !baselineKnown {
		return nil
	}
	if len(baseline) != len(got) {
		return boolp(false)
	}
	for i := range baseline {
		if baseline[i] != got[i] {
			return boolp(false)
		}
	}
	return boolp(true)
}

const sourcesCharCountNote = "reported, never asserted: this is nepraper's own line-grouping extractor, " +
	"not the pymupdf the survey used, so the two character counts are not expected to be equal and no " +
	"test pins this number."

// sourcesBodyPrefix quotes at most the first 16 bytes of a body, for a refusal
// message. It is the only path by which a refused body's bytes are ever shown,
// and 16 bytes cannot carry a document.
func sourcesBodyPrefix(raw []byte) string {
	if len(raw) > 16 {
		raw = raw[:16]
	}
	return fmt.Sprintf("%q", string(raw))
}

// sourcesFetchDriftMessage describes a baseline disagreement for --strict.
func sourcesFetchDriftMessage(res sourcesFetchResult) string {
	if res.Measured.Matches == nil {
		return ""
	}
	var bad []string
	m := res.Measured.Matches
	if m.Bytes != nil && !*m.Bytes {
		base := "unmeasured"
		if res.Baseline != nil {
			base = commaBytes(*res.Baseline)
		}
		bad = append(bad, "bytes "+commaBytes(res.Measured.Bytes)+" against a baseline of "+base)
	}
	if m.Pages != nil && !*m.Pages {
		bad = append(bad, "page count")
	}
	if m.Creator != nil && !*m.Creator {
		bad = append(bad, "/Creator")
	}
	if m.LowTextPages != nil && !*m.LowTextPages {
		bad = append(bad, "low-text pages")
	}
	if len(bad) == 0 {
		return ""
	}
	return res.ID + " disagrees with its baseline on " + strings.Join(bad, ", ")
}

// sourcesMarshalMeasurement is the canonical content bytes for the artifact
// hash. Hashing the MEASUREMENT rather than the response is deliberate: two
// identical fetches of one NEPRA page seconds apart returned the same 74,431
// bytes with different SHA-256s, because Cloudflare's email-obfuscation filter
// rewrites every data-cfemail token per response. sha256_raw records what this
// fetch received; sha256_content is the one that is comparable.
func sourcesMarshalMeasurement(m sourcesMeasurement) ([]byte, error) {
	return json.Marshal(m)
}
