// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): tests for `digg-pp-cli rankings`.
//
// Strategy:
//   1. Spin up an httptest server that serves the trimmed
//      rankings-companies fixture for /ai/x/rankings/companies.
//   2. Point rankingsCompaniesURL at the test server.
//   3. Execute each subcommand via the public RootCmd entry and
//      capture stdout/stderr; assert JSON shape and stderr drift
//      warnings without hitting the live network.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadRankingsFixtureForCLI reads the canonical pristine fixture.
// Path discovery mirrors the diggparse-side test helper so a single
// fixture file serves both layers.
func loadRankingsFixtureForCLI(t *testing.T) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "testdata", "rankings-companies-fixture.html"),
		filepath.Join("testdata", "rankings-companies-fixture.html"),
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return data
		}
	}
	t.Fatalf("rankings-companies-fixture.html not found; tried: %v", candidates)
	return nil
}

// rankingsTestServer hosts the pristine fixture and rewires
// rankingsCompaniesURL for the lifetime of the test. Returns a
// cleanup closure (call via t.Cleanup).
func rankingsTestServer(t *testing.T) string {
	t.Helper()
	fixture := loadRankingsFixtureForCLI(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	prev := rankingsCompaniesURL
	rankingsCompaniesURL = srv.URL
	t.Cleanup(func() { rankingsCompaniesURL = prev })
	return srv.URL
}

// runCLI executes RootCmd with the given args and returns stdout +
// stderr buffers and the exit error. Uses a context so we can cancel
// mid-flight if a test runs long.
func runCLI(t *testing.T, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	root := RootCmd()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = root.ExecuteContext(ctx)
	return stdout, stderr, err
}

func TestRankingsEmerging_EndToEnd(t *testing.T) {
	rankingsTestServer(t)
	stdout, stderr, err := runCLI(t,
		"rankings", "emerging", "--json", "--no-color", "--yes", "--no-input",
	)
	if err != nil {
		t.Fatalf("rankings emerging: %v\nstderr: %s", err, stderr.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("decode output: %v; output: %s", err, stdout.String())
	}
	if len(rows) < 5 {
		t.Errorf("got %d rows, want >= 5", len(rows))
	}
	// Spot-check that the expected curated names land in the section.
	usernames := map[string]bool{}
	for _, r := range rows {
		if u, ok := r["username"].(string); ok {
			usernames[u] = true
		}
	}
	for _, want := range []string{"inspiredco_ai", "EvolvingPx"} {
		if !usernames[want] {
			t.Errorf("expected curated emerging fixture to contain @%s; got %v", want, usernames)
		}
	}
}

func TestRankingsMovers_DirectionFiltering(t *testing.T) {
	rankingsTestServer(t)
	cases := []struct {
		direction string
		wantDir   map[string]bool // every output row's `direction` must be in this set
	}{
		{"up", map[string]bool{"up": true}},
		{"down", map[string]bool{"down": true}},
		{"both", map[string]bool{"up": true, "down": true}},
	}
	for _, tc := range cases {
		t.Run(tc.direction, func(t *testing.T) {
			stdout, stderr, err := runCLI(t,
				"rankings", "movers", "--direction", tc.direction,
				"--json", "--no-color", "--yes", "--no-input",
			)
			if err != nil {
				t.Fatalf("rankings movers --direction %s: %v\nstderr: %s", tc.direction, err, stderr.String())
			}
			var rows []map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
				t.Fatalf("decode: %v\noutput: %s", err, stdout.String())
			}
			if len(rows) == 0 {
				t.Fatalf("expected at least one row for direction=%s", tc.direction)
			}
			for i, r := range rows {
				dir, _ := r["direction"].(string)
				if !tc.wantDir[dir] {
					t.Errorf("row %d direction=%q, want one of %v", i, dir, tc.wantDir)
				}
			}
		})
	}
}

func TestRankingsMovers_RejectsInvalidDirection(t *testing.T) {
	rankingsTestServer(t)
	_, _, err := runCLI(t,
		"rankings", "movers", "--direction", "sideways",
		"--json", "--no-color", "--yes", "--no-input",
	)
	if err == nil {
		t.Fatal("expected error for invalid direction value, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --direction") {
		t.Errorf("error = %q; expected 'invalid --direction' message", err.Error())
	}
}

func TestRankingsList_LimitFlag(t *testing.T) {
	rankingsTestServer(t)
	// Fixture has 30 main entries (some are RSC references that the
	// parser skips). With --limit 5, output must be exactly 5.
	stdout, stderr, err := runCLI(t,
		"rankings", "list", "--limit", "5",
		"--json", "--no-color", "--yes", "--no-input",
	)
	if err != nil {
		t.Fatalf("rankings list --limit 5: %v\nstderr: %s", err, stderr.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("decode: %v\noutput: %s", err, stdout.String())
	}
	if len(rows) != 5 {
		t.Errorf("got %d rows, want exactly 5", len(rows))
	}
	// First row should be OpenAI (rank 1).
	if u, _ := rows[0]["username"].(string); u != "OpenAI" {
		t.Errorf("rows[0].username = %q, want OpenAI", u)
	}
	// Every row must have a rank field set; missing => mis-parsing.
	for i, r := range rows {
		if _, ok := r["rank"]; !ok {
			t.Errorf("row %d missing rank: %+v", i, r)
		}
	}
}

func TestRankingsAll_NoSchemaDriftWarningOnPristineFixture(t *testing.T) {
	rankingsTestServer(t)
	cases := [][]string{
		{"rankings", "emerging"},
		{"rankings", "movers"},
		{"rankings", "list"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			fullArgs := append(append([]string{}, args...),
				"--json", "--no-color", "--yes", "--no-input")
			_, stderr, err := runCLI(t, fullArgs...)
			if err != nil {
				t.Fatalf("%v: %v\nstderr: %s", args, err, stderr.String())
			}
			// Pristine fixture should not emit any drift warning lines.
			if strings.Contains(stderr.String(), "warn: rankings.") {
				t.Errorf("unexpected drift warning on pristine fixture; stderr:\n%s", stderr.String())
			}
		})
	}
}

func TestRankingsEmerging_DirtyFixtureTripsThresholdAtEquality(t *testing.T) {
	// Dirty fixture has 1 Emerging entry with rank:"oops" out of 10
	// total — exactly 10%, which equals the default threshold (0.10).
	// Threshold uses >= so equality DOES trip an error. This locks in
	// the boundary behavior of parse_stats.Threshold + the suggested
	// --max-skip-ratio hint in the error message.
	dirty := loadRankingsFixtureForCLI_named(t, "rankings-companies-dirty-fixture.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(dirty)
	}))
	t.Cleanup(srv.Close)
	prev := rankingsCompaniesURL
	rankingsCompaniesURL = srv.URL
	t.Cleanup(func() { rankingsCompaniesURL = prev })

	_, _, err := runCLI(t,
		"rankings", "emerging", "--json", "--no-color", "--yes", "--no-input",
	)
	if err == nil {
		t.Fatal("expected threshold error at 10%% skip ratio, got nil")
	}
	if !strings.Contains(err.Error(), "rankings.emerging") {
		t.Errorf("error didn't mention section name: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "schema") {
		t.Errorf("error didn't suggest schema check: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "--max-skip-ratio") {
		t.Errorf("error didn't suggest --max-skip-ratio: %q", err.Error())
	}
}

func TestRankingsEmerging_MaxSkipRatioOverrideRelaxes(t *testing.T) {
	// Same dirty fixture, but pass --max-skip-ratio 0.5 to relax the
	// gate. The 10% drift is now well below threshold; expect success
	// AND a stderr warning (Skipped > 0, below threshold = warn path).
	dirty := loadRankingsFixtureForCLI_named(t, "rankings-companies-dirty-fixture.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(dirty)
	}))
	t.Cleanup(srv.Close)
	prev := rankingsCompaniesURL
	rankingsCompaniesURL = srv.URL
	t.Cleanup(func() { rankingsCompaniesURL = prev })

	stdout, stderr, err := runCLI(t,
		"rankings", "emerging",
		"--max-skip-ratio", "0.5",
		"--json", "--no-color", "--yes", "--no-input",
	)
	if err != nil {
		t.Fatalf("rankings emerging --max-skip-ratio 0.5: %v\nstderr: %s", err, stderr.String())
	}
	if len(stdout.Bytes()) == 0 {
		t.Error("expected partial JSON output when threshold relaxed")
	}
	stderrText := stderr.String()
	if !strings.Contains(stderrText, "warn: rankings.emerging") {
		t.Errorf("expected stderr warning on partial-parse below threshold; got:\n%s", stderrText)
	}
}

