// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command tests. The wiring smoke test is kept; behaviour cases follow.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Shared test scaffolding for swap, multibuy and basket.
// ---------------------------------------------------------------------------

// wowTestTile is a product tile fixture. Its JSON field names match the live
// payload exactly, so a decoding regression in wowTile shows up here.
type wowTestTile struct {
	Stockcode   int              `json:"Stockcode"`
	Name        string           `json:"Name"`
	DisplayName string           `json:"DisplayName"`
	Brand       string           `json:"Brand"`
	Price       float64          `json:"Price"`
	WasPrice    float64          `json:"WasPrice"`
	IsOnSpecial bool             `json:"IsOnSpecial"`
	IsHalfPrice bool             `json:"IsHalfPrice"`
	CupPrice    float64          `json:"CupPrice"`
	CupMeasure  string           `json:"CupMeasure"`
	CupString   string           `json:"CupString"`
	PackageSize string           `json:"PackageSize"`
	IsAvailable bool             `json:"IsAvailable"`
	IsInStock   bool             `json:"IsInStock"`
	CentreTag   *wowTestCentre   `json:"CentreTag"`
	Multibuy    *wowMultibuyData `json:"-"`
}

type wowTestCentre struct {
	TagType      string           `json:"TagType"`
	MultibuyData *wowMultibuyData `json:"MultibuyData"`
}

// wowRecordedRequest is one request the fake server saw.
type wowRecordedRequest struct {
	Path string
	Body map[string]any
}

// wowFakeServer stands in for the Woolworths API. It records every request so
// tests can assert on what went over the wire — most importantly that PageSize
// never exceeds the server's hard cap of 36.
type wowFakeServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []wowRecordedRequest
	// pages maps a 1-based page number to the tiles it should return. A page
	// with no entry returns an empty result, which ends the scan.
	pages map[int][]wowTestTile
	// searchResultsCount is echoed back as the total; zero means "unknown".
	searchResultsCount int
	// failPages returns HTTP 500 for these page numbers, to exercise
	// partial-failure accounting.
	failPages map[int]bool
	// nullProducts makes page 1 return the null Products list the live API
	// sends for a term that matches nothing.
	nullProducts bool
	// byTerm, when non-nil, serves page 1 per search term. A term with no
	// entry gets the null Products list, matching how the live API answers a
	// term it cannot match at all.
	byTerm map[string][]wowTestTile
	// failTerms returns HTTP 500 for these search terms, so a per-line
	// transport failure can be told apart from a line that simply did not
	// match anything.
	failTerms map[string]bool
}

// seen returns the API requests the server saw, excluding the Akamai cookie
// warm-up GET of /shop. Warming is per-Client and best-effort, and this fixture
// sets no cookies, so a command can legitimately warm more than once; none of
// those GETs is part of what these tests are asserting about.
func (s *wowFakeServer) seen() []wowRecordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]wowRecordedRequest, 0, len(s.requests))
	for _, req := range s.requests {
		if req.Path == "/shop" {
			continue
		}
		out = append(out, req)
	}
	return out
}

