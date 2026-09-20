// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
)

// TestNovelDiscoHelpWires smoke-tests that the disco command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDiscoHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"disco", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("disco --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "disco"} {
		if !strings.Contains(help, want) {
			t.Fatalf("disco --help missing %q in output:\n%s", want, help)
		}
	}
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

type discoRoundTripFunc func(*http.Request) (*http.Response, error)

func (f discoRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// discoTestRoundTrip, when non-nil, replaces every constructed client's
// transport. It is how a test proves the command made NO request: the hook is
// registered once for the whole test binary and reads this variable each time.
var discoTestRoundTrip func(*http.Request) (*http.Response, error)

func init() {
	registerClientHook(func(c *client.Client) error {
		if discoTestRoundTrip != nil && c != nil && c.HTTPClient != nil {
			c.HTTPClient.Transport = discoRoundTripFunc(discoTestRoundTrip)
		}
		return nil
	})
}

// discoForbidRequests installs a transport that records any attempted request
// and fails it. A command that must not touch the network is verified by the
// recorded count, not by inference from an exit code.
func discoForbidRequests(t *testing.T) *int {
	t.Helper()
	n := 0
	discoTestRoundTrip = func(r *http.Request) (*http.Response, error) {
		n++
		t.Errorf("unexpected HTTP request to %s: this command must make none", r.URL)
		return nil, io.ErrUnexpectedEOF
	}
	t.Cleanup(func() { discoTestRoundTrip = nil })
	return &n
}

func runDisco(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := RootCmd()
	var so, se bytes.Buffer
	cmd.SetOut(&so)
	cmd.SetErr(&se)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err != nil {
		code = ExitCode(err)
		// Cobra's own printing is suppressed by SilenceUsage/SilenceErrors in
		// places, so append the message the caller would have seen.
		se.WriteString("\n" + err.Error())
	}
	return so.String(), se.String(), code
}

// discoSyntheticPDF writes the committed gzipped fixture out as a real PDF so
// the --pdf path can be driven end to end with no network.
func discoSyntheticPDF(t *testing.T) string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "per-synthetic.pdf.gz"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "per-synthetic.pdf")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func discoDecode(t *testing.T, out string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("stdout is not a JSON object: %v\n%s", err, out)
	}
	return v
}

// discoCatalogueSections returns the catalogue's named sections, reaching
// through the standard {meta, results} envelope.
//
// It is separate from discoDecode because disco emits BOTH shapes: the
// catalogue is a report (results is a keyed object of sections — report_years,
// metrics, excluded_entities — the same shape verify, fleet and conflicts use)
// while a panel read returns rows (results is an array). The catalogue used to
// emit its sections as a bare top-level object with no envelope at all, which
// is the defect this reaches through; a single shared unwrap would break every
// row caller.
func discoCatalogueSections(t *testing.T, out string) map[string]any {
	t.Helper()
	doc := discoDecode(t, out)
	sections, ok := doc["results"].(map[string]any)
	if !ok {
		t.Fatalf("disco catalogue did not emit {meta, results} with results as the keyed "+
			"section object; results is %T\n%s", doc["results"], truncate(out, 400))
	}
	return sections
}

// ---------------------------------------------------------------------------
// Catalogue
// ---------------------------------------------------------------------------

