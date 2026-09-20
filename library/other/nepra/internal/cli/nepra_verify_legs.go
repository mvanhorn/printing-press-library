// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// Patch record to be written at .printing-press-patches/nepra-verify-fetch-gate.json;
// it is not in this change because the writable set for this task is source only.

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// Leg verdicts. "reported" is a measurement this gate deliberately refuses to
// turn into a pass or a fail, and it never changes the exit code.
const (
	verifyPass     = "pass"
	verifyFail     = "fail"
	verifyReported = "reported"
	verifySkipped  = "skipped"
)

// gateErr is exit code 6: a catalogued surface answered, and what it answered
// is not what this manifest says it must be.
//
// It is deliberately its OWN code rather than an overload of 5 (api) or 3
// (not-found). A cron's whole interface to this command is the exit status,
// and "NEPRA is down" (5), "NEPRA never published this" (3) and "NEPRA served
// something that failed the schema gate" (6) demand three different human
// responses. Codes 2/3/4/5/7/10 are already taken; 6, 8 and 9 are free.
//
// NOTE FOR THE OWNER: 6 is a proposal, not an established convention. Confirm
// it before a cron gates on it, because changing it later breaks every caller.
func gateErr(err error) error { return &cliError{code: 6, err: err} }

// verifyLeg is one independent check.
//
// Facts is a map rather than a struct with a field per leg so that an
// unmeasured number is ABSENT from the payload rather than serialised as 0.
// Nothing writes a key it did not measure; a key present with a nil value is
// a deliberate statement that the value is unobservable (status_exact on a
// success is the only such case today).
type verifyLeg struct {
	Leg     string         `json:"leg"`
	Verdict string         `json:"verdict"`
	Note    string         `json:"note,omitempty"`
	Facts   map[string]any `json:"facts,omitempty"`
}

func leg(name, verdict, note string, facts map[string]any) verifyLeg {
	return verifyLeg{Leg: name, Verdict: verdict, Note: note, Facts: facts}
}

// verifySurfaceResult is one surface's whole gate outcome.
//
// Every field is emitted unconditionally, including the nullable ones. That is
// not verbosity: --compact / --agent projects a list of objects through a
// keys-present-in-80%-of-rows rule, so a key that appears on only some rows
// (fy, which only workbook surfaces have) would be SILENTLY DROPPED from an
// 11-surface --all run. An explicit null survives; an omitted key does not.
type verifySurfaceResult struct {
	Surface string  `json:"surface"`
	Kind    string  `json:"kind"`
	Label   string  `json:"label"`
	FY      *string `json:"fy"`
	URL     string  `json:"url"`
	Verdict string  `json:"verdict"`
	// Asserted is false when this surface's internals were never measured,
	// which makes its structural and invariant legs report-only.
	Asserted bool        `json:"asserted"`
	Legs     []verifyLeg `json:"legs"`
	// MeasuredButNotAsserted carries the internals of a surface whose
	// internals have never been observed, plus the reason they cannot fail
	// the gate. Null for an asserted surface.
	MeasuredButNotAsserted map[string]any `json:"measured_but_not_asserted"`
	// Artifact is the provenance of the bytes these legs ran over. Null when
	// no body was received.
	Artifact *nepraArtifact `json:"artifact"`

	// sigma is the Sum-GWh total of this surface, kept unexported so
	// --crosscheck can report a number that came from the bytes this run
	// actually fetched rather than from a second parse of a second fetch.
	sigma *verifySigmaResult
}

func (r *verifySurfaceResult) add(l verifyLeg) {
	r.Legs = append(r.Legs, l)
	if l.Verdict == verifyFail {
		r.Verdict = verifyFail
	}
}

func (r *verifySurfaceResult) skipRest(reason string, names ...string) {
	for _, n := range names {
		r.Legs = append(r.Legs, leg(n, verifySkipped, reason, nil))
	}
}

func newVerifySurfaceResult(s verifySurface, url string) verifySurfaceResult {
	res := verifySurfaceResult{
		Surface: s.ID, Kind: s.Kind, Label: s.Label, URL: url,
		Verdict: verifyPass, Asserted: s.Asserted,
	}
	if s.FY != "" {
		fy := s.FY
		res.FY = &fy
	}
	return res
}

