package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func TestNovelNewSince(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := []map[string]any{
		{"ID": "new-nrt-a", "RouteID": "jfk-nrt-a", "Route": map[string]any{"ID": "jfk-nrt-a", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "aeroplan"}, "Date": "2026-09-10", "Source": "aeroplan", "JAvailable": true, "JDirect": true, "JMileageCost": "29000", "JMileageCostRaw": 29000, "JDirectMileageCostRaw": 29000, "JRemainingSeats": 2, "JAirlines": "NH"},
		{"ID": "new-lhr", "RouteID": "jfk-lhr", "Route": map[string]any{"ID": "jfk-lhr", "OriginAirport": "JFK", "DestinationAirport": "LHR", "Source": "united"}, "Date": "2026-09-11", "Source": "united", "JAvailable": true, "JDirect": false, "JMileageCost": "45000", "JMileageCostRaw": 45000, "JRemainingSeats": 1, "JAirlines": "UA"},
		{"ID": "old-nrt", "RouteID": "jfk-nrt-b", "Route": map[string]any{"ID": "jfk-nrt-b", "OriginAirport": "JFK", "DestinationAirport": "NRT", "Source": "alaska"}, "Date": "2026-09-12", "Source": "alaska", "JAvailable": true, "JDirect": true, "JMileageCost": "60000", "JMileageCostRaw": 60000, "JDirectMileageCostRaw": 60000, "JRemainingSeats": 1, "JAirlines": "JL"},
		{"ID": "ewr", "RouteID": "ewr-nrt", "Route": map[string]any{"OriginAirport": "EWR", "DestinationAirport": "NRT"}, "Date": "2026-09-13T10:00:00Z", "Source": "united", "JAvailable": true, "JMileageCostRaw": 70000},
	}
	for _, item := range rows {
		raw, _ := json.Marshal(item)
		if err := db.UpsertAvailability(raw); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().UTC().Add(-72 * time.Hour).Format("2006-01-02 15:04:05")
	if _, err := db.DB().Exec(`UPDATE availability_first_seen SET first_seen_at=? WHERE id=?`, old, "old-nrt"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		args   []string
		count  int
		wantID string
	}{
		{"day", []string{"--since", "24h"}, 3, "new-nrt-a"},
		{"week", []string{"--since", "7d"}, 4, "old-nrt"},
		{"destination", []string{"--since", "7d", "--destination", "LHR"}, 1, "new-lhr"},
		{"empty cabin", []string{"--since", "7d", "--cabin", "first"}, 0, ""},
		{"origin", []string{"--since", "7d", "--origin", "JFK"}, 3, ""},
		{"source", []string{"--since", "7d", "--source", "united"}, 2, ""},
		{"business cabin", []string{"--since", "24h", "--origin", "JFK", "--destination", "LHR", "--cabin", "business"}, 1, "new-lhr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"new-since", "--json", "--db", path}, tt.args...)
			out, _, err := executeRoot(args...)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			var got []newSinceRow
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON %q: %v", out.String(), err)
			}
			if len(got) != tt.count {
				t.Fatalf("got %d rows, want %d: %s", len(got), tt.count, out.String())
			}
			if tt.count == 0 && strings.TrimSpace(out.String()) != "[]" {
				t.Fatalf("empty output=%q, want []", out.String())
			}
			if tt.wantID != "" {
				found := false
				for _, r := range got {
					if r.ID == tt.wantID {
						found = true
						if r.Source == "" || r.FirstSeenAt == "" {
							t.Fatalf("required fields not populated: %+v", r)
						}
					}
				}
				if !found {
					t.Fatalf("missing id %q", tt.wantID)
				}
			}
		})
	}
}

func TestNovelEmptyTabularOutputIncludesHeader(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		sep  string
	}{
		{"new csv", []string{"new-since", "--db", path, "--since", "1h", "--csv"}, ","},
		{"new plain", []string{"new-since", "--db", path, "--since", "1h", "--plain"}, "\t"},
		{"direct csv", []string{"direct-scan", "--db", path, "--csv"}, ","},
		{"direct plain", []string{"direct-scan", "--db", path, "--plain"}, "\t"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := executeRoot(tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(out.String(), "\n") != 1 || !strings.Contains(out.String(), tc.sep) {
				t.Fatalf("stdout=%q", out.String())
			}
		})
	}
}