// TestDiscoCatalogueMakesNoRequest pins the no-selector branch: it must answer
// from the recorded corpus alone.
func TestDiscoCatalogueMakesNoRequest(t *testing.T) {
	calls := discoForbidRequests(t)
	stdout, _, code := runDisco(t, "disco")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if *calls != 0 {
		t.Fatalf("%d HTTP request(s) made; want 0", *calls)
	}
	cat := discoCatalogueSections(t, stdout)
	years, ok := cat["report_years"].([]any)
	if !ok || len(years) != 8 {
		t.Fatalf("report_years = %v, want 8 entries", cat["report_years"])
	}
	if cat["request_made"] != false {
		t.Errorf("request_made = %v, want false", cat["request_made"])
	}
	sawUnavailable, sawTrailingSpace := false, false
	for _, y := range years {
		e := y.(map[string]any)
		if e["report_fy"] == "FY2023-24" {
			if e["availability"] != "unavailable" {
				t.Errorf("FY2023-24 availability = %v, want unavailable", e["availability"])
			}
			sawUnavailable = true
		}
		if e["report_fy"] == "FY2020-21" {
			// The trailing encoded space is load-bearing: the same URL
			// without it returns 404. A "cleaned" path fails here.
			if u, _ := e["url"].(string); !strings.HasSuffix(u, "Companies%20.pdf") {
				t.Errorf("FY2020-21 url = %q, want a trailing %%20 before .pdf", u)
			}
			sawTrailingSpace = true
		}
	}
	if !sawUnavailable || !sawTrailingSpace {
		t.Fatalf("catalogue did not carry both the unavailable year and the trailing-space URL")
	}
	// The report years NEPRA has and this build does not must be NAMED, so a
	// caller is never told a document does not exist.
	unrec, ok := cat["reachable_but_unrecorded_report_years"].([]any)
	if !ok || len(unrec) < 5 {
		t.Fatalf("reachable_but_unrecorded_report_years = %v, want at least 5", cat["reachable_but_unrecorded_report_years"])
	}
}

// TestDiscoAnnotationsSatisfyThePublishGate pins the four annotations and the
// absence of the scaffold marker. Leaving pp:novel-scaffold on makes
// addNovelCommandIfAbsent treat the implementation as a stub.
func TestDiscoAnnotationsSatisfyThePublishGate(t *testing.T) {
	cmd, _, err := RootCmd().Find([]string{"disco"})
	if err != nil {
		t.Fatalf("Find(disco): %v", err)
	}
	want := map[string]string{
		"mcp:read-only":       "true",
		"pp:happy-args":       "--metric=saidi;--fy=2024-25",
		"pp:typed-exit-codes": "0,2,3",
		"pp:novel-hand-coded": "true",
	}
	for k, v := range want {
		if got := cmd.Annotations[k]; got != v {
			t.Errorf("annotation %s = %q, want %q", k, got, v)
		}
	}
	if _, bad := cmd.Annotations["pp:novel-scaffold"]; bad {
		t.Error("pp:novel-scaffold is still set; addNovelCommandIfAbsent will treat this as a TODO stub")
	}
	// pp:typed-exit-codes must agree with the command's own help.
	for _, code := range []string{"0", "2", "3"} {
		if !regexp.MustCompile(`(?m)^\s+` + code + `\s+\S`).MatchString(cmd.Long) {
			t.Errorf("exit code %s is declared in the annotation but not documented in Long", code)
		}
	}
}

// ---------------------------------------------------------------------------
// Refusals
// ---------------------------------------------------------------------------

// TestDiscoRefusesChartVariant pins that a variant the parser cannot assign is
// refused with its measured reason rather than silently mapped onto headline.
func TestDiscoRefusesChartVariant(t *testing.T) {
	calls := discoForbidRequests(t)
	_, stderr, code := runDisco(t, "disco", "--metric", "saifi", "--variant", "chart")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if *calls != 0 {
		t.Fatalf("%d HTTP request(s) made; want 0", *calls)
	}
	for _, want := range []string{
		"headline", "five_year_comparison", "with_lt_interruptions", "without_lt_interruptions",
		"15.39, 49.49, 28.16, 68.46", "15.39, 49.49, 28.61, 68.64",
		"15.31, 49.41, 28.16, 68.64", "15.31, 49.41, 28.61, 68.64",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("chart refusal is missing %q:\n%s", want, stderr)
		}
	}
}