// newWowFakeServer starts a fake API and points the CLI at it for the duration
// of the test.
func newWowFakeServer(t *testing.T, srv *wowFakeServer) *wowFakeServer {
	t.Helper()
	if srv == nil {
		srv = &wowFakeServer{}
	}
	if srv.failPages == nil {
		srv.failPages = map[int]bool{}
	}
	if srv.failTerms == nil {
		srv.failTerms = map[string]bool{}
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := readAllLimited(r)
		_ = json.Unmarshal(raw, &body)
		srv.mu.Lock()
		srv.requests = append(srv.requests, wowRecordedRequest{Path: r.URL.Path, Body: body})
		srv.mu.Unlock()

		page := 1
		if v, ok := body["PageNumber"].(float64); ok {
			page = int(v)
		}
		if v, ok := body["pageNumber"].(float64); ok {
			page = int(v)
		}
		term, _ := body["SearchTerm"].(string)
		if srv.failPages[page] || srv.failTerms[term] {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"fixture failure"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if srv.nullProducts && page == 1 {
			_, _ = w.Write([]byte(`{"Products":null,"SearchResultsCount":0}`))
			return
		}
		tiles := srv.pages[page]
		if srv.byTerm != nil {
			match, ok := srv.byTerm[term]
			if !ok || page > 1 {
				_, _ = w.Write([]byte(`{"Products":null,"SearchResultsCount":0}`))
				return
			}
			tiles = match
		}
		payload := map[string]any{
			"Products":           []any{map[string]any{"Products": wowEncodeTiles(tiles)}},
			"SearchResultsCount": srv.searchResultsCount,
			"Bundles":            []any{map[string]any{"Products": wowEncodeTiles(tiles)}},
			"TotalRecordCount":   srv.searchResultsCount,
		}
		_ = json.NewEncoder(w).Encode(payload)
	})
	srv.Server = httptest.NewServer(handler)
	t.Cleanup(srv.Server.Close)
	t.Setenv("WOOLWORTHS_BASE_URL", srv.Server.URL)
	return srv
}

func readAllLimited(r *http.Request) ([]byte, error) {
	defer func() { _ = r.Body.Close() }()
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r.Body)
	return buf.Bytes(), err
}

// wowEncodeTiles renders fixtures into the wire shape, attaching the multibuy
// centre tag where one was requested.
func wowEncodeTiles(tiles []wowTestTile) []any {
	out := make([]any, 0, len(tiles))
	for _, tile := range tiles {
		if tile.Multibuy != nil {
			tile.CentreTag = &wowTestCentre{TagType: "MultiBuy", MultibuyData: tile.Multibuy}
		}
		raw, err := json.Marshal(tile)
		if err != nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out
}

// runCLI executes the root command with args and returns stdout plus the error.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

// decodeEnvelope parses command stdout as JSON, failing loudly on empty output
// or on a top-level null — both of which are what a broken command looks like
// to a machine caller.
func decodeEnvelope(t *testing.T, out string) map[string]any {
	t.Helper()
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		t.Fatalf("command produced empty stdout; a --json caller cannot tell that from a crash")
	}
	if trimmed == "null" {
		t.Fatalf("command produced a bare null; expected a JSON object")
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, trimmed)
	}
	return env
}

// requireEmptyJSONArray asserts a field is present and is [] rather than null.
func requireEmptyJSONArray(t *testing.T, env map[string]any, field string) {
	t.Helper()
	raw, ok := env[field]
	if !ok {
		t.Fatalf("envelope has no %q field: %v", field, env)
	}
	if raw == nil {
		t.Fatalf("%q serialised as null; must be [] so a machine caller can range over it", field)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("%q is %T, want an array", field, raw)
	}
	if len(arr) != 0 {
		t.Fatalf("%q has %d entries, want none", field, len(arr))
	}
}

// assertNoOversizedPageSize is the wire guard: the API answers HTTP 400 to any
// PageSize above 36, so no request this CLI builds may carry a larger one.
func assertNoOversizedPageSize(t *testing.T, requests []wowRecordedRequest) {
	t.Helper()
	if len(requests) == 0 {
		t.Fatalf("no requests were recorded; the page-size guard proved nothing")
	}
	checked := 0
	for i, req := range requests {
		for _, key := range []string{"PageSize", "pageSize"} {
			raw, ok := req.Body[key]
			if !ok {
				continue
			}
			size, ok := raw.(float64)
			if !ok {
				t.Fatalf("request %d %s=%v is not a number", i, key, raw)
			}
			if size > wowMaxPageSize {
				t.Fatalf("request %d to %s sent %s=%v; the API rejects anything above %d with HTTP 400",
					i, req.Path, key, size, wowMaxPageSize)
			}
			checked++
			t.Logf("request %d %s %s=%v (cap %d)", i, req.Path, key, size, wowMaxPageSize)
		}
	}
	if checked == 0 {
		t.Fatalf("no request carried a page size; the guard proved nothing")
	}
}