// verifyFetchResult is one fetch and everything observable about it.
type verifyFetchResult struct {
	Body []byte
	// BodyIsErrorCapture is true when Body came from the transport's typed
	// error rather than from a successful read. That body has been through
	// the client's own error-body handling, which TRUNCATES a long body and
	// SUMMARISES an HTML one, so its length is not necessarily the length of
	// the response. It is still exact for the case this gate cares about:
	// NEPRA's 9-byte stub is far below either threshold and arrives verbatim.
	BodyIsErrorCapture bool
	// StatusExact is the HTTP status code, and it is non-nil ONLY on a
	// failure. The generated client discards the status on success (it
	// returns the body alone), so recording 200 would be asserting a value
	// this process never saw. See nepra_artifact_provenance.go for the same
	// discipline applied to stored provenance.
	StatusExact *int
	ContentType string
	Err         error
}

// verifyFetch performs one uncached GET.
//
// The NoCache variant is mandatory. c.GetWithHeaders reads the response cache
// first, while newNepraArtifact stamps fetched_at = time.Now() unconditionally
// — so a warm cache would make this gate report a fresh verdict, with a fresh
// timestamp, over bytes it never fetched. `source: live` in the output is only
// true because of this call.
func verifyFetch(ctx context.Context, c *client.Client, path string) verifyFetchResult {
	body, err := c.GetWithHeadersNoCache(ctx, path, nil,
		map[string]string{client.HTMLResponseHeader: "true"})
	out := verifyFetchResult{ContentType: c.LastContentType()}
	if err != nil {
		out.Err = err
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			code := apiErr.StatusCode
			out.StatusExact = &code
			// The 404 body is the evidence, and it is only 9 bytes.
			out.Body = []byte(apiErr.Body)
			out.BodyIsErrorCapture = true
		}
		return out
	}
	out.Body = []byte(body)
	return out
}

// verifyStatusLeg records what the fetch answered.
func verifyStatusLeg(f verifyFetchResult) verifyLeg {
	if f.Err == nil {
		return leg("status", verifyPass,
			"status_exact is null on purpose: this client returns the body without the status code on "+
				"success, so any number here would be a value never observed.",
			map[string]any{"status_class": "2xx-3xx", "status_exact": nil})
	}
	facts := map[string]any{"status_class": "error", "status_exact": nil, "error": f.Err.Error()}
	if f.StatusExact != nil {
		facts["status_class"] = fmt.Sprintf("%dxx", *f.StatusExact/100)
		facts["status_exact"] = *f.StatusExact
	}
	return leg("status", verifyFail, "", facts)
}

// verifyByteFloorLeg checks the decoded body against the per-surface floor.
func verifyByteFloorLeg(s verifySurface, n int) verifyLeg {
	facts := map[string]any{
		"floor": s.ByteFloor, "observed": n, "floor_measured_on": s.ByteFloorAsOf,
	}
	note := "A FLOOR, not an equality, for two independent reasons. NEPRA may add rows; and the " +
		"body length depends on the REQUEST, because Cloudflare injects a RUM <script> before " +
		"</body> when Accept advertises text/html. MEASURED on FY2023-24: 493,187 bytes under " +
		"Accept: */* (byte-identical to the committed fixture) against 493,548 under this " +
		"client's own Accept — so above_floor_by of roughly 361 is injected furniture, not new " +
		"data. The floor is per-surface because the frameset decoy is byte-identical across " +
		"years, so one global floor would let a shell served for an unfloored year through."
	if n < s.ByteFloor {
		return leg("byte_floor", verifyFail, note, facts)
	}
	if n > s.ByteFloor {
		facts["above_floor_by"] = n - s.ByteFloor
	}
	return leg("byte_floor", verifyPass, note, facts)
}

// verify404StubBytes is the length of NEPRA's 404 body. A body this short is
// never data on any surface in this manifest.
const verify404StubBytes = 9

// verifyFramesetDecoyBytes is the measured length of the frameset shell.
const verifyFramesetDecoyBytes = 9838

// verify404StubBody is NEPRA's entire 404 response body, verbatim.
const verify404StubBodyText = "Not Found"

func verify404StubLeg(f verifyFetchResult) verifyLeg {
	n := len(f.Body)
	note := "NEPRA's 404 body is 9 bytes, \"Not Found\" (hex 4e6f7420466f756e64), served as " +
		"charset=iso-8859-1. That iso-8859-1 default is the origin of the whole encoding trap."
	facts := map[string]any{"refuse_at_or_below": verify404StubBytes}
	if f.BodyIsErrorCapture {
		// The length here is the TRANSPORT'S CAPTURED error body, which the
		// client truncates and may summarise, so it is reported under a name
		// that cannot be mistaken for the response length.
		facts["error_body_bytes"] = n
		facts["error_body_is_nepra_stub"] = string(f.Body) == verify404StubBodyText
		if string(f.Body) == verify404StubBodyText {
			facts["body_hex"] = hex.EncodeToString(f.Body)
			return leg("decoy_404_stub", verifyFail, note, facts)
		}
		return leg("decoy_404_stub", verifyReported,
			note+" This run's body came from the transport's error capture rather than a read, and "+
				"it is not byte-exactly that stub, so this leg makes no claim about it.", facts)
	}
	facts["observed_bytes"] = n
	if n <= verify404StubBytes {
		facts["body_hex"] = hex.EncodeToString(f.Body)
		return leg("decoy_404_stub", verifyFail, note, facts)
	}
	return leg("decoy_404_stub", verifyPass, note, facts)
}

