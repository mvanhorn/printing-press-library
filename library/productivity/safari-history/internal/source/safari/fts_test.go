package safari

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

func TestSanitizeFTSAndSearch(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, _ = db.Exec(`CREATE TABLE history_items(id INTEGER PRIMARY KEY, url TEXT, visit_count INTEGER)`)
	_, _ = db.Exec(`CREATE TABLE history_visits(id INTEGER PRIMARY KEY, history_item INTEGER, visit_time REAL, title TEXT, origin INTEGER)`)
	_, _ = db.Exec(`CREATE VIRTUAL TABLE history_fts USING fts5(url, title, search_terms)`)

	now := timeToSafariSeconds(time.Now().UTC())
	_, _ = db.Exec(`INSERT INTO history_items(id, url, visit_count) VALUES (1, 'https://example.test/1', 3), (2, 'https://example.test/2', 2)`)
	_, _ = db.Exec(`INSERT INTO history_visits(id, history_item, visit_time, title, origin) VALUES (1, 1, ?, 'zzz nonexistent foo bar c tutorial a b', 0), (2, 2, ?, 'other row', 0)`, now, now)
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

func TestFullTextSearchDeviceFilterNoSinceDoesNotLeakSynced(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, _ = db.Exec(`CREATE TABLE history_items(id INTEGER PRIMARY KEY, url TEXT, visit_count INTEGER)`)
	_, _ = db.Exec(`CREATE TABLE history_visits(id INTEGER PRIMARY KEY, history_item INTEGER, visit_time REAL, title TEXT, origin INTEGER)`)
	_, _ = db.Exec(`CREATE VIRTUAL TABLE history_fts USING fts5(url, title, search_terms)`)

	now := timeToSafariSeconds(time.Now().UTC())
	_, _ = db.Exec(`INSERT INTO history_items(id, url, visit_count) VALUES (1, 'https://example.test/this', 1), (2, 'https://example.test/synced', 1)`)
	_, _ = db.Exec(`INSERT INTO history_visits(id, history_item, visit_time, title, origin) VALUES (1, 1, ?, 'needle phrase', 0), (2, 2, ?, 'needle phrase', 1)`, now, now)
	_, _ = db.Exec(`INSERT INTO history_fts(url, title, search_terms) VALUES ('https://example.test/this', 'needle phrase', ''), ('https://example.test/synced', 'needle phrase', '')`)

	src := New()
	noSince, err := src.FullTextSearch(db, "needle", source.VisitFilter{Device: "this", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(noSince) != 1 || noSince[0].URL != "https://example.test/this" {
		t.Fatalf("unexpected no-since rows: %#v", noSince)
	}

	withSince, err := src.FullTextSearch(db, "needle", source.VisitFilter{Device: "this", Since: time.Now().AddDate(-10, 0, 0), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(withSince) != 1 || withSince[0].URL != "https://example.test/this" {
		t.Fatalf("unexpected with-since rows: %#v", withSince)
	}
}