// liveTestsEnabled reports whether the live-API tests should run. They are
// skipped in -short mode and when a fixture server is in play, so an offline
// checkout still gets a clean `go test ./...`.
func liveTestsEnabled(t *testing.T) bool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping live API test in -short mode")
		return false
	}
	if os.Getenv("WOOLWORTHS_BASE_URL") != "" {
		t.Skip("skipping live API test: WOOLWORTHS_BASE_URL points somewhere else")
		return false
	}
	return true
}

// skipIfUnreachable turns a transport failure into a skip rather than a
// failure, so the live tests do not red-flag an offline machine. Anything that
// is not a transport failure is a real failure and is reported as one.
func skipIfUnreachable(t *testing.T, err error, out string) {
	t.Helper()
	if err == nil {
		return
	}
	if isNetworkError(err) || strings.Contains(err.Error(), "no such host") || strings.Contains(err.Error(), "timeout") {
		t.Skipf("skipping live API test: %v", err)
	}
	t.Fatalf("live command failed: %v\n%s", err, out)
}

// tempDBPath returns a path inside the test's own temp dir. Pointing --db at a
// file that does not exist keeps these tests off the developer's real database.
func tempDBPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

// exitCodeFor unwraps the CLI's exit-code error so tests can assert on it.
func exitCodeFor(err error) int {
	var codeErr *cliError
	if As(err, &codeErr) {
		return codeErr.code
	}
	return 1
}

// ---------------------------------------------------------------------------
// swap
// ---------------------------------------------------------------------------

// TestNovelSwapHelpWires smoke-tests that the swap command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSwapHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"swap", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("swap --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "swap", "--max-scan-pages", "--limit", "--db"} {
		if !strings.Contains(help, want) {
			t.Fatalf("swap --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestWowClampPageSize(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero defaults to the cap", 0, wowMaxPageSize},
		{"negative defaults to the cap", -5, wowMaxPageSize},
		{"below the cap passes through", 24, 24},
		{"at the cap passes through", 36, 36},
		{"above the cap is clamped", 37, wowMaxPageSize},
		{"far above the cap is clamped", 500, wowMaxPageSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wowClampPageSize(tt.in); got != tt.want {
				t.Fatalf("wowClampPageSize(%d) = %d, want %d (the API answers HTTP 400 above %d)", tt.in, got, tt.want, wowMaxPageSize)
			}
		})
	}
}

func TestWowResolveMaxScanPages(t *testing.T) {
	tests := []struct {
		name    string
		dogfood bool
		in      int
		want    int
	}{
		{"default", false, wowDefaultMaxScanPages, wowDefaultMaxScanPages},
		{"explicit", false, 8, 8},
		{"zero falls back to the default", false, 0, wowDefaultMaxScanPages},
		{"dogfood drops to one page", true, 5, 1},
		{"dogfood leaves a single page alone", true, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dogfood {
				t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
			} else {
				t.Setenv("PRINTING_PRESS_DOGFOOD", "")
			}
			if got := wowResolveMaxScanPages(tt.in); got != tt.want {
				t.Fatalf("wowResolveMaxScanPages(%d) with dogfood=%v = %d, want %d", tt.in, tt.dogfood, got, tt.want)
			}
		})
	}
}

func TestSwapBareInvocationPrintsHelp(t *testing.T) {
	out, err := runCLI(t, "swap")
	if err != nil {
		t.Fatalf("bare swap should print help and exit 0, got %v", err)
	}
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("bare swap did not print usage:\n%s", out)
	}
}

func TestSwapDryRunSkipsAllWork(t *testing.T) {
	out, err := runCLI(t, "swap", "--dry-run")
	if err != nil {
		t.Fatalf("swap --dry-run error = %v", err)
	}
	if !strings.Contains(out, "dry-run") {
		t.Fatalf("swap --dry-run did not report the skipped action:\n%s", out)
	}
}

