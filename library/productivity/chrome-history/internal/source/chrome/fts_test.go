package chrome

import (
	"database/sql"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
	_ "modernc.org/sqlite"
)

func TestSanitizeFTSAndSearch(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, _ = db.Exec(`CREATE TABLE urls(id INTEGER PRIMARY KEY, url TEXT, visit_count INTEGER, last_visit_time INTEGER)`)
	_, _ = db.Exec(`CREATE TABLE visits(id INTEGER PRIMARY KEY, url INTEGER, visit_time INTEGER)`)
	_, _ = db.Exec(`CREATE TABLE visit_source(id INTEGER PRIMARY KEY, source INTEGER, originator_cache_guid TEXT)`)
	_, _ = db.Exec(`CREATE VIRTUAL TABLE history_fts USING fts5(url, title, search_terms)`)
	now := timeToChromeMicros(time.Now().UTC())
	_, _ = db.Exec(`CREATE UNIQUE INDEX idx_urls_url ON urls(url)`)
	_, _ = db.Exec(`INSERT INTO urls(id, url, visit_count, last_visit_time) VALUES (1, 'https://example.test/1', 3, ?), (2, 'https://example.test/2', 2, ?)`, now, now)
	_, _ = db.Exec(`INSERT INTO visits(id, url, visit_time) VALUES (11, 1, ?), (12, 2, ?)`, now, now)
	_, _ = db.Exec(`INSERT INTO history_fts(url, title, search_terms) VALUES ('https://example.test/1', 'zzz nonexistent foo bar c tutorial a b', ''), ('https://example.test/2', 'other row', '')`)

	src := New()
	queries := []string{`zzz-nonexistent`, `c++ tutorial`, `foo:bar`, `a"b`}
	for _, q := range queries {
		rows, err := src.FullTextSearch(db, q, source.VisitFilter{Limit: 5})
		if err != nil {
			t.Fatalf("query %q err: %v", q, err)
		}
		if q == "zzz-nonexistent" && len(rows) == 0 {
			t.Fatalf("expected hit for %q", q)
		}
	}
}

func TestFullTextSearchDeviceThisNoSinceNoLeak(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, _ = db.Exec(`CREATE TABLE urls(id INTEGER PRIMARY KEY, url TEXT, visit_count INTEGER, last_visit_time INTEGER)`)
	_, _ = db.Exec(`CREATE TABLE visits(id INTEGER PRIMARY KEY, url INTEGER, visit_time INTEGER, originator_cache_guid TEXT)`)
	_, _ = db.Exec(`CREATE TABLE visit_source(id INTEGER PRIMARY KEY, source INTEGER, originator_cache_guid TEXT)`)
	_, _ = db.Exec(`CREATE VIRTUAL TABLE history_fts USING fts5(url, title, search_terms)`)
	now := time.Now().UTC()
	nowMicros := timeToChromeMicros(now)
	_, _ = db.Exec(`CREATE UNIQUE INDEX idx_urls_url ON urls(url)`)
	_, _ = db.Exec(`INSERT INTO urls(id, url, visit_count, last_visit_time) VALUES
		(1, 'https://this.test/a', 2, ?),
		(2, 'https://synced.test/a', 1, ?),
		(3, 'https://ext.test/a', 1, ?),
		(4, 'https://imported.test/a', 1, ?)`, nowMicros, nowMicros, nowMicros, nowMicros)
	_, _ = db.Exec(`INSERT INTO visits(id, url, visit_time, originator_cache_guid) VALUES
		(11, 1, ?, ''),
		(12, 2, ?, 'guid-sync'),
		(13, 3, ?, ''),
		(14, 4, ?, '')`, nowMicros, nowMicros, nowMicros, nowMicros)
	_, _ = db.Exec(`INSERT INTO visit_source(id, source, originator_cache_guid) VALUES
		(11, 1, ''),
		(12, 0, 'guid-sync'),
		(13, 2, ''),
		(14, 3, '')`)
	_, _ = db.Exec(`INSERT INTO history_fts(url, title, search_terms) VALUES
		('https://this.test/a', 'match token', ''),
		('https://synced.test/a', 'match token', ''),
		('https://ext.test/a', 'match token', ''),
		('https://imported.test/a', 'match token', '')`)

	src := New()
	filterNoSince := source.VisitFilter{Limit: 10, Device: "this"}
	filterSince := source.VisitFilter{Limit: 10, Device: "this", Since: now.AddDate(-10, 0, 0)}
	rowsNoSince, err := src.FullTextSearch(db, "match", filterNoSince)
	if err != nil {
		t.Fatal(err)
	}
	rowsSince, err := src.FullTextSearch(db, "match", filterSince)
	if err != nil {
		t.Fatal(err)
	}
	if len(rowsNoSince) != len(rowsSince) {
		t.Fatalf("this-device count mismatch no-since=%d since=%d", len(rowsNoSince), len(rowsSince))
	}
	if len(rowsNoSince) != 1 {
		t.Fatalf("expected only one local row, got %d", len(rowsNoSince))
	}
	if rowsNoSince[0].URL != "https://this.test/a" {
		t.Fatalf("expected local URL only, got %s", rowsNoSince[0].URL)
	}
}