// verifyFramesetMarkers counts the frameset shell's structural signature.
//
// This check is deliberately INDEPENDENT of the byte floor. The shell returns
// HTTP 200 and is byte-identical across three fiscal years, so a floor tuned
// per year is the only thing that catches a shell served for a year we have no
// floor for — and the structural signature is the only thing that catches a
// shell whose length changed. Neither guard is allowed to carry the case
// alone, so both run and both can fail on their own.
func verifyFramesetMarkers(body []byte) (frameset, frames int) {
	lower := bytes.ToLower(body)
	return bytes.Count(lower, []byte("<frameset")), bytes.Count(lower, []byte("<frame "))
}

func verifyFramesetLeg(n int, body []byte) verifyLeg {
	frameset, frames := verifyFramesetMarkers(body)
	facts := map[string]any{
		"observed_bytes": n, "decoy_bytes": verifyFramesetDecoyBytes,
		"frameset_elements": frameset, "frame_elements": frames,
	}
	note := "The per-year parent .htm is a 9,838-byte <frameset> shell that returns 200 and is " +
		"byte-identical across FY2017-18, FY2020-21 and FY2023-24. This leg refuses on the " +
		"structural signature alone, so it still fires if the shell's length changes."
	if frameset > 0 {
		return leg("decoy_frameset", verifyFail, note, facts)
	}
	if n == verifyFramesetDecoyBytes {
		facts["matched_decoy_length"] = true
		return leg("decoy_frameset", verifyFail, note, facts)
	}
	return leg("decoy_frameset", verifyPass, note, facts)
}

// verifyCharsetLeg proves the payload really is windows-1252 from its bytes.
func verifyCharsetLeg(body []byte) verifyLeg {
	_, charset, declared, err := nepraparse.Decode(body)
	nbsp := bytes.Count(body, []byte{0xA0})
	facts := map[string]any{
		"charset": charset, "declared_in_document": declared, "nbsp_0xa0_bytes": nbsp,
	}
	note := "The HTTP header declares NO charset; only the in-document <meta> does. 0xA0 is the " +
		"only non-ASCII byte in these files, so its count is positive proof from the payload " +
		"that windows-1252 is load-bearing. A naive UTF-8 read of this file reports 0 <tr> and 0 <td>."
	if err != nil {
		facts["error"] = err.Error()
		return leg("charset", verifyFail, note, facts)
	}
	if charset != nepraparse.CharsetWindows1252 {
		return leg("charset", verifyFail, note, facts)
	}
	if !declared {
		// Decode falls back to windows-1252 when nothing is declared. That
		// fallback is correct for this family but it is an ASSUMPTION, so it
		// is reported rather than passed silently.
		return leg("charset", verifyReported,
			note+" This body declared no charset, so windows-1252 was ASSUMED rather than read.", facts)
	}
	return leg("charset", verifyPass, note, facts)
}

