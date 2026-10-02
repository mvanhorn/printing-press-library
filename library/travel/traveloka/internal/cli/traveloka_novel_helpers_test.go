package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/travelokacompare"
	"github.com/spf13/cobra"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These acceptance tests use SIMULATED snapshots only. They do not prove live availability.
func TestNovelAnalyticsDryRunBeforeIOAndValidation(t *testing.T) {
	testenv.Isolate(t)
	paths := [][]string{{"flights", "date-grid"}, {"hotels", "date-grid"}, {"flights", "shortlist", "--snapshot", "/SIMULATED/missing"}, {"hotels", "flexibility", "--snapshot", "/SIMULATED/missing"}, {"quotes", "diff", "--before", "/SIMULATED/missing"}}
	for _, path := range paths {
		t.Run(strings.Join(path[:2], "-"), func(t *testing.T) {
			v, e := runTravelokaTest(t, append(path, "--session-file", "/SIMULATED/missing", "--data-source", "invalid", "--dry-run")...)
			if e != nil || v["dry_run"] != true {
				t.Fatalf("dry run performed input/session IO: %v %v", v, e)
			}
		})
	}
}
func TestNovelAnalyticsExactHelpAndRedirects(t *testing.T) {
	testenv.Isolate(t)
	for _, path := range [][]string{{"flights", "date-grid"}, {"hotels", "date-grid"}, {"flights", "shortlist"}, {"hotels", "flexibility"}, {"quotes", "diff"}} {
		t.Run(strings.Join(path, "-"), func(t *testing.T) {
			cmd := RootCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(append(path, "--help"))
			if e := cmd.Execute(); e != nil {
				t.Fatal(e)
			}
			help := out.String()
			leaf := "traveloka-pp-cli " + strings.Join(path, " ") + " [flags]"
			if !strings.Contains(help, leaf) || !strings.Contains(help, "Do NOT use this command") || !strings.Contains(help, "  traveloka-pp-cli "+strings.Join(path, " ")) {
				t.Fatalf("wrong leaf/redirect/example help: %s", help)
			}
		})
	}
}
func TestNovelAnalyticsLocalCacheAndSelections(t *testing.T) {
	testenv.Isolate(t)
	root := t.TempDir()
	empty := filepath.Join(root, "SIMULATED-empty.sqlite")
	db, e := store.OpenWithContext(context.Background(), empty)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name        string
		path        []string
		collections []string
	}{
		{"frontier", []string{"flights", "shortlist"}, []string{"offers", "unknown_dimensions"}},
		{"flexibility", []string{"hotels", "flexibility"}, []string{"pairs", "unpaired"}},
		{"diff", []string{"quotes", "diff"}, []string{"changes", "not_returned", "new_returned", "unmatched_before", "unmatched_after"}},
	}
	for _, tt := range cases {
		for _, dbPath := range []string{filepath.Join(root, "SIMULATED-absent.sqlite"), empty} {
			t.Run(tt.name+"-"+filepath.Base(dbPath), func(t *testing.T) {
				argv := append(append([]string{}, tt.path...), "--db", dbPath)
				v, e := runTravelokaTest(t, argv...)
				if e != nil || v["status"] != "empty_local_cache" || v["hint"] == "" {
					t.Fatalf("empty local state hidden: %v %v", v, e)
				}
				for _, key := range tt.collections {
					rows, ok := v[key].([]any)
					if !ok || len(rows) != 0 {
						t.Fatalf("empty %s must be []: %v", key, v[key])
					}
				}
				selected, e := runTravelokaTest(t, append(argv, "--select", "status")...)
				if e != nil || len(selected) != 1 || selected["status"] != "empty_local_cache" {
					t.Fatalf("projection ignored %v %v", selected, e)
				}
			})
		}
	}
}
func TestNovelAnalyticsModesAndGridValidationBeforeSession(t *testing.T) {
	testenv.Isolate(t)
	cases := []struct {
		name string
		args []string
		code string
	}{
		{"frontier_live", []string{"flights", "shortlist", "--data-source", "live"}, "UNSUPPORTED_OPERATION"},
		{"flexibility_live", []string{"hotels", "flexibility", "--data-source", "live"}, "UNSUPPORTED_OPERATION"},
		{"diff_live", []string{"quotes", "diff", "--data-source", "live"}, "UNSUPPORTED_OPERATION"},
		{"flight_local", []string{"flights", "date-grid", "--data-source", "local"}, "UNSUPPORTED_OPERATION"},
		{"hotel_local", []string{"hotels", "date-grid", "--data-source", "local"}, "UNSUPPORTED_OPERATION"},
		{"invalid_second_return", []string{"flights", "date-grid", "--origin", "SIN", "--destination", "CGK", "--depart-dates", "2027-01-06,2027-01-12", "--return-dates", "2027-01-10"}, "INVALID_INPUT"},
		{"unequal_stays", []string{"hotels", "date-grid", "--property-id", "SIMULATED-property", "--stays", "2027-01-06:2027-01-08,2027-01-13:2027-01-14"}, "INVALID_INPUT"},
		{"missing_child_age", []string{"hotels", "date-grid", "--property-id", "SIMULATED-property", "--stays", "2027-01-06:2027-01-08", "--children", "1"}, "INVALID_INPUT"},
		{"half_file_pair", []string{"quotes", "diff", "--before", "/SIMULATED/missing"}, "INVALID_INPUT"},
		{"half_id_pair", []string{"quotes", "diff", "--before-id", "SIMULATED-missing"}, "INVALID_INPUT"},
		{"unexpected_position", []string{"flights", "shortlist", "unexpected"}, "INVALID_INPUT"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			v, e := runTravelokaTest(t, tt.args...)
			detail, _ := v["error"].(map[string]any)
			if e == nil || detail["code"] != tt.code {
				t.Fatalf("error hidden or reached session: %v %v", v, e)
			}
		})
	}
}
func simulatedNovelSnapshot(id, kind, at string) *traveloka.Snapshot {
	query := traveloka.Query{Kind: kind, Market: "SG", Locale: "en-SG", Currency: "SGD", Origin: "SIN", Destination: "CGK", Depart: "2027-01-06", Cabin: "ECONOMY", Adults: 1}
	if kind == "rooms" {
		query.PropertyID = "SIMULATED-property"
		query.CheckIn = "2027-01-06"
		query.CheckOut = "2027-01-08"
		query.Rooms = 1
	}
	return &traveloka.Snapshot{ID: id, Kind: kind, RetrievedAt: at, Status: "success", Query: query, Offers: []traveloka.Offer{}}
}
func TestNovelAnalyticsLatestPairReadOnlyAndNoChanges(t *testing.T) {
	testenv.Isolate(t)
	path := filepath.Join(t.TempDir(), "SIMULATED-history.sqlite")
	db, e := store.OpenWithContext(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	rows := []*traveloka.Snapshot{simulatedNovelSnapshot("SIMULATED-flight-before", "flights", "2026-10-02T00:00:00Z"), simulatedNovelSnapshot("SIMULATED-flight-after", "flights", "2026-10-02T01:00:00Z"), simulatedNovelSnapshot("SIMULATED-room-before", "rooms", "2026-10-02T02:00:00Z"), simulatedNovelSnapshot("SIMULATED-room-after", "rooms", "2026-10-02T03:00:00Z")}
	for _, s := range rows {
		if e = traveloka.SaveSnapshot(context.Background(), db, s); e != nil {
			db.Close()
			t.Fatal(e)
		}
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name          string
		args          []string
		before, after string
	}{
		{"latest_same_kind", []string{"quotes", "diff", "--db", path}, "SIMULATED-room-before", "SIMULATED-room-after"},
		{"explicit_kind", []string{"quotes", "diff", "--db", path, "--kind", "flights"}, "SIMULATED-flight-before", "SIMULATED-flight-after"},
		{"explicit_ids", []string{"quotes", "diff", "--db", path, "--before-id", "SIMULATED-flight-before", "--after-id", "SIMULATED-flight-after"}, "SIMULATED-flight-before", "SIMULATED-flight-after"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			v, e := runTravelokaTest(t, tt.args...)
			if e != nil || v["before_snapshot_id"] != tt.before || v["after_snapshot_id"] != tt.after || len(v["changes"].([]any)) != 0 || v["before_retrieved_at"] == "" || v["after_retrieved_at"] == "" {
				t.Fatalf("latest pair/empty diff/time lost: %v %v", v, e)
			}
		})
	}
	verify, e := store.OpenReadOnly(path)
	if e != nil {
		t.Fatal(e)
	}
	defer verify.Close()
	var count int
	if e = verify.DB().QueryRow("SELECT COUNT(*) FROM traveloka_snapshots").Scan(&count); e != nil || count != 4 {
		t.Fatalf("local diff modified snapshots: count=%d err=%v", count, e)
	}
}
func TestNovelAnalyticsSingleLatestSnapshotDefaults(t *testing.T) {
	testenv.Isolate(t)
	path := filepath.Join(t.TempDir(), "SIMULATED-local.sqlite")
	db, e := store.OpenWithContext(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"flights", "rooms"} {
		if e = traveloka.SaveSnapshot(context.Background(), db, simulatedNovelSnapshot("SIMULATED-"+kind, kind, "2026-10-02T00:00:00Z")); e != nil {
			t.Fatal(e)
		}
	}
	db.Close()
	for _, pathArgs := range [][]string{{"flights", "shortlist"}, {"hotels", "flexibility"}} {
		v, e := runTravelokaTest(t, append(pathArgs, "--db", path)...)
		if e != nil || v["snapshot_id"] == "" || v["freshness"] != "saved_snapshot" || v["scanned_offers"] != float64(0) {
			t.Fatalf("latest local snapshot not loaded %v %v", v, e)
		}
	}
}
func TestNovelAnalyticsFileSnapshotOutputPreservesSource(t *testing.T) {
	testenv.Isolate(t)
	scale := 2
	stops, duration := 0, 100
	s := simulatedNovelSnapshot("SIMULATED-file", "flights", "2026-10-02T00:00:00Z")
	s.Offers = []traveloka.Offer{{ID: "SIMULATED-offer", Kind: "flight", Stops: &stops, DurationMinutes: &duration, Price: traveloka.Price{Total: &traveloka.Money{Currency: "SGD", MinorUnits: "9007199254740993", Amount: "90071992547409.93", Decimals: &scale}}, Details: map[string]any{"price_basis": "party_trip_total"}}}
	file := filepath.Join(t.TempDir(), "SIMULATED-flight.json")
	if e := traveloka.SaveSnapshotFile(file, s); e != nil {
		t.Fatal(e)
	}
	v, e := runTravelokaTest(t, "flights", "shortlist", "--snapshot", file)
	if e != nil || v["retrieved_at"] != s.RetrievedAt || v["frontier_count"] != float64(1) {
		t.Fatalf("file provenance/frontier lost %v %v", v, e)
	}
	b, _ := json.Marshal(v)
	if !strings.Contains(string(b), "9007199254740993") {
		t.Fatal("exact source units rounded")
	}
}
func TestNovelAnalyticsDurationDayWeekForms(t *testing.T) {
	testenv.Isolate(t)
	for _, path := range [][]string{{"flights", "shortlist"}, {"hotels", "flexibility"}, {"quotes", "diff"}} {
		for _, duration := range []string{"7d", "1w", "24h", "0"} {
			t.Run(strings.Join(path, "-")+"-"+duration, func(t *testing.T) {
				v, e := runTravelokaTest(t, append(append([]string{}, path...), "--max-age", duration)...)
				if e != nil || v["status"] != "empty_local_cache" {
					t.Fatalf("duration %s rejected or cache semantics changed: %v %v", duration, v, e)
				}
			})
		}
	}
	for _, raw := range []string{"1.5d", "-1w", "tomorrow"} {
		v, e := runTravelokaTest(t, "flights", "shortlist", "--max-age", raw)
		detail, _ := v["error"].(map[string]any)
		if e == nil || detail["code"] != "INVALID_INPUT" {
			t.Fatalf("invalid duration %s was not typed: %v %v", raw, v, e)
		}
	}
}
func TestNovelAnalyticsGridFailureListsAndDenominator(t *testing.T) {
	cases := []struct {
		name, code      string
		attempted, exit int
	}{
		{"partial_failure", "UPSTREAM_ERROR", 2, 0}, {"all_auth_failed", "AUTH_REQUIRED", 1, 4}, {"all_upstream_failed", "UPSTREAM_ERROR", 1, 5}, {"all_rate_limited", "RATE_LIMITED", 1, 7},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			detail := &traveloka.APIError{Code: tt.code, Message: "SIMULATED-failure-message", Status: 503}
			failure := travelokacompare.Failure{CellIndex: 0, Error: detail, At: "2026-10-02T00:00:00Z"}
			original := &cliutil.RateLimitError{URL: "https://www.traveloka.com/api/v2/flight/search/poll", Body: "SIMULATED-throttle", Cause: &traveloka.APIError{Code: "UPSTREAM_ERROR", Message: "SIMULATED-nested", Status: 429}}
			cell := travelokacompare.Cell{Attempted: true, Error: detail, Status: "fetch_failed"}
			if tt.code == "RATE_LIMITED" {
				cell.Cause = original
			}
			view := travelokacompare.GridResult{Attempted: tt.attempted, FetchFailureCount: 1, FetchFailures: []travelokacompare.Failure{failure}, SaveFailures: []travelokacompare.Failure{}, Cells: []travelokacompare.Cell{cell}}
			cmd := &cobra.Command{Use: "SIMULATED-grid"}
			var out, errout bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errout)
			e := novelGridOutput(cmd, &rootFlags{asJSON: true}, view)
			if (e != nil) != (tt.exit != 0) {
				t.Fatalf("failed grid exit hidden: %v", e)
			}
			if e != nil {
				var typed *cliError
				if !errors.As(e, &typed) || typed.code != tt.exit {
					t.Fatalf("wrong typed exit: %v", e)
				}
			}
			if tt.code == "RATE_LIMITED" {
				var rate *cliutil.RateLimitError
				if !errors.As(e, &rate) || rate != original {
					t.Fatal("original rate-limit cause lost")
				}
			}
			var got map[string]any
			if e = json.Unmarshal(out.Bytes(), &got); e != nil {
				t.Fatalf("must emit one JSON document: %s", out.String())
			}
			rows := got["fetch_failures"].([]any)
			message := rows[0].(map[string]any)["error"].(map[string]any)["message"]
			if len(rows) != 1 || message != "SIMULATED-failure-message" || got["fetch_failure_count"] != float64(1) {
				t.Fatalf("failure details/count lost: %v", got)
			}
			if !strings.Contains(errout.String(), "attempted fetches failed") || !strings.Contains(errout.String(), "successful retrievals") {
				t.Fatalf("denominator missing: %s", errout.String())
			}
		})
	}
}

