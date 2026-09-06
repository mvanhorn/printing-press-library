package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func TestNovelReachLiveRanksFiltersAndCrossChecks(t *testing.T) {
	isolateNovelTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/destinations" {
			t.Errorf("path = %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("origin_airport"); got != "JFK" {
			t.Errorf("origin_airport = %q", got)
		}
		fmt.Fprint(w, `{"success":true,"origin_airport":"JFK","destinations":[{"airport":"LHR","business":65000},{"airport":"CDG","business":55000},{"airport":"NRT","business":90000},{"airport":"SFO","business":null}]}`)
	}))
	defer server.Close()
	t.Setenv("SEATS_AERO_BASE_URL", server.URL)
	t.Setenv("SEATS_AERO_API_KEY", "dummy")
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for i, date := range []string{"2099-01-02", "2099-01-03"} {
		raw := json.RawMessage(fmt.Sprintf(`{"ID":"award-%d","RouteID":"route-1","Route":{"ID":"route-1","OriginAirport":"JFK","DestinationAirport":"CDG","Source":"united"},"Date":"%s","Source":"united","JAvailable":true,"JDirect":true,"JMileageCost":"29000","JMileageCostRaw":29000,"JDirectMileageCostRaw":29000,"JRemainingSeats":2,"JAirlines":"UA"}`, i, date))
		if err := db.UpsertAvailability(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		extra []string
		want  []string
	}{
		{"ordering", nil, []string{"CDG", "LHR", "NRT"}},
		{"max mileage", []string{"--max-mileage", "60000"}, []string{"CDG"}},
		{"top", []string{"--top", "2"}, []string{"CDG", "LHR"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"reach", "--origin", "JFK", "--cabin", "business", "--db", dbPath, "--json"}
			args = append(args, tc.extra...)
			var out, stderr bytes.Buffer
			cmd := RootCmd()
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute: %v; stderr=%s", err, stderr.String())
			}
			var got reachEnvelope
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatalf("decode %q: %v", out.String(), err)
			}
			if len(got.Destinations) != len(tc.want) {
				t.Fatalf("destinations=%+v want %v", got.Destinations, tc.want)
			}
			for i, want := range tc.want {
				if got.Destinations[i].Airport != want {
					t.Errorf("destination[%d]=%q want %q", i, got.Destinations[i].Airport, want)
				}
			}
			if len(got.Destinations) > 0 && got.Destinations[0].Airport == "CDG" {
				ev := got.Destinations[0].LocalEvidence
				if ev == nil || ev.Rows != 2 || ev.MinMiles != 29000 || ev.NextDate != "2099-01-02" || ev.DirectRows != 2 {
					t.Errorf("CDG evidence=%+v", ev)
				}
			}
			for _, d := range got.Destinations {
				if d.Airport != "CDG" && d.LocalEvidence != nil {
					t.Errorf("%s evidence=%+v want nil", d.Airport, d.LocalEvidence)
				}
				if d.Airport == "SFO" {
					t.Error("null-cost SFO was not excluded")
				}
			}
		})
	}
}

func TestNovelReachConfirmLiveCapsAndSkipsEvidence(t *testing.T) {
	isolateNovelTest(t)
	var searches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/destinations":
			fmt.Fprint(w, `{"success":true,"destinations":[{"airport":"CDG","business":29000},{"airport":"LHR","business":40000},{"airport":"NRT","business":50000}]}`)
		case "/search":
			searches.Add(1)
			fmt.Fprint(w, `{"data":[{"Date":"2099-02-03T12:00:00Z","JMileageCostRaw":42000}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("SEATS_AERO_BASE_URL", server.URL)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"ID":"cdg","RouteID":"r","Route":{"OriginAirport":"JFK","DestinationAirport":"CDG"},"Date":"2099-01-02T10:00:00Z","Source":"united","JAvailable":true,"JDirect":true,"JMileageCostRaw":29000}`)
	if err := db.UpsertAvailability(raw); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, _, err := executeRoot("reach", "--origin", "JFK", "--db", path, "--confirm-live", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got reachEnvelope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if searches.Load() != 2 {
		t.Fatalf("searches=%d want 2", searches.Load())
	}
	for _, d := range got.Destinations {
		if d.Airport == "CDG" && d.LiveCheck != nil {
			t.Fatalf("evidenced destination checked: %+v", d)
		}
		if d.Airport != "CDG" && (d.LiveCheck == nil || d.LiveCheck.NextDate != "2099-02-03" || d.LiveCheck.Miles != 42000) {
			t.Fatalf("missing live check: %+v", d)
		}
	}

	searches.Store(0)
	t.Setenv(cliutil.DogfoodEnvVar, "1")
	out, _, err = executeRoot("reach", "--origin", "JFK", "--db", path, "--confirm-live", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if searches.Load() != 0 {
		t.Fatalf("dogfood searches=%d want 0", searches.Load())
	}
	for _, d := range got.Destinations {
		if d.LiveCheck != nil {
			t.Fatalf("dogfood live check=%+v", d.LiveCheck)
		}
	}
}

func TestNovelReachConfirmLiveHardCapTen(t *testing.T) {
	isolateNovelTest(t)
	var searches atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			searches.Add(1)
			fmt.Fprint(w, `{"data":[{"Date":"2099-01-01","JMileageCostRaw":10000}]}`)
			return
		}
		dests := make([]map[string]any, 15)
		for i := range dests {
			dests[i] = map[string]any{"airport": fmt.Sprintf("X%02d", i), "business": 10000 + i}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"destinations": dests})
	}))
	defer srv.Close()
	t.Setenv("SEATS_AERO_BASE_URL", srv.URL)
	out, _, err := executeRoot("reach", "--origin", "JFK", "--confirm-live", "--top", "15", "--json", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}
	var got reachEnvelope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, d := range got.Destinations {
		if d.LiveCheck != nil {
			checked++
		}
	}
	if searches.Load() != 10 || checked != 10 {
		t.Fatalf("searches=%d checked=%d", searches.Load(), checked)
	}
}

