package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/haneda"
)

func runHanedaTest(args ...string) (map[string]any, string, error) {
	cmd := RootCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	var d map[string]any
	if out.Len() > 0 {
		_ = json.Unmarshal(out.Bytes(), &d)
	}
	if results, ok := d["results"].(map[string]any); ok {
		d = results
	}
	return d, stderr.String(), err
}
func hanedaMock(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		kind := "international"
		if strings.Contains(r.URL.Path, "/dms/") {
			kind = "domestic"
		}
		name := ""
		if strings.HasSuffix(r.URL.Path, "/flight/search") {
			var p map[string]any
			_ = json.NewDecoder(r.Body).Decode(&p)
			if p["flightType"] == float64(1) {
				kind = "domestic"
			}
			direction := "departures"
			if p["arrivalType"] == float64(2) {
				direction = "arrivals"
			}
			data, err := os.ReadFile(filepath.Join("../haneda/testdata", kind+"-"+direction+".json"))
			if err != nil {
				t.Error(err)
			}
			var b haneda.RawBoard
			_ = json.Unmarshal(data, &b)
			day, _ := time.ParseInLocation("20060102", fmt.Sprint(p["searchDt"]), haneda.JST)
			for i := range b.Flights {
				b.Flights[i].Date.Key = day.Format("20060102")
				b.Flights[i].Date.Display = day.Format("2006/01/02")
				if b.Flights[i].Date.Change != "-" {
					b.Flights[i].Date.Change = day.AddDate(0, 0, -1).Format("2006/01/02")
				}
			}
			if p["exactMatch"] == true {
				if p["flightNumber"] == b.Flights[0].Airlines[0].Number {
					b.Flights = b.Flights[:1]
				} else {
					b.Flights = []haneda.RawFlight{}
				}
				n := len(b.Flights)
				b.Count = &n
			}
			_ = json.NewEncoder(w).Encode(b)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "city_list_search.json"):
			name = kind + "-city"
		case strings.HasSuffix(r.URL.Path, "company_list_search.json"):
			name = kind + "-company"
		case strings.HasSuffix(r.URL.Path, "flight_status.json"):
			name = "disruption-summary"
		case strings.HasSuffix(r.URL.Path, "hdacfdsc.json"):
			name = "monthly-" + kind + "-departures"
		case strings.HasSuffix(r.URL.Path, "hdacfasc.json"):
			name = "monthly-" + kind + "-arrivals"
		}
		if name == "" {
			http.NotFound(w, r)
			return
		}
		data, err := os.ReadFile(filepath.Join("../haneda/testdata", name+".json"))
		if err != nil {
			t.Error(err)
		}
		_, _ = w.Write(data)
	}))
	return server, &requests
}

func TestHanedaDryRunBeforeIO(t *testing.T) {
	testenv.Isolate(t)
	server, requests := hanedaMock(t)
	defer server.Close()
	t.Setenv("HANEDA_AIRPORT_BASE_URL", server.URL)
	missing := filepath.Join(t.TempDir(), "not-created", "snapshot.json")
	for _, args := range [][]string{{"flights", "search"}, {"flights", "detail"}, {"flights", "disruptions"}, {"flights", "rollover"}, {"plan"}, {"catalog", "airports"}, {"catalog", "airlines"}, {"schedule", "search"}, {"snapshot", "save", "--file", missing}, {"snapshot", "search", "--file", missing}, {"snapshot", "diff", "--before", missing}} {
		args = append(args, "--dry-run", "--json")
		d, stderr, err := runHanedaTest(args...)
		if err != nil || d["dry_run"] != true {
			t.Fatalf("dry run %v: %v, %s, %+v", args, err, stderr, d)
		}
	}
	if *requests != 0 {
		t.Fatal("dry runs transmitted source requests")
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Fatal("dry run created snapshot directories")
	}
}

