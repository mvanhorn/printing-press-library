// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// These tests cover the parts of `gen` that the parser-level tests in
// gen_test.go never reach: the RunE pipeline itself (fetch -> parse -> write
// rows -> report), and the two completeness mechanics that only misbehave on
// inputs the three committed fixtures never produce.
//
// Every number asserted here was MEASURED by running this code against the
// committed FY2023-24 fixture, which is the published bytes verbatim:
// 133 plants, 1,596 plant-month rows, 493,187 decoded bytes.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// genServeFixture serves one fiscal year's published bytes from a local
// server and points the CLI's base URL at it, so the whole command runs
// against the real document over a real HTTP round trip.
func genServeFixture(t *testing.T, fy string) *string {
	t.Helper()
	body := genFixture(t, fy)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("NEPRA_BASE_URL", srv.URL)
	return &gotPath
}

// genRunLive drives the real root command end to end over the network (the
// local fixture server) and returns stdout, stderr and the RunE error.
// It is the network-permitting twin of genRun, which fails on any request.
func genRunLive(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	err := cmd.Execute()
	return out.String(), errBuf.String(), err
}

// TestGenLiveRunEmitsEveryRow pins that a successful fetch actually produces
// the rows. Nothing else in this package runs the command's fetch step, so a
// pipeline that returned early after building its client — with exit 0 and an
// empty stdout — would otherwise look exactly like success.
func TestGenLiveRunEmitsEveryRow(t *testing.T) {
	gotPath := genServeFixture(t, "2023-24")

	stdout, stderr, err := genRunLive(t, "gen", "--fy", "2023-24", "--format", "csv", "--no-cache")
	if err != nil {
		t.Fatalf("gen --fy 2023-24: %v (stderr %q)", err, stderr)
	}
	if stdout == "" {
		t.Fatal("gen exited 0 and wrote nothing to stdout; the rows ARE the product")
	}
	if want := "/publications/State of Industry Reports/Detail of Generation/" +
		"List of Companies Genenration wise 2023-24_files/sheet001.htm"; *gotPath != want {
		t.Errorf("requested path = %q, want %q", *gotPath, want)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if got, want := lines[0], strings.Join(genRowHeader, ","); got != want {
		t.Errorf("csv header = %q, want %q", got, want)
	}
	// 133 plants x 12 months, measured from the committed fixture.
	if got, want := len(lines)-1, 1596; got != want {
		t.Errorf("gen wrote %d data rows, want %d", got, want)
	}
	if got, want := lines[1], "2023-24,1,Tarbela Hydropower Project (WAPDA),"; !strings.HasPrefix(got, want) {
		t.Errorf("first data row = %q, want prefix %q", got, want)
	}
}

// TestGenReportFollowsTheRows pins the second half of step 7: the rows are
// written first and the completeness commentary follows on stderr. A run that
// stopped once the rows were on stdout would still print 1,596 correct rows
// and exit 0, while silently dropping every completeness claim the command
// exists to make.
func TestGenReportFollowsTheRows(t *testing.T) {
	genServeFixture(t, "2023-24")

	stdout, stderr, err := genRunLive(t, "gen", "--fy", "2023-24", "--format", "csv", "--no-cache")
	if err != nil {
		t.Fatalf("gen --fy 2023-24: %v (stderr %q)", err, stderr)
	}
	if n := len(strings.Split(strings.TrimRight(stdout, "\n"), "\n")) - 1; n != 1596 {
		t.Fatalf("precondition: %d data rows on stdout, want 1596", n)
	}

	// The one-screen summary, measured against the fixture.
	for _, want := range []string{
		"FY2023-24  plant-month  windows-1252",
		"1596 rows from 133 plants  493187 bytes",
		"row classes: 118 active, 12 delicensed, 1 decommissioned, 2 listed_no_data",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr is missing the summary line %q; got:\n%s", want, stderr)
		}
	}
	// And every assertion, named, after the rows.
	for _, name := range []string{
		"plant_row_floor", "decoded_bytes_floor", "schema_matches", "census_balanced",
		"pct_then_gwh_proved", "rows_equal_plants_times_12", "row_classes_account_for_every_plant",
		"every_reconcilable_plant_is_active", "status_rows_year_bounded",
		"capacity_classes_exhaust_published_total",
	} {
		if !strings.Contains(stderr, "assert  "+name) {
			t.Errorf("stderr never reported assertion %q; got:\n%s", name, stderr)
		}
	}
	if n := strings.Count(stderr, "assert  "); n != 10 {
		t.Errorf("stderr carries %d assertion lines, want 10", n)
	}
}

