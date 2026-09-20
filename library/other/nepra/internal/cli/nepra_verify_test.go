// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// Patch record to be written at .printing-press-patches/nepra-verify-fetch-gate.json;
// it is not in this change because the writable set for this task is source only.
//
// Every number asserted in this file was MEASURED by running this code over a
// byte-exact fixture or a live fetch, not copied from a document. Where a
// research figure disagreed with what the code produces, the code won and the
// disagreement is recorded in the run report.

package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// ---------------------------------------------------------------- fixtures

// verifyWorkbookFixture returns one fiscal year's sheet001.htm verbatim.
//
// It reads nepraparse's committed fixtures rather than a second copy: those
// three files are byte-exact copies of the live payloads (I re-fetched
// FY2023-24 and got a body byte-identical to full-fy2023-24.htm.gz under
// Accept: */*), and duplicating 75 KB of blobs to avoid a relative path would
// be the worse trade.
func verifyWorkbookFixture(t *testing.T, fy string) []byte {
	t.Helper()
	return verifyGunzip(t, filepath.Join("..", "nepraparse", "testdata", "full-fy"+fy+".htm.gz"))
}

func verifyGunzip(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path) // #nosec G304 -- test fixture path.
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip %s: %v", path, err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress %s: %v", path, err)
	}
	return out
}

// verifyFramesetFixture is the real per-year parent .htm: HTTP 200, exactly
// 9,838 bytes, and byte-identical across FY2017-18, FY2020-21 and FY2023-24.
func verifyFramesetFixture(t *testing.T) []byte {
	t.Helper()
	return verifyGunzip(t, filepath.Join("testdata", "verify-decoy-frameset.htm.gz"))
}

// verifySIRStubFixture is the real 374-byte stub that answers to both
// "SIR Data 2024.htm" and "SIR Data 2025.htm" and frames FY2023-24.
func verifySIRStubFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "verify-decoy-sir-stub.htm"))
	if err != nil {
		t.Fatalf("reading SIR stub fixture: %v", err)
	}
	return raw
}

// verify404StubBody is NEPRA's entire 404 response body.
const verify404StubBody = "Not Found"

// ---------------------------------------------------------------- helpers

func verifyOKFetch(body []byte) verifyFetchResult {
	return verifyFetchResult{Body: body, ContentType: "text/html"}
}

func verifyLegByName(t *testing.T, res verifySurfaceResult, name string) verifyLeg {
	t.Helper()
	for _, l := range res.Legs {
		if l.Leg == name {
			return l
		}
	}
	t.Fatalf("leg %q not present; legs were %s", name, verifyLegNames(res))
	return verifyLeg{}
}

func verifyLegNames(res verifySurfaceResult) string {
	var out []string
	for _, l := range res.Legs {
		out = append(out, l.Leg+"="+l.Verdict)
	}
	return strings.Join(out, " ")
}

func verifyMustSurface(t *testing.T, id string) verifySurface {
	t.Helper()
	s, ok := verifySurfaceByID(id)
	if !ok {
		t.Fatalf("manifest has no surface %q", id)
	}
	return s
}

func verifyFact(t *testing.T, l verifyLeg, key string) any {
	t.Helper()
	v, ok := l.Facts[key]
	if !ok {
		t.Fatalf("leg %q has no fact %q; facts were %v", l.Leg, key, l.Facts)
	}
	return v
}

func verifyFactInt(t *testing.T, l verifyLeg, key string) int {
	t.Helper()
	switch v := verifyFact(t, l, key).(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		t.Fatalf("leg %q fact %q is %T, not a number", l.Leg, key, v)
		return 0
	}
}

// ------------------------------------------------- the manifest itself

// TestVerifyManifestIsInternallyConsistent is this command's completeness
// assertion. Every claim the manifest makes about itself is checked here, so a
// hand edit that puts a floor below a decoy, marks an unmeasured year as
// asserted, or spells a band label the parser would never produce, fails the
// build rather than a live run.
func TestVerifyManifestIsInternallyConsistent(t *testing.T) {
	if len(verifySurfaces) != 11 {
		t.Fatalf("manifest has %d surfaces, want 11 (7 generation years + 4 Excel sheets)", len(verifySurfaces))
	}
	gen, sheets, asserted := 0, 0, 0
	seen := map[string]bool{}
	for _, s := range verifySurfaces {
		if seen[s.ID] {
			t.Fatalf("duplicate surface id %q", s.ID)
		}
		seen[s.ID] = true

		if s.ByteFloor <= 0 {
			t.Fatalf("%s has no byte floor", s.ID)
		}
		// Every floor must clear both decoys by a wide margin, or the floor
		// leg could pass on a decoy body.
		if s.ByteFloor <= verifyFramesetDecoyBytes {
			t.Fatalf("%s floor %d does not clear the %d-byte frameset decoy",
				s.ID, s.ByteFloor, verifyFramesetDecoyBytes)
		}
		if s.ByteFloor <= verify404StubBytes {
			t.Fatalf("%s floor %d does not clear the %d-byte 404 stub", s.ID, s.ByteFloor, verify404StubBytes)
		}
		if s.ByteFloorAsOf == "" {
			t.Fatalf("%s records a floor with no measurement date", s.ID)
		}
		if !strings.HasPrefix(s.Path, "/") || strings.Contains(s.Path, "{") {
			t.Fatalf("%s has an unresolved or relative path %q", s.ID, s.Path)
		}
		if s.Asserted != (s.Internals != nil) {
			t.Fatalf("%s: Asserted=%v but Internals==nil is %v; the flag must mean exactly "+
				"\"internals were measured\"", s.ID, s.Asserted, s.Internals == nil)
		}
		if s.Asserted {
			asserted++
			if s.Internals.InternalsAsOf == "" {
				t.Fatalf("%s asserts internals with no measurement date", s.ID)
			}
		}

		switch s.Kind {
		case verifyKindWorkbook:
			gen++
			fy, err := nepraparse.ParseFiscalYear(s.FY)
			if err != nil {
				t.Fatalf("%s: FY %q does not parse: %v", s.ID, s.FY, err)
			}
			if s.ExpectedBandLabel != fy.BandLabel() {
				t.Fatalf("%s: manifest band label %q, parser renders %q",
					s.ID, s.ExpectedBandLabel, fy.BandLabel())
			}
			// The path token must be the bare year. "FY2023-24" is a 404 with
			// a 9-byte body on every year.
			if !strings.Contains(s.Path, "%20"+fy.Label()+"_files/") {
				t.Fatalf("%s path does not carry the bare year token %q: %s", s.ID, fy.Label(), s.Path)
			}
			if strings.Contains(s.Path, "FY"+fy.Label()) {
				t.Fatalf("%s path carries the FY-prefixed token, which 404s: %s", s.ID, s.Path)
			}
			if s.RowFloor != 0 {
				t.Fatalf("%s is a workbook and must not carry an extracted-row floor", s.ID)
			}
		case verifyKindSheet:
			sheets++
			if s.FY != "" || s.ExpectedBandLabel != "" {
				t.Fatalf("%s is a sheet and must not claim a fiscal year", s.ID)
			}
			if s.RowFloor <= 0 {
				t.Fatalf("%s has no extracted-row floor", s.ID)
			}
			shared, err := resourceReadPath(s.ID)
			if err != nil || shared != s.Path {
				t.Fatalf("%s path %q has drifted from the shared resource table %q (err=%v)",
					s.ID, s.Path, shared, err)
			}
			// The bare <name>.htm form is the frameset wrapper. Four spec
			// paths shipped with that bug.
			if !strings.HasSuffix(s.Path, "_files/sheet001.htm") {
				t.Fatalf("%s path must end in _files/sheet001.htm, got %q", s.ID, s.Path)
			}
		default:
			t.Fatalf("%s has unknown kind %q", s.ID, s.Kind)
		}
	}
	if gen != 7 || sheets != 4 {
		t.Fatalf("manifest has %d generation years and %d sheets, want 7 and 4", gen, sheets)
	}
	// Exactly three generation years and four sheets were measured.
	if asserted != 7 {
		t.Fatalf("%d surfaces assert internals, want 7 (FY2017-18, FY2020-21, FY2023-24 and the four sheets)", asserted)
	}
	for _, id := range []string{"gen-2017-18", "gen-2020-21", "gen-2023-24", "fca", "quarterly", "sro", "hydel"} {
		if !verifyMustSurface(t, id).Asserted {
			t.Fatalf("%s must assert its internals", id)
		}
	}
	for _, id := range []string{"gen-2018-19", "gen-2019-20", "gen-2021-22", "gen-2022-23"} {
		if verifyMustSurface(t, id).Asserted {
			t.Fatalf("%s has no fixture and its internals have never been observed twice; "+
				"it must NOT be asserted", id)
		}
	}

	// A year cannot be both catalogued as reachable and recorded as never published.
	for _, u := range verifyUnpublishedYears {
		if _, ok := verifySurfaceForFY(u.FY); ok {
			t.Fatalf("FY%s is both a catalogued surface and recorded as unpublished", u.FY)
		}
		if _, err := nepraparse.ParseFiscalYear(u.FY); err != nil {
			t.Fatalf("unpublished year %q does not parse: %v", u.FY, err)
		}
	}
	if len(verifyDecoys) != 3 {
		t.Fatalf("%d decoys catalogued, want 3 (404 stub, frameset shell, SIR stub)", len(verifyDecoys))
	}
}