// verifyWorkbookLegs runs the generation-workbook gate over one fetched body.
func verifyWorkbookLegs(s verifySurface, url string, f verifyFetchResult) verifySurfaceResult {
	res := newVerifySurfaceResult(s, url)
	res.add(verifyStatusLeg(f))
	n := len(f.Body)

	if f.Err != nil {
		res.add(verify404StubLeg(f))
		res.skipRest("the fetch failed, so nothing downstream was measured",
			"byte_floor", "decoy_frameset", "charset", "year_token",
			"column_fingerprint", "structure", "sum_invariant", "column_order", "content_checksum")
		return res
	}

	res.add(verifyByteFloorLeg(s, n))
	res.add(verify404StubLeg(f))
	res.add(verifyFramesetLeg(n, f.Body))
	res.add(verifyCharsetLeg(f.Body))

	// ONE parse supplies the year token, the column fingerprint and the
	// structure at once, and its three sentinel errors map onto three
	// distinct leg verdicts.
	w, err := nepraparse.ParseWorkbook(f.Body, s.FY)
	if err != nil {
		verifyMapParseError(&res, s, err)
		return res
	}

	res.add(leg("year_token", verifyPass, verifyYearTokenNote, map[string]any{
		"expected": s.ExpectedBandLabel, "observed": w.Header.BandLabel,
		"read_from": "in-document <td colspan=26> band label",
	}))
	res.add(verifyFingerprintLeg(w))
	res.add(verifyStructureLeg(s, w, n, f.Body))
	res.add(verifySumInvariantLeg(s, w))
	res.add(verifyColumnOrderLeg(w))

	content, cerr := json.Marshal(w.Plants)
	if cerr != nil {
		res.add(leg("content_checksum", verifyFail, "", map[string]any{"error": cerr.Error()}))
		return res
	}
	// skipped = table rows read but NOT emitted as plants: the title row,
	// the three header-band rows and the separator rows. It is not the
	// ineligible-row count, which is a property of the DATA and is reported
	// by the sum_invariant leg.
	art := newNepraArtifact(url, f.Body, content, f.ContentType, len(w.Plants), w.TableRows-len(w.Plants))
	res.Artifact = &art
	res.add(verifyContentChecksumLeg(art))

	sigma := verifySumGWh(w)
	res.sigma = &sigma

	if !s.Asserted {
		res.MeasuredButNotAsserted = verifyWorkbookInternals(w, n, f.Body)
	}
	return res
}

const verifyYearTokenNote = "The filename is an untrusted hint: \"SIR Data 2025.htm\" returns 200 and " +
	"serves FY2023-24. The year is read from the in-document band label, which is the only place " +
	"the file states which year it holds, and a disagreement is a refusal."

// verifyMapParseError turns nepraparse's sentinels into distinct leg verdicts.
//
// ErrNoTable is in this set even though the spec named only three sentinels:
// re-measuring all three decoys live showed that every one of them — the
// 9-byte stub, the 9,838-byte frameset shell and the 374-byte SIR stub —
// reaches ParseWorkbook as ErrNoTable, not as a year or schema error. Without
// it a decoy would land in the default branch with no named leg.
func verifyMapParseError(res *verifySurfaceResult, s verifySurface, err error) {
	switch {
	case errors.Is(err, nepraparse.ErrFiscalYearMismatch):
		res.add(leg("year_token", verifyFail, verifyYearTokenNote, map[string]any{
			"expected": s.ExpectedBandLabel, "observed": "disagreed",
			"read_from": "in-document <td colspan=26> band label",
			"error":     err.Error(),
		}))
		res.skipRest("the file is not the year that was requested, so its internals were not measured",
			"column_fingerprint", "structure", "sum_invariant", "column_order", "content_checksum")
	case errors.Is(err, nepraparse.ErrSchemaDrift):
		res.add(leg("year_token", verifySkipped, "the header band did not match the schema, so the year was not read", nil))
		res.add(leg("column_fingerprint", verifyFail,
			"Schema drift is fatal on purpose: a shifted or renamed column produces numbers that look "+
				"right. The parser's own diff of the 32 columns is carried in the error.",
			map[string]any{"columns": verifyCanonicalColumnCount, "error": err.Error()}))
		res.skipRest("the schema drifted, so no column could be trusted",
			"structure", "sum_invariant", "column_order", "content_checksum")
	case errors.Is(err, nepraparse.ErrNoHeaderBand), errors.Is(err, nepraparse.ErrNoTable):
		res.add(leg("structure", verifyFail,
			"No three-row header band and/or no table at all. Every catalogued decoy lands here: "+
				"the 9-byte stub, the 9,838-byte frameset shell and the 374-byte SIR stub all parse "+
				"to zero tables.",
			map[string]any{"error": err.Error()}))
		res.skipRest("there is no table to measure",
			"year_token", "column_fingerprint", "sum_invariant", "column_order", "content_checksum")
	default:
		res.add(leg("structure", verifyFail, "the workbook parse failed for a reason this gate does not model", map[string]any{"error": err.Error()}))
		res.skipRest("the parse failed",
			"year_token", "column_fingerprint", "sum_invariant", "column_order", "content_checksum")
	}
}

// verifyFingerprintSHA256 hashes the 32-column canonical fingerprint.
func verifyFingerprintSHA256(fp string) string {
	sum := sha256.Sum256([]byte(fp))
	return hex.EncodeToString(sum[:])
}

