package cli

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func explainUsesIndex(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var detail strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var text string
		if err := rows.Scan(&id, &parent, &unused, &text); err != nil {
			t.Fatal(err)
		}
		detail.WriteString(text)
	}
	if !strings.Contains(detail.String(), "idx_availability_od_date") {
		t.Fatalf("plan %q does not use idx_availability_od_date", detail.String())
	}
}

func TestNovelCalendar(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	seed := []map[string]any{
		{"ID": "d2-united", "RouteID": "jfk-nrt-u", "Route": map[string]any{"ID": "jfk-nrt-u", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "united"}, "Date": "2026-10-02T23:59:59Z", "Source": "united", "JAvailable": true, "JDirect": true, "JMileageCost": "65000", "JMileageCostRaw": 65000, "JDirectMileageCostRaw": 65000, "JRemainingSeats": 1, "JAirlines": "UA"},
		{"ID": "d3-excluded", "RouteID": "jfk-nrt-u", "Route": map[string]any{"ID": "jfk-nrt-u", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "united"}, "Date": "2026-10-03T00:00:00Z", "Source": "united", "JAvailable": true},
		{"ID": "d1-united", "RouteID": "jfk-nrt-u", "Route": map[string]any{"ID": "jfk-nrt-u", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "united"}, "Date": "2026-10-01", "Source": "united", "JAvailable": true, "JDirect": false, "JMileageCost": "70000", "JMileageCostRaw": 70000, "JRemainingSeats": 2, "JAirlines": "UA"},
		{"ID": "d1-aeroplan", "RouteID": "jfk-nrt-a", "Route": map[string]any{"ID": "jfk-nrt-a", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "aeroplan"}, "Date": "2026-10-01", "Source": "aeroplan", "JAvailable": true, "JDirect": true, "JMileageCost": "55000", "JMileageCostRaw": 55000, "JDirectMileageCostRaw": 60000, "JRemainingSeats": 4, "JAirlines": "NH"},
	}
	for _, item := range seed {
		raw, _ := json.Marshal(item)
		if err := db.UpsertAvailability(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, start, end string
		want             int
	}{
		{"pivot", "2026-10-01", "2026-10-02", 2},
		{"out of range", "2026-11-01", "2026-11-30", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, err := executeRoot("calendar", "--json", "--db", path, "--origin", "JFK", "--destination", "NRT", "--start", tt.start, "--end", tt.end)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			var got []calendarDateEntry
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON %q: %v", out.String(), err)
			}
			if len(got) != tt.want {
				t.Fatalf("got %d entries, want %d: %s", len(got), tt.want, out.String())
			}
			if tt.want == 0 {
				if strings.TrimSpace(out.String()) != "[]" {
					t.Fatalf("empty output=%q, want []", out.String())
				}
				return
			}
			if got[0].Date != "2026-10-01" || got[1].Date != "2026-10-02" {
				t.Fatalf("dates out of order: %+v", got)
			}
			if got[0].Business.Miles != 55000 || got[0].Business.CheapestSource != "aeroplan" {
				t.Fatalf("min business cost/source wrong: %+v", got[0].Business)
			}
			if !got[0].Business.Direct || got[0].Business.DirectMiles != 60000 || got[0].Business.Seats != 4 {
				t.Fatalf("direct/seats aggregation wrong: %+v", got[0].Business)
			}
			if strings.Join(got[0].Sources, ",") != "aeroplan,united" {
				t.Fatalf("sources=%v", got[0].Sources)
			}
		})
	}
}

func TestNovelCalendarMissingOriginIsUsageError(t *testing.T) {
	isolateNovelTest(t)
	_, _, err := executeRoot("calendar", "--json", "--destination", "NRT")
	ce, ok := err.(*cliError)
	if !ok || ce.code != 2 {
		t.Fatalf("error=%T %v, want exit-2 cliError", err, err)
	}
}

func TestNovelCalendarUsageValidation(t *testing.T) {
	isolateNovelTest(t)
	for _, args := range [][]string{{"calendar", "--json", "--origin", "JFK"}, {"calendar", "--json", "--origin", "JFK", "--destination", "NRT", "--start", "2026-10-02", "--end", "2026-10-01"}} {
		_, _, err := executeRoot(args...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestCalendarQueryUsesODDateIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query, args := buildCalendarQuery("JFK", "NRT", "2026-10-01", "2026-10-02", "")
	explainUsesIndex(t, db.DB(), query, args...)
}