// genFixtureAssertions builds the completeness block for a fiscal year with
// the row and byte counts the caller names, so the failure arms of the block
// can be exercised on inputs the three clean fixtures never produce.
func genFixtureAssertions(t *testing.T, fy, grain string, rows, bodyBytes int) []genAssertion {
	t.Helper()
	w, _ := genWorkbook(t, fy)
	y, ok := genYearByLabel(fy)
	if !ok {
		t.Fatalf("FY%s is not catalogued", fy)
	}
	return genAssertions(w, y, grain, rows, bodyBytes, w.ColumnOrder(0))
}

func genAssertionByName(t *testing.T, as []genAssertion, name string) genAssertion {
	t.Helper()
	for _, a := range as {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("no assertion named %q in %d assertions", name, len(as))
	return genAssertion{}
}

// TestGenRowsEqualPlantsTimes12IsARealCheck pins that the plant-month row
// count is actually compared, and that the same entry asserts NOTHING at the
// rollup grain instead of reporting a comparison it did not make.
//
// On all three fixtures the count holds, so an entry that simply returned OK
// at the plant-month grain would pass the whole suite while permitting exactly
// the silent drop it is there to catch: a delicensed plant losing its twelve
// rows.
func TestGenRowsEqualPlantsTimes12IsARealCheck(t *testing.T) {
	// 133 plants x 12 = 1,596. Anything less at this grain is a dropped plant.
	short := genAssertionByName(t, genFixtureAssertions(t, "2023-24", "plant-month", 1584, 493187),
		"rows_equal_plants_times_12")
	if !short.Asserts {
		t.Error("at the plant-month grain the row-count identity must assert")
	}
	if short.OK {
		t.Errorf("1584 rows against 133 plants must FAIL the row-count identity, got ok (%s)", genAssertionValues(short))
	}
	if short.Expected == nil || *short.Expected != 1596 || short.Actual == nil || *short.Actual != 1584 {
		t.Errorf("expected/actual = %v/%v, want 1596/1584", short.Expected, short.Actual)
	}
	if got := genShortfalls([]genAssertion{short}); len(got) != 1 {
		t.Error("a failed row-count identity must reach the shortfall list")
	}

	// Same numbers, rollup grain: 9 technology groups x 12 = 108 rows. The
	// plant-month identity does not hold there and must not be claimed.
	roll := genAssertionByName(t, genFixtureAssertions(t, "2023-24", "group-month", 108, 493187),
		"rows_equal_plants_times_12")
	if roll.Asserts {
		t.Error("at the group-month grain the plant-month row identity asserts nothing and must say so")
	}
	if !roll.OK {
		t.Error("an entry that asserts nothing must not be reported as a failure")
	}
	if got := genShortfalls([]genAssertion{roll}); len(got) != 0 {
		t.Error("an unasserted entry must never become a shortfall")
	}
}

// TestGenCompletenessCarriesMeasurementProvenance pins the provenance half of
// the COMPLETENESS line: a floor measured on a recorded date says so, and a
// check with no recorded date says "this run" rather than printing an empty
// date. Both fixtures pass every assertion, so no test in the package ever
// reaches this loop.
func TestGenCompletenessCarriesMeasurementProvenance(t *testing.T) {
	// A short body fails the byte floor, which carries the date the floor was
	// measured on; a short row count fails the row-count identity, which has
	// no recorded date because it is computed from the bytes in hand.
	as := genFixtureAssertions(t, "2023-24", "plant-month", 1584, 1024)
	short := genShortfalls(as)
	if len(short) != 2 {
		names := []string{}
		for _, a := range short {
			names = append(names, a.Name)
		}
		t.Fatalf("precondition: %d shortfalls %v, want decoded_bytes_floor and rows_equal_plants_times_12", len(short), names)
	}

	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&errBuf)
	// jsonl keeps genWriteSummary out of the way: this test is about the
	// COMPLETENESS lines only.
	if got := genReport(cmd, genMeta{Assertions: as}, genFormatJSONL); len(got) != 2 {
		t.Fatalf("genReport returned %d shortfalls, want 2", len(got))
	}

	lines := map[string]string{}
	for _, l := range strings.Split(errBuf.String(), "\n") {
		if !strings.HasPrefix(l, "COMPLETENESS: ") {
			continue
		}
		lines[strings.Fields(l)[1]] = l
	}
	if len(lines) != 2 {
		t.Fatalf("got %d COMPLETENESS lines, want 2; stderr:\n%s", len(lines), errBuf.String())
	}

	dated := lines["decoded_bytes_floor"]
	if want := fmt.Sprintf("Measured on %s.", genAsOfDate); !strings.Contains(dated, want) {
		t.Errorf("the byte floor was measured on a recorded date and must cite it: want %q in %q", want, dated)
	}
	undated := lines["rows_equal_plants_times_12"]
	if want := "Measured on this run."; !strings.Contains(undated, want) {
		t.Errorf("a check with no recorded date must say %q, got %q", want, undated)
	}
	if strings.Contains(errBuf.String(), "Measured on .") {
		t.Errorf("a COMPLETENESS line printed an empty measurement date:\n%s", errBuf.String())
	}
	// The dated and the undated line must not collapse into each other.
	if strings.Contains(dated, "this run") {
		t.Errorf("a dated floor must not be relabelled as measured on this run: %q", dated)
	}
}