func TestSwapMissingPositionalIsUsageError(t *testing.T) {
	out, err := runCLI(t, "swap", "--limit", "3")
	if err == nil {
		t.Fatalf("swap with flags but no positional should be a usage error; output:\n%s", out)
	}
	if got := exitCodeFor(err); got != 2 {
		t.Fatalf("exit code = %d, want 2 (usage)", got)
	}
}

// TestSwapRanksAcrossMeasureBases is the command-level version of the
// unitprice correctness test: the fixture mixes a per-100G tile with a per-1KG
// tile, so a raw CupPrice sort would invert the answer.
func TestSwapRanksAcrossMeasureBases(t *testing.T) {
	srv := newWowFakeServer(t, &wowFakeServer{
		searchResultsCount: 5,
		pages: map[int][]wowTestTile{
			1: {
				{Stockcode: 1, Name: "Anchor Coffee 200g", Brand: "Anchor", Price: 12, CupPrice: 6, CupMeasure: "100G", PackageSize: "200g", IsAvailable: true, IsInStock: true},
				{Stockcode: 2, Name: "Small Jar Coffee 100g", Brand: "Small", Price: 3, CupPrice: 3, CupMeasure: "100G", PackageSize: "100g", IsAvailable: true, IsInStock: true},
				{Stockcode: 3, Name: "Big Bag Coffee 1kg", Brand: "Big", Price: 25, CupPrice: 25, CupMeasure: "1KG", PackageSize: "1kg", IsAvailable: true, IsInStock: true},
				{Stockcode: 4, Name: "Cold Brew Coffee 1L", Brand: "Cold", Price: 5, CupPrice: 5, CupMeasure: "1L", PackageSize: "1L", IsAvailable: true, IsInStock: true},
				{Stockcode: 5, Name: "Mystery Coffee", Brand: "Mystery", Price: 1, CupPrice: 0, CupMeasure: "", PackageSize: "", IsAvailable: true, IsInStock: true},
			},
		},
	})

	out, err := runCLI(t, "swap", "coffee", "--limit", "5", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	if err != nil {
		t.Fatalf("swap error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	alts, _ := env["alternatives"].([]any)
	if len(alts) != 2 {
		t.Fatalf("alternatives = %d, want 2 (the two other mass tiles)", len(alts))
	}
	first, _ := alts[0].(map[string]any)
	if first["stockcode"] != "3" {
		t.Fatalf("cheapest alternative is stockcode %v, want 3 (the 1kg bag at $25/kg); a raw CupPrice sort would wrongly pick the $3.00/100G jar", first["stockcode"])
	}
	if got, _ := first["unit_price"].(float64); got != 25 {
		t.Fatalf("cheapest unit_price = %v, want 25", got)
	}
	second, _ := alts[1].(map[string]any)
	if got, _ := second["unit_price"].(float64); got != 30 {
		t.Fatalf("second unit_price = %v, want 30 ($3.00/100G restated per kilo)", got)
	}
	if got, _ := env["excluded_incomparable"].(float64); got != 1 {
		t.Fatalf("excluded_incomparable = %v, want 1 (the 1L cold brew)", got)
	}
	if got, _ := env["excluded_unparseable_measure"].(float64); got != 1 {
		t.Fatalf("excluded_unparseable_measure = %v, want 1 (the tile with no measure)", got)
	}
	assertNoOversizedPageSize(t, srv.seen())
}

// TestSwapEmptyResultIsValidJSON covers the machine-caller contract on a term
// the API matches nothing for: real JSON, an [] alternatives list, and a note.
func TestSwapEmptyResultIsValidJSON(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{nullProducts: true})

	out, err := runCLI(t, "swap", "zzqxwvblorptigmuffin", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	if err != nil {
		t.Fatalf("swap error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)
	requireEmptyJSONArray(t, env, "alternatives")
	requireEmptyJSONArray(t, env, "fetch_failures")
	if note, _ := env["note"].(string); note == "" {
		t.Fatalf("empty result carried no note explaining why")
	}
	if _, ok := env["scanned_products"]; !ok {
		t.Fatalf("envelope is missing scanned_products")
	}
}

// TestSwapUnknownStockcodeIsNotSubstituted is the honesty case for a numeric
// positional: asking for one specific product and getting savings measured
// against whatever the search engine returned first is worse than getting
// nothing, because nothing in the output says the anchor was substituted.
func TestSwapUnknownStockcodeIsNotSubstituted(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{
		searchResultsCount: 2,
		pages: map[int][]wowTestTile{
			1: {
				{Stockcode: 111, Name: "Some Other Coffee 200g", Price: 12, CupPrice: 6, CupMeasure: "100G", PackageSize: "200g", IsAvailable: true, IsInStock: true},
				{Stockcode: 222, Name: "Yet Another Coffee 1kg", Price: 25, CupPrice: 25, CupMeasure: "1KG", PackageSize: "1kg", IsAvailable: true, IsInStock: true},
			},
		},
	})

	out, err := runCLI(t, "swap", "999999", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	if err != nil {
		t.Fatalf("swap error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)
	if anchor := env["anchor"]; anchor != nil {
		t.Fatalf("stockcode 999999 was not in the results, but swap anchored on %v anyway", anchor)
	}
	requireEmptyJSONArray(t, env, "alternatives")
	note, _ := env["note"].(string)
	if !strings.Contains(note, "999999") {
		t.Fatalf("note = %q, want it to name the stockcode that was not found", note)
	}
	if !strings.Contains(note, "substituted") {
		t.Fatalf("note = %q, want it to say no substitute anchor was used", note)
	}
}

// TestSwapPageSizeNeverExceedsServerCap drives a multi-page scan and asserts
// every request stayed within the cap.
func TestSwapPageSizeNeverExceedsServerCap(t *testing.T) {
	full := make([]wowTestTile, 0, wowMaxPageSize)
	for i := 0; i < wowMaxPageSize; i++ {
		full = append(full, wowTestTile{
			Stockcode: 1000 + i, Name: "Coffee Bag", Brand: "B", Price: float64(10 + i),
			CupPrice: float64(10 + i), CupMeasure: "1KG", PackageSize: "1kg",
			IsAvailable: true, IsInStock: true,
		})
	}
	srv := newWowFakeServer(t, &wowFakeServer{
		searchResultsCount: 500,
		pages:              map[int][]wowTestTile{1: full, 2: full, 3: full},
	})

	out, err := runCLI(t, "swap", "coffee", "--json", "--max-scan-pages", "3", "--db", tempDBPath(t, "absent.db"))
	if err != nil {
		t.Fatalf("swap error = %v\n%s", err, out)
	}
	requests := srv.seen()
	if len(requests) < 3 {
		t.Fatalf("expected at least 3 requests across the scan, saw %d", len(requests))
	}
	assertNoOversizedPageSize(t, requests)
}

// TestSwapPartialFailureExcludesFailedPages proves a failed page is reported
// rather than silently shrinking the result set into a confident answer.
func TestSwapPartialFailureExcludesFailedPages(t *testing.T) {
	full := make([]wowTestTile, 0, wowMaxPageSize)
	full = append(full,
		wowTestTile{Stockcode: 1, Name: "Anchor Coffee 200g", Price: 12, CupPrice: 6, CupMeasure: "100G", PackageSize: "200g", IsAvailable: true, IsInStock: true},
		wowTestTile{Stockcode: 3, Name: "Big Bag Coffee 1kg", Price: 25, CupPrice: 25, CupMeasure: "1KG", PackageSize: "1kg", IsAvailable: true, IsInStock: true},
	)
	for i := len(full); i < wowMaxPageSize; i++ {
		full = append(full, wowTestTile{Stockcode: 5000 + i, Name: "Filler Coffee", Price: 40, CupPrice: 40, CupMeasure: "1KG", PackageSize: "1kg", IsAvailable: true, IsInStock: true})
	}
	newWowFakeServer(t, &wowFakeServer{
		searchResultsCount: 200,
		pages:              map[int][]wowTestTile{1: full, 2: full, 3: full},
		failPages:          map[int]bool{2: true},
	})

	out, err := runCLI(t, "swap", "coffee", "--json", "--max-scan-pages", "3", "--db", tempDBPath(t, "absent.db"))
	if err != nil {
		t.Fatalf("swap error = %v\n%s", err, out)
	}
	env := decodeEnvelope(t, out)
	failures, _ := env["fetch_failures"].([]any)
	if len(failures) != 1 {
		t.Fatalf("fetch_failures = %d, want 1 (page 2 returned HTTP 500)\n%s", len(failures), out)
	}
	if got, _ := env["scanned_pages"].(float64); got != 2 {
		t.Fatalf("scanned_pages = %v, want 2; the failed page must not be counted in the denominator", got)
	}
	t.Logf("fetch_failures=%v scanned_pages=%v scanned_products=%v", env["fetch_failures"], env["scanned_pages"], env["scanned_products"])
}

// TestSwapLiveOliveOil is the live acceptance case: every alternative must
// share the anchor's measure kind, and normalised unit prices must be
// monotonically non-decreasing down the list.
func TestSwapLiveOliveOil(t *testing.T) {
	if !liveTestsEnabled(t) {
		return
	}
	out, err := runCLI(t, "swap", "olive oil", "--limit", "5", "--json", "--max-scan-pages", "2", "--db", tempDBPath(t, "absent.db"))
	skipIfUnreachable(t, err, out)
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	anchor, ok := env["anchor"].(map[string]any)
	if !ok || anchor == nil {
		t.Fatalf("no anchor resolved for a live 'olive oil' search")
	}
	anchorKind, _ := anchor["measure_kind"].(string)
	if anchorKind == "" || anchorKind == "unknown" {
		t.Fatalf("anchor measure_kind = %q, want a real kind", anchorKind)
	}
	alts, _ := env["alternatives"].([]any)
	if len(alts) == 0 {
		t.Fatalf("live swap returned no alternatives for 'olive oil'")
	}
	if len(alts) > 5 {
		t.Fatalf("--limit 5 returned %d alternatives", len(alts))
	}
	prev := -1.0
	for i, raw := range alts {
		alt, _ := raw.(map[string]any)
		kind, _ := alt["measure_kind"].(string)
		if kind != anchorKind {
			t.Fatalf("alternative %d has measure_kind %q, anchor is %q; cross-kind rows must be excluded, not ranked", i, kind, anchorKind)
		}
		unit, _ := alt["unit_price"].(float64)
		if unit <= 0 {
			t.Fatalf("alternative %d has unit_price %v; a non-positive unit price would sort to the front as a fake bargain", i, unit)
		}
		if unit < prev {
			t.Fatalf("alternative %d unit_price %v is below the previous %v; the ranking is not monotonically non-decreasing", i, unit, prev)
		}
		prev = unit
		t.Logf("alt %d: %v | $%v | %v/%v | kind=%s", i, alt["name"], alt["price"], alt["unit_price"], alt["unit_basis"], kind)
	}
}

// TestSwapLiveNonsenseTermReturnsNoSwaps is the negative live case. Woolworths
// search silently ignores terms it cannot match, so a total-nonsense query
// returns nothing at all — and must not come back dressed as valid swaps.
func TestSwapLiveNonsenseTermReturnsNoSwaps(t *testing.T) {
	if !liveTestsEnabled(t) {
		return
	}
	out, err := runCLI(t, "swap", "zzqxwvblorptigmuffin", "--limit", "5", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	skipIfUnreachable(t, err, out)
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	if anchor := env["anchor"]; anchor != nil {
		t.Fatalf("a nonsense term resolved to anchor %v; nothing should have matched", anchor)
	}
	requireEmptyJSONArray(t, env, "alternatives")
	note, _ := env["note"].(string)
	if !strings.Contains(note, "no product matched") {
		t.Fatalf("note = %q, want an explicit statement that nothing matched", note)
	}
}
