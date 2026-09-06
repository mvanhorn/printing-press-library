package store

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAvailabilityAllPrefersFreshestCopy(t *testing.T) {
	s := openTestStore(t)
	insertCopy := func(table, id, marker, synced string) {
		t.Helper()
		data := `{"ID":"` + id + `","Route":{"OriginAirport":"JFK","DestinationAirport":"NRT"},"marker":"` + marker + `"}`
		if _, err := s.DB().Exec(`INSERT OR REPLACE INTO "`+table+`" (id,data,synced_at,date) VALUES (?,?,?,'2026-10-01')`, id, data, synced); err != nil {
			t.Fatal(err)
		}
	}
	insertCopy("availability", "shared", "availability", "2026-01-01T00:00:00Z")
	insertCopy("awards", "shared", "awards", "2026-01-02T00:00:00Z")
	insertCopy("availability", "availability-only", "availability-only", "2026-01-01T00:00:00Z")
	insertCopy("awards", "awards-only", "awards-only", "2026-01-01T00:00:00Z")
	assertViewMarker(t, s.DB(), "shared", "awards")
	if _, err := s.DB().Exec(`UPDATE availability SET synced_at='2026-01-03T00:00:00Z' WHERE id='shared'`); err != nil {
		t.Fatal(err)
	}
	assertViewMarker(t, s.DB(), "shared", "availability")
	insertCopy("availability", "equal", "availability-equal", "2026-01-04T00:00:00Z")
	insertCopy("awards", "equal", "awards-equal", "2026-01-04T00:00:00Z")
	var equalCount int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM availability_all WHERE id='equal'`).Scan(&equalCount); err != nil || equalCount != 1 {
		t.Fatalf("equal count=%d err=%v", equalCount, err)
	}
	var count, distinct int
	if err := s.DB().QueryRow(`SELECT COUNT(*), COUNT(DISTINCT id) FROM availability_all`).Scan(&count, &distinct); err != nil {
		t.Fatal(err)
	}
	if count != 4 || distinct != 4 {
		t.Fatalf("view counts = %d/%d, want 4/4", count, distinct)
	}
}

func TestFirstSeenSurvivesInsertOrReplace(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.DB().Exec(`INSERT INTO availability(id,data) VALUES('replace-me','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE availability_first_seen SET first_seen_at='2000-01-01T00:00:00Z' WHERE id='replace-me'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT OR REPLACE INTO availability(id,data) VALUES('replace-me','{"changed":true}')`); err != nil {
		t.Fatal(err)
	}
	assertFirstSeen(t, s.DB(), "replace-me", 1, "2000-01-01T00:00:00Z", "availability")
}

func TestFirstSeenBackfillNormalisesFormat(t *testing.T) {
	path := t.TempDir() + "/data.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO availability_first_seen(id,first_seen_at,source_table) VALUES('old','2026-09-06 12:38:13','availability')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`DELETE FROM store_extras_meta WHERE key='extras_version'`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	assertFirstSeen(t, s.DB(), "old", 1, "2026-09-06T12:38:13Z", "availability")
	if _, err := s.DB().Exec(`INSERT INTO awards(id,data) VALUES('triggered','{}')`); err != nil {
		t.Fatal(err)
	}
	var bad int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM availability_first_seen WHERE first_seen_at NOT LIKE '____-__-__T__:__:__Z'`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Fatalf("non-RFC3339 first_seen rows = %d", bad)
	}
}

func TestExtrasOneTimeWorkIsGated(t *testing.T) {
	path := t.TempDir() + "/data.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO availability(id,data) VALUES('deleted-first-seen','{}')`); err != nil {
		t.Fatal(err)
	}
	var schema int
	var viewSQL string
	if err := s.DB().QueryRow(`PRAGMA schema_version`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='view' AND name='availability_all'`).Scan(&viewSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`DELETE FROM availability_first_seen WHERE id='deleted-first-seen'`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	var schema2, count int
	var viewSQL2 string
	if err := s.DB().QueryRow(`PRAGMA schema_version`).Scan(&schema2); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='view' AND name='availability_all'`).Scan(&viewSQL2); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM availability_first_seen WHERE id='deleted-first-seen'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if schema2 != schema || viewSQL2 != viewSQL || count != 0 {
		t.Fatalf("routine reopen changed state: schema %d/%d, view equal %v, row count %d", schema, schema2, viewSQL == viewSQL2, count)
	}
}

func TestAvailabilityAllColumnsMatchTable(t *testing.T) {
	s := openTestStore(t)
	got, want := tableColumns(t, s.DB(), "availability_all"), tableColumns(t, s.DB(), "awards")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("view columns = %v, want %v", got, want)
	}
}

