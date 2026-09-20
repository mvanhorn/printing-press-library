// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The writers and the command surface. None of these tests makes a network
// request; the refusal cases assert that explicitly with a RoundTripper that
// fails the test if it is ever called.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// genFailingTransport fails the test if anything reaches the network.
type genFailingTransport struct{ t *testing.T }

func (g genFailingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	g.t.Errorf("a request was made to %s; this path must make none", r.URL)
	return nil, errors.New("no requests allowed")
}

// genRun executes the real root command with args, returning stdout, stderr
// and the typed exit code.
func genRun(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	restore := http.DefaultTransport
	http.DefaultTransport = genFailingTransport{t: t}
	defer func() { http.DefaultTransport = restore }()

	cmd := RootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	code := 0
	if err != nil {
		code = ExitCode(err)
		fmt.Fprintf(&errOut, "error: %v\n", err)
	}
	return out.String(), errOut.String(), code
}

// TestGenCSVAndTSVAreLossless pins the 25-column header, its exact order, and
// that a not-reported row's numeric fields are EMPTY rather than 0.
//
// The exact header string is asserted because it is what pins that nothing
// sorted the columns: printCSV/writeCSVRows sorts keys with sort.Strings,
// which would alphabetise these 25 into an order no consumer asked for, and
// printAutoTable would cut them to the first six without saying so.
func TestGenCSVAndTSVAreLossless(t *testing.T) {
	const wantHeader = "fy,sno,plant,technology,technology_known,fuel,fuel_known," +
		"installed_mw,installed_mw_state,dependable_mw,dependable_mw_state," +
		"row_class,block_status,month,month_index,period," +
		"utilisation_pct,utilisation_state,gwh,gwh_state," +
		"sum_gwh_reported,sum_gwh_reported_state,sum_reconciles,note,source_url"
	if got := strings.Join(genRowHeader, ","); got != wantHeader {
		t.Fatalf("header =\n %s\nwant\n %s", got, wantHeader)
	}

	rows := genTestRows(t, "2023-24")
	cells := make([][]string, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, r.Cells())
	}

	var csvOut bytes.Buffer
	writeCSVRow(&csvOut, genRowHeader)
	for _, c := range cells {
		writeCSVRow(&csvOut, c)
	}
	lines := strings.Split(strings.TrimRight(csvOut.String(), "\n"), "\n")
	if len(lines) != 1597 {
		t.Fatalf("csv lines = %d, want 1597 (header + 1596 rows)", len(lines))
	}
	if lines[0] != wantHeader {
		t.Errorf("csv header = %q", lines[0])
	}

	var tsvOut bytes.Buffer
	if err := genWriteTSV(&tsvOut, genRowHeader, cells); err != nil {
		t.Fatalf("genWriteTSV: %v", err)
	}
	tsvLines := strings.Split(strings.TrimRight(tsvOut.String(), "\n"), "\n")
	if len(tsvLines) != 1597 {
		t.Fatalf("tsv lines = %d, want 1597", len(tsvLines))
	}
	if tsvLines[0] != strings.Join(genRowHeader, "\t") {
		t.Errorf("tsv header = %q", tsvLines[0])
	}
	for i, l := range tsvLines[1:] {
		if n := len(strings.Split(l, "\t")); n != 25 {
			t.Fatalf("tsv row %d has %d fields, want 25: %q", i, n, l)
		}
	}

	// No cell may carry a tab, CR or LF in any of the 1,596 rows.
	// collapseText makes that true upstream; this is the check that the
	// guarantee held.
	for i, c := range cells {
		for j, cell := range c {
			if strings.ContainsAny(cell, "\t\r\n") {
				t.Fatalf("row %d column %q carries a control character: %q", i, genRowHeader[j], cell)
			}
		}
	}

	// The blank-vs-zero distinction in the flat formats: EMPTY, not "0".
	idx := map[string]int{}
	for i, h := range genRowHeader {
		idx[h] = i
	}
	var sawBlank, sawZero bool
	for _, c := range cells {
		switch c[idx["plant"]] {
		case "Reshma Power Generation (Private) Limited. (RPGPL)":
			sawBlank = true
			if c[idx["gwh"]] != "" || c[idx["gwh_state"]] != "not_reported" {
				t.Errorf("Reshma csv gwh = %q state %q, want empty/not_reported", c[idx["gwh"]], c[idx["gwh_state"]])
			}
			if c[idx["sum_reconciles"]] != "" {
				t.Errorf("Reshma csv sum_reconciles = %q, want empty (not checked is not false)", c[idx["sum_reconciles"]])
			}
			if c[idx["installed_mw"]] != "97" {
				t.Errorf("Reshma csv installed_mw = %q, want 97", c[idx["installed_mw"]])
			}
		case "Kot Addu Power Company (KAPCO)":
			sawZero = true
			if c[idx["gwh"]] != "0" || c[idx["gwh_state"]] != "numeric" {
				t.Errorf("KAPCO csv gwh = %q state %q, want 0/numeric", c[idx["gwh"]], c[idx["gwh_state"]])
			}
			if c[idx["sum_reconciles"]] != "true" {
				t.Errorf("KAPCO csv sum_reconciles = %q, want true", c[idx["sum_reconciles"]])
			}
		}
	}
	if !sawBlank || !sawZero {
		t.Fatal("the blank-vs-zero pair was not exercised")
	}
}