func TestRankings_MaxSkipRatioOutOfRangeRejected(t *testing.T) {
	rankingsTestServer(t)
	for _, ratio := range []string{"-0.1", "1.5", "2.0"} {
		t.Run(ratio, func(t *testing.T) {
			_, _, err := runCLI(t,
				"rankings", "list", "--max-skip-ratio", ratio,
				"--json", "--no-color", "--yes", "--no-input",
			)
			if err == nil {
				t.Fatalf("expected error for --max-skip-ratio=%s, got nil", ratio)
			}
			if !strings.Contains(err.Error(), "--max-skip-ratio must be in") {
				t.Errorf("error = %q; expected '--max-skip-ratio must be in' phrase", err.Error())
			}
		})
	}
}

// loadRankingsFixtureForCLI_named is the named-fixture variant used by
// dirty-fixture tests; mirrors loadRankingsFixtureForCLI but parametric
// on filename.
func loadRankingsFixtureForCLI_named(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "testdata", name),
		filepath.Join("testdata", name),
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return data
		}
	}
	t.Fatalf("%s not found; tried: %v", name, candidates)
	return nil
}

func TestRankings_TotalFailureSuggestsSchemaCheckNotFlagBump(t *testing.T) {
	// When every entry fails to decode (SkipRatio = 1.0), no
	// --max-skip-ratio value can recover (1.0 >= 1.0 still trips).
	// The error message must point at the schema instead of
	// suggesting a flag value that would be silently useless.
	//
	// Synthesize 100% failure by serving an emerging wrapper whose
	// entries are all type-mismatched (rank as string -> Unmarshal
	// fails on every row).
	failureHTML := `<!DOCTYPE html><html><body><script>
self.__next_f.push([1,"3a:[\"$\",\"$L39\",null,{\"direction\":\"emerging\",\"entries\":[` +
		`{\"rank\":\"oops\",\"target_x_id\":\"x1\",\"username\":\"u1\",\"followed_by_count\":1,\"followers_count\":1,\"score\":0},` +
		`{\"rank\":\"oops\",\"target_x_id\":\"x2\",\"username\":\"u2\",\"followed_by_count\":1,\"followers_count\":1,\"score\":0}` +
		`]}]"])
</script></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, failureHTML)
	}))
	t.Cleanup(srv.Close)
	prev := rankingsCompaniesURL
	rankingsCompaniesURL = srv.URL
	t.Cleanup(func() { rankingsCompaniesURL = prev })

	_, _, err := runCLI(t,
		"rankings", "emerging", "--json", "--no-color", "--yes", "--no-input",
	)
	if err == nil {
		t.Fatal("expected error on 100%% drift, got nil")
	}
	if !strings.Contains(err.Error(), "every entry failed") {
		t.Errorf("error didn't note 100%% failure: %q", err.Error())
	}
	if strings.Contains(err.Error(), "--max-skip-ratio") {
		t.Errorf("error suggested --max-skip-ratio at 100%% failure (no value would help): %q", err.Error())
	}
}

func TestRankingsList_PageShapeChangedReturnsTypedError(t *testing.T) {
	// Serve HTML with NO RSC pushes — simulates upstream removing the
	// data layer. The list command should exit non-zero with a clear
	// "page shape may have changed" message.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><body>no data here</body></html>`)
	}))
	t.Cleanup(srv.Close)
	prev := rankingsCompaniesURL
	rankingsCompaniesURL = srv.URL
	t.Cleanup(func() { rankingsCompaniesURL = prev })

	_, _, err := runCLI(t,
		"rankings", "list", "--json", "--no-color", "--yes", "--no-input",
	)
	if err == nil {
		t.Fatal("expected error for empty-RSC response, got nil")
	}
	if !strings.Contains(err.Error(), "page shape") && !strings.Contains(err.Error(), "RSC") {
		t.Errorf("error = %q; expected 'page shape' / 'RSC' phrase", err.Error())
	}
}

// Compile-time sanity that the command tree wires up correctly. This
// would catch a developer accidentally removing newRankingsCmd from
// root.go's AddCommand block.
func TestRankings_RegisteredInRoot(t *testing.T) {
	root := RootCmd()
	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "rankings" {
			found = true
			subs := map[string]bool{}
			for _, sc := range c.Commands() {
				subs[sc.Name()] = true
			}
			for _, want := range []string{"emerging", "movers", "list"} {
				if !subs[want] {
					t.Errorf("missing subcommand %q under 'rankings' (have %v)", want, subs)
				}
			}
			break
		}
	}
	if !found {
		t.Fatal("'rankings' command not registered on root")
	}
}