// TestDiscoFY2023_24IsUnavailableNotEmpty pins the difference between "the
// document could not be obtained" and "the report says nothing". No request is
// made, the recorded 404 reason and URL are printed, the second-hand route
// through a later report's comparison columns is named, and the exit is 3.
func TestDiscoFY2023_24IsUnavailableNotEmpty(t *testing.T) {
	calls := discoForbidRequests(t)
	stdout, _, code := runDisco(t, "disco", "--metric", "saidi", "--fy", "2023-24")
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
	if *calls != 0 {
		t.Fatalf("%d HTTP request(s) made; want 0 (the refusal is decided from the corpus record)", *calls)
	}
	payload := discoDecode(t, stdout)
	un, ok := payload["unavailable"].(map[string]any)
	if !ok {
		t.Fatalf("no unavailable block:\n%s", stdout)
	}
	if un["kind"] != "unavailable" {
		t.Errorf("kind = %v, want unavailable", un["kind"])
	}
	if r, _ := un["reason"].(string); !strings.Contains(r, "404") {
		t.Errorf("reason does not mention 404: %q", r)
	}
	if u, _ := un["url"].(string); !strings.Contains(u, "PER%20DISCOs%202023-24.pdf") {
		t.Errorf("url = %q, want the recorded PER path", u)
	}
	if r, _ := un["second_hand_route"].(string); !strings.Contains(r, "--variant comparison") ||
		!strings.Contains(r, "--period 2023-24") {
		t.Errorf("second_hand_route = %q, want the comparison-column route", r)
	}
	if res, ok := payload["results"].([]any); !ok || len(res) != 0 {
		t.Errorf("results = %v, want an empty array alongside the refusal", payload["results"])
	}
	meta := payload["meta"].(map[string]any)
	if meta["request_made"] != false {
		t.Errorf("meta.request_made = %v, want false", meta["request_made"])
	}
	// Nothing was fetched, so nothing may be reported as measured.
	if meta["document"] != nil {
		t.Errorf("meta.document = %v, want null: a zero-valued block would claim a measurement nobody took", meta["document"])
	}
	c := meta["completeness"].(map[string]any)
	if c["bytes_match"] != nil || c["pages_match"] != nil {
		t.Errorf("bytes_match/pages_match = %v/%v, want null/null", c["bytes_match"], c["pages_match"])
	}
}

// TestDiscoFlagsAndExitCodes pins every usage refusal, each with the accepted
// values in its message. All are decided without a request.
func TestDiscoFlagsAndExitCodes(t *testing.T) {
	calls := discoForbidRequests(t)
	cases := []struct {
		name  string
		args  []string
		code  int
		wants []string
	}{
		{"unknown metric", []string{"--metric", "bogus"}, 2,
			[]string{"tnd", "recovery", "saifi", "saidi", "complaints", "safety", "all"}},
		{"raw nepraper key refused", []string{"--metric", "td_losses"}, 2,
			[]string{"not internal metric keys", "--metric tnd"}},
		{"unknown variant", []string{"--metric", "saidi", "--variant", "bogus"}, 2,
			[]string{"headline", "comparison", "with-lt", "without-lt"}},
		{"unrecorded fy names it as upstream-reachable", []string{"--metric", "saidi", "--fy", "2015-16"}, 2,
			[]string{"FY2015-16", "reached upstream", "no measured byte or page count", "--pdf"}},
		{"negative limit", []string{"--metric", "saidi", "--limit", "-1"}, 2,
			[]string{"--limit must be zero or positive"}},
		{"data-source local", []string{"--metric", "saidi", "--data-source", "local"}, 2,
			[]string{"no local data source"}},
		{"unknown entity", []string{"--metric", "saidi", "--entity", "ZESCO"}, 2,
			[]string{"PESCO", "K-Electric", "TESCO", "BTPL"}},
		{"weighted average is not an entity", []string{"--metric", "saidi", "--entity", "W.Av"}, 2,
			[]string{"weighted_average", "not a licensee"}},
		{"Total resolves to the summary row", []string{"--metric", "saidi", "--entity", "Total"}, 2,
			[]string{"weighted_average"}},
		{"bad period", []string{"--metric", "saidi", "--period", "nope"}, 2,
			[]string{"is not a fiscal year"}},
		{"positional arg", []string{"--metric", "saidi", "extra"}, 2,
			[]string{"takes no positional arguments"}},
		{"selector without metric", []string{"--entity", "MEPCO"}, 2,
			[]string{"--metric is required"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runDisco(t, append([]string{"disco"}, tc.args...)...)
			if code != tc.code {
				t.Fatalf("exit = %d, want %d\n%s", code, tc.code, stderr)
			}
			for _, w := range tc.wants {
				if !strings.Contains(stderr, w) {
					t.Errorf("message is missing %q:\n%s", w, stderr)
				}
			}
		})
	}
	if *calls != 0 {
		t.Fatalf("%d HTTP request(s) made; want 0 (validation precedes every fetch)", *calls)
	}
}