// TestGenTSVRefusesABrokenRow pins that a cell carrying a tab is refused
// rather than emitted. collapseText makes it unreachable from real data, which
// is exactly why the guard needs its own test.
func TestGenTSVRefusesABrokenRow(t *testing.T) {
	var out bytes.Buffer
	err := genWriteTSV(&out, []string{"a", "b"}, [][]string{{"ok", "still ok"}, {"bad\ttab", "x"}})
	if err == nil {
		t.Fatal("a tab-bearing cell was written as if it were a valid row")
	}
	if !strings.Contains(err.Error(), "silently split the row") {
		t.Errorf("error does not explain the risk: %v", err)
	}
}

// TestGenJSONEnvelopeCarriesMetaAndRows pins the {meta, results} shape and
// that a null in JSON is a null, not a zero.
func TestGenJSONEnvelopeCarriesMetaAndRows(t *testing.T) {
	w, body := genWorkbook(t, "2023-24")
	y, _ := genYearByLabel("2023-24")
	rows := genRows(w, genTestURL)
	meta := genBuildMeta(w, y, "plant-month", len(rows), body, genTestURL, "text/html", genRowsJSON(rows))

	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := genWrite(cmd, &rootFlags{asJSON: true}, genFormatJSON, genRowHeader, nil, meta, rows); err != nil {
		t.Fatalf("genWrite: %v", err)
	}

	var envelope struct {
		Meta    json.RawMessage `json:"meta"`
		Results []genRow        `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshalling the envelope: %v\n%s", err, out.String()[:200])
	}
	if len(envelope.Results) != 1596 {
		t.Fatalf("results = %d, want 1596", len(envelope.Results))
	}
	var m genMeta
	if err := json.Unmarshal(envelope.Meta, &m); err != nil {
		t.Fatalf("unmarshalling meta: %v", err)
	}
	if m.Rows != 1596 || m.Plants != 133 || m.FY != "2023-24" || m.Grain != "plant-month" || m.Source != "live" {
		t.Errorf("meta = %d rows / %d plants / %q / %q / %q", m.Rows, m.Plants, m.FY, m.Grain, m.Source)
	}
	if m.Artifact.Bytes != 493187 || m.Artifact.Rows != 1596 || m.Artifact.SHA256Raw == "" || m.Artifact.SHA256Content == "" {
		t.Errorf("artifact = %+v", m.Artifact)
	}
	// sha256_content is over the extracted rows, so it must differ from the
	// raw markup hash and must be stable for identical rows.
	if m.Artifact.SHA256Raw == m.Artifact.SHA256Content {
		t.Error("sha256_content must be over the rows, not the markup")
	}
	// A round-trip must not turn a null into a zero.
	for _, r := range envelope.Results {
		if r.Plant == "Reshma Power Generation (Private) Limited. (RPGPL)" && r.GWh != nil {
			t.Fatalf("a JSON round-trip turned Reshma's blank into %v", *r.GWh)
		}
	}
}

// TestGenJSONLIsOneRowPerLine pins that jsonl puts nothing but rows on stdout
// and the meta block on stderr, so a `while read` loop sees rows only.
func TestGenJSONLIsOneRowPerLine(t *testing.T) {
	w, body := genWorkbook(t, "2023-24")
	y, _ := genYearByLabel("2023-24")
	rows := genRows(w, genTestURL)
	meta := genBuildMeta(w, y, "plant-month", len(rows), body, genTestURL, "text/html", genRowsJSON(rows))

	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := genWrite(cmd, &rootFlags{}, genFormatJSONL, genRowHeader, nil, meta, rows); err != nil {
		t.Fatalf("genWrite: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 1596 {
		t.Fatalf("jsonl lines = %d, want 1596", len(lines))
	}
	for i, l := range lines {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(l), &obj); err != nil {
			t.Fatalf("line %d is not one JSON object: %v", i, err)
		}
		if len(obj) != 25 {
			t.Fatalf("line %d has %d keys, want 25", i, len(obj))
		}
	}
	var m genMeta
	if err := json.Unmarshal(errOut.Bytes(), &m); err != nil {
		t.Fatalf("stderr is not the meta block: %v", err)
	}
	if m.Rows != 1596 {
		t.Errorf("meta on stderr reports %d rows", m.Rows)
	}
}

// TestGenResolveFormat pins the first-match-wins order and that a bad value is
// refused rather than defaulted.
func TestGenResolveFormat(t *testing.T) {
	for _, tc := range []struct {
		name     string
		explicit string
		flags    rootFlags
		want     string
	}{
		{"explicit beats everything", "csv", rootFlags{asJSON: true, plain: true}, genFormatCSV},
		{"explicit tsv", "TSV", rootFlags{}, genFormatTSV},
		{"explicit jsonl", "jsonl", rootFlags{}, genFormatJSONL},
		{"--csv", "", rootFlags{csv: true}, genFormatCSV},
		{"--plain is tab-separated", "", rootFlags{plain: true}, genFormatTSV},
		{"--json", "", rootFlags{asJSON: true}, genFormatJSON},
		{"--csv beats --json", "", rootFlags{csv: true, asJSON: true}, genFormatCSV},
		{"piped stdout", "", rootFlags{}, genFormatJSON},
	} {
		got, err := genResolveFormat(tc.explicit, &tc.flags, &bytes.Buffer{})
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: format = %q, want %q", tc.name, got, tc.want)
		}
	}
	if _, err := genResolveFormat("xml", &rootFlags{}, &bytes.Buffer{}); err == nil {
		t.Error("--format xml was accepted")
	} else if !strings.Contains(err.Error(), "csv, tsv, json, jsonl") {
		t.Errorf("the error does not name the accepted values: %v", err)
	}
	// A bad value must NEVER silently become a default, because the caller
	// would not notice until the file was in a spreadsheet.
	if got, err := genResolveFormat("xml", &rootFlags{csv: true}, &bytes.Buffer{}); err == nil || got != "" {
		t.Errorf("--format xml fell back to %q", got)
	}
}

func TestGenResolveRollup(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""}, {"technology", genRollupTechnology}, {"TECHNOLOGY", genRollupTechnology},
		{" fuel ", genRollupFuel},
	} {
		got, err := genResolveRollup(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("genResolveRollup(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"plant", "province", "system", "month"} {
		if _, err := genResolveRollup(bad); err == nil {
			t.Errorf("--rollup %q was accepted", bad)
		}
	}
}

// TestGenCatalogueIsTheHelpOnlyBranch pins that a bare `gen` prints something
// useful and makes NO request.
func TestGenCatalogueIsTheHelpOnlyBranch(t *testing.T) {
	out, _, code := genRun(t, "gen", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var payload struct {
		Results []struct {
			FY              string `json:"fy"`
			Reachable       bool   `json:"reachable"`
			DecodedBytes    *int   `json:"decoded_bytes"`
			PlantRows       *int   `json:"plant_rows"`
			PlantRowsPinned bool   `json:"plant_rows_pinned"`
			Path            string `json:"path"`
			AsOfDate        string `json:"as_of_date"`
			Note            string `json:"note"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		// The catalogue may come back as a bare array depending on the
		// platform wrapper; accept that too.
		if err2 := json.Unmarshal([]byte(out), &payload.Results); err2 != nil {
			t.Fatalf("catalogue is neither an envelope nor an array: %v / %v\n%s", err, err2, out)
		}
	}
	if len(payload.Results) != 11 {
		t.Fatalf("catalogue entries = %d, want 11", len(payload.Results))
	}
	reachable := 0
	for _, e := range payload.Results {
		if e.AsOfDate != genAsOfDate {
			t.Errorf("FY%s as_of_date = %q", e.FY, e.AsOfDate)
		}
		if e.Note == "" {
			t.Errorf("FY%s has no note", e.FY)
		}
		if !e.Reachable {
			if e.DecodedBytes != nil || e.PlantRows != nil || e.Path != "" {
				t.Errorf("FY%s is unreachable but carries floors or a path", e.FY)
			}
			continue
		}
		reachable++
		if e.DecodedBytes == nil || e.PlantRows == nil {
			t.Errorf("FY%s is reachable with no floors", e.FY)
		}
		if !strings.Contains(e.Path, "Genenration") {
			t.Errorf("FY%s path lost the upstream typo: %s", e.FY, e.Path)
		}
	}
	if reachable != 7 {
		t.Errorf("reachable years = %d, want 7", reachable)
	}
}