func TestHanedaCLISourceIdentityAndProjection(t *testing.T) {
	testenv.Isolate(t)
	server, _ := hanedaMock(t)
	defer server.Close()
	t.Setenv("HANEDA_AIRPORT_BASE_URL", server.URL)
	for _, tc := range []struct {
		args  []string
		key   string
		count int
	}{{[]string{"flights", "search", "--kind", "international", "--flight", "UA8003", "--limit", "5", "--json"}, "flights", 1}, {[]string{"catalog", "airports", "--kind", "domestic", "--query", "札幌", "--json"}, "airports", 1}, {[]string{"flights", "search", "--destination", "ImaginaryAirportXYZ", "--json"}, "flights", 0}, {[]string{"flights", "disruptions", "--json"}, "flights", 1}} {
		d, stderr, err := runHanedaTest(tc.args...)
		if err != nil {
			t.Fatalf("%v: %v %s", tc.args, err, stderr)
		}
		rows, ok := d[tc.key].([]any)
		if !ok || len(rows) != tc.count {
			t.Fatalf("%v wanted %d rows, got %+v", tc.args, tc.count, d)
		}
		if tc.key == "flights" && tc.count > 0 {
			f := rows[0].(map[string]any)
			if f["actual_at"] != nil || f["operating_flight"] != nil {
				t.Fatal("CLI invented actual/operator facts")
			}
			if strings.Contains(strings.Join(tc.args, " "), "UA8003") && f["source_primary_flight"] != "NH849" {
				t.Fatal("marketing alias replaced primary")
			}
		}
	}
	d, stderr, err := runHanedaTest("flights", "search", "--flight", "NH849", "--agent", "--select", "flights.source_primary_flight,flights.actual_at,coverage.requested_date")
	if err != nil {
		t.Fatal(err, stderr)
	}
	if _, ok := d["sources"]; ok {
		t.Fatal("--select retained unrequested large fields")
	}
	if rows, ok := d["flights"].([]any); !ok || len(rows) != 1 || rows[0].(map[string]any)["source_primary_flight"] != "NH849" {
		t.Fatal("agent projection stripped required identity")
	}
	d, stderr, err = runHanedaTest("plan", "UA8003", "--kind", "international", "--json")
	if err != nil {
		t.Fatal(err, stderr)
	}
	b := d["flight_board"].(map[string]any)
	if len(b["flights"].([]any)) != 1 || d["terminal_floor_urls"].(map[string]any)["T2"] == nil {
		t.Fatal("terminal planning handoff missing")
	}
	for _, number := range []string{"NH849", "UA8003", "NH0849"} {
		d, stderr, err := runHanedaTest("flights", "detail", number, "--kind", "international", "--direction", "departure", "--json")
		if err != nil {
			t.Fatalf("detail %s: %v %s", number, err, stderr)
		}
		rows := d["flights"].([]any)
		if len(rows) != 1 || rows[0].(map[string]any)["source_primary_flight"] != "NH849" {
			t.Fatalf("detail %s lost source group identity", number)
		}
		mode := "exact_primary_lookup"
		if number != "NH849" {
			mode = "board_with_local_flight_lookup"
		}
		if d["coverage"].(map[string]any)["query_mode"] != mode {
			t.Fatalf("detail %s did not disclose its resolution scope", number)
		}
	}
}

func TestHanedaSnapshotCLICompleteOfflineAndDiff(t *testing.T) {
	testenv.Isolate(t)
	server, requests := hanedaMock(t)
	defer server.Close()
	t.Setenv("HANEDA_AIRPORT_BASE_URL", server.URL)
	path := filepath.Join(t.TempDir(), "before.json")
	d, stderr, err := runHanedaTest("snapshot", "save", "--kind", "international", "--file", path, "--json")
	if err != nil {
		t.Fatal(err, stderr)
	}
	if d["flight_groups"].(float64) <= 1 {
		t.Fatal("save must preserve complete scope rather than a page")
	}
	s, err := haneda.LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	initialRequests := *requests
	d, stderr, err = runHanedaTest("snapshot", "search", "--file", path, "--flight", "UA8003", "--json")
	if err != nil {
		t.Fatal(err, stderr)
	}
	if d["total_matches"].(float64) != 1 || d["budget"].(map[string]any)["request_count"] != float64(0) || *requests != initialRequests {
		t.Fatal("offline query made requests or lost codeshare matches")
	}
	d, stderr, err = runHanedaTest("snapshot", "diff", "--before", path, "--after", path, "--json")
	if err != nil {
		t.Fatal(err, stderr)
	}
	cmp := d["comparison"].(map[string]any)
	if len(cmp["changes"].([]any)) != 0 {
		t.Fatal("unchanged observation fabricated drift")
	}
	s.Board.Flights[0].BoardingGates = []string{"changed-gate"}
	after := filepath.Join(t.TempDir(), "after.json")
	if err = haneda.SaveSnapshot(after, s, false); err != nil {
		t.Fatal(err)
	}
	d, stderr, err = runHanedaTest("snapshot", "diff", "--before", path, "--after", after, "--json")
	if err != nil {
		t.Fatal(err, stderr)
	}
	cmp = d["comparison"].(map[string]any)
	if cmp["total_changes"] != float64(1) {
		t.Fatal("material gate change was not detected")
	}
	if *requests != initialRequests {
		t.Fatal("snapshot comparison contacted provider")
	}
	s.Board.ObservedAt = time.Now().In(haneda.JST).Add(-10 * time.Minute).Format(time.RFC3339)
	aged := filepath.Join(t.TempDir(), "aged.json")
	if err = haneda.SaveSnapshot(aged, s, false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args  []string
		stale bool
	}{{nil, true}, {[]string{"--max-age", "30m"}, false}} {
		args := append([]string{"snapshot", "search", "--file", aged, "--json"}, tc.args...)
		d, stderr, err := runHanedaTest(args...)
		if err != nil || d["stale_snapshot"] != tc.stale {
			t.Fatalf("snapshot age policy %v: %v %s %+v", args, err, stderr, d)
		}
	}
	_, _, err = runHanedaTest("snapshot", "save", "--kind", "international", "--max-scan-records", "1", "--file", filepath.Join(t.TempDir(), "partial.json"), "--json")
	if err == nil {
		t.Fatal("partial scan must not save a complete snapshot")
	}
}