// TestVerifyFingerprintSHA256IsPinnedToTheShippedColumns pins the 32-column
// fingerprint hash to what the SHIPPED CanonicalColumns composes, not to a
// number from a document. This is the value a schema-drift refusal is
// measured against, so it has to come from the code in the same commit.
func TestVerifyFingerprintSHA256IsPinnedToTheShippedColumns(t *testing.T) {
	fp := nepraparse.HeaderFingerprint{Columns: nepraparse.CanonicalColumns()}.Fingerprint()
	const wantHead = "S.No|Name of Companies|Technology|Fuel|Installed Capacity (MW)|Dependable Capacity (MW)|Jul % age|Jul GWh|"
	const wantTail = "|Sum % age|Sum GWh"
	if !strings.HasPrefix(fp, wantHead) || !strings.HasSuffix(fp, wantTail) {
		t.Fatalf("canonical fingerprint has drifted:\n%s", fp)
	}
	// MEASURED from CanonicalColumns() in this tree.
	const want = "8164675647f214386ef43699f33fa1d1cc48c82452334a5345de4c6698d6da9f"
	if got := verifyFingerprintSHA256(fp); got != want {
		t.Fatalf("fingerprint sha256 = %s, want %s", got, want)
	}
	if nepraparse.LogicalColumns != 32 {
		t.Fatalf("LogicalColumns = %d, want 32", nepraparse.LogicalColumns)
	}
}

// ------------------------------------------------- the happy path

// TestVerifyGatePassesOnTheRealFY2023_24Fixture runs the whole leg set over
// the byte-exact FY2023-24 payload and pins every number it reports.
func TestVerifyGatePassesOnTheRealFY2023_24Fixture(t *testing.T) {
	body := verifyWorkbookFixture(t, "2023-24")
	if len(body) != 493187 {
		t.Fatalf("fixture is %d bytes, want 493187", len(body))
	}
	s := verifyMustSurface(t, "gen-2023-24")
	res := verifyWorkbookLegs(s, "https://nepra.org.pk"+s.Path, verifyOKFetch(body))
	if res.Verdict != verifyPass {
		t.Fatalf("verdict = %s, want pass; legs: %s", res.Verdict, verifyLegNames(res))
	}
	if res.MeasuredButNotAsserted != nil {
		t.Fatalf("an asserted year must not carry a measured_but_not_asserted block")
	}

	for _, want := range []struct {
		leg  string
		key  string
		want int
	}{
		{"byte_floor", "observed", 493187},
		{"byte_floor", "floor", 493187},
		{"decoy_404_stub", "observed_bytes", 493187},
		{"decoy_frameset", "frameset_elements", 0},
		{"charset", "nbsp_0xa0_bytes", 132},
		{"column_fingerprint", "physical_width", 39},
		{"column_fingerprint", "pct_leaf_count", 13},
		{"structure", "tables", 1},
		{"structure", "th_elements", 0},
		{"structure", "table_rows", 139},
		{"structure", "raw_cells", 4965},
		{"structure", "plants", 133},
		{"structure", "separator_rows", 2},
		{"structure", "residue_cells", 1},
		{"structure", "measured_zeros", 471},
		{"structure", "mixed_blank_zero", 0},
		{"sum_invariant", "passed", 118},
		{"sum_invariant", "eligible", 118},
		{"sum_invariant", "ineligible", 15},
	} {
		l := verifyLegByName(t, res, want.leg)
		if got := verifyFactInt(t, l, want.key); got != want.want {
			t.Fatalf("%s.%s = %d, want %d", want.leg, want.key, got, want.want)
		}
	}

	if got := verifyLegByName(t, res, "charset").Facts["charset"]; got != "windows-1252" {
		t.Fatalf("charset = %v, want windows-1252", got)
	}
	if got := verifyLegByName(t, res, "charset").Facts["declared_in_document"]; got != true {
		t.Fatalf("declared_in_document = %v, want true", got)
	}
	yt := verifyLegByName(t, res, "year_token")
	if yt.Facts["expected"] != "FY 2023-24" || yt.Facts["observed"] != "FY 2023-24" {
		t.Fatalf("year_token facts = %v", yt.Facts)
	}
	if got := verifyLegByName(t, res, "column_fingerprint").Facts["fingerprint_sha256"]; got !=
		"8164675647f214386ef43699f33fa1d1cc48c82452334a5345de4c6698d6da9f" {
		t.Fatalf("fingerprint = %v", got)
	}
	if got := verifyLegByName(t, res, "column_order").Facts["pct_first"]; got != true {
		t.Fatalf("pct_first = %v, want true", got)
	}
	if got := verifyLegByName(t, res, "column_order").Facts["gwh"]; got != "118/118" {
		t.Fatalf("column_order gwh = %v, want 118/118", got)
	}
	if got := verifyLegByName(t, res, "column_order").Facts["pct"]; got != "3/118" {
		t.Fatalf("column_order pct = %v, want 3/118", got)
	}

	// The ineligible breakdown is the null-discipline proof: 15 rows carry no
	// numeric Sum and every one of them is counted with its reason.
	byReason, ok := verifyLegByName(t, res, "sum_invariant").Facts["ineligible_by_reason"].(map[string]int)
	if !ok {
		t.Fatalf("ineligible_by_reason is %T", verifyLegByName(t, res, "sum_invariant").Facts["ineligible_by_reason"])
	}
	if byReason["status"] != 13 || byReason["not_reported"] != 2 {
		t.Fatalf("ineligible_by_reason = %v, want status:13 not_reported:2", byReason)
	}

	if res.Artifact == nil {
		t.Fatal("no artifact recorded")
	}
	if res.Artifact.Rows != 133 {
		t.Fatalf("artifact rows = %d, want 133", res.Artifact.Rows)
	}
	// 139 table rows minus 133 plants: the title row, the three band rows and
	// the two separators are read and not emitted.
	if res.Artifact.SkippedRows != 6 {
		t.Fatalf("artifact skipped_rows = %d, want 6", res.Artifact.SkippedRows)
	}
	if res.Artifact.Bytes != 493187 {
		t.Fatalf("artifact bytes = %d, want 493187", res.Artifact.Bytes)
	}
}

// TestVerifySheetGatePassesOnTheFourFixtures pins each Excel sheet's shape.
func TestVerifySheetGatePassesOnTheFourFixtures(t *testing.T) {
	for _, tc := range []struct {
		id                              string
		bytes, gridRows, rawCells, rows int
	}{
		{"fca", 79843, 53, 572, 48},
		{"quarterly", 79672, 168, 582, 163},
		{"sro", 40402, 23, 245, 19},
		{"hydel", 14783, 33, 126, 30},
	} {
		t.Run(tc.id, func(t *testing.T) {
			body := sheetFixture(t, tc.id)
			if len(body) != tc.bytes {
				t.Fatalf("fixture is %d bytes, want %d", len(body), tc.bytes)
			}
			s := verifyMustSurface(t, tc.id)
			res := verifySheetLegs(s, "https://nepra.org.pk"+s.Path, verifyOKFetch(body))
			if res.Verdict != verifyPass {
				t.Fatalf("verdict = %s; legs: %s", res.Verdict, verifyLegNames(res))
			}
			st := verifyLegByName(t, res, "structure")
			if got := verifyFactInt(t, st, "grid_rows"); got != tc.gridRows {
				t.Fatalf("grid_rows = %d, want %d", got, tc.gridRows)
			}
			if got := verifyFactInt(t, st, "raw_cells"); got != tc.rawCells {
				t.Fatalf("raw_cells = %d, want %d", got, tc.rawCells)
			}
			if got := verifyFactInt(t, st, "tables"); got != 1 {
				t.Fatalf("tables = %d, want 1", got)
			}
			rf := verifyLegByName(t, res, "row_floor")
			if got := verifyFactInt(t, rf, "rows"); got != tc.rows {
				t.Fatalf("rows = %d, want %d", got, tc.rows)
			}
			if res.FY != nil {
				t.Fatalf("a sheet surface must carry fy: null, got %q", *res.FY)
			}
		})
	}
}

// ------------------------------------------------- the three decoys