func TestNovelAnalyticsStaleHintsUseSelectedRetrievals(t *testing.T) {
	testenv.Isolate(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "SIMULATED-history.sqlite")
	db, e := store.OpenWithContext(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	before := simulatedNovelSnapshot("SIMULATED-stale-before", "flights", now.Add(-72*time.Hour).Format(time.RFC3339Nano))
	after := simulatedNovelSnapshot("SIMULATED-stale-after", "flights", now.Add(-48*time.Hour).Format(time.RFC3339Nano))
	fresh := simulatedNovelSnapshot("SIMULATED-fresh", "flights", now.Format(time.RFC3339Nano))
	for _, snapshot := range []*traveloka.Snapshot{before, after, fresh} {
		if e := traveloka.SaveSnapshot(context.Background(), db, snapshot); e != nil {
			db.Close()
			t.Fatal(e)
		}
	}
	if e := db.Close(); e != nil {
		t.Fatal(e)
	}
	beforeFile, afterFile := filepath.Join(dir, "SIMULATED-before.json"), filepath.Join(dir, "SIMULATED-after.json")
	if e := traveloka.SaveSnapshotFile(beforeFile, before); e != nil {
		t.Fatal(e)
	}
	if e := traveloka.SaveSnapshotFile(afterFile, after); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name     string
		args     []string
		staleIDs []string
	}{
		{"explicit_id", []string{"flights", "shortlist", "--db", path, "--snapshot-id", before.ID}, []string{before.ID}},
		{"file", []string{"flights", "shortlist", "--snapshot", beforeFile}, []string{before.ID}},
		{"explicit_diff_ids", []string{"quotes", "diff", "--db", path, "--before-id", before.ID, "--after-id", after.ID}, []string{before.ID, after.ID}},
		{"diff_files", []string{"quotes", "diff", "--before", beforeFile, "--after", afterFile}, []string{before.ID, after.ID}},
		{"latest_pair", []string{"quotes", "diff", "--db", path}, []string{after.ID}},
	}
	for _, tt := range cases {
		for _, age := range []string{"24h", "0"} {
			t.Run(tt.name+"-"+age, func(t *testing.T) {
				cmd := RootCmd()
				var out, errout bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&errout)
				argv := []string{"--home", t.TempDir(), "--no-learn", "--json"}
				argv = append(argv, tt.args...)
				cmd.SetArgs(append(argv, "--max-age", age))
				if e := cmd.Execute(); e != nil {
					t.Fatalf("local comparison failed: %v; stderr=%s", e, errout.String())
				}
				var view map[string]any
				if e := json.Unmarshal(out.Bytes(), &view); e != nil {
					t.Fatalf("stale hints contaminated stdout: %s", out.String())
				}
				if age == "0" {
					if strings.Contains(errout.String(), "is stale") {
						t.Fatalf("zero max age must disable hints: %s", errout.String())
					}
					return
				}
				if got := strings.Count(errout.String(), "is stale"); got != len(tt.staleIDs) {
					t.Fatalf("expected %d selected stale hints, got %d: %s", len(tt.staleIDs), got, errout.String())
				}
				for _, id := range tt.staleIDs {
					if !strings.Contains(errout.String(), "snapshot "+id+" retrieved at ") {
						t.Fatalf("selected stale identity %s missing: %s", id, errout.String())
					}
				}
				if strings.Contains(errout.String(), fresh.ID) {
					t.Fatalf("fresh snapshot received a stale hint: %s", errout.String())
				}
			})
		}
	}
}