func verifyFingerprintLeg(w *nepraparse.Workbook) verifyLeg {
	drift := w.Header.Diff()
	if drift == nil {
		drift = []string{}
	}
	facts := map[string]any{
		"columns":            verifyCanonicalColumnCount,
		"physical_width":     w.PhysicalWidth,
		"pct_leaf_count":     w.Header.PctLeafCount,
		"fingerprint_sha256": verifyFingerprintSHA256(w.Header.Fingerprint()),
		"drift":              drift,
	}
	note := "32 logical columns against a wider physical row; the surplus is trailing Excel padding " +
		"and the logical width comes from the header band, never from stripping empty columns " +
		"(one row per year carries a stray \";\" in its last padding cell)."
	if !w.Header.Matches() {
		return leg("column_fingerprint", verifyFail, note, facts)
	}
	return leg("column_fingerprint", verifyPass, note, facts)
}

// verifyStructureLeg asserts what is true of EVERY sampled year and reports
// what is only true of the fixtured ones.
//
// tables==1 and <th>==0 are asserted everywhere: zero <th> is why header
// detection in nepraparse is text-matched rather than semantic, and a second
// table would mean the export changed shape. Row, cell, plant and width counts
// are asserted only for a surface whose internals were measured, because the
// four unfixtured years demonstrably differ (FY2018-19 and FY2019-20 are 32
// columns wide, not 39 or 40).
func verifyStructureLeg(s verifySurface, w *nepraparse.Workbook, n int, body []byte) verifyLeg {
	facts := map[string]any{
		"tables": w.TableCount, "th_elements": w.HeaderCellCount,
		"table_rows": w.TableRows, "raw_cells": w.RawCells,
		"plants": len(w.Plants), "physical_width": w.PhysicalWidth,
		"separator_rows": w.SeparatorRows, "bytes": n,
		"nbsp_0xa0_bytes": bytes.Count(body, []byte{0xA0}),
		// NOTE: span collisions are a Grid field, not a Workbook one, so
		// this leg does not report them for a workbook. Reporting a
		// hardcoded 0 would be asserting a value never measured — the same
		// mistake as recording http_status 200.
		"s_no_contiguous":  w.SNoContiguous,
		"names_unique":     w.NamesUnique,
		"warnings":         len(w.Warnings),
		"census_balanced":  w.Census.Balanced(),
		"residue_cells":    w.Census.ResidueCells,
		"measured_zeros":   w.Census.MeasuredZeros,
		"mixed_blank_zero": w.Census.MixedBlankZeroRows,
	}
	var problems []string
	if w.TableCount != 1 {
		problems = append(problems, fmt.Sprintf("expected exactly 1 <table>, got %d", w.TableCount))
	}
	if w.HeaderCellCount != 0 {
		problems = append(problems, fmt.Sprintf("expected 0 <th>, got %d", w.HeaderCellCount))
	}
	if !w.Census.Balanced() {
		problems = append(problems, "the cell-state census does not balance: a cell escaped classification")
	}
	if w.Census.MixedBlankZeroRows != 0 {
		problems = append(problems, fmt.Sprintf("%d rows mix a blank with a 0.00, which no sampled year does", w.Census.MixedBlankZeroRows))
	}
	if in := s.Internals; in != nil {
		facts["asserted_against"] = in.InternalsAsOf
		for _, chk := range []struct {
			name           string
			want, observed int
		}{
			{"table_rows", in.TableRows, w.TableRows},
			{"raw_cells", in.RawCells, w.RawCells},
			{"plants", in.Plants, len(w.Plants)},
			{"physical_width", in.PhysicalWidth, w.PhysicalWidth},
			{"nbsp_0xa0_bytes", in.NBSPBytes, bytes.Count(body, []byte{0xA0})},
		} {
			if chk.want != chk.observed {
				problems = append(problems, fmt.Sprintf("%s: manifest %d, observed %d", chk.name, chk.want, chk.observed))
			}
		}
	}
	if len(problems) > 0 {
		facts["problems"] = problems
		return leg("structure", verifyFail, "", facts)
	}
	if s.Internals == nil {
		return leg("structure", verifyReported,
			"tables and <th> were asserted; the counts are REPORTED ONLY. This year has no fixture "+
				"and its internals have never been observed on a second date, so they cannot fail "+
				"the gate. See measured_but_not_asserted.", facts)
	}
	return leg("structure", verifyPass, "", facts)
}