// TestDiscoDryRunMakesNoRequest pins the verify harness's probe: the guard sits
// after the pure validation and before the fetch.
func TestDiscoDryRunMakesNoRequest(t *testing.T) {
	calls := discoForbidRequests(t)
	stdout, _, code := runDisco(t, "disco", "--metric", "saidi", "--fy", "2024-25", "--dry-run", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if *calls != 0 {
		t.Fatalf("%d HTTP request(s) made under --dry-run; want 0", *calls)
	}
	// A silent return leaves a --json caller with empty stdout, which reads as
	// a broken command rather than a deliberate no-op.
	if !strings.Contains(stdout, "\"dry_run\": true") && !strings.Contains(stdout, `"dry_run":true`) {
		t.Fatalf("dry-run wrote no payload:\n%q", stdout)
	}
}

// ---------------------------------------------------------------------------
// End to end over the committed PDF, offline
// ---------------------------------------------------------------------------

// TestDiscoPanelFromSyntheticPDF drives ExtractText -> ParseReliability ->
// panel over the one committed PDF, with no network. It pins that actual,
// target and breach survive as three separate numeric fields, and that a
// five-year comparison row becomes five rows with five distinct periods whose
// target and breach are ABSENT by the table's construction.
func TestDiscoPanelFromSyntheticPDF(t *testing.T) {
	calls := discoForbidRequests(t)
	path := discoSyntheticPDF(t)

	stdout, _, code := runDisco(t, "disco", "--metric", "saidi", "--pdf", path,
		"--fy", "2018-19", "--variant", "headline", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if *calls != 0 {
		t.Fatalf("%d HTTP request(s) made with --pdf; want 0", *calls)
	}
	p := discoDecode(t, stdout)
	meta := p["meta"].(map[string]any)
	if meta["source"] != "file" {
		t.Errorf("meta.source = %v, want file", meta["source"])
	}
	if meta["request_made"] != false {
		t.Errorf("meta.request_made = %v, want false", meta["request_made"])
	}
	rows := p["results"].([]any)
	var pesco map[string]any
	for _, r := range rows {
		m := r.(map[string]any)
		if m["entity"] == "PESCO" {
			pesco = m
		}
	}
	if pesco == nil {
		t.Fatalf("no PESCO row in %d results", len(rows))
	}
	// Three SEPARATE quantities. Writing the target into the actual field, or
	// emitting one merged row, fails here.
	for field, wantNum := range map[string]float64{"actual": 16696.51, "target": 17358.60, "breach": 0} {
		v := pesco[field].(map[string]any)
		if v["kind"] != "numeric" {
			t.Fatalf("PESCO %s kind = %v, want numeric", field, v["kind"])
		}
		if got := v["value"].(float64); got != wantNum {
			t.Errorf("PESCO %s = %v, want %v", field, got, wantNum)
		}
	}
	// A real published 0.00 breach must stay numeric, not become absent.
	if raw := pesco["breach"].(map[string]any)["raw"]; raw != "0.00" {
		t.Errorf("PESCO breach raw = %v, want \"0.00\" (a met target is a real zero)", raw)
	}

	// The comparison leg: one printed row becomes five rows, one per period.
	stdout, _, code = runDisco(t, "disco", "--metric", "saidi", "--pdf", path,
		"--fy", "2018-19", "--variant", "comparison", "--entity", "MEPCO", "--json")
	if code != 0 {
		t.Fatalf("comparison exit = %d, want 0", code)
	}
	p = discoDecode(t, stdout)
	rows = p["results"].([]any)
	if len(rows) != 5 {
		t.Fatalf("MEPCO comparison rows = %d, want 5", len(rows))
	}
	wantByPeriod := map[string]float64{
		"FY2020-21": 39.733, "FY2021-22": 2794, "FY2022-23": 4723.73,
		"FY2023-24": 3726.61, "FY2024-25": 1182.56,
	}
	seen := map[string]bool{}
	for _, r := range rows {
		m := r.(map[string]any)
		period := m["period_fy"].(string)
		want, ok := wantByPeriod[period]
		if !ok {
			t.Fatalf("unexpected period %q", period)
		}
		if seen[period] {
			t.Fatalf("period %q emitted twice", period)
		}
		seen[period] = true
		if got := m["actual"].(map[string]any)["value"].(float64); got != want {
			t.Errorf("%s actual = %v, want %v", period, got, want)
		}
		// A comparison table publishes no target column. That is the table's
		// SHAPE, not a missing figure, and it must never read as 0.
		for _, field := range []string{"target", "breach"} {
			v := m[field].(map[string]any)
			if v["kind"] != "absent" {
				t.Errorf("%s %s kind = %v, want absent", period, field, v["kind"])
			}
			if _, hasNum := v["value"]; hasNum {
				t.Errorf("%s %s carries a value key; an absent cell must not serialise a number", period, field)
			}
		}
		if m["restated"] != true {
			t.Errorf("%s restated = %v, want true (the report is FY2018-19)", period, m["restated"])
		}
	}
	if len(seen) != 5 {
		t.Fatalf("distinct periods = %d, want 5", len(seen))
	}
}

// TestDiscoAssertsBytesAndPages is the only in-band defence against the
// measured corrupt-PDF shape (a 3,952,485-byte body from a 3,024,893-byte
// source that file(1) still validated as a PDF). The synthetic fixture stands
// in for one: 27,046 bytes over 2 pages against FY2018-19's recorded
// 3,024,893 bytes over 28 pages.
func TestDiscoAssertsBytesAndPages(t *testing.T) {
	discoForbidRequests(t)
	path := discoSyntheticPDF(t)
	args := []string{"disco", "--metric", "saidi", "--pdf", path, "--fy", "2018-19", "--json"}

	stdout, stderr, code := runDisco(t, args...)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 without --strict\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "COMPLETENESS") {
		t.Fatalf("no COMPLETENESS line on stderr; the shortfall must never be silent:\n%s", stderr)
	}
	for _, want := range []string{"3024893", "28"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not name the measured value %q:\n%s", want, stderr)
		}
	}
	c := discoDecode(t, stdout)["meta"].(map[string]any)["completeness"].(map[string]any)
	if n := c["assertions_failed"].(float64); n < 1 {
		t.Errorf("assertions_failed = %v, want at least 1", n)
	}
	if c["bytes_match"] != false {
		t.Errorf("bytes_match = %v, want false", c["bytes_match"])
	}
	if c["pages_match"] != false {
		t.Errorf("pages_match = %v, want false", c["pages_match"])
	}

	if _, _, code = runDisco(t, append(args, "--strict")...); code != 1 {
		t.Fatalf("--strict exit = %d, want 1", code)
	}
}