func TestHanedaSnapshotCLIRejectsUncoveredExplicitScope(t *testing.T) {
	testenv.Isolate(t)
	server, requests := hanedaMock(t)
	defer server.Close()
	t.Setenv("HANEDA_AIRPORT_BASE_URL", server.URL)
	path := filepath.Join(t.TempDir(), "board.json")
	if _, stderr, err := runHanedaTest("snapshot", "save", "--kind", "international", "--direction", "departure", "--file", path, "--json"); err != nil {
		t.Fatal(err, stderr)
	}
	initialRequests := *requests
	for _, filter := range [][]string{
		{"--kind", "domestic"}, {"--kind", "all"}, {"--direction", "arrival"}, {"--direction", "both"},
		{"--date", time.Now().In(haneda.JST).AddDate(0, 0, 1).Format("2006-01-02")},
	} {
		args := append([]string{"snapshot", "search", "--file", path, "--json"}, filter...)
		_, _, err := runHanedaTest(args...)
		if err == nil || !strings.Contains(err.Error(), "not covered") {
			t.Fatalf("uncovered query %v must be an error: %v", filter, err)
		}
	}
	d, stderr, err := runHanedaTest("snapshot", "search", "--file", path, "--flight", "ZZ9999", "--json")
	if err != nil || d["total_matches"] != float64(0) {
		t.Fatalf("a covered scope with no matching flight is a real empty result: %+v %v %s", d, err, stderr)
	}
	if *requests != initialRequests {
		t.Fatal("saved-scope validation made network requests")
	}
}