// TestVerifyRefusesTheFramesetDecoy proves that TWO INDEPENDENT guards each
// refuse the shell on their own.
//
// Neither is allowed to carry the case. The shell is byte-identical across
// three fiscal years, so a per-year floor is the only thing that catches a
// shell served for a year with no floor — and the structural signature is the
// only thing that catches a shell whose length changes. The test asserts each
// leg's verdict separately, which is the durable form of "delete one guard and
// it must still fail".
func TestVerifyRefusesTheFramesetDecoy(t *testing.T) {
	body := verifyFramesetFixture(t)
	if len(body) != verifyFramesetDecoyBytes {
		t.Fatalf("frameset fixture is %d bytes, want %d", len(body), verifyFramesetDecoyBytes)
	}
	s := verifyMustSurface(t, "gen-2023-24")
	res := verifyWorkbookLegs(s, "https://nepra.org.pk"+s.Path, verifyOKFetch(body))
	if res.Verdict != verifyFail {
		t.Fatalf("verdict = %s, want fail; legs: %s", res.Verdict, verifyLegNames(res))
	}

	// Guard 1: the structural signature, on its own.
	fs := verifyLegByName(t, res, "decoy_frameset")
	if fs.Verdict != verifyFail {
		t.Fatalf("decoy_frameset = %s, want fail", fs.Verdict)
	}
	// MEASURED on the real shell: three <frameset elements, one of them
	// inside the <noframes> fallback.
	if got := verifyFactInt(t, fs, "frameset_elements"); got != 3 {
		t.Fatalf("frameset_elements = %d, want 3", got)
	}

	// Guard 2: the per-year byte floor, on its own.
	bf := verifyLegByName(t, res, "byte_floor")
	if bf.Verdict != verifyFail {
		t.Fatalf("byte_floor = %s, want fail", bf.Verdict)
	}

	// Guard 3, independent of both: the shell has no table at all, so no
	// column could ever be read out of it.
	stLeg := verifyLegByName(t, res, "structure")
	if stLeg.Verdict != verifyFail {
		t.Fatalf("structure = %s, want fail", stLeg.Verdict)
	}
	if _, _, _, err := nepraparse.Decode(body); err != nil {
		t.Fatalf("the shell decodes cleanly; it is not an encoding failure: %v", err)
	}
	if _, err := nepraparse.ParseWorkbook(body, "2023-24"); !errors.Is(err, nepraparse.ErrNoTable) {
		t.Fatalf("ParseWorkbook(shell) = %v, want ErrNoTable — this is why ErrNoTable is mapped", err)
	}

	// No leg that judges CONTENT may reach a verdict over a decoy body: each
	// is recorded as skipped, which is visible, rather than absent, which
	// would read as agreement.
	for _, name := range []string{"year_token", "column_fingerprint", "sum_invariant", "column_order", "content_checksum"} {
		if got := verifyLegByName(t, res, name).Verdict; got != verifySkipped {
			t.Fatalf("%s = %s over a decoy body, want skipped", name, got)
		}
	}
	// decoy_404_stub correctly PASSES: 9,838 bytes is not the 9-byte stub.
	// Two decoys are two different checks and neither substitutes for the
	// other.
	if got := verifyLegByName(t, res, "decoy_404_stub").Verdict; got != verifyPass {
		t.Fatalf("decoy_404_stub = %s on a 9,838-byte body, want pass", got)
	}
	if res.Artifact != nil {
		t.Fatal("no artifact may be recorded for a body that failed the gate: a provenance record " +
			"over a decoy would make the decoy citable")
	}
}

// TestVerifyRefusesTheNineByteStub covers both codes the 9-byte body can
// produce, and proves they are different codes.
func TestVerifyRefusesTheNineByteStub(t *testing.T) {
	// A body of exactly "Not Found" served under a 200 is still not data.
	s := verifyMustSurface(t, "gen-2023-24")
	res := verifyWorkbookLegs(s, "https://nepra.org.pk"+s.Path, verifyOKFetch([]byte(verify404StubBody)))
	if res.Verdict != verifyFail {
		t.Fatalf("verdict = %s, want fail; legs: %s", res.Verdict, verifyLegNames(res))
	}
	stub := verifyLegByName(t, res, "decoy_404_stub")
	if stub.Verdict != verifyFail {
		t.Fatalf("decoy_404_stub = %s, want fail", stub.Verdict)
	}
	if got := verifyFactInt(t, stub, "observed_bytes"); got != 9 {
		t.Fatalf("observed_bytes = %d, want 9", got)
	}
	if _, present := stub.Facts["error_body_bytes"]; present {
		t.Fatal("a body read from a SUCCESSFUL response must not be labelled an error-body capture")
	}
	// The exact bytes NEPRA sends, recorded as evidence.
	if got := stub.Facts["body_hex"]; got != "4e6f7420466f756e64" {
		t.Fatalf("body_hex = %v, want 4e6f7420466f756e64", got)
	}

	// A manifest-reachable year that 404s is a GATE failure (6). A year the
	// manifest records as never published is NOT FOUND (3). They must not be
	// the same code, because they demand different human responses.
	srv, hits := verifyStubServer(t, http.StatusNotFound, "text/html; charset=iso-8859-1", []byte(verify404StubBody))
	defer srv.Close()
	_, code := verifyRun(t, srv.URL, "verify", "--fy", "2022-23", "--json")
	if code != 6 {
		t.Fatalf("a reachable year answering 404 exited %d, want 6", code)
	}
	if hits.Load() == 0 {
		t.Fatal("no request was made")
	}
	_, code = verifyRun(t, srv.URL, "verify", "--fy", "2024-25", "--json")
	if code != 3 {
		t.Fatalf("a manifest-unpublished year exited %d, want 3", code)
	}
}

// TestVerifyRefusesTheSIRStub proves the gate rests on the in-document year
// and not on a hash of a static stub.
//
// The 374-byte body is titled "SIR Data 2024", answers to "SIR Data 2025.htm"
// and frames FY2023-24. Nothing about it is a fiscal year, and the refusal
// here comes from there being no table and no band label — not from the body
// matching a known md5, which would stop working the moment NEPRA edited one
// character of the stub.
func TestVerifyRefusesTheSIRStub(t *testing.T) {
	body := verifySIRStubFixture(t)
	if len(body) != 374 {
		t.Fatalf("SIR stub fixture is %d bytes, want 374", len(body))
	}
	if !bytes.Contains(body, []byte("<title>SIR Data 2024</title>")) {
		t.Fatal("SIR stub fixture lost its 2024 title, which is the whole point of it")
	}
	if !bytes.Contains(body, []byte("Genenration%20wise%202023-24.htm")) {
		t.Fatal("SIR stub fixture lost the FY2023-24 frame it points at")
	}
	s := verifyMustSurface(t, "gen-2023-24")
	res := verifyWorkbookLegs(s, "https://nepra.org.pk"+s.Path, verifyOKFetch(body))
	if res.Verdict != verifyFail {
		t.Fatalf("verdict = %s, want fail; legs: %s", res.Verdict, verifyLegNames(res))
	}
	if verifyLegByName(t, res, "byte_floor").Verdict != verifyFail {
		t.Fatal("byte_floor must refuse a 374-byte body")
	}
	if verifyLegByName(t, res, "decoy_frameset").Verdict != verifyFail {
		t.Fatal("decoy_frameset must refuse a <frameset> stub")
	}
	if verifyLegByName(t, res, "structure").Verdict != verifyFail {
		t.Fatal("structure must refuse a body with no table")
	}
	// The year_token leg is recorded as SKIPPED, not passed: there was no
	// band label to read, and a silent absence would read as agreement.
	if verifyLegByName(t, res, "year_token").Verdict != verifySkipped {
		t.Fatalf("year_token = %s, want skipped", verifyLegByName(t, res, "year_token").Verdict)
	}
}

// TestVerifyYearTokenRefusesAFilenameSubstitution is the year-substitution
// case with a REAL workbook behind it: serve the FY2023-24 payload at the
// FY2018-19 path and the gate must refuse on the in-document band label.
//
// This is the failure the filename can never catch. Every byte of this body is
// genuine published data, every column is in place, the checksum is stable —
// and dating it FY2018-19 would mis-date 133 plants by five years.
func TestVerifyYearTokenRefusesAFilenameSubstitution(t *testing.T) {
	body := verifyWorkbookFixture(t, "2023-24")
	s := verifyMustSurface(t, "gen-2018-19")
	res := verifyWorkbookLegs(s, "https://nepra.org.pk"+s.Path, verifyOKFetch(body))
	if res.Verdict != verifyFail {
		t.Fatalf("verdict = %s, want fail; legs: %s", res.Verdict, verifyLegNames(res))
	}
	yt := verifyLegByName(t, res, "year_token")
	if yt.Verdict != verifyFail {
		t.Fatalf("year_token = %s, want fail", yt.Verdict)
	}
	if yt.Facts["expected"] != "FY 2018-19" {
		t.Fatalf("expected = %v, want \"FY 2018-19\"", yt.Facts["expected"])
	}
	if !strings.Contains(fmt.Sprint(yt.Facts["error"]), "2023-24") {
		t.Fatalf("the refusal must name the year that actually arrived: %v", yt.Facts["error"])
	}
	// Every leg that cannot be judged without the right year is SKIPPED.
	for _, name := range []string{"column_fingerprint", "structure", "sum_invariant", "column_order", "content_checksum"} {
		if got := verifyLegByName(t, res, name).Verdict; got != verifySkipped {
			t.Fatalf("%s = %s, want skipped", name, got)
		}
	}
	// The byte floor PASSED, and that is exactly why it cannot be the gate:
	// the body is a real workbook of a real year.
	if verifyLegByName(t, res, "byte_floor").Verdict != verifyPass {
		t.Fatal("the floor should pass here; if it fails this test proves nothing about year_token")
	}
}

