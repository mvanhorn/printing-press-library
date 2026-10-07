// Hand-authored coverage for the per-work "yearly counts synced" marker. Not generated.

package lancet

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func countsSyncedAt(t *testing.T, db *sql.DB, workID string) sql.NullString {
	t.Helper()
	var v sql.NullString
	if err := db.QueryRow(`SELECT counts_synced_at FROM lancet_works WHERE work_id = ?`, workID).Scan(&v); err != nil {
		t.Fatalf("read counts_synced_at of %s: %v", workID, err)
	}
	return v
}

// a) a synced work with no yearly rows is a true zero, an unsynced one is no-data.
func TestVelocityUnsyncedIsNullSyncedZeroIsZero(t *testing.T) {
	db := velDB(t,
		velSeed{id: "1", title: "Rate synced zero", pubYear: 2015, cited: 5, counts: map[int]int{}},
		velSeed{id: "2", title: "Rate never refreshed", pubYear: 2015, cited: 900},
		velSeed{id: "3", title: "Rate synced busy", pubYear: 2015, cited: 50, counts: map[int]int{time.Now().UTC().Year() - 1: 10}},
	)
	rows := mustVelCurate(t, db, "velocity", 10)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	by := byTitle(rows)
	zero := 0.0
	wantField(t, "synced zero", by["Rate synced zero"], "velocity", &zero)
	wantField(t, "synced zero", by["Rate synced zero"], "citations_last_year", &zero)
	wantField(t, "never refreshed", by["Rate never refreshed"], "velocity", nil)
	wantField(t, "never refreshed", by["Rate never refreshed"], "citations_last_year", nil)
	wantField(t, "never refreshed", by["Rate never refreshed"], "acceleration", nil)
	order := []string{rows[0]["title"].(string), rows[1]["title"].(string), rows[2]["title"].(string)}
	want := []string{"Rate synced busy", "Rate synced zero", "Rate never refreshed"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v (unsynced sorts after a synced zero)", order, want)
		}
	}
}

// b) decoding tells an empty counts_by_year array from an absent field.
func TestDecodeCountsByYearEmptyArrayVersusAbsent(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"empty array", `{"id":"https://openalex.org/W1","counts_by_year":[]}`, true},
		{"filled array", `{"id":"https://openalex.org/W1","counts_by_year":[{"year":2023,"cited_by_count":4}]}`, true},
		{"absent", `{"id":"https://openalex.org/W1"}`, false},
		{"null", `{"id":"https://openalex.org/W1","counts_by_year":null}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var r rawWork
			if err := json.Unmarshal([]byte(c.body), &r); err != nil {
				t.Fatal(err)
			}
			if got := decodeWork(r).CountsSynced; got != c.want {
				t.Fatalf("CountsSynced = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRefreshSetsMarkerOnlyWhenFieldPresent(t *testing.T) {
	db := velDB(t,
		velSeed{id: "1", title: "Rate empty", pubYear: 2015, counts: map[int]int{}},
		velSeed{id: "2", title: "Rate absent", pubYear: 2015},
	)
	if v := countsSyncedAt(t, db, "W1"); !v.Valid || v.String == "" {
		t.Errorf("W1 (counts_by_year: []) counts_synced_at = %+v, want a timestamp", v)
	}
	if v := countsSyncedAt(t, db, "W2"); v.Valid {
		t.Errorf("W2 (no counts_by_year) counts_synced_at = %+v, want NULL", v)
	}
}

// d) the column is added idempotently, also on a store that already has the U4 columns.
func TestEnsureSchemaAddsCountsSyncedAtIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "u4.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE lancet_works (work_id TEXT PRIMARY KEY, doi TEXT, title TEXT, journal_issn TEXT,
		journal_name TEXT, pub_year INTEGER, pub_date TEXT, cited_count INTEGER, is_oa INTEGER, topic TEXT, synced_at TEXT,
		fwci REAL, citation_normalized_percentile REAL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO lancet_works (work_id, title, pub_year, cited_count) VALUES ('W9','Old u4 work',2015,7)`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := EnsureSchema(ctx, db); err != nil {
			t.Fatalf("EnsureSchema run %d: %v", i+1, err)
		}
	}
	have, err := worksColumns(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if !have["counts_synced_at"] {
		t.Fatal("counts_synced_at column missing after migration")
	}
	if v := countsSyncedAt(t, db, "W9"); v.Valid {
		t.Errorf("pre-existing work counts_synced_at = %+v, want NULL", v)
	}
}