func TestNovelQueriesUseExpressionIndex(t *testing.T) {
	s := openTestStore(t)
	rows, err := s.DB().Query(`EXPLAIN QUERY PLAN SELECT id FROM availability_all WHERE json_extract(data,'$.Route.OriginAirport')='JFK' AND json_extract(data,'$.Route.DestinationAirport')='NRT' AND date >= '2026-10-01' AND date < '2026-10-02'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		found = found || strings.Contains(detail, "idx_availability_od_date")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("query plan did not use idx_availability_od_date")
	}
}

func TestLegacyResourcesSeedFirstSeen(t *testing.T) {
	path := t.TempDir() + "/data.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO resources(id,resource_type,data,synced_at) VALUES('legacy','availability','{}','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`DELETE FROM store_extras_meta WHERE key='extras_version'`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	assertFirstSeen(t, s.DB(), "legacy", 1, "2026-01-01T00:00:00Z", "availability")
}

func TestAvailabilityExtras(t *testing.T) {
	s, err := Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	availabilityColumns := tableColumns(t, s.DB(), "availability")
	awardsColumns := tableColumns(t, s.DB(), "awards")
	if !reflect.DeepEqual(availabilityColumns, awardsColumns) {
		t.Fatalf("typed table columns differ:\navailability=%v\nawards=%v", availabilityColumns, awardsColumns)
	}

	first := json.RawMessage(`{"ID":"shared","RouteID":"route-1","Date":"2026-09-10","Source":"united","JAvailable":true,"JDirect":true,"JMileageCost":"29000","JMileageCostRaw":29000,"JRemainingSeats":2,"JAirlines":"UA","marker":"first"}`)
	second := json.RawMessage(`{"ID":"shared","RouteID":"route-1","Date":"2026-09-11","Source":"united","JAvailable":true,"JDirect":false,"JMileageCost":"31000","JMileageCostRaw":31000,"JRemainingSeats":1,"JAirlines":"UA","marker":"second"}`)
	if err := s.UpsertAvailability(first); err != nil {
		t.Fatalf("first UpsertAvailability: %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE availability SET synced_at='2001-02-03T04:05:06Z' WHERE id='shared'`); err != nil {
		t.Fatal(err)
	}
	const sentinel = "2001-02-03 04:05:06"
	if _, err := s.DB().Exec(`UPDATE availability_first_seen SET first_seen_at = ? WHERE id = ?`, sentinel, "shared"); err != nil {
		t.Fatalf("set first_seen_at sentinel: %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE availability SET synced_at = ? WHERE id = ?`, sentinel, "shared"); err != nil {
		t.Fatalf("set synced_at sentinel: %v", err)
	}
	if err := s.UpsertAvailability(second); err != nil {
		t.Fatalf("second UpsertAvailability: %v", err)
	}
	assertFirstSeen(t, s.DB(), "shared", 1, sentinel, "availability")
	var syncedAt string
	if err := s.DB().QueryRow(`SELECT synced_at FROM availability WHERE id = ?`, "shared").Scan(&syncedAt); err != nil {
		t.Fatalf("query updated synced_at: %v", err)
	}
	if syncedAt == sentinel {
		t.Fatal("second upsert did not update synced_at")
	}

	if err := s.UpsertAwards(json.RawMessage(`{"ID":"shared","RouteID":"route-1","Date":"2026-09-12","Source":"united","marker":"award-copy"}`)); err != nil {
		t.Fatalf("UpsertAwards shared: %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE awards SET synced_at='2001-02-03T04:05:05Z' WHERE id='shared'`); err != nil {
		t.Fatal(err)
	}
	assertFirstSeen(t, s.DB(), "shared", 1, sentinel, "availability")
	if err := s.UpsertAwards(json.RawMessage(`{"ID":"award-only","RouteID":"route-2","Date":"2026-09-13","Source":"aeroplan","marker":"award-only"}`)); err != nil {
		t.Fatalf("UpsertAwards award-only: %v", err)
	}

	rows, err := s.DB().Query(`SELECT id, json_extract(data, '$.marker') FROM availability_all ORDER BY id`)
	if err != nil {
		t.Fatalf("query availability_all: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	rowCount := 0
	for rows.Next() {
		rowCount++
		var id, marker string
		if err := rows.Scan(&id, &marker); err != nil {
			t.Fatalf("scan availability_all: %v", err)
		}
		got[id] = marker
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate availability_all: %v", err)
	}
	if rowCount != 2 {
		t.Fatalf("availability_all row count = %d, want 2", rowCount)
	}
	if want := map[string]string{"award-only": "award-only", "shared": "second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("availability_all = %v, want %v", got, want)
	}
}

func TestExtrasUpgradeRebuildsExistingV1Objects(t *testing.T) {
	path := t.TempDir() + "/data.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	oldView := `CREATE VIEW availability_all AS SELECT * FROM "availability" UNION ALL SELECT * FROM "awards" WHERE "awards".id NOT IN (SELECT id FROM "availability")`
	for _, q := range []string{
		`DROP VIEW availability_all`, oldView,
		`DROP TRIGGER trg_availability_first_seen`, `DROP TRIGGER trg_awards_first_seen`,
		`CREATE TRIGGER trg_availability_first_seen AFTER INSERT ON "availability" BEGIN INSERT OR IGNORE INTO availability_first_seen(id, first_seen_at, source_table) VALUES (NEW.id, CURRENT_TIMESTAMP, 'availability'); END`,
		`CREATE TRIGGER trg_awards_first_seen AFTER INSERT ON "awards" BEGIN INSERT OR IGNORE INTO availability_first_seen(id, first_seen_at, source_table) VALUES (NEW.id, CURRENT_TIMESTAMP, 'awards'); END`,
		`DROP TABLE store_extras_meta`,
	} {
		if _, err := s.DB().Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var viewSQL string
	if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='view' AND name='availability_all'`).Scan(&viewSQL); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(viewSQL, "NOT EXISTS") {
		t.Fatalf("view=%s", viewSQL)
	}
	for _, name := range []string{"trg_availability_first_seen", "trg_awards_first_seen"} {
		var sqlText string
		if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='trigger' AND name=?`, name).Scan(&sqlText); err != nil || !strings.Contains(sqlText, "WHEN NOT EXISTS") {
			t.Fatalf("trigger %s=%q err=%v", name, sqlText, err)
		}
	}
	var version, hash string
	if err := s.DB().QueryRow(`SELECT value FROM store_extras_meta WHERE key='extras_version'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT value FROM store_extras_meta WHERE key='view_hash'`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if version != "2" || hash == "" {
		t.Fatalf("version=%q hash=%q", version, hash)
	}
}

func TestAvailabilityAllSupportsLegacyAvailabilityShape(t *testing.T) {
	path := t.TempDir() + "/data.db"
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.UpsertAvailability(json.RawMessage(`{"ID":"availability-only"}`)); err != nil {
		t.Fatalf("UpsertAvailability: %v", err)
	}
	if err := s.UpsertAwards(json.RawMessage(`{"ID":"award-only"}`)); err != nil {
		t.Fatalf("UpsertAwards: %v", err)
	}
	if _, err := s.DB().Exec(`ALTER TABLE "availability" ADD COLUMN legacy_take INTEGER`); err != nil {
		t.Fatalf("add legacy column: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM availability_all`).Scan(&count); err != nil {
		t.Fatalf("count availability_all: %v", err)
	}
	if count != 2 {
		t.Fatalf("availability_all count = %d, want 2", count)
	}
}

func TestAvailabilityFirstSeenBackfillsPreUpgradeRows(t *testing.T) {
	path := t.TempDir() + "/data.db"
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := s.DB().Exec(`INSERT INTO availability(id,data,date) VALUES('pre-upgrade','{"ID":"pre-upgrade","Date":"2026-09-10"}','2026-09-10')`); err != nil {
		t.Fatalf("insert availability: %v", err)
	}
	const syncedAt = "2026-09-01 02:03:04"
	if _, err := s.DB().Exec(`UPDATE availability SET synced_at = ? WHERE id = ?`, syncedAt, "pre-upgrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`DELETE FROM availability_first_seen WHERE id = ?`, "pre-upgrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`DELETE FROM store_extras_meta WHERE key = 'extras_version'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`DROP TRIGGER trg_availability_first_seen`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	var equal int
	var gotTime string
	if err := s.DB().QueryRow(`SELECT datetime(first_seen_at) = datetime(?), first_seen_at FROM availability_first_seen WHERE id = ?`, syncedAt, "pre-upgrade").Scan(&equal, &gotTime); err != nil {
		t.Fatal(err)
	}
	if equal != 1 {
		t.Fatalf("backfilled first_seen_at %q does not equal synced_at", gotTime)
	}
}

func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatalf("scan table_info(%s): %v", table, err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info(%s): %v", table, err)
	}
	return columns
}

func assertFirstSeen(t *testing.T, db *sql.DB, id string, wantCount int, wantTime, wantSource string) {
	t.Helper()
	var count int
	var seenAt, source string
	if err := db.QueryRow(`SELECT COUNT(*), MIN(first_seen_at), MIN(source_table) FROM availability_first_seen WHERE id = ?`, id).Scan(&count, &seenAt, &source); err != nil {
		t.Fatalf("query first-seen row: %v", err)
	}
	if count != wantCount || seenAt != wantTime || source != wantSource {
		t.Fatalf("first-seen = (%d, %q, %q), want (%d, %q, %q)", count, seenAt, source, wantCount, wantTime, wantSource)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func assertViewMarker(t *testing.T, db *sql.DB, id, want string) {
	t.Helper()
	var got string
	if err := db.QueryRow(`SELECT json_extract(data,'$.marker') FROM availability_all WHERE id=?`, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("marker for %s = %q, want %q", id, got, want)
	}
}