// TestDiscoEmptyResultAlwaysExplainsItself pins that a zero-row panel names its
// cause. An empty results array printed as success is indistinguishable from a
// report that published nothing.
func TestDiscoEmptyResultAlwaysExplainsItself(t *testing.T) {
	discoForbidRequests(t)
	path := discoSyntheticPDF(t)
	cases := []struct {
		name  string
		args  []string
		wants []string
	}{
		{
			// The synthetic fixture carries only SAIDI, so asking for
			// recovery is the "no such table" leg, and it must name what the
			// report DID carry.
			name: "metric absent from the report",
			args: []string{"--metric", "recovery", "--pdf", path, "--fy", "2018-19"},
			wants: []string{"published no recovery table at all",
				"the parameters it did carry are", "saidi"},
		},
		{
			// The FY2022-23 variant trap in miniature: the metric exists, the
			// requested variant does not.
			name:  "variant not the one this report used",
			args:  []string{"--metric", "saidi", "--pdf", path, "--fy", "2018-19", "--variant", "with-lt"},
			wants: []string{"DOES publish saidi", "headline", "five_year_comparison"},
		},
		{
			name:  "period matches no column",
			args:  []string{"--metric", "saidi", "--pdf", path, "--fy", "2018-19", "--period", "2010-11"},
			wants: []string{"periods actually present", "FY2018-19"},
		},
		{
			// The corpus note is quoted verbatim for a recorded year: this is
			// FY2014-15's "the figures are chart images" answer.
			name:  "recorded corpus note is quoted verbatim",
			args:  []string{"--metric", "recovery", "--pdf", path, "--fy", "2014-15"},
			wants: []string{"recorded note for FY2014-15", "embedded Excel chart images with no text layer"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runDisco(t, append([]string{"disco"}, append(tc.args, "--json")...)...)
			if code != 3 {
				t.Fatalf("exit = %d, want 3\n%s", code, stderr)
			}
			p := discoDecode(t, stdout)
			if res, ok := p["results"].([]any); !ok || len(res) != 0 {
				t.Fatalf("results = %v, want empty", p["results"])
			}
			reason, _ := p["refusal"].(string)
			if strings.TrimSpace(reason) == "" {
				t.Fatalf("no refusal field in the payload:\n%s", stdout)
			}
			for _, w := range tc.wants {
				if !strings.Contains(reason, w) {
					t.Errorf("refusal is missing %q:\n%s", w, reason)
				}
			}
		})
	}
}