// TestGenUtilisationRefusalStatesTheIdentityItRestsOn pins the headline
// refusal's reasoning against the verdict the parse actually measured.
//
// The refusal declines to emit any aggregate utilisation, and its whole
// argument is that the annual-total IDENTITY holds on GWh and fails on the
// percentage column: additive on one, nonsense on the other. The counts in
// that sentence are interpolated from the measured column-order verdict, so
// the sentence must state the identity as an equality and carry the measured
// counts — a refusal whose stated arithmetic contradicts the file it just
// read is not a reason, it is noise.
func TestGenUtilisationRefusalStatesTheIdentityItRestsOn(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")
	order := w.ColumnOrder(0)

	// Measured on the committed FY2023-24 fixture: the annual-total identity
	// holds on every eligible GWh row and on almost none of the percentage
	// rows, which is what makes one additive and the other a ratio.
	if order.GWh.Passed != 118 || order.GWh.Eligible != 118 {
		t.Errorf("measured GWh identity = %d/%d, want 118/118", order.GWh.Passed, order.GWh.Eligible)
	}
	if order.Pct.Passed != 3 || order.Pct.Eligible != 118 {
		t.Errorf("measured %%-age identity = %d/%d, want 3/118", order.Pct.Passed, order.Pct.Eligible)
	}

	refusals := genTestMeta(t, "2023-24").Refusals
	if len(refusals) == 0 {
		t.Fatal("gen emitted no refusals")
	}
	joined := strings.Join(refusals, "\n")

	want := fmt.Sprintf("Sum == sum(12 months) holds %d/%d on GWh and only %d/%d on %q",
		order.GWh.Passed, order.GWh.Eligible, order.Pct.Passed, order.Pct.Eligible, nepraparse.MetricPct)
	if !strings.Contains(joined, want) {
		t.Errorf("the utilisation refusal does not state the identity it rests on.\nwant substring: %s\ngot:\n%s", want, joined)
	}
	// The claim is that the identity HOLDS on GWh. Stating the negation would
	// argue the opposite of what was measured.
	if strings.Contains(joined, "Sum != sum(12 months) holds") {
		t.Errorf("the refusal asserts the NEGATION of the identity it measured:\n%s", joined)
	}
}