// ------------------------------------------------- the checksum contract

// TestVerifyRawHashNeverGates is the most important test in this file.
//
// It drives two bodies that are byte-DIFFERENT and row-IDENTICAL, which is the
// measured Cloudflare behaviour: two identical fetches of one NEPRA page
// returned the same 74,431 bytes with different SHA-256s because every
// data-cfemail token is rewritten per response. sha256_raw must differ,
// sha256_content must not, and the verdict must be pass. Compare raw hashes
// instead and this gate reports drift on every run, forever, for a page that
// has not changed.
func TestVerifyRawHashNeverGates(t *testing.T) {
	body := verifyWorkbookFixture(t, "2023-24")
	s := verifyMustSurface(t, "gen-2023-24")

	// A per-response obfuscation token in a comment: not a cell, not a
	// column, and invisible to the extracted rows.
	mutated := bytes.Replace(body, []byte("<body"),
		[]byte("<!--cfemail=8f7a1b2c3d4e5f60--><body"), 1)
	if bytes.Equal(body, mutated) {
		t.Fatal("mutation did not apply")
	}
	if len(mutated) == len(body) {
		t.Fatal("mutation must change the byte length too")
	}

	first := verifyWorkbookLegs(s, "u", verifyOKFetch(body))
	second := verifyWorkbookLegs(s, "u", verifyOKFetch(mutated))
	if first.Verdict != verifyPass || second.Verdict != verifyPass {
		t.Fatalf("verdicts = %s / %s, want pass / pass. A rotating obfuscation token must not "+
			"fail the gate; legs: %s | %s", first.Verdict, second.Verdict,
			verifyLegNames(first), verifyLegNames(second))
	}
	if first.Artifact.SHA256Raw == second.Artifact.SHA256Raw {
		t.Fatal("sha256_raw did not change over different bytes; this test is not exercising the case")
	}
	if first.Artifact.SHA256Content != second.Artifact.SHA256Content {
		t.Fatalf("sha256_content changed over row-identical bodies:\n%s\n%s\n"+
			"THIS IS THE FAILURE THE WHOLE CHECKSUM LEG EXISTS TO PREVENT.",
			first.Artifact.SHA256Content, second.Artifact.SHA256Content)
	}
	// And the payload must say which hash is the test.
	cs := verifyLegByName(t, first, "content_checksum")
	if cs.Facts["raw_is_evidence_only"] != true {
		t.Fatal("content_checksum must flag sha256_raw as evidence only")
	}
	if !strings.Contains(cs.Note, "sha256_raw is NOT a test") {
		t.Fatalf("content_checksum note must say raw is not a test: %q", cs.Note)
	}
}

// TestVerifySchemaDriftIsFatalAndDiffed renames one header leaf and requires a
// refusal that names the column.
func TestVerifySchemaDriftIsFatalAndDiffed(t *testing.T) {
	body := verifyWorkbookFixture(t, "2023-24")
	// ">GWh<" occurs exactly 13 times and only in the header band's leaf row,
	// so replacing the first one drifts exactly one column.
	if n := bytes.Count(body, []byte(">GWh<")); n != 13 {
		t.Fatalf(">GWh< occurs %d times, want 13; the mutation is no longer surgical", n)
	}
	drifted := bytes.Replace(body, []byte(">GWh<"), []byte(">GWH<"), 1)

	s := verifyMustSurface(t, "gen-2023-24")
	res := verifyWorkbookLegs(s, "u", verifyOKFetch(drifted))
	if res.Verdict != verifyFail {
		t.Fatalf("verdict = %s, want fail; legs: %s", res.Verdict, verifyLegNames(res))
	}
	fp := verifyLegByName(t, res, "column_fingerprint")
	if fp.Verdict != verifyFail {
		t.Fatalf("column_fingerprint = %s, want fail", fp.Verdict)
	}
	msg := fmt.Sprint(fp.Facts["error"])
	if !strings.Contains(msg, "GWH") {
		t.Fatalf("the refusal must carry the parser's own column diff, naming the drifted leaf: %q", msg)
	}
	if _, err := nepraparse.ParseWorkbook(drifted, "2023-24"); !errors.Is(err, nepraparse.ErrSchemaDrift) {
		t.Fatalf("ParseWorkbook = %v, want ErrSchemaDrift; the errors.Is wiring is what maps this leg", err)
	}
	// Nothing downstream may claim a verdict over a drifted schema.
	for _, name := range []string{"structure", "sum_invariant", "column_order", "content_checksum"} {
		if got := verifyLegByName(t, res, name).Verdict; got != verifySkipped {
			t.Fatalf("%s = %s, want skipped: no column can be trusted after drift", name, got)
		}
	}
}

// TestVerifyColumnOrderSwapIsCaught drives the one failure that has no other
// symptom.
//
// It builds a workbook whose pair members are the wrong way round — the "% age"
// slot carries the energy series and the "GWh" slot carries a ratio — using
// nepraparse's own Value decoder so the cell states are real. Every individual
// number still parses and still looks plausible; only the Sum identity
// distinguishes them, and that is what ColumnOrder tests.
func TestVerifyColumnOrderSwapIsCaught(t *testing.T) {
	correct := verifySyntheticWorkbook(t, false)
	if v := verifyColumnOrderLeg(correct); v.Verdict != verifyPass {
		t.Fatalf("correctly-ordered synthetic workbook = %s (%v); the control case must pass",
			v.Verdict, v.Facts)
	}
	swapped := verifySyntheticWorkbook(t, true)
	l := verifyColumnOrderLeg(swapped)
	if l.Verdict != verifyFail {
		t.Fatalf("column_order = %s over a swapped pair, want fail. Facts: %v", l.Verdict, l.Facts)
	}
	if l.Facts["pct_first"] != false {
		t.Fatalf("pct_first = %v, want false", l.Facts["pct_first"])
	}
}

// verifySyntheticWorkbook builds a minimal workbook where Sum == sum(12
// months) holds on exactly one member of each pair.
func verifySyntheticWorkbook(t *testing.T, swap bool) *nepraparse.Workbook {
	t.Helper()
	num := func(v float64) nepraparse.Value {
		t.Helper()
		var out nepraparse.Value
		if err := json.Unmarshal([]byte(fmt.Sprintf(`{"state":"numeric","value":%g}`, v)), &out); err != nil {
			t.Fatalf("building a Value: %v", err)
		}
		if got, ok := out.Float64(); !ok || got != v {
			t.Fatalf("Value round-trip lost %g", v)
		}
		return out
	}
	w := &nepraparse.Workbook{}
	for p := 0; p < 8; p++ {
		var plant nepraparse.Plant
		plant.SNo = p + 1
		plant.Name = fmt.Sprintf("Synthetic Plant %d", p+1)
		energyTotal := 0.0
		for m := 0; m < 12; m++ {
			energy := float64(100 + p*10 + m)
			// A utilisation ratio: bounded, plausible, and NOT additive.
			ratio := 40.0 + float64(m)
			energyTotal += energy
			obs := nepraparse.MonthlyObservation{Month: nepraparse.MonthsInFiscalOrder[m]}
			if swap {
				obs.Utilisation, obs.Generation = num(energy), num(ratio)
			} else {
				obs.Utilisation, obs.Generation = num(ratio), num(energy)
			}
			plant.Months[m] = obs
		}
		total := nepraparse.MonthlyObservation{IsTotal: true}
		// The annual ratio is the mean, which is not the sum of the monthly
		// ratios — exactly how the real workbooks behave.
		if swap {
			total.Utilisation, total.Generation = num(energyTotal), num(45.5)
		} else {
			total.Utilisation, total.Generation = num(45.5), num(energyTotal)
		}
		plant.Total = total
		w.Plants = append(w.Plants, plant)
	}
	return w
}

// ------------------------------------------------- unasserted years