// TestDiscoWireShapeIsSnakeCase is the regression that follows from
// marshalling nepraper.Observation, Provenance, Conflict or Key directly: none
// of them carries a json tag, so the encoder emits PascalCase keys into a
// codebase whose every other envelope is snake_case.
func TestDiscoWireShapeIsSnakeCase(t *testing.T) {
	discoForbidRequests(t)
	path := discoSyntheticPDF(t)
	stdout, _, code := runDisco(t, "disco", "--metric", "saidi", "--pdf", path, "--fy", "2024-25", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, bad := range []string{"\"Prov\"", "\"AbsDiff\"", "\"ReportFY\"", "\"PeriodFY\"", "\"Num\"", "\"Kind\""} {
		if strings.Contains(stdout, bad) {
			t.Errorf("payload carries the Go field name %s; a nepraper struct was marshalled directly", bad)
		}
	}
	upper := regexp.MustCompile(`"[A-Z][A-Za-z0-9]*"\s*:`)
	if m := upper.FindAllString(stdout, 5); len(m) > 0 {
		t.Errorf("payload has capitalised keys %v; every envelope in this CLI is snake_case", m)
	}
}

// TestDiscoRowModeNoticeNamesConflicts pins that a flat-output caller is TOLD
// about the conflicts a row set cannot carry. Silence here would let a --csv
// reader take one figure as the report's answer when the same report publishes
// another.
func TestDiscoRowModeNoticeNamesConflicts(t *testing.T) {
	discoForbidRequests(t)
	path := discoSyntheticPDF(t)
	// Parsing the fixture as FY2024-25 makes its headline table's period
	// collide with the comparison table's 2024-25 column, which is the same
	// self-contradiction shape the real FY2024-25 report carries.
	_, stderr, code := runDisco(t, "disco", "--metric", "saidi", "--pdf", path,
		"--fy", "2024-25", "--variant", "headline", "--csv")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "same-key conflict") || !strings.Contains(stderr, "--json") {
		t.Fatalf("the flat-output notice does not name the conflicts:\n%s", stderr)
	}
}

// TestDiscoQuietWritesNothingToStdout pins that --quiet governs stdout only:
// a failed assertion still reaches stderr.
func TestDiscoQuietWritesNothingToStdout(t *testing.T) {
	discoForbidRequests(t)
	path := discoSyntheticPDF(t)
	stdout, stderr, code := runDisco(t, "disco", "--metric", "saidi", "--pdf", path, "--fy", "2018-19", "--quiet")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty under --quiet", stdout)
	}
	if !strings.Contains(stderr, "COMPLETENESS") {
		t.Errorf("--quiet suppressed the completeness shortfall; it must never be silent:\n%s", stderr)
	}
}