func TestNovelMetaSyncedFalseOnUnsyncedStore(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := rawObject(t, map[string]any{
		"ID":      "unsynced-availability",
		"RouteID": "jfk-nrt",
		"Route": map[string]any{
			"OriginAirport":      "JFK",
			"DestinationAirport": "NRT",
		},
		"Date":       "2099-01-01",
		"Source":     "aeroplan",
		"JAvailable": true,
	})
	if err := db.UpsertAvailability(raw); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	readMeta := func() map[string]any {
		t.Helper()
		out, _, err := executeRoot("new-since", "--db", path, "--since", "1h", "--json", "--agent")
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Meta map[string]any `json:"meta"`
		}
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
			t.Fatalf("decode envelope %q: %v", out.String(), err)
		}
		if envelope.Meta == nil {
			t.Fatalf("output %q has no meta envelope", out.String())
		}
		return envelope.Meta
	}

	meta := readMeta()
	if meta["synced"] != false || meta["last_synced_at"] != nil {
		t.Fatalf("unsynced meta=%v, want synced=false and last_synced_at=null", meta)
	}

	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	wantSyncedAt := time.Date(2026, 9, 6, 15, 4, 5, 0, time.UTC)
	if err := db.SaveSyncStateAt("availability", "", 1, wantSyncedAt); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	meta = readMeta()
	if meta["synced"] != true {
		t.Fatalf("synced meta=%v, want synced=true", meta)
	}
	lastSyncedAt, ok := meta["last_synced_at"].(string)
	if !ok {
		t.Fatalf("last_synced_at=%T %v, want RFC3339 string", meta["last_synced_at"], meta["last_synced_at"])
	}
	parsed, err := time.Parse(time.RFC3339, lastSyncedAt)
	if err != nil {
		t.Fatalf("last_synced_at=%q is not RFC3339: %v", lastSyncedAt, err)
	}
	if !parsed.Equal(wantSyncedAt) {
		t.Fatalf("last_synced_at=%q, want %q", lastSyncedAt, wantSyncedAt.Format(time.RFC3339))
	}
}

func TestNovelNewSinceDatetimeBoundary(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"before", "after"} {
		raw := json.RawMessage(`{"ID":"` + id + `","RouteID":"r","Route":{"OriginAirport":"JFK","DestinationAirport":"NRT"},"Date":"2099-01-01T10:00:00Z","Source":"united","JAvailable":true}`)
		if err := db.UpsertAvailability(raw); err != nil {
			t.Fatal(err)
		}
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	if _, err := db.DB().Exec(`UPDATE availability_first_seen SET first_seen_at=? WHERE id='before'`, cutoff.Add(-time.Second).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE availability_first_seen SET first_seen_at=? WHERE id='after'`, cutoff.Add(time.Second).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, _, err := executeRoot("new-since", "--db", path, "--since", "24h", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got []newSinceRow
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "after" || got[0].Date != "2099-01-01" {
		t.Fatalf("rows=%+v", got)
	}
}

func TestNovelNewSinceRejectsLiveDataSource(t *testing.T) {
	isolateNovelTest(t)
	_, _, err := executeRoot("new-since", "--data-source", "live", "--since", "24h", "--json")
	if err == nil {
		t.Fatal("expected usage error")
	}
	ce, ok := err.(*cliError)
	if !ok || ce.code != 2 {
		t.Fatalf("error=%T %v, want exit-2 cliError", err, err)
	}
	if !strings.Contains(err.Error(), "no live equivalent") {
		t.Fatalf("error=%v", err)
	}
}

func TestNovelNewSinceDeterministicTieAndNullDate(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	for _, item := range []struct{ id, date string }{{"z", "2099-01-02"}, {"b", "2099-01-01"}, {"a", "2099-01-01"}, {"null", "2099-01-03"}} {
		raw, _ := json.Marshal(map[string]any{"ID": item.id, "Route": map[string]any{"OriginAirport": "JFK", "DestinationAirport": "NRT"}, "Date": item.date, "JAvailable": true})
		if err := db.UpsertAvailability(raw); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().Exec(`UPDATE availability_first_seen SET first_seen_at=? WHERE id=?`, stamp, item.id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.DB().Exec(`UPDATE availability SET date=NULL WHERE id='null'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 2; run++ {
		out, _, err := executeRoot("new-since", "--db", path, "--since", "1h", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var got []newSinceRow
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 || got[0].ID != "a" || got[1].ID != "b" || got[2].ID != "z" {
			t.Fatalf("run %d rows=%+v", run, got)
		}
	}
}