// TestGenRefusesUnreachableYears pins exit 3 with no request, and that the
// message names the measured evidence and the escape hatch.
func TestGenRefusesUnreachableYears(t *testing.T) {
	for _, fy := range []string{"2024-25", "2016-17", "2025-26", "2015-16"} {
		_, errOut, code := genRun(t, "gen", "--fy", fy)
		if code != 3 {
			t.Errorf("--fy %s exit = %d, want 3", fy, code)
		}
		if !strings.Contains(errOut, "not published at this path") {
			t.Errorf("--fy %s message does not say it is unpublished: %s", fy, errOut)
		}
		if !strings.Contains(errOut, "generation year "+fy) {
			t.Errorf("--fy %s message does not name the escape hatch: %s", fy, errOut)
		}
		if !strings.Contains(errOut, genAsOfDate) {
			t.Errorf("--fy %s message does not date the measurement: %s", fy, errOut)
		}
	}
	// A parseable year the catalogue has never seen is also exit 3, not a
	// silent empty result.
	_, errOut, code := genRun(t, "gen", "--fy", "1999-00")
	if code != 3 {
		t.Errorf("--fy 1999-00 exit = %d, want 3", code)
	}
	if !strings.Contains(errOut, "outside the recorded window") {
		t.Errorf("message = %s", errOut)
	}
}