func TestNovelAnalyticsUnattemptedGridCancellationReturnsTypedError(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			view := travelokacompare.GridResult{
				Status: "failed", Requested: 2, NotAttempted: 2, ContextErr: cause,
				Cells:         []travelokacompare.Cell{{Status: "not_attempted"}, {Status: "not_attempted"}},
				FetchFailures: []travelokacompare.Failure{}, SaveFailures: []travelokacompare.Failure{},
			}
			cmd := &cobra.Command{Use: "SIMULATED-grid"}
			var out bytes.Buffer
			cmd.SetOut(&out)
			e := novelGridOutput(cmd, &rootFlags{asJSON: true}, view)
			var typed *cliError
			if !errors.As(e, &typed) || typed.code != 5 || !errors.Is(e, cause) {
				t.Fatalf("grid cancellation did not retain typed error/cause: %v", e)
			}
			var got map[string]any
			if e := json.Unmarshal(out.Bytes(), &got); e != nil {
				t.Fatalf("must emit one JSON document before failing: %s", out.String())
			}
			if got["requested_cells"] != float64(2) || got["attempted_cells"] != float64(0) || len(got["cells"].([]any)) != 2 {
				t.Fatalf("requested cell output lost: %v", got)
			}
			if _, ok := got["ContextErr"]; ok {
				t.Fatal("private context cause appeared in JSON")
			}
		})
	}
}
