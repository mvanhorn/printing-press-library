package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func makeSyntheticSnapshot(t *testing.T, path string) {
	t.Helper()
	db := openTestDB(t, path)
	stmts := []string{
		`CREATE TABLE history_items (id INTEGER PRIMARY KEY, url TEXT, domain_expansion TEXT, visit_count INTEGER)`,
		`CREATE TABLE history_visits (id INTEGER PRIMARY KEY, history_item INTEGER, visit_time REAL, title TEXT, origin INTEGER, redirect_source INTEGER)`,
		`INSERT INTO history_items(id, url, domain_expansion, visit_count) VALUES
			(1, 'https://example.com/a', 'example.com', 2),
			(2, 'https://openai.com/docs', 'openai.com', 1)`,
		`INSERT INTO history_visits(id, history_item, visit_time, title, origin, redirect_source) VALUES
			(10, 1, 101.25, 'Example Older', 0, 0),
			(11, 1, 102.50, 'Example Newer', 0, 0),
			(12, 2, 201.75, 'CodexManual Docs', 1, 10)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
}

func TestInitArchiveSchemaIdempotent(t *testing.T) {
	db := openTestDB(t, filepath.Join(t.TempDir(), "archive.db"))
	if err := InitArchiveSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	if err := InitArchiveSchema(db); err != nil {
		t.Fatalf("second init schema: %v", err)
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM archive_meta WHERE id=1`).Scan(&rows); err != nil {
		t.Fatalf("count archive_meta: %v", err)
	}
	if rows != 1 {
		t.Fatalf("archive_meta rows = %d, want 1", rows)
	}
}

func TestAccumulateFromSourceDedupCompatViewsAndFTS(t *testing.T) {
	dir := t.TempDir()
	snapshot := filepath.Join(dir, "snapshot.db")
	archive := filepath.Join(dir, "archive.db")
	makeSyntheticSnapshot(t, snapshot)

	now := time.Date(2026, 6, 12, 10, 30, 0, 0, time.UTC)
	first, err := AccumulateFromSource(archive, snapshot, now)
	if err != nil {
		t.Fatalf("first accumulate: %v", err)
	}
	if first.Before != 0 || first.Inserted != 3 || first.After != 3 {
		t.Fatalf("first counts = %+v, want before=0 inserted=3 after=3", first)
	}
	second, err := AccumulateFromSource(archive, snapshot, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("second accumulate: %v", err)
	}
	if second.Before != 3 || second.Inserted != 0 || second.After != 3 {
		t.Fatalf("second counts = %+v, want before=3 inserted=0 after=3", second)
	}

	db := openTestDB(t, archive)
	var pages, visits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_items`).Scan(&pages); err != nil {
		t.Fatalf("count history_items view: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_visits`).Scan(&visits); err != nil {
		t.Fatalf("count history_visits view: %v", err)
	}
	if pages != 2 || visits != 3 {
		t.Fatalf("view counts pages=%d visits=%d, want 2/3", pages, visits)
	}

	var visitCount int
	var domain string
	if err := db.QueryRow(`SELECT visit_count, domain_expansion FROM history_items WHERE url='https://example.com/a'`).Scan(&visitCount, &domain); err != nil {
		t.Fatalf("query history_items view: %v", err)
	}
	if visitCount != 2 || domain != "example.com" {
		t.Fatalf("history_items example row visit_count=%d domain=%q, want 2/example.com", visitCount, domain)
	}

	var title string
	var origin, redirect int
	if err := db.QueryRow(`SELECT title, origin, redirect_source FROM history_visits WHERE title='Example Newer'`).Scan(&title, &origin, &redirect); err != nil {
		t.Fatalf("query history_visits view: %v", err)
	}
	if title != "Example Newer" || origin != 0 || redirect != 0 {
		t.Fatalf("history_visits row title=%q origin=%d redirect=%d", title, origin, redirect)
	}

	if err := db.QueryRow(`SELECT origin, redirect_source FROM history_visits WHERE title='CodexManual Docs'`).Scan(&origin, &redirect); err != nil {
		t.Fatalf("query history_visits origin row: %v", err)
	}
	if origin != 1 || redirect != 0 {
		t.Fatalf("archive compat view origin=%d redirect=%d, want origin=1 redirect=0", origin, redirect)
	}

	var matches int
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_fts WHERE history_fts MATCH '"CodexManual"'`).Scan(&matches); err != nil {
		t.Fatalf("fts match: %v", err)
	}
	if matches != 1 {
		t.Fatalf("fts matches = %d, want 1", matches)
	}

	var enabled int
	var baseline string
	if err := db.QueryRow(`SELECT archive_enabled, baseline_at FROM archive_meta WHERE id=1`).Scan(&enabled, &baseline); err != nil {
		t.Fatalf("archive_meta: %v", err)
	}
	if enabled != 1 || baseline != now.Format(time.RFC3339) {
		t.Fatalf("archive metadata enabled=%d baseline=%q", enabled, baseline)
	}
}

func TestVacuumArchiveAbsentReturnsNotExist(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.db")
	// archive.db does NOT exist — VacuumArchive must return an os.IsNotExist error
	// and must NOT create the file.
	err := VacuumArchive(archivePath)
	if err == nil {
		t.Fatal("VacuumArchive on absent path: got nil error, want os.IsNotExist")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("VacuumArchive on absent path: got %v, want os.IsNotExist", err)
	}
	if _, statErr := os.Stat(archivePath); !os.IsNotExist(statErr) {
		t.Fatal("VacuumArchive on absent path: archive.db was created (should not be)")
	}
}

func TestActiveStorePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	snapshot, err := SnapshotPath()
	if err != nil {
		t.Fatalf("snapshot path: %v", err)
	}
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("archive path: %v", err)
	}
	if err := os.WriteFile(snapshot, []byte{}, 0o644); err != nil {
		t.Fatalf("write snapshot marker: %v", err)
	}

	got, isArchive, err := ActiveStorePath()
	if err != nil {
		t.Fatalf("active absent: %v", err)
	}
	if got != snapshot || isArchive {
		t.Fatalf("active absent path=%q isArchive=%v, want snapshot false", got, isArchive)
	}

	db := openTestDB(t, archive)
	if err := InitArchiveSchema(db); err != nil {
		t.Fatalf("init archive: %v", err)
	}
	got, isArchive, err = ActiveStorePath()
	if err != nil {
		t.Fatalf("active disabled: %v", err)
	}
	if got != snapshot || isArchive {
		t.Fatalf("active disabled path=%q isArchive=%v, want snapshot false", got, isArchive)
	}

	if _, err := db.Exec(`UPDATE archive_meta SET archive_enabled=1 WHERE id=1`); err != nil {
		t.Fatalf("enable archive: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	got, isArchive, err = ActiveStorePath()
	if err != nil {
		t.Fatalf("active enabled: %v", err)
	}
	if got != archive || !isArchive {
		t.Fatalf("active enabled path=%q isArchive=%v, want archive true", got, isArchive)
	}
}