// TestGenUsageRefusals pins exit 2 on every bad input, with no request made.
func TestGenUsageRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"blank fy", []string{"gen", "--fy", ""}, "--fy is required"},
		{"whitespace fy", []string{"gen", "--fy", "   "}, "--fy is required"},
		{"unparseable fy", []string{"gen", "--fy", "twenty-twenty"}, "is not a fiscal year"},
		{"two-year span", []string{"gen", "--fy", "2020-25"}, "is not a fiscal year"},
		{"bad format", []string{"gen", "--fy", "2023-24", "--format", "xml"}, "not one of csv, tsv, json, jsonl"},
		{"bad rollup", []string{"gen", "--fy", "2023-24", "--rollup", "plant"}, "not one of technology, fuel"},
		{"local data source", []string{"gen", "--fy", "2023-24", "--data-source", "local"}, "no local data source"},
	} {
		_, errOut, code := genRun(t, tc.args...)
		if code != 2 {
			t.Errorf("%s: exit = %d, want 2 (%s)", tc.name, code, errOut)
		}
		if !strings.Contains(errOut, tc.want) {
			t.Errorf("%s: message %q does not contain %q", tc.name, errOut, tc.want)
		}
	}
	// A rollup selector with no --fy is a usage error, not the catalogue:
	// the caller asked for something specific.
	_, errOut, code := genRun(t, "gen", "--rollup", "technology")
	if code != 2 || !strings.Contains(errOut, "--fy is required") {
		t.Errorf("gen --rollup with no --fy: exit %d, %s", code, errOut)
	}
}