// TestVerifyUnassertedYearsAreReportedNeverFailed is the discipline this gate
// applies to every claim it cannot stand behind.
//
// FY2021-22 fails the published GWh sum identity on 9 of 125 rows and
// FY2018-19/FY2019-20 are 32 physical columns wide rather than 39 or 40. Those
// are facts about NEPRA's files, measured live, not parser bugs — so a gate
// that asserted another year's internals on them would fail on real data. They
// are reported instead, and the report says why.
func TestVerifyUnassertedYearsAreReportedNeverFailed(t *testing.T) {
	// FY2020-21's real, published mismatch: (NPPCL) - Balloki at +39.56 GWh.
	// The year IS asserted, and the manifest records 104/105, so the gate
	// PASSES while reporting the mismatch. A gate that demanded 105/105 would
	// be demanding that NEPRA's arithmetic be right.
	body := verifyWorkbookFixture(t, "2020-21")
	s := verifyMustSurface(t, "gen-2020-21")
	res := verifyWorkbookLegs(s, "u", verifyOKFetch(body))
	if res.Verdict != verifyPass {
		t.Fatalf("FY2020-21 verdict = %s, want pass; legs: %s", res.Verdict, verifyLegNames(res))
	}
	si := verifyLegByName(t, res, "sum_invariant")
	if si.Verdict != verifyPass {
		t.Fatalf("sum_invariant = %s, want pass (the manifest records 104/105)", si.Verdict)
	}
	if got := verifyFactInt(t, si, "passed"); got != 104 {
		t.Fatalf("passed = %d, want 104", got)
	}
	if got := verifyFactInt(t, si, "eligible"); got != 105 {
		t.Fatalf("eligible = %d, want 105", got)
	}
	mm, ok := si.Facts["mismatches"].([]map[string]any)
	if !ok || len(mm) != 1 {
		t.Fatalf("mismatches = %v, want exactly 1", si.Facts["mismatches"])
	}
	if mm[0]["plant"] != "(NPPCL) - Balloki" {
		t.Fatalf("mismatch plant = %v, want \"(NPPCL) - Balloki\"", mm[0]["plant"])
	}
	if d, _ := mm[0]["delta"].(float64); math.Abs(d-39.56) > 0.01 {
		t.Fatalf("mismatch delta = %v, want +39.56", mm[0]["delta"])
	}

	// Now the same shape on an UNASSERTED year: serve FY2020-21's body at the
	// FY2020-21 surface but with internals stripped, and the sum leg must
	// become report-only while the verdict stays pass.
	unasserted := s
	unasserted.Asserted = false
	unasserted.Internals = nil
	res2 := verifyWorkbookLegs(unasserted, "u", verifyOKFetch(body))
	if res2.Verdict != verifyPass {
		t.Fatalf("unasserted verdict = %s, want pass; legs: %s", res2.Verdict, verifyLegNames(res2))
	}
	for _, name := range []string{"sum_invariant", "structure"} {
		if got := verifyLegByName(t, res2, name).Verdict; got != verifyReported {
			t.Fatalf("%s on an unasserted year = %s, want reported", name, got)
		}
	}
	if res2.MeasuredButNotAsserted == nil {
		t.Fatal("an unasserted surface must carry a measured_but_not_asserted block")
	}
	if _, ok := res2.MeasuredButNotAsserted["reason"]; !ok {
		t.Fatal("measured_but_not_asserted must carry the reason it cannot be asserted")
	}
	if got := res2.MeasuredButNotAsserted["table_rows"]; got != 114 {
		t.Fatalf("measured_but_not_asserted.table_rows = %v, want 114 — the numbers must still be "+
			"COMPUTED and reported, not omitted", got)
	}
	// Flipping Asserted back on must make the same body assert again. This is
	// the mutation guard: if the flag stopped being load-bearing, both halves
	// of this test would pass for the wrong reason.
	if verifyLegByName(t, res, "sum_invariant").Verdict == verifyLegByName(t, res2, "sum_invariant").Verdict {
		t.Fatal("the Asserted flag changed nothing; it is not load-bearing")
	}
}

// TestVerifyBlanksAreNotZerosInTheSigma pins the Sum-GWh total and proves the
// excluded rows are excluded rather than zeroed.
//
// MEASURED over the byte-exact fixtures. These figures appear in no research
// document; they are this code's own output, which is why they are pinned here
// in the same commit as the code that computes them.
func TestVerifyBlanksAreNotZerosInTheSigma(t *testing.T) {
	for _, tc := range []struct {
		fy               string
		total            float64
		summed, excluded int
		byState          map[string]int
	}{
		{"2017-18", 121125.65, 97, 11, map[string]int{"not_reported": 11}},
		{"2020-21", 129580.38, 105, 3, map[string]int{"not_reported": 3}},
		{"2023-24", 126765.10, 118, 15, map[string]int{"delicensed": 12, "decommissioned": 1, "not_reported": 2}},
	} {
		t.Run(tc.fy, func(t *testing.T) {
			w, err := nepraparse.ParseWorkbook(verifyWorkbookFixture(t, tc.fy), tc.fy)
			if err != nil {
				t.Fatal(err)
			}
			got := verifySumGWh(w)
			if math.Abs(got.TotalGWh-tc.total) > 0.005 {
				t.Fatalf("sigma = %.2f, want %.2f", got.TotalGWh, tc.total)
			}
			if got.RowsSummed != tc.summed || got.RowsExcluded != tc.excluded {
				t.Fatalf("rows summed/excluded = %d/%d, want %d/%d",
					got.RowsSummed, got.RowsExcluded, tc.summed, tc.excluded)
			}
			if len(got.ExcludedByState) != len(tc.byState) {
				t.Fatalf("excluded_by_state = %v, want %v", got.ExcludedByState, tc.byState)
			}
			for k, v := range tc.byState {
				if got.ExcludedByState[k] != v {
					t.Fatalf("excluded_by_state[%s] = %d, want %d", k, got.ExcludedByState[k], v)
				}
			}
			// Summed + excluded must account for every plant row: a row can
			// never be silently dropped from both sides.
			if got.RowsSummed+got.RowsExcluded != len(w.Plants) {
				t.Fatalf("%d summed + %d excluded != %d plants", got.RowsSummed, got.RowsExcluded, len(w.Plants))
			}
			// The mutation guard the plan asks for: coercing an excluded cell
			// to a measured 0.00 must move the ROW COUNT even though the
			// total is unchanged. If only the total were asserted, reading
			// blanks as zeros would be invisible.
			if tc.excluded > 0 {
				zeroed := 0.0
				rows := 0
				for _, p := range w.Plants {
					v := p.Total.Generation
					if n, ok := v.Float64(); ok && v.State() == nepraparse.StateNumeric {
						zeroed += n
						rows++
						continue
					}
					// The corruption under test: treat absence as 0.00.
					rows++
				}
				if rows == got.RowsSummed {
					t.Fatal("the blank-as-zero mutation did not change the row count; this guard is inert")
				}
				if math.Abs(zeroed-got.TotalGWh) > 0.005 {
					t.Fatalf("the total moved (%.2f vs %.2f); the row count is the only signal "+
						"that would catch this, which is why it is asserted", zeroed, got.TotalGWh)
				}
			}
		})
	}
}

// ------------------------------------------------- crosscheck

// TestVerifyCrosscheckNeverEmitsADelta pins the refusal.
func TestVerifyCrosscheckNeverEmitsADelta(t *testing.T) {
	w, err := nepraparse.ParseWorkbook(verifyWorkbookFixture(t, "2023-24"), "2023-24")
	if err != nil {
		t.Fatal(err)
	}
	cc := verifyBuildCrosscheck("2023-24", verifySumGWh(w))
	if cc.Delta.ValueGWh != nil {
		t.Fatalf("delta.value_gwh = %v, want nil. NEPRA is Jul-Jun, IEA is calendar and stops at "+
			"2023, and K-Electric's fleet is absent from the NEPRA dataset: any number here is "+
			"a fabrication.", *cc.Delta.ValueGWh)
	}
	if cc.Delta.ValuePct != nil {
		t.Fatalf("delta.value_pct = %v, want nil", *cc.Delta.ValuePct)
	}
	if cc.Delta.BasisAligned {
		t.Fatal("basis_aligned must be false")
	}
	if cc.Verdict != verifyReported {
		t.Fatalf("verdict = %s, want reported: a structural gap is not a gate failure", cc.Verdict)
	}
	if cc.NEPRA == nil || math.Abs(cc.NEPRA.TotalGWh-126765.10) > 0.005 {
		t.Fatalf("the NEPRA side must be real and measured: %+v", cc.NEPRA)
	}
	if cc.NEPRA.RowsExcluded != 15 {
		t.Fatalf("rows_excluded = %d, want 15", cc.NEPRA.RowsExcluded)
	}
	// The four reasons ARE the deliverable.
	if len(cc.GapReasons) != 4 {
		t.Fatalf("%d gap reasons, want 4", len(cc.GapReasons))
	}
	joined := strings.ToLower(strings.Join(cc.GapReasons, " | "))
	for _, want := range []string{"jul-jun", "calendar", "k-electric", "delicensed"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("gap reasons must name %q: %s", want, joined)
		}
	}
	// And the IEA side is a DECLARED gap, not a silent omission.
	if cc.Fetched {
		t.Fatal("this build does not read the IEA series; fetched must be false")
	}
	if cc.IEA != nil {
		t.Fatal("iea must be null when the series was not read, never a remembered figure")
	}
	if cc.DeclaredGap == nil || !strings.Contains(cc.DeclaredGap.Reason, "api.iea.org") {
		t.Fatalf("the IEA gap must be declared and must name the third-party host: %+v", cc.DeclaredGap)
	}
}