// verifySumInvariantLeg reports Sum == sum(12 monthly GWh).
func verifySumInvariantLeg(s verifySurface, w *nepraparse.Workbook) verifyLeg {
	rep := w.CheckSum(nepraparse.FieldGWh, 0)
	byState := map[string]int{}
	for _, sk := range rep.Skipped {
		byState[sk.Reason.String()]++
	}
	var mismatches []map[string]any
	for _, m := range rep.Mismatches {
		mismatches = append(mismatches, map[string]any{
			"plant": m.Plant, "row_index": m.RowIndex,
			"monthly_sum": m.MonthlySum, "reported": m.Reported, "delta": m.Delta,
		})
	}
	if mismatches == nil {
		mismatches = []map[string]any{}
	}
	facts := map[string]any{
		"field": nepraparse.FieldGWh.String(), "passed": rep.Passed, "eligible": rep.Eligible,
		"tolerance": nepraparse.DefaultSumTolerance, "ineligible": len(rep.Skipped),
		"ineligible_by_reason": byState, "mismatches": mismatches,
	}
	note := "Ineligible rows are EXCLUDED and counted, never read as zero. A blank month is an " +
		"absence of measurement: K-2 is blank for all 26 monthly cells in FY2017-18 (licensed, " +
		"not built) and then reports 16 explicit 0.00 months alongside a non-zero Sum of " +
		"1,705.91 GWh in FY2020-21."
	if in := s.Internals; in != nil {
		facts["manifest_passed"] = in.SumPassed
		facts["manifest_eligible"] = in.SumEligible
		facts["manifest_ineligible"] = in.Ineligible
		if rep.Passed != in.SumPassed || rep.Eligible != in.SumEligible || len(rep.Skipped) != in.Ineligible {
			facts["problem"] = fmt.Sprintf("manifest %d/%d passed with %d ineligible, observed %d/%d with %d",
				in.SumPassed, in.SumEligible, in.Ineligible, rep.Passed, rep.Eligible, len(rep.Skipped))
			return leg("sum_invariant", verifyFail, note, facts)
		}
		return leg("sum_invariant", verifyPass, note, facts)
	}
	return leg("sum_invariant", verifyReported,
		note+" REPORTED ONLY for this year: the identity does not hold universally on published "+
			"data (FY2021-22 fails it on 9 of 125 rows and FY2019-20 on 1 of 107 by -142.45 GWh), "+
			"so a year whose internals were never measured cannot fail this leg.", facts)
}

// verifyColumnOrderLeg asserts percent-then-GWh from the published numbers.
//
// This one IS asserted for every year, including the unfixtured ones, because
// it is not a count claim: it re-derives the ordering from the data in the file
// under inspection. Sum == sum(12 months) is an identity for energy and not for
// a utilisation ratio, so whichever member of each pair reconciles is the GWh
// column. A future export that swapped the pair would yield a plausible panel
// with no other symptom, and this is the only thing that would notice.
func verifyColumnOrderLeg(w *nepraparse.Workbook) verifyLeg {
	v := w.ColumnOrder(0)
	facts := map[string]any{
		"pct_first":   v.PctFirst,
		"gwh":         fmt.Sprintf("%d/%d", v.GWh.Passed, v.GWh.Eligible),
		"pct":         fmt.Sprintf("%d/%d", v.Pct.Passed, v.Pct.Eligible),
		"margin":      nepraparse.ColumnOrderMargin,
		"explanation": v.Explanation,
	}
	if v.GWh.Eligible == 0 {
		return leg("column_order", verifyReported,
			"INDETERMINATE: no row had all thirteen cells numeric, so the ordering cannot be "+
				"re-derived from this file. Reported rather than failed — an unprovable claim is "+
				"not a refutation.", facts)
	}
	if !v.PctFirst {
		return leg("column_order", verifyFail,
			"The GWh series did not reconcile better than the \"% age\" series, which means the pair "+
				"is not in the order this parser reads it. Every monthly figure would be wrong and "+
				"still look plausible.", facts)
	}
	return leg("column_order", verifyPass, "", facts)
}

// verifyContentChecksumLeg records both hashes and says which one is a test.
func verifyContentChecksumLeg(art nepraArtifact) verifyLeg {
	return leg("content_checksum", verifyPass,
		"sha256_raw is NOT a test. MEASURED: two identical fetches of one NEPRA page returned the "+
			"same 74,431 bytes with DIFFERENT SHA-256s, because Cloudflare rewrites every "+
			"data-cfemail token per response, and the same page returns 74,431 vs 74,792 bytes "+
			"depending on the request's Accept header. A raw-hash gate would report drift on every "+
			"run for a page that has not changed. sha256_content, over the extracted rows, is the "+
			"only comparable value.",
		map[string]any{
			"sha256_content": art.SHA256Content, "sha256_raw": art.SHA256Raw,
			"raw_is_evidence_only": true, "rows": art.Rows,
		})
}