// e) a second refresh keeps the marker, with or without the field in the response.
func TestSecondRefreshKeepsAndUpdatesMarker(t *testing.T) {
	ctx := context.Background()
	db := velDB(t, velSeed{id: "1", title: "Rate once", pubYear: 2015, counts: map[int]int{}})
	if _, err := db.Exec(`UPDATE lancet_works SET counts_synced_at = '2000-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Refresh(ctx, stubFetcher{payload: velPayload(t, velSeed{id: "1", title: "Rate once", pubYear: 2015, counts: map[int]int{}})}, db, velJournal, 0, 0, 1, nil); err != nil {
		t.Fatal(err)
	}
	if v := countsSyncedAt(t, db, "W1"); !v.Valid || v.String == "2000-01-01T00:00:00Z" {
		t.Errorf("marker after a refresh with the field = %+v, want it updated", v)
	}
	if _, err := db.Exec(`UPDATE lancet_works SET counts_synced_at = '2000-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Refresh(ctx, stubFetcher{payload: velPayload(t, velSeed{id: "1", title: "Rate once", pubYear: 2015})}, db, velJournal, 0, 0, 1, nil); err != nil {
		t.Fatal(err)
	}
	if v := countsSyncedAt(t, db, "W1"); !v.Valid || v.String != "2000-01-01T00:00:00Z" {
		t.Errorf("marker after a refresh without the field = %+v, want it kept", v)
	}
}

// staleMarker backdates a work's marker to a year before the current one.
func staleMarker(t *testing.T, db *sql.DB, workID string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE lancet_works SET counts_synced_at = '2000-01-01T00:00:00Z' WHERE work_id = ?`, workID); err != nil {
		t.Fatal(err)
	}
}

// f) a marker from before the current year is no-data, like an unsynced work.
func TestVelocityStaleMarkerIsNullAndSortsLast(t *testing.T) {
	y := time.Now().UTC().Year()
	db := velDB(t,
		velSeed{id: "1", title: "Rate stale high", pubYear: 2015, cited: 900, counts: map[int]int{y - 1: 500, y - 2: 500, y - 3: 500}},
		velSeed{id: "2", title: "Rate fresh low", pubYear: 2015, cited: 5, counts: map[int]int{y - 1: 1}},
	)
	staleMarker(t, db, "W1")
	rows := mustVelCurate(t, db, "velocity", 10)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	by := byTitle(rows)
	wantField(t, "stale", by["Rate stale high"], "velocity", nil)
	wantField(t, "stale", by["Rate stale high"], "citations_last_year", nil)
	wantField(t, "stale", by["Rate stale high"], "acceleration", nil)
	if rows[0]["title"] != "Rate fresh low" || rows[1]["title"] != "Rate stale high" {
		t.Fatalf("order = %v, %v; want the fresh work first", rows[0]["title"], rows[1]["title"])
	}
}

// g) the coverage helper behind the notice counts a stale work as not current.
func TestVelocityCoverageCountsStaleAsNotCurrent(t *testing.T) {
	y := time.Now().UTC().Year()
	db := velDB(t,
		velSeed{id: "1", title: "Rate stale", pubYear: 2015, counts: map[int]int{y - 1: 5}},
		velSeed{id: "2", title: "Rate fresh", pubYear: 2015, counts: map[int]int{y - 1: 1}},
	)
	staleMarker(t, db, "W1")
	m, n, err := VelocityCoverage(context.Background(), db, "rate", "", false)
	if err != nil || m != 2 || n != 1 {
		t.Fatalf("coverage = (%d, %d, %v), want (2, 1, nil)", m, n, err)
	}
}