// TestVerifyCrosscheckIsRefusedOffline checks that a source-restricted
// invocation is refused with exit 2 and makes ZERO requests.
func TestVerifyCrosscheckIsRefusedOffline(t *testing.T) {
	srv, hits := verifyStubServer(t, http.StatusOK, "text/html", verifyWorkbookFixture(t, "2023-24"))
	defer srv.Close()
	out, code := verifyRun(t, srv.URL, "verify", "--fy", "2023-24", "--crosscheck", "iea",
		"--data-source", "local", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if hits.Load() != 0 {
		t.Fatalf("%d requests were made; a refused invocation must make none", hits.Load())
	}
	if !strings.Contains(out, "api.iea.org") {
		t.Fatalf("the refusal must name the third-party host:\n%s", out)
	}
}

func TestVerifyCrosscheckRejectsAnUnknownSource(t *testing.T) {
	srv, hits := verifyStubServer(t, http.StatusOK, "text/html", verifyWorkbookFixture(t, "2023-24"))
	defer srv.Close()
	out, code := verifyRun(t, srv.URL, "verify", "--fy", "2023-24", "--crosscheck", "eia", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if hits.Load() != 0 {
		t.Fatalf("%d requests made before the flag was validated", hits.Load())
	}
	if !strings.Contains(out, "iea") {
		t.Fatalf("the refusal must name the one known source:\n%s", out)
	}
}

// ------------------------------------------------- staleness

// TestVerifyStalenessGateIsOptInAndBounded pins every boundary.
func TestVerifyStalenessGateIsOptInAndBounded(t *testing.T) {
	asOf := "2026-06-02"
	now := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC) // 100 days later
	for _, tc := range []struct {
		window      int
		gated       bool
		wantVerdict string
		wantGate    bool
	}{
		{90, false, verifyFail, false}, // measured stale, not gated -> exit 0
		{90, true, verifyFail, true},   // gated -> exit 6
		{120, true, verifyPass, false}, // inside the window -> exit 0
		{100, true, verifyPass, false}, // exactly at the window is not past it
	} {
		st := verifyBuildStaleness(asOf, now, tc.window, tc.gated)
		if st.ManifestAgeDays != 100 {
			t.Fatalf("manifest_age_days = %d, want 100", st.ManifestAgeDays)
		}
		if st.ManifestVerdict != tc.wantVerdict {
			t.Fatalf("window %d: manifest_verdict = %s, want %s", tc.window, st.ManifestVerdict, tc.wantVerdict)
		}
		gate := verifyStalenessGate(&st)
		if (gate != nil) != tc.wantGate {
			t.Fatalf("window %d gated=%v: gate = %v, want gate?=%v", tc.window, tc.gated, gate, tc.wantGate)
		}
		if gate != nil && ExitCode(gate) != 6 {
			t.Fatalf("a stale manifest exits %d, want 6", ExitCode(gate))
		}
		// The growing number is present in every case and gated in none.
		if st.CoverageGated {
			t.Fatal("coverage_gated must always be false: gating a monotonically growing number " +
				"makes the exit code a constant")
		}
		if st.DaysSinceCoverageEnd <= 0 {
			t.Fatalf("days_since_coverage_end = %d; it must be reported in every mode", st.DaysSinceCoverageEnd)
		}
	}
	// The measured value for today's manifest, pinned: FY2023-24 coverage
	// ended 2024-06-30, which is 802 days before 2026-09-10.
	st := verifyBuildStaleness(verifyManifestAsOf, now, 90, true)
	if st.DaysSinceCoverageEnd != 802 {
		t.Fatalf("days_since_coverage_end = %d, want 802", st.DaysSinceCoverageEnd)
	}
	// An unreadable date must never read as "0 days old", which is perfectly fresh.
	bad := verifyBuildStaleness("not-a-date", now, 90, true)
	if bad.ManifestAgeDays != -1 || bad.ManifestVerdict != verifyFail {
		t.Fatalf("an unparseable manifest date must fail, got age=%d verdict=%s",
			bad.ManifestAgeDays, bad.ManifestVerdict)
	}
}

func TestVerifyStaleAfterMustBePositive(t *testing.T) {
	srv, hits := verifyStubServer(t, http.StatusOK, "text/html", verifyWorkbookFixture(t, "2023-24"))
	defer srv.Close()
	for _, v := range []string{"0", "-1"} {
		_, code := verifyRun(t, srv.URL, "verify", "--fy", "2023-24", "--stale-after", v, "--json")
		if code != 2 {
			t.Fatalf("--stale-after %s exited %d, want 2", v, code)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("%d requests made before the window was validated", hits.Load())
	}
}

// TestVerifyNextFYProbeDistinguishesRealFromDecoy is the only staleness
// measurement allowed to change the exit code, and only in one direction.
func TestVerifyNextFYProbeDistinguishesRealFromDecoy(t *testing.T) {
	next, err := verifyNextFY()
	if err != nil {
		t.Fatal(err)
	}
	if next.Label() != "2024-25" {
		t.Fatalf("next fiscal year = %s, want 2024-25", next.Label())
	}
	notFound := 404

	for _, tc := range []struct {
		name        string
		f           verifyFetchResult
		wantVerdict string
	}{
		{
			name: "404 is the expected answer",
			f: verifyFetchResult{StatusExact: &notFound, Body: []byte(verify404StubBody),
				Err: &client.APIError{Method: "GET", Path: "/x", StatusCode: 404, Body: verify404StubBody}},
			wantVerdict: verifyPass,
		},
		{
			name:        "the 9-byte stub under a 200 is not data",
			f:           verifyOKFetch([]byte(verify404StubBody)),
			wantVerdict: verifyPass,
		},
		{
			name:        "the frameset shell is a wrapper, not a year",
			f:           verifyOKFetch(verifyFramesetFixture(t)),
			wantVerdict: verifyPass,
		},
		{
			// The SIR stub IS a frameset, and it is classified as one. That
			// is the right answer: it is a wrapper, not a year, and the
			// classification does not depend on its length or its md5.
			name:        "the SIR stub is a frameset wrapper, not a year",
			f:           verifyOKFetch(verifySIRStubFixture(t)),
			wantVerdict: verifyPass,
		},
		{
			name:        "a real workbook for the WRONG year is a substitution, not a new year",
			f:           verifyOKFetch(verifyWorkbookFixture(t, "2023-24")),
			wantVerdict: verifyPass,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := verifyClassifyNextFY(&verifyNextFYProbe{FY: next.Label()}, next, tc.f)
			if p.Verdict != tc.wantVerdict {
				t.Fatalf("verdict = %s (%s), want %s", p.Verdict, p.Note, tc.wantVerdict)
			}
			st := verifyBuildStaleness(verifyManifestAsOf, time.Now().UTC(), 90, true)
			st.NextFYProbe = p
			if gate := verifyStalenessGate(&st); gate != nil {
				t.Fatalf("this probe outcome must NOT change the exit code: %v", gate)
			}
		})
	}

	// The one case that must fail: a real workbook whose own band label reads
	// the next fiscal year.
	body := verifyWorkbookFixture(t, "2023-24")
	// The band cell is the ONLY place the file states its year, and the
	// label is split across source lines inside it:
	//   <td colspan=26 ...><span style='mso-spacerun:yes'>\xA0</span>FY
	//   2023-24</td>
	// so the mutation targets that exact sequence, which occurs once.
	const bandCell = "FY\n  2023-24</td>"
	if n := bytes.Count(body, []byte(bandCell)); n != 1 {
		t.Fatalf("the band cell sequence occurs %d times, want 1", n)
	}
	future := bytes.Replace(body, []byte(bandCell), []byte("FY\n  2024-25</td>"), 1)
	if bytes.Equal(body, future) {
		t.Fatal("the band-label mutation did not apply")
	}
	p := verifyClassifyNextFY(&verifyNextFYProbe{FY: next.Label()}, next, verifyOKFetch(future))
	if p.Verdict != verifyFail {
		t.Fatalf("a published FY2024-25 must FAIL the stale gate, got %s (%s)", p.Verdict, p.Note)
	}
	if p.BandLabel != "FY 2024-25" {
		t.Fatalf("band_label = %q, want \"FY 2024-25\"", p.BandLabel)
	}
	st := verifyBuildStaleness(verifyManifestAsOf, time.Now().UTC(), 90, true)
	st.NextFYProbe = p
	gate := verifyStalenessGate(&st)
	if gate == nil || ExitCode(gate) != 6 {
		t.Fatalf("a newly published fiscal year must exit 6, got %v", gate)
	}
}

// ------------------------------------------------- the transport contract

// TestVerifyNeverAssertsAStatusItDidNotSee guards the deferral's discipline.
func TestVerifyNeverAssertsAStatusItDidNotSee(t *testing.T) {
	ok := verifyStatusLeg(verifyOKFetch([]byte("x")))
	if ok.Verdict != verifyPass {
		t.Fatalf("verdict = %s", ok.Verdict)
	}
	v, present := ok.Facts["status_exact"]
	if !present {
		t.Fatal("status_exact must be PRESENT as an explicit null, not omitted")
	}
	if v != nil {
		t.Fatalf("status_exact = %v on a success, want null: this client returns the body without "+
			"the status code, so any number would be a value never observed", v)
	}
	if ok.Facts["status_class"] != "2xx-3xx" {
		t.Fatalf("status_class = %v, want 2xx-3xx", ok.Facts["status_class"])
	}

	code := 404
	fail := verifyStatusLeg(verifyFetchResult{
		StatusExact: &code,
		Err:         &client.APIError{Method: "GET", Path: "/x", StatusCode: 404, Body: verify404StubBody},
	})
	if fail.Verdict != verifyFail {
		t.Fatalf("verdict = %s, want fail", fail.Verdict)
	}
	if got := verifyFactInt(t, fail, "status_exact"); got != 404 {
		t.Fatalf("status_exact = %d, want 404: on a failure the exact code IS observable", got)
	}
	if fail.Facts["status_class"] != "4xx" {
		t.Fatalf("status_class = %v, want 4xx", fail.Facts["status_class"])
	}
	// And the JSON must carry the null rather than dropping the key.
	raw, err := json.Marshal(ok)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"status_exact":null`) {
		t.Fatalf("serialised leg lost the explicit null:\n%s", raw)
	}
}

// TestVerifyUsesTheUncachedFetch is the guard against a warm cache producing a
// fresh-looking verdict over bytes that were never fetched.
func TestVerifyUsesTheUncachedFetch(t *testing.T) {
	srv, hits := verifyStubServer(t, http.StatusOK, "text/html", verifyWorkbookFixture(t, "2023-24"))
	defer srv.Close()
	home := t.TempDir()
	for i := 1; i <= 3; i++ {
		_, code := verifyRunHome(t, srv.URL, home, "verify", "--fy", "2023-24", "--json")
		if code != 0 {
			t.Fatalf("run %d exited %d", i, code)
		}
		if int(hits.Load()) != i {
			t.Fatalf("after %d runs the server saw %d requests; a cache read would freeze this "+
				"count while fetched_at kept advancing", i, hits.Load())
		}
	}
}

// TestVerifyFYTokenIsRebuiltNotEchoed proves the URL token comes from the
// PARSED year, never from the argument.
func TestVerifyFYTokenIsRebuiltNotEchoed(t *testing.T) {
	body := verifyWorkbookFixture(t, "2023-24")
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath())
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	// All three accepted spellings must produce the same request.
	for _, arg := range []string{"2023-24", "FY 2023-24", "2023-2024"} {
		paths = nil
		_, code := verifyRun(t, srv.URL, "verify", "--fy", arg, "--json")
		if code != 0 {
			t.Fatalf("--fy %q exited %d", arg, code)
		}
		if len(paths) != 1 {
			t.Fatalf("--fy %q made %d requests", arg, len(paths))
		}
		got := paths[0]
		if !strings.Contains(got, "wise%202023-24_files/sheet001.htm") {
			t.Fatalf("--fy %q requested %q; the token must be the bare year", arg, got)
		}
		for _, forbidden := range []string{"FY2023-24", "FY%202023-24", "2023-2024"} {
			if strings.Contains(got, forbidden) {
				t.Fatalf("--fy %q leaked %q into the path %q; that form is a 404 with a 9-byte body "+
					"on every year", arg, forbidden, got)
			}
		}
	}
}

// ------------------------------------------------- the output contract

// TestVerifyJSONEnvelopeAccompaniesTheExitCode requires that every non-zero
// path still writes a parseable document with meta.verdict set. A gate that
// exited 6 with empty stdout would be indistinguishable from a crash.
func TestVerifyJSONEnvelopeAccompaniesTheExitCode(t *testing.T) {
	srv, _ := verifyStubServer(t, http.StatusOK, "text/html", verifyFramesetFixture(t))
	defer srv.Close()
	out, code := verifyRun(t, srv.URL, "verify", "--fy", "2023-24", "--json")
	if code != 6 {
		t.Fatalf("exit = %d, want 6", code)
	}
	var doc struct {
		Meta struct {
			Verdict  string `json:"verdict"`
			ExitCode int    `json:"exit_code"`
			Source   string `json:"source"`
		} `json:"meta"`
		Results   []map[string]any `json:"results"`
		Staleness map[string]any   `json:"staleness"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("stdout is not parseable JSON on a gate failure: %v\n%s", err, out)
	}
	if doc.Meta.Verdict != verifyFail {
		t.Fatalf("meta.verdict = %q, want fail", doc.Meta.Verdict)
	}
	if doc.Meta.ExitCode != 6 {
		t.Fatalf("meta.exit_code = %d, want 6: the document must state the code it is exiting with", doc.Meta.ExitCode)
	}
	if doc.Meta.Source != "live" {
		t.Fatalf("meta.source = %q, want live", doc.Meta.Source)
	}
	if len(doc.Results) != 1 {
		t.Fatalf("%d results, want 1", len(doc.Results))
	}
	if doc.Staleness == nil {
		t.Fatal("staleness must be present on every gated run")
	}
}

