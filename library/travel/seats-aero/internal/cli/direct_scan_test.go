package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func seedDirectScanStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := []map[string]any{
		{"ID": "direct-j-under", "RouteID": "r1", "Route": map[string]any{"ID": "r1", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "united"}, "Date": "2026-10-10", "Source": "united", "JAvailable": true, "JDirect": true, "JMileageCost": "80000", "JMileageCostRaw": 80000, "JDirectMileageCostRaw": 85000, "JRemainingSeats": 2, "JDirectRemainingSeats": 1, "JAirlines": "UA", "JDirectAirlines": "UA", "JDirectTotalTaxes": 5.6, "TaxesCurrency": "USD"},
		{"ID": "direct-effective-over", "RouteID": "rx", "Route": map[string]any{"OriginAirport": "JFK", "DestinationAirport": "NRT"}, "Date": "2026-10-10T12:00:00Z", "Source": "united", "JAvailable": true, "JDirect": true, "JMileageCostRaw": 80000, "JDirectMileageCostRaw": 95000},
		{"ID": "direct-j-over", "RouteID": "r2", "Route": map[string]any{"ID": "r2", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "aeroplan"}, "Date": "2026-10-11", "Source": "aeroplan", "JAvailable": true, "JDirect": true, "JMileageCost": "110000", "JMileageCostRaw": 110000, "JDirectMileageCostRaw": 100000, "JRemainingSeats": 2, "JAirlines": "AC"},
		{"ID": "non-direct-j", "RouteID": "r3", "Route": map[string]any{"ID": "r3", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "delta"}, "Date": "2026-10-12", "Source": "delta", "JAvailable": true, "JDirect": false, "JMileageCost": "70000", "JMileageCostRaw": 70000, "JRemainingSeats": 3, "JAirlines": "DL"},
		{"ID": "direct-f-only", "RouteID": "r4", "Route": map[string]any{"ID": "r4", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "etihad"}, "Date": "2026-10-13", "Source": "etihad", "FAvailable": true, "FDirect": true, "FMileageCost": "80000", "FMileageCostRaw": 80000, "FDirectMileageCostRaw": 80000, "FRemainingSeats": 1, "FAirlines": "EY"},
	}
	for _, row := range rows {
		b, _ := json.Marshal(row)
		if err := db.UpsertAvailability(b); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func runDirectScan(t *testing.T, args ...string) ([]directScanRow, error, string) {
	t.Helper()
	out, _, err := executeRoot(append([]string{"direct-scan"}, args...)...)
	var got []directScanRow
	if err == nil {
		if e := json.Unmarshal(out.Bytes(), &got); e != nil {
			t.Fatalf("decode %q: %v", out.String(), e)
		}
	}
	return got, err, out.String()
}

func TestDirectScanFiltersLocalAwards(t *testing.T) {
	isolateNovelTest(t)
	db := seedDirectScanStore(t)
	tests := []struct {
		name    string
		args    []string
		wantIDs []string
	}{
		{"business ceiling", []string{"--db", db, "--origin", "JFK", "--destination", "NRT", "--cabin", "business", "--max-mileage", "90000", "--json"}, []string{"direct-j-under"}},
		{"end day includes timestamps", []string{"--db", db, "--origin", "JFK", "--destination", "NRT", "--cabin", "business", "--max-mileage", "100000", "--end", "2026-10-10", "--json"}, []string{"direct-j-under", "direct-effective-over"}},
		{"source empty", []string{"--db", db, "--cabin", "business", "--sources", "aeroplan", "--max-mileage", "90000", "--json"}, []string{}},
		{"source unlimited", []string{"--db", db, "--cabin", "business", "--sources", "aeroplan", "--max-mileage", "0", "--json"}, []string{"direct-j-over"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err, raw := runDirectScan(t, tt.args...)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("rows=%s want IDs %v", raw, tt.wantIDs)
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Fatalf("id=%q want %q", got[i].ID, id)
				}
			}
			if tt.name == "business ceiling" {
				r := got[0]
				if r.Miles != 85000 || r.Seats != 1 || r.Airlines != "UA" || r.Taxes != 5.6 || r.TaxesCurrency != "USD" || r.Date != "2026-10-10" {
					t.Fatalf("payload=%+v", r)
				}
			}
			if len(tt.wantIDs) == 0 && raw != "[]\n" {
				t.Fatalf("empty stdout=%q want []", raw)
			}
		})
	}
}

func TestDirectScanMissingDBIsEmpty(t *testing.T) {
	isolateNovelTest(t)
	got, err, raw := runDirectScan(t, "--db", "/nonexistent/path.db", "--json")
	if err != nil || len(got) != 0 || raw != "[]\n" {
		t.Fatalf("got=%v err=%v raw=%q", got, err, raw)
	}
}

func TestDirectScanUsageErrors(t *testing.T) {
	isolateNovelTest(t)
	db := seedDirectScanStore(t)
	for _, args := range [][]string{{"--db", db, "--cabin", "coach", "--json"}, {"--db", db, "--data-source", "live", "--json"}} {
		_, err, _ := runDirectScan(t, args...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("args=%v error=%v code=%d, want 2", args, err, ExitCode(err))
		}
	}
}

func TestDirectScanWarnsOnUnknownSource(t *testing.T) {
	isolateNovelTest(t)
	db := seedDirectScanStore(t)
	out, stderr, err := executeRoot("direct-scan", "--db", db, "--cabin", "business", "--sources", "bogussource,united", "--max-mileage", "0", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []directScanRow
	if e := json.Unmarshal(out.Bytes(), &got); e != nil {
		t.Fatalf("decode %q: %v", out.String(), e)
	}
	if len(got) == 0 {
		t.Fatalf("known source must still return rows: got=%v", got)
	}
	for _, r := range got {
		if r.Source != "united" {
			t.Fatalf("unknown source must be dropped from the filter, not widen it: got=%v", got)
		}
	}
	msg := stderr.String()
	if !strings.Contains(msg, "warning: --sources bogussource not present in the local store") || !strings.Contains(msg, "synced sources: aeroplan,delta,etihad,united") {
		t.Fatalf("expected unknown-source warning naming bogussource and the synced list, got stderr=%q", msg)
	}
	if strings.Contains(msg, "united not present") {
		t.Fatalf("known source must not be reported as missing: %q", msg)
	}
	_, stderr, err = executeRoot("direct-scan", "--db", db, "--cabin", "business", "--sources", "united", "--max-mileage", "0", "--json")
	if err != nil || strings.Contains(stderr.String(), "warning: --sources") {
		t.Fatalf("known-only --sources must not warn: err=%v stderr=%q", err, stderr.String())
	}
}

func TestDirectScanPredicateUsesODDateIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := `SELECT id FROM availability_all WHERE json_extract(data,'$.Route.OriginAirport')=? AND json_extract(data,'$.Route.DestinationAirport')=? AND date >= ? AND date < date(?, '+1 day') ORDER BY date,id`
	explainUsesIndex(t, db.DB(), query, "JFK", "NRT", "2026-10-01", "2026-10-02")
}