func TestHanedaSnapshotDefaultDiffFindsOlderCompatibleOriginPair(t *testing.T) {
	testenv.Isolate(t)
	server, requests := hanedaMock(t)
	defer server.Close()
	t.Setenv("HANEDA_AIRPORT_BASE_URL", server.URL)
	path := filepath.Join(t.TempDir(), "source.json")
	if _, stderr, err := runHanedaTest("snapshot", "save", "--file", path, "--json"); err != nil {
		t.Fatal(err, stderr)
	}
	s, err := haneda.LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := hanedaSnapshotDir()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().In(haneda.JST).Add(-time.Minute)
	for i := 1; i <= 3; i++ {
		s.Board.ObservedAt = start.Add(time.Duration(i) * time.Second).Format(time.RFC3339)
		s.Board.Coverage.Origin = "https://older-source.example"
		if i == 2 {
			s.Board.Coverage.QueryMode = "board" // The old v1 empty mode remains compatible.
			s.Board.Flights[0].BoardingGates = []string{"changed-gate"}
		}
		if i == 3 {
			s.Board.Coverage.Origin = "https://newest-source.example"
		}
		file := filepath.Join(dir, fmt.Sprintf("hnd-snapshot-%d_international_departure_%s.json", i, s.Board.Coverage.RequestedDate))
		if err := haneda.SaveSnapshot(file, s, false); err != nil {
			t.Fatal(err)
		}
	}
	initialRequests := *requests
	d, stderr, err := runHanedaTest("snapshot", "diff", "--json")
	if err != nil {
		t.Fatalf("default pairing must skip the newest unpaired origin: %v %s", err, stderr)
	}
	cmp := d["comparison"].(map[string]any)
	if d["baseline_sufficient"] != true || cmp["coverage"].(map[string]any)["origin"] != "https://older-source.example" || len(cmp["changes"].([]any)) != 1 {
		t.Fatalf("wrong compatible baseline: %+v", d)
	}
	if *requests != initialRequests {
		t.Fatal("automatic saved snapshot pairing made network requests")
	}

	// Completing an older pair first must not conceal a newer compatible pair.
	pairDir := t.TempDir()
	paths := []string{}
	for i, origin := range []string{"https://latest.example", "https://older.example", "https://older.example", "https://latest.example"} {
		s.Board.Coverage.Origin = origin
		s.Board.ObservedAt = start.Add(time.Duration(10-i) * time.Second).Format(time.RFC3339)
		file := filepath.Join(pairDir, fmt.Sprintf("hnd-snapshot-%d.json", 4-i))
		if err := haneda.SaveSnapshot(file, s, false); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, file)
	}
	before, after, notes, err := hanedaLatestCompatiblePair(paths)
	if err != nil || before != paths[3] || after != paths[0] || len(notes) != 0 {
		t.Fatalf("the newest compatible cache pair must win: %q %q %v", before, after, err)
	}

	// A known valid pair stays usable when an unrelated older file stops the
	// bounded selection. Its ranking uncertainty must be exposed to the caller.
	for _, tc := range []struct {
		name string
		make func(string) error
	}{
		{"budget", func(path string) error {
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			err = f.Truncate(65 << 20)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			return closeErr
		}},
		{"malformed", func(path string) error { return os.WriteFile(path, []byte("{"), 0600) }},
		{"missing", func(path string) error { return nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			older := filepath.Join(t.TempDir(), "older-unrelated.json")
			if err := tc.make(older); err != nil {
				t.Fatal(err)
			}
			// paths[0] is unpaired; paths[1:3] are a valid older-origin pair.
			before, after, notes, err := hanedaLatestCompatiblePair([]string{paths[0], paths[1], paths[2], older})
			if err != nil || before != paths[2] || after != paths[1] || len(notes) != 1 || !strings.Contains(notes[0], "may remain unexamined") {
				t.Fatalf("valid pair lost or uncertainty hidden: %q %q %+v %v", before, after, notes, err)
			}
			// Without a known valid pair, a stopped scan must not claim an
			// empty, sufficient baseline.
			if _, _, _, err := hanedaLatestCompatiblePair([]string{paths[0], older}); err == nil {
				t.Fatal("stopped selection without a baseline must fail explicitly")
			}
		})
	}
}

func TestHanedaUsageAndEmptyLocalBaseline(t *testing.T) {
	testenv.Isolate(t)
	for _, args := range [][]string{{"flights", "search", "--date", "2026-02-30", "--json"}, {"flights", "search", "--limit", "0", "--json"}, {"flights", "search", "--status", "canceled,", "--json"}, {"flights", "detail", "--json"}, {"snapshot", "diff", "--before", "missing", "--json"}, {"snapshot", "diff", "--data-source", "live", "--json"}, {"schedule", "search", "--kind", "invalid", "--json"}, {"schedule", "search", "--flight", "hnd:international:departure:20261003:NH849", "--json"}} {
		_, _, err := runHanedaTest(args...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("invalid usage %v gave %v", args, err)
		}
	}
	for _, args := range [][]string{{"snapshot", "search", "--json"}, {"snapshot", "diff", "--json"}} {
		d, stderr, err := runHanedaTest(args...)
		if err != nil {
			t.Fatal(err, stderr)
		}
		if d["budget"].(map[string]any)["request_count"] != float64(0) {
			t.Fatal("empty local state must not fetch network")
		}
	}
}

func TestHanedaCapabilitiesAreReadOnlyAndDiscoverable(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	if cmd, _, err := root.Find([]string{"import"}); err == nil && cmd.Name() == "import" {
		t.Fatal("public read-only sources must not advertise a generic import")
	}
	if _, exists := resourceWritePaths["source"]; exists {
		t.Fatal("POST flight search must not be registered as a resource writer")
	}
	for _, path := range []string{"flights search", "flights detail", "flights disruptions", "catalog airports", "catalog airlines", "schedule search"} {
		matches := rankWhich(whichIndex, path, 1)
		if len(matches) != 1 || matches[0].Entry.Command != path {
			t.Fatalf("which did not resolve %q to its exact leaf: %+v", path, matches)
		}
	}
}