// TestGenDryRunMakesNoRequest pins the verify pipeline's probe. --dry-run with
// a year returns the writeDryRun envelope; --dry-run alone reaches the
// help-only branch first, which is the required ordering for this project and
// also makes no request.
func TestGenDryRunMakesNoRequest(t *testing.T) {
	out, _, code := genRun(t, "gen", "--fy", "2023-24", "--dry-run")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "dry-run") || !strings.Contains(out, "gen") {
		t.Errorf("dry-run output = %q", out)
	}
	// --dry-run must beat the usage errors, so a probe that supplies no
	// valid input still exits 0 rather than 2.
	_, _, code = genRun(t, "gen", "--rollup", "technology", "--dry-run")
	if code != 0 {
		t.Errorf("gen --rollup --dry-run exit = %d, want 0", code)
	}
	out, _, code = genRun(t, "gen", "--dry-run")
	if code != 0 {
		t.Fatalf("bare --dry-run exit = %d, want 0", code)
	}
	if !strings.Contains(out, "2023-24") {
		t.Errorf("bare --dry-run did not reach the catalogue: %q", out)
	}
}

// TestGenWrongFiscalYearIsRefused is the stale-copy decoy guard. NEPRA
// republishes byte-identical copies of one year's file under two years' names,
// so parsing with the caller's label is the only thing that stops a whole year
// being dated wrongly.
//
// MUTATION-CHECKED: passing "" as the fy argument makes ParseWorkbook accept
// whatever the band says and this test fails.
func TestGenWrongFiscalYearIsRefused(t *testing.T) {
	body := genFixture(t, "2020-21")
	_, err := nepraparse.ParseWorkbook(body, "2023-24")
	if err == nil {
		t.Fatal("the FY2020-21 bytes parsed as FY2023-24")
	}
	if !errors.Is(err, nepraparse.ErrFiscalYearMismatch) {
		t.Fatalf("err = %v, want ErrFiscalYearMismatch", err)
	}
	classified := genClassifyParseError(err, "2023-24", len(body))
	if code := ExitCode(classified); code != 5 {
		t.Errorf("exit = %d, want 5", code)
	}
	if !strings.Contains(classified.Error(), "stale-copy guard") {
		t.Errorf("the message does not explain what it caught: %v", classified)
	}
	// And the empty-label call, which is the mutation this guards against,
	// really does succeed — so the guard is load-bearing rather than
	// decorative.
	if _, err := nepraparse.ParseWorkbook(body, ""); err != nil {
		t.Fatalf("fy=\"\" should parse anything: %v", err)
	}
}