func TestNovelReachUsageCaps(t *testing.T) {
	isolateNovelTest(t)
	for _, args := range [][]string{{"reach", "--origin", "JFK", "--top", "51", "--json"}, {"reach", "--origin", "JFK", "--data-source", "local", "--json"}} {
		_, _, err := executeRoot(args...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestNovelReachEmptyDestinationsIsArray(t *testing.T) {
	isolateNovelTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":true,"origin_airport":"JFK","destinations":[]}`)
	}))
	defer server.Close()
	t.Setenv("SEATS_AERO_BASE_URL", server.URL)
	t.Setenv("SEATS_AERO_API_KEY", "dummy")
	dbPath := filepath.Join(t.TempDir(), "missing.db")
	var out, stderr bytes.Buffer
	cmd := RootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"reach", "--origin", "JFK", "--data-source", "live", "--db", dbPath, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if string(got["destinations"]) != "[]" {
		t.Fatalf("destinations=%s want []", got["destinations"])
	}
}

func TestReadReachEvidenceExcludesPastAvailability(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	today := time.Now().UTC()
	seed := func(id string, date time.Time, miles int) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"ID": id, "Route": map[string]any{"OriginAirport": "JFK", "DestinationAirport": "LHR"}, "Date": date.Format(time.RFC3339), "JAvailable": true, "JDirect": true, "JMileageCost": fmt.Sprint(miles), "JMileageCostRaw": miles})
		if err := db.UpsertAvailability(raw); err != nil {
			t.Fatal(err)
		}
	}
	seed("past", today.AddDate(0, 0, -2), 10000)
	seed("future", today.AddDate(0, 0, 2), 45000)

	ev, err := readReachEvidence(t.Context(), db, "JFK", "LHR", "j")
	if err != nil {
		t.Fatal(err)
	}
	if ev == nil || ev.Rows != 1 || ev.MinMiles != 45000 || ev.DirectRows != 1 {
		t.Fatalf("evidence=%+v", ev)
	}
	if _, err := db.DB().Exec(`DELETE FROM availability WHERE id = ?`, "future"); err != nil {
		t.Fatal(err)
	}
	ev, err = readReachEvidence(t.Context(), db, "JFK", "LHR", "j")
	if err != nil || ev != nil {
		t.Fatalf("past-only evidence=%+v err=%v, want nil", ev, err)
	}
}

func TestNovelReachLiveFailureDegrades(t *testing.T) {
	isolateNovelTest(t)
	t.Setenv(cliutil.VerifyEnvVar, "1")
	t.Setenv(cliutil.VerifyLiveHTTPEnvVar, "1")
	var searches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/destinations":
			fmt.Fprint(w, `{"success":true,"destinations":[{"airport":"CDG","business":29000},{"airport":"LHR","business":40000},{"airport":"NRT","business":50000}]}`)
		case "/search":
			if searches.Add(1) == 1 {
				fmt.Fprint(w, `{"data":[{"Date":"2099-02-03T12:00:00Z","JMileageCostRaw":42000}]}`)
				return
			}
			http.Error(w, "upstream failed", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("SEATS_AERO_BASE_URL", server.URL)
	out, _, err := executeRoot("reach", "--origin", "JFK", "--confirm-live", "--data-source", "live", "--json")
	if err != nil {
		t.Fatalf("exit should degrade to success: %v", err)
	}
	var got reachEnvelope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Destinations) != 3 || len(got.Warnings) == 0 || searches.Load() > 2 {
		t.Fatalf("envelope=%+v searches=%d", got, searches.Load())
	}
	errors := 0
	for _, destination := range got.Destinations {
		if destination.LiveCheckError != "" {
			errors++
		}
	}
	if errors != 1 {
		t.Fatalf("live check errors=%d envelope=%+v", errors, got)
	}
}