// verifyWorkbookInternals is the report-only block for an unfixtured year.
func verifyWorkbookInternals(w *nepraparse.Workbook, n int, body []byte) map[string]any {
	rep := w.CheckSum(nepraparse.FieldGWh, 0)
	byState := map[string]int{}
	for _, sk := range rep.Skipped {
		byState[sk.Reason.String()]++
	}
	sigma := verifySumGWh(w)
	return map[string]any{
		"reason": "This fiscal year has no committed fixture and its internals have never been " +
			"observed on a second date, so nothing below can fail the gate. Re-measuring the four " +
			"unfixtured years live showed their internals genuinely differ from the three fixtured " +
			"ones — FY2018-19 and FY2019-20 are 32 physical columns wide, not 39 or 40, and " +
			"FY2021-22 fails the GWh sum identity on 9 of 125 rows — so inheriting another year's " +
			"expectations would have produced a false verdict in both directions.",
		"bytes":                n,
		"nbsp_0xa0_bytes":      bytes.Count(body, []byte{0xA0}),
		"table_rows":           w.TableRows,
		"raw_cells":            w.RawCells,
		"plants":               len(w.Plants),
		"physical_width":       w.PhysicalWidth,
		"separator_rows":       w.SeparatorRows,
		"band_label":           w.Header.BandLabel,
		"sum_gwh":              fmt.Sprintf("%d/%d", rep.Passed, rep.Eligible),
		"ineligible":           len(rep.Skipped),
		"ineligible_by_reason": byState,
		"sigma_sum_gwh":        sigma.TotalGWh,
		"sigma_rows_summed":    sigma.RowsSummed,
		"sigma_rows_excluded":  sigma.RowsExcluded,
		"unknown_text_cells":   w.Census.UnknownText,
		"warnings":             w.Warnings,
	}
}

// verifySheetLegs runs the Excel-sheet gate over one fetched body.
func verifySheetLegs(s verifySurface, url string, f verifyFetchResult) verifySurfaceResult {
	res := newVerifySurfaceResult(s, url)
	res.add(verifyStatusLeg(f))
	n := len(f.Body)

	if f.Err != nil {
		res.add(verify404StubLeg(f))
		res.skipRest("the fetch failed, so nothing downstream was measured",
			"byte_floor", "decoy_frameset", "charset", "structure", "row_floor", "content_checksum")
		return res
	}
	res.add(verifyByteFloorLeg(s, n))
	res.add(verify404StubLeg(f))
	res.add(verifyFramesetLeg(n, f.Body))
	res.add(verifyCharsetLeg(f.Body))

	// The sheet path must end in _files/sheet001.htm. The bare <name>.htm
	// form is the frameset wrapper, and four spec paths shipped with that
	// bug: 200 at ~9.6 KB with 3 <tr> against real sheets of 79,843 /
	// 79,672 / 40,402 / 14,783 bytes.
	doc, _, _, derr := nepraparse.Decode(f.Body)
	if derr != nil {
		res.add(leg("structure", verifyFail, "", map[string]any{"error": derr.Error()}))
		res.skipRest("the body could not be decoded", "row_floor", "content_checksum")
		return res
	}
	g, gerr := nepraparse.BuildGrid(doc)
	if gerr != nil {
		res.add(leg("structure", verifyFail,
			"No table at all. This is what the frameset wrapper and the 9-byte 404 stub both produce.",
			map[string]any{"error": gerr.Error()}))
		res.skipRest("there is no table to measure", "row_floor", "content_checksum")
		return res
	}
	res.add(verifySheetStructureLeg(s, g))

	rows, rerr := extractNepraTable(f.Body)
	if rerr != nil {
		res.add(leg("row_floor", verifyFail, "", map[string]any{"error": rerr.Error()}))
		res.skipRest("no rows were extracted", "content_checksum")
		return res
	}
	var parsed []map[string]any
	if uerr := json.Unmarshal(rows, &parsed); uerr != nil {
		res.add(leg("row_floor", verifyFail, "", map[string]any{"error": uerr.Error()}))
		res.skipRest("no rows were extracted", "content_checksum")
		return res
	}
	res.add(verifyRowFloorLeg(s, len(parsed)))

	art := newNepraArtifact(url, f.Body, rows, f.ContentType, len(parsed), 0)
	res.Artifact = &art
	res.add(verifyContentChecksumLeg(art))
	return res
}