// TestVerifyAgentModeKeepsTheVerdictAndTheSparseKeys guards the projection.
//
// --agent implies --compact, which projects a list of objects through a
// keys-present-in-80%-of-rows rule and flattens a two-key {meta, results:[…]}
// envelope down to the bare array. Both would silently destroy this report: fy
// is present on 7 of 11 surfaces and meta.verdict would disappear entirely.
//
// The envelope is ONE level, not two. verify marshals its own {meta, results}
// and then hands printOutputWithFlagsMeta a hand-picked subset of that same
// meta (source, verdict, exit_code, manifest_as_of); the agent wrapper used to
// nest the whole report under that subset, so the 11 surfaces sat at
// .results.results[] and .results[] iterated nothing. wrapAgentOutput now
// merges into the envelope the command already built. The compact projection
// is unaffected either way — it runs on the report BEFORE any wrapper — so the
// sparse-key assertions below are the same test they always were, one level up.
// mapKeys names a JSON object's keys, sorted, so an envelope-shape failure
// says what the envelope actually contained.
func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestVerifyAgentModeKeepsTheVerdictAndTheSparseKeys(t *testing.T) {
	body := verifyWorkbookFixture(t, "2023-24")
	sheet := sheetFixture(t, "hydel")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if strings.Contains(r.URL.Path, "Hydel") {
			_, _ = w.Write(sheet)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	out, code := verifyRun(t, srv.URL, "verify", "--all", "--agent")
	if code != 6 {
		// Six of the seven years get FY2023-24's body, so six year_token legs
		// refuse. That is the correct outcome and it is what makes this a
		// good --agent test: the document must survive a failing run.
		t.Fatalf("exit = %d, want 6", code)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("agent output is not parseable: %v\n%s", err, out)
	}
	// The verdict must be reachable in agent mode.
	meta, ok := doc["meta"].(map[string]any)
	if !ok {
		t.Fatalf("agent output lost meta:\n%s", truncate(out, 400))
	}
	if meta["verdict"] != verifyFail {
		t.Fatalf("agent meta.verdict = %v, want fail", meta["verdict"])
	}
	if nested, ok := doc["results"].(map[string]any); ok {
		if _, doubled := nested["results"]; doubled {
			t.Fatalf("agent output nested a second envelope: .results.results exists, "+
				"so .results[] iterates nothing. keys=%v", mapKeys(nested))
		}
		t.Fatalf("agent output flattened the report away; results is a %T", doc["results"])
	}
	rows, ok := doc["results"].([]any)
	if !ok || len(rows) != 11 {
		t.Fatalf("expected 11 surface results at .results[], got %T len %d",
			doc["results"], len(rows))
	}
	// The subset meta verify hands the wrapper must not have displaced the
	// rich meta it built itself: exit_code rides along with verdict.
	if _, present := meta["exit_code"]; !present {
		t.Fatalf("merged meta lost exit_code: %v", mapKeys(meta))
	}
	if _, displaced := meta["envelope_meta"]; displaced {
		t.Fatalf("merging displaced a meta key that disagreed; envelope_meta = %v",
			meta["envelope_meta"])
	}
	// fy is present on 7 of 11 rows. As an explicit null on the other 4 it
	// survives the projection; omitted, it would be dropped.
	withFY := 0
	for _, r := range rows {
		row, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("surface row is %T", r)
		}
		if _, present := row["fy"]; !present {
			t.Fatalf("--agent dropped the fy key from a surface row: %v", row)
		}
		if row["fy"] != nil {
			withFY++
		}
		if _, present := row["verdict"]; !present {
			t.Fatalf("--agent dropped the verdict from a surface row: %v", row)
		}
	}
	if withFY != 7 {
		t.Fatalf("%d rows carry a non-null fy, want 7", withFY)
	}
}