// TestGenSchemaDriftIsFatal pins that a renamed column stops the parse dead
// and that the per-column difference reaches the message.
func TestGenSchemaDriftIsFatal(t *testing.T) {
	body := genFixture(t, "2023-24")
	// Rename the "Fuel" header cell. A shifted or renamed column yields
	// numbers that still look plausible, which is why this is fatal rather
	// than a warning.
	drifted := bytes.Replace(body, []byte(">Fuel<"), []byte(">Feedstock<"), 1)
	if bytes.Equal(drifted, body) {
		t.Fatal("the fixture does not contain the Fuel header cell; this test is not testing anything")
	}
	_, err := nepraparse.ParseWorkbook(drifted, "2023-24")
	if err == nil {
		t.Fatal("a renamed column parsed cleanly")
	}
	if !errors.Is(err, nepraparse.ErrSchemaDrift) {
		t.Fatalf("err = %v, want ErrSchemaDrift", err)
	}
	classified := genClassifyParseError(err, "2023-24", len(drifted))
	if code := ExitCode(classified); code != 5 {
		t.Errorf("exit = %d, want 5", code)
	}
	msg := classified.Error()
	for _, want := range []string{"column 3", "Feedstock", "NO rows were emitted"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the message is missing %q: %s", want, msg)
		}
	}
}

// TestGenClassifyParseErrorCoversEveryFatal pins the exit code and the
// explanation for each fatal parse outcome.
func TestGenClassifyParseErrorCoversEveryFatal(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{nepraparse.ErrNoHeaderBand, "ZERO <th>"},
		{nepraparse.ErrNoTable, "9-byte body here is NEPRA's 404"},
		{nepraparse.ErrFiscalYear, "only in-document statement"},
	} {
		got := genClassifyParseError(fmt.Errorf("wrapped: %w", tc.err), "2023-24", 9)
		if code := ExitCode(got); code != 5 {
			t.Errorf("%v: exit = %d, want 5", tc.err, code)
		}
		if !strings.Contains(got.Error(), tc.want) {
			t.Errorf("%v: message missing %q: %v", tc.err, tc.want, got)
		}
	}
	// An unrecognised error is passed through unchanged, not relabelled as
	// a parse failure it is not.
	plain := errors.New("connection reset")
	if got := genClassifyParseError(plain, "2023-24", 0); !errors.Is(got, plain) || got.Error() != plain.Error() {
		t.Errorf("an unknown error was rewritten: %v", got)
	}
}