func verifySheetStructureLeg(s verifySurface, g *nepraparse.Grid) verifyLeg {
	facts := map[string]any{
		"tables": g.Tables, "th_elements": g.HeaderCells,
		"grid_rows": len(g.Rows), "raw_cells": g.RawCells,
		"width": g.Width, "span_collisions": g.SpanCollisions,
	}
	var problems []string
	if g.Tables != 1 {
		problems = append(problems, fmt.Sprintf("expected exactly 1 <table>, got %d", g.Tables))
	}
	if g.SpanCollisions != 0 {
		problems = append(problems, fmt.Sprintf("%d span collisions: two cells claim one position, so the column mapping cannot be trusted", g.SpanCollisions))
	}
	if in := s.Internals; in != nil {
		facts["asserted_against"] = in.InternalsAsOf
		facts["manifest_grid_rows"] = in.GridRows
		facts["manifest_raw_cells"] = in.GridRawCells
		if len(g.Rows) < in.GridRows {
			problems = append(problems, fmt.Sprintf("grid rows: manifest floor %d, observed %d", in.GridRows, len(g.Rows)))
		}
		if g.RawCells < in.GridRawCells {
			problems = append(problems, fmt.Sprintf("raw cells: manifest floor %d, observed %d", in.GridRawCells, g.RawCells))
		}
	}
	if len(problems) > 0 {
		facts["problems"] = problems
		return leg("structure", verifyFail, "", facts)
	}
	return leg("structure", verifyPass, "", facts)
}

func verifyRowFloorLeg(s verifySurface, rows int) verifyLeg {
	facts := map[string]any{"rows": rows, "floor": s.RowFloor, "floor_measured_on": s.ByteFloorAsOf}
	note := "A FLOOR. Coming back with fewer extracted rows than the measured count under an HTTP " +
		"200 is the signature of the documented silent truncation, where a NEPRA page returned " +
		"1,028 of 2,440 rows."
	if s.RowFloor <= 0 {
		return leg("row_floor", verifyReported, "no row floor recorded for this surface", facts)
	}
	if rows < s.RowFloor {
		return leg("row_floor", verifyFail, note, facts)
	}
	if rows > s.RowFloor {
		facts["above_floor_by"] = rows - s.RowFloor
	}
	return leg("row_floor", verifyPass, note, facts)
}

// verifyWorstVerdict folds surface verdicts into one.
func verifyWorstVerdict(results []verifySurfaceResult) string {
	for _, r := range results {
		if r.Verdict == verifyFail {
			return verifyFail
		}
	}
	return verifyPass
}

// verifyLegSummary renders a one-line-per-leg summary for a human reader.
func verifyLegSummary(w io.Writer, results []verifySurfaceResult) {
	for _, r := range results {
		fmt.Fprintf(w, "%s  %s  %s\n", strings.ToUpper(r.Verdict), r.Surface, r.URL)
		for _, l := range r.Legs {
			line := fmt.Sprintf("    %-8s %-19s %s", strings.ToUpper(l.Verdict), l.Leg, verifyFactsLine(l.Facts))
			// A refusal or a skip must carry its reason on the line a human
			// reads. A bare "FAIL structure" would make the operator open the
			// JSON to find out what happened.
			switch l.Verdict {
			case verifyFail, verifySkipped, verifyReported:
				if reason := verifyLegReason(l); reason != "" {
					line += "  <- " + truncate(reason, 220)
				}
			}
			fmt.Fprintln(w, strings.TrimRight(line, " "))
		}
	}
}

// verifyLegReason is the one-line explanation a failing or skipped leg
// carries: the parser's own error where there is one, and otherwise the leg's
// note or its listed problems.
func verifyLegReason(l verifyLeg) string {
	if msg, ok := l.Facts["error"].(string); ok && msg != "" {
		return msg
	}
	if problems, ok := l.Facts["problems"].([]string); ok && len(problems) > 0 {
		return strings.Join(problems, "; ")
	}
	if msg, ok := l.Facts["problem"].(string); ok && msg != "" {
		return msg
	}
	// A leg's Note is the durable EXPLANATION of why the leg exists, and it
	// is long on purpose. It is not the reason THIS run refused, so it only
	// reaches the summary line when it is short enough to be one; the facts
	// (floor=493187 observed=9838) already say what happened, and the full
	// note is in the JSON either way.
	if len(l.Note) <= 120 {
		return l.Note
	}
	return ""
}

// verifyFactsLine renders a leg's measured facts deterministically.
func verifyFactsLine(facts map[string]any) string {
	keys := make([]string, 0, len(facts))
	for k := range facts {
		switch k {
		case "note", "explanation", "error", "body_hex", "warnings":
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := facts[k]
		switch tv := v.(type) {
		case nil:
			parts = append(parts, k+"=null")
		case []string:
			if len(tv) > 0 {
				parts = append(parts, fmt.Sprintf("%s=%v", k, tv))
			}
		case []map[string]any:
			if len(tv) > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", k, len(tv)))
			}
		case map[string]int:
			if len(tv) > 0 {
				parts = append(parts, fmt.Sprintf("%s=%v", k, tv))
			}
		default:
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}
	return strings.Join(parts, " ")
}