// TestVerifyManifestMakesNoRequest is the offline promise.
func TestVerifyManifestMakesNoRequest(t *testing.T) {
	srv, hits := verifyStubServer(t, http.StatusOK, "text/html", []byte("should never be fetched"))
	defer srv.Close()
	for _, args := range [][]string{
		{"verify", "--json"},
		{"verify", "--manifest", "--json"},
		{"verify", "--manifest", "--agent"},
	} {
		out, code := verifyRun(t, srv.URL, args...)
		if code != 0 {
			t.Fatalf("%v exited %d:\n%s", args, code, out)
		}
		if hits.Load() != 0 {
			t.Fatalf("%v made %d requests; --manifest must make none", args, hits.Load())
		}
		if !strings.Contains(out, "8164675647f214386ef43699f33fa1d1cc48c82452334a5345de4c6698d6da9f") {
			t.Fatalf("%v did not print the column fingerprint:\n%s", args, out)
		}
		if !strings.Contains(out, "declared_gaps") {
			t.Fatalf("%v did not print the declared gaps:\n%s", args, out)
		}
	}
	// Combining --manifest with a fetch selector is a usage error, because a
	// manifest print that also fetched would make the offline mode a lie.
	for _, args := range [][]string{
		{"verify", "--manifest", "--fy", "2023-24", "--json"},
		{"verify", "--manifest", "--all", "--json"},
		{"verify", "--manifest", "--surface", "fca", "--json"},
		{"verify", "--manifest", "--crosscheck", "iea", "--json"},
	} {
		_, code := verifyRun(t, srv.URL, args...)
		if code != 2 {
			t.Fatalf("%v exited %d, want 2", args, code)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("a refused --manifest combination made %d requests", hits.Load())
	}
}

// TestVerifyManifestEntriesHaveAnIdenticalKeySet guards the projection.
//
// --compact keeps a list-item key only when it appears in about 80% of rows.
// fy appears on 7 of 11 surfaces and the internals on 7, so an OMITTED key
// would be stripped from the whole listing under --agent while an explicit
// null survives. Every entry therefore carries the same keys.
func TestVerifyManifestEntriesHaveAnIdenticalKeySet(t *testing.T) {
	raw, err := json.Marshal(verifyBuildManifestPayload())
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Surfaces []map[string]json.RawMessage `json:"surfaces"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Surfaces) != 11 {
		t.Fatalf("%d surfaces serialised, want 11", len(doc.Surfaces))
	}
	var keys []string
	for k := range doc.Surfaces[0] {
		keys = append(keys, k)
	}
	for i, e := range doc.Surfaces {
		if len(e) != len(keys) {
			t.Fatalf("surface %d has %d keys, surface 0 has %d; a sparse key would be dropped by "+
				"--compact", i, len(e), len(keys))
		}
		for _, k := range keys {
			if _, present := e[k]; !present {
				t.Fatalf("surface %d is missing key %q", i, k)
			}
		}
	}
	// And an unmeasured internal must be null, never 0.
	for _, e := range doc.Surfaces {
		if string(e["surface"]) != `"gen-2018-19"` {
			continue
		}
		for _, k := range []string{"table_rows", "raw_cells", "plants", "physical_width", "sigma_sum_gwh"} {
			if got := string(e[k]); got != "null" {
				t.Fatalf("gen-2018-19.%s = %s, want null: a zero here would be a measurement", k, got)
			}
		}
		if string(e["internals_asserted"]) != "false" {
			t.Fatal("gen-2018-19 must not assert internals")
		}
		return
	}
	t.Fatal("gen-2018-19 is not in the serialised manifest")
}

// TestVerifyAnnotationsMatchTheDocumentedExitCodes keeps the annotation, the
// help text and the code constructors in agreement.
func TestVerifyAnnotationsMatchTheDocumentedExitCodes(t *testing.T) {
	cmd, _, err := RootCmd().Find([]string{"verify"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Annotations["mcp:read-only"] != "true" {
		t.Fatalf("mcp:read-only = %q; verify only reads published documents",
			cmd.Annotations["mcp:read-only"])
	}
	if cmd.Annotations["pp:novel-scaffold"] != "" {
		t.Fatal("the implemented command must not carry pp:novel-scaffold")
	}
	if got := cmd.Annotations["pp:happy-args"]; got != "--fy=2023-24" {
		t.Fatalf("pp:happy-args = %q; a flag-driven command needs the --flag=value form, because a "+
			"bare label=value is parsed as a positional", got)
	}
	codes := cmd.Annotations["pp:typed-exit-codes"]
	// 5 was ADDED after a code review found meta.exit_code reporting 0 while
	// the process exited 5 on an unreachable surface — a 5xx, DNS failure or
	// timeout is not a gate verdict, so it keeps its own code, and the
	// document must carry it rather than leaving a machine caller to infer it.
	if codes != "0,2,3,5,6" {
		t.Fatalf("pp:typed-exit-codes = %q, want \"0,2,3,5,6\"", codes)
	}
	for _, c := range strings.Split(codes, ",") {
		if !strings.Contains(cmd.Long, "\n  "+c+"  ") {
			t.Fatalf("exit code %s is annotated but not documented in Long help", c)
		}
	}
	// Exit 6 must be reachable and must be what gateErr produces.
	if got := ExitCode(gateErr(errors.New("x"))); got != 6 {
		t.Fatalf("gateErr exits %d, want 6", got)
	}
	// The scaffold's false claim must be gone from this command's own text.
	if strings.Contains(cmd.Short, "cached artifact") {
		t.Fatal("the Short description still claims a cached-artifact store that does not exist")
	}
	if !strings.Contains(cmd.Long, "FETCH gate") {
		t.Fatal("the Long help must say what this command actually gates")
	}
}

// TestVerifyDryRunMakesNoRequest keeps the verify harness's probe green.
func TestVerifyDryRunMakesNoRequest(t *testing.T) {
	srv, hits := verifyStubServer(t, http.StatusOK, "text/html", []byte("x"))
	defer srv.Close()
	for _, args := range [][]string{
		{"verify", "--fy", "2023-24", "--dry-run", "--json"},
		{"verify", "--all", "--dry-run", "--json"},
		{"verify", "--surface", "fca", "--dry-run"},
		// A malformed year must still short-circuit at the dry-run guard
		// rather than at validation: the harness probes with --dry-run and
		// expects exit 0.
		{"verify", "--fy", "not-a-year", "--dry-run", "--json"},
	} {
		out, code := verifyRun(t, srv.URL, args...)
		if code != 0 {
			t.Fatalf("%v exited %d:\n%s", args, code, out)
		}
		if hits.Load() != 0 {
			t.Fatalf("%v made %d requests", args, hits.Load())
		}
	}
}

// TestVerifyTransportFailureIsNotAGateFailure keeps a 5xx out of exit 6.
//
// A gate failure is a statement about what NEPRA publishes. "The site returned
// 503" is a statement about the site being up, and conflating the two would
// have a cron paging a data engineer about an outage.
func TestVerifyTransportFailureIsNotAGateFailure(t *testing.T) {
	// The client retries a 5xx three times with exponential backoff unless a
	// harness env var is set; this test is about classification, not the
	// retry schedule, so it opts out of the wait.
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	srv, _ := verifyStubServer(t, http.StatusServiceUnavailable, "text/html", []byte("upstream down"))
	defer srv.Close()
	out, code := verifyRun(t, srv.URL, "verify", "--fy", "2023-24", "--json", "--timeout", "5s")
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (api error), not 6 (gate failure)\n%s", code, out)
	}
	// The document still reaches stdout so the caller can see which surface
	// stopped the run.
	if !strings.Contains(out, "\"results\"") {
		t.Fatalf("a transport failure must still print the document:\n%s", out)
	}
}

// ------------------------------------------------- harness

// verifyStubServer serves one body for every path and counts requests.
func verifyStubServer(t *testing.T, status int, contentType string, body []byte) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	return srv, &hits
}

// verifyRun drives the real command tree against a stub base URL in an
// isolated home, and returns stdout plus the exit code.
func verifyRun(t *testing.T, baseURL string, args ...string) (string, int) {
	t.Helper()
	return verifyRunHome(t, baseURL, t.TempDir(), args...)
}

func verifyRunHome(t *testing.T, baseURL, home string, args ...string) (string, int) {
	t.Helper()
	t.Setenv("NEPRA_BASE_URL", baseURL)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("NEPRA_CONFIG", filepath.Join(home, "config", "config.toml"))

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append(args, "--no-learn"))
	err := root.Execute()
	code := 0
	if err != nil {
		code = ExitCode(err)
	}
	return stdout.String(), code
}