// TestGenReportPrintsAssertionsAndRefusals pins that the completeness block
// reaches stderr in every mode, and that a failure is loud.
func TestGenReportPrintsAssertionsAndRefusals(t *testing.T) {
	meta := genTestMeta(t, "2023-24")
	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	short := genReport(cmd, meta, genFormatCSV)
	if len(short) != 0 {
		t.Errorf("FY2023-24 reported shortfalls: %+v", short)
	}
	if out.Len() != 0 {
		t.Errorf("the report wrote to stdout, which must carry rows only: %q", out.String())
	}
	e := errOut.String()
	for _, want := range []string{
		"NO AGGREGATE UTILISATION IS EMITTED",
		"assert  plant_row_floor",
		"assert  pct_then_gwh_proved",
		"471 read exactly 0.00 and are MEASUREMENTS",
		"44686.0 MW/133 plants",
	} {
		if !strings.Contains(e, want) {
			t.Errorf("stderr is missing %q:\n%s", want, e)
		}
	}

	// A failing assertion prints a COMPLETENESS line naming the measured
	// value and its as-of date.
	broken := meta
	broken.Assertions = append([]genAssertion{}, meta.Assertions...)
	for i := range broken.Assertions {
		if broken.Assertions[i].Name == "plant_row_floor" {
			broken.Assertions[i].OK = false
		}
	}
	errOut.Reset()
	short = genReport(cmd, broken, genFormatCSV)
	if len(short) != 1 {
		t.Fatalf("shortfalls = %d, want 1", len(short))
	}
	if !strings.Contains(errOut.String(), "COMPLETENESS: plant_row_floor failed") {
		t.Errorf("no COMPLETENESS line:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Do not treat this extract as complete") {
		t.Errorf("the COMPLETENESS line does not say what it means:\n%s", errOut.String())
	}
}

// TestGenNoteCarriesUnmodelledText pins that unmodelled cell text travels into
// the row rather than being dropped or coerced.
func TestGenNoteCarriesUnmodelledText(t *testing.T) {
	rows := genTestRows(t, "2023-24")
	// FY2023-24's one residue cell is a stray ";" on Warsak, past logical
	// column 31. It is not a number and it is not dropped.
	got := genRowsFor(rows, "Warsak Hydropower Project (WAPDA)")
	if len(got) != 12 {
		t.Fatalf("Warsak rows = %d, want 12", len(got))
	}
	for _, r := range got {
		if !strings.Contains(r.Note, "past logical column 31") || !strings.Contains(r.Note, ";") {
			t.Errorf("Warsak %s note = %q, want the residue cell", r.Month, r.Note)
		}
	}
	// Every other FY2023-24 row has an EMPTY note, not a placeholder.
	empty := 0
	for _, r := range rows {
		if r.Note == "" {
			empty++
		}
	}
	if empty != 1596-12 {
		t.Errorf("%d rows have an empty note, want %d", empty, 1596-12)
	}
}

// TestGenUnmodelledBlockClassification pins the fifth row class against a
// synthetic copy of the FY2022-23 typo.
//
// FY2022-23 publishes the bare word "DELICENSE" — no trailing D — in a
// <td colspan=26> for two plants. nepraparse models DELICENSED only, so those
// 52 cells are unknown_text, and WITHOUT the fifth class the two rows come out
// "active": two plants that look like they reported, with a null wherever a
// number belongs. That year has no committed fixture, so the case is
// reproduced here by editing the FY2023-24 bytes the same way NEPRA did.
//
// MUTATION-CHECKED: deleting the unmodelled-block step makes both rows read
// "active" and fails this test.
func TestGenUnmodelledBlockClassification(t *testing.T) {
	body := genFixture(t, "2023-24")
	typo := bytes.Replace(body, []byte(">DELICENSED<"), []byte(">DELICENSE<"), 1)
	if bytes.Equal(typo, body) {
		t.Fatal("the fixture has no DELICENSED cell to mistype; this test is not testing anything")
	}
	w, err := nepraparse.ParseWorkbook(typo, "2023-24")
	if err != nil {
		t.Fatalf("ParseWorkbook: %v", err)
	}
	classes := genCountRowClasses(w)
	if classes.UnmodelledBlock != 1 {
		t.Fatalf("unmodelled_block = %d, want 1 (classes %+v)", classes.UnmodelledBlock, classes)
	}
	if classes.Delicensed != 11 {
		t.Errorf("delicensed = %d, want 11 after one row was mistyped", classes.Delicensed)
	}
	if classes.Total() != len(w.Plants) {
		t.Errorf("classes total %d, want %d", classes.Total(), len(w.Plants))
	}

	var affected []genRow
	for _, r := range genRows(w, genTestURL) {
		if r.RowClass == genRowClassUnmodelledBlock {
			affected = append(affected, r)
		}
	}
	if len(affected) != 12 {
		t.Fatalf("unmodelled_block rows = %d, want 12", len(affected))
	}
	for _, r := range affected {
		if r.GWh != nil || r.GWhState != "unknown_text" {
			t.Errorf("%s %s: gwh = %v/%q, want nil/unknown_text", r.Plant, r.Month, r.GWh, r.GWhState)
		}
		// The verbatim text travels. It is NOT normalised to DELICENSED.
		if !strings.Contains(r.Note, `"DELICENSE"`) {
			t.Errorf("%s %s: note = %q, want the verbatim text", r.Plant, r.Month, r.Note)
		}
		if strings.Contains(r.GWhState, "delicensed") {
			t.Errorf("%s %s: the typo was normalised to a known sentinel", r.Plant, r.Month)
		}
	}

	y, _ := genYearByLabel("2023-24")
	rows := genRows(w, genTestURL)
	meta := genBuildMeta(w, y, "plant-month", len(rows), typo, genTestURL, "text/html", genRowsJSON(rows))
	if len(meta.Warnings) == 0 || !strings.Contains(meta.Warnings[0], "UNMODELLED BLOCK") {
		t.Fatalf("the first warning must name the unmodelled block: %v", meta.Warnings)
	}
	if !strings.Contains(meta.Warnings[0], `"DELICENSE"`) {
		t.Errorf("the warning does not quote the text: %q", meta.Warnings[0])
	}
	// The capacity of an unmodelled-block row is real and stays in its own
	// bucket rather than being folded into non_operating.
	if u := meta.CapacityMW.UnmodelledBlock; !u.Measured || u.MW == nil || u.Plants != 1 {
		t.Errorf("unmodelled_block capacity = %+v, want a measured sum over 1 plant", u)
	}
	if ok, checkable := genCapacityIdentity(meta.CapacityMW); !checkable || !ok {
		t.Errorf("capacity identity checkable=%t ok=%t after the reclassification", checkable, ok)
	}
}

// TestGenStrictIsTheExitCodeContract pins that --strict turns a completeness
// failure into a non-zero exit while leaving the assertions on stderr either
// way, which is what makes this command usable as a cron gate.
//
// The RunE path needs a network, so the two halves are pinned separately: that
// genReport finds the shortfall regardless of --strict (above), and that the
// plain error --strict returns maps to exit 1 rather than to a typed code that
// would read as a usage or not-found problem.
func TestGenStrictIsTheExitCodeContract(t *testing.T) {
	plain := fmt.Errorf("%d of %d completeness assertions failed for FY2023-24", 1, 10)
	if code := ExitCode(plain); code != 1 {
		t.Errorf("a --strict failure exits %d, want 1", code)
	}
	// The three typed codes must stay distinguishable from it and from each
	// other, because the whole point of the annotation is that a caller can
	// branch on them.
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"usage", usageErr(errors.New("x")), 2},
		{"not found", notFoundErr(errors.New("x")), 3},
		{"fatal parse", apiErr(errors.New("x")), 5},
	} {
		if code := ExitCode(tc.err); code != tc.want {
			t.Errorf("%s exits %d, want %d", tc.name, code, tc.want)
		}
	}
}

// TestGenMWStringNeverPrintsAnUnmeasuredNumber pins the human rendering of a
// capacity sum. An unmeasured sum must not render as "0.0 MW".
func TestGenMWStringNeverPrintsAnUnmeasuredNumber(t *testing.T) {
	zero := 0.0
	measured := genMW{Measured: true, MW: &zero, Plants: 0}
	if got := genMWString(measured); !strings.Contains(got, "0.0 MW/0 plants") {
		t.Errorf("a measured zero renders as %q, want a number", got)
	}
	unmeasured := genMW{Measured: false, Plants: 0, PlantsNotNumeric: 11}
	got := genMWString(unmeasured)
	if strings.Contains(got, "0.0 MW") || !strings.Contains(got, "not measured") {
		t.Errorf("an unmeasured sum renders as %q; it must not look like a number", got)
	}
	if !strings.Contains(got, "11 plants") {
		t.Errorf("the unmeasured rendering hides its population: %q", got)
	}
	partial := genMW{Measured: true, MW: &zero, Plants: 97, PlantsNotNumeric: 11}
	if got := genMWString(partial); !strings.Contains(got, "+11 plants whose capacity cell is not a number") {
		t.Errorf("a partial sum hides its shortfall: %q", got)
	}
}
