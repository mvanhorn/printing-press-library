package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
	chromeSource "github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source/chrome"
	_ "modernc.org/sqlite"
)

func TestAccumulateDedupIdempotency(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	source := newSourceFixture(t, 1, 10)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	first, err := AccumulateFromSource(archive, source, time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("first accumulate: %v", err)
	}
	second, err := AccumulateFromSource(archive, source, time.Date(2026, 6, 11, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("second accumulate: %v", err)
	}
	if first.Appended != 10 || first.Total != 10 {
		t.Fatalf("first counts = %+v, want appended=10 total=10", first)
	}
	if second.Appended != 0 || second.Total != 10 {
		t.Fatalf("second counts = %+v, want appended=0 total=10", second)
	}
}

func TestAccumulateDurabilityAcrossSourcePrune(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	source := newSourceFixture(t, 1, 10)
	if _, err := AccumulateFromSource(archive, source, time.Now()); err != nil {
		t.Fatalf("first accumulate: %v", err)
	}
	prunedSource := newSourceFixture(t, 8, 15)
	counts, err := AccumulateFromSource(archive, prunedSource, time.Now())
	if err != nil {
		t.Fatalf("second accumulate: %v", err)
	}
	if counts.Appended != 5 || counts.Total != 15 {
		t.Fatalf("counts = %+v, want appended=5 total=15", counts)
	}
	if got := countRows(t, archive, "history_archive"); got != 15 {
		t.Fatalf("archive rows = %d, want 15", got)
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot.db")
	copyFile(t, prunedSource, snapshot)
	if _, err := BuildSnapshotIndex(snapshot, "Default"); err != nil {
		t.Fatalf("BuildSnapshotIndex: %v", err)
	}
	if got := countRows(t, snapshot, "visits"); got != 8 {
		t.Fatalf("snapshot visits = %d, want 8", got)
	}
}

func TestActiveStorePathMatrix(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	snapshot, err := SnapshotPath()
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	path, isArchive, err := ActiveStorePath()
	if err != nil {
		t.Fatalf("ActiveStorePath absent: %v", err)
	}
	if path != snapshot || isArchive {
		t.Fatalf("absent archive path=%q isArchive=%v, want snapshot %q false", path, isArchive, snapshot)
	}
	source := newSourceFixture(t, 1, 1)
	if _, err := AccumulateFromSource(archive, source, time.Now()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	path, isArchive, err = ActiveStorePath()
	if err != nil {
		t.Fatalf("ActiveStorePath enabled: %v", err)
	}
	if path != archive || !isArchive {
		t.Fatalf("enabled archive path=%q isArchive=%v, want archive %q true", path, isArchive, archive)
	}
	if err := os.Remove(archive); err != nil {
		t.Fatalf("remove archive: %v", err)
	}
	path, isArchive, err = ActiveStorePath()
	if err != nil {
		t.Fatalf("ActiveStorePath removed: %v", err)
	}
	if path != snapshot || isArchive {
		t.Fatalf("removed archive path=%q isArchive=%v, want snapshot %q false", path, isArchive, snapshot)
	}
}

func TestArchiveFTSIncrementalAndPrunedRowStillFound(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	source := newSourceFixtureWithTitle(t, []fixtureVisit{
		{N: 1, URL: "https://example.test/old", Title: "durable needle"},
		{N: 2, URL: "https://example.test/new", Title: "current row"},
		{N: 3, URL: "https://example.test/repeated", Title: "repeat needle"},
		{N: 4, URL: "https://example.test/repeated", Title: "repeat needle again"},
	})
	if _, err := AccumulateFromSource(archive, source, time.Now()); err != nil {
		t.Fatalf("first accumulate: %v", err)
	}
	st, err := Open(archive)
	if err != nil {
		t.Fatalf("Open archive: %v", err)
	}
	src := chromeSource.New()
	matches, err := src.FullTextSearch(st.DB(), "repeat", archiveReadFilter())
	if err != nil {
		t.Fatalf("FullTextSearch repeat: %v", err)
	}
	if len(matches) != 1 || matches[0].URL != "https://example.test/repeated" {
		t.Fatalf("repeat matches = %+v, want one repeated URL", matches)
	}
	st.Close()
	pruned := newSourceFixtureWithTitle(t, []fixtureVisit{
		{N: 2, URL: "https://example.test/new", Title: "current row"},
		{N: 3, URL: "https://example.test/repeated", Title: "repeat needle"},
	})
	if _, err := AccumulateFromSource(archive, pruned, time.Now()); err != nil {
		t.Fatalf("second accumulate: %v", err)
	}
	st, err = Open(archive)
	if err != nil {
		t.Fatalf("reopen archive: %v", err)
	}
	defer st.Close()
	matches, err = src.FullTextSearch(st.DB(), "durable", archiveReadFilter())
	if err != nil {
		t.Fatalf("FullTextSearch durable: %v", err)
	}
	if len(matches) != 1 || matches[0].URL != "https://example.test/old" {
		t.Fatalf("durable matches = %+v, want pruned archive row", matches)
	}
}

func TestArchiveAUTriggerNoDuplicateFTSOnURLChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		t.Fatalf("InitArchiveSchema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO history_archive(url, visit_time, title, visit_count) VALUES
		('https://example.test/a', 1, 'A', 1),
		('https://example.test/b', 2, 'B', 1)`); err != nil {
		t.Fatalf("insert archive rows: %v", err)
	}
	if _, err := db.Exec(`UPDATE history_archive SET url='https://example.test/b' WHERE url='https://example.test/a'`); err != nil {
		t.Fatalf("update archive url: %v", err)
	}
	var ftsRows int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_fts WHERE url='https://example.test/b'`).Scan(&ftsRows); err != nil {
		t.Fatalf("count FTS rows: %v", err)
	}
	if ftsRows != 1 {
		t.Fatalf("FTS rows for updated URL = %d, want 1", ftsRows)
	}
}

func TestArchiveFTSTitleRefreshedOnRevisit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		t.Fatalf("InitArchiveSchema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO history_archive(url, visit_time, title) VALUES
		('https://x.test/page', 100, 'old boring title'),
		('https://x.test/page', 200, 'fresh kayak rental')`); err != nil {
		t.Fatalf("insert archive rows: %v", err)
	}
	var ftsRows int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_fts WHERE url='https://x.test/page'`).Scan(&ftsRows); err != nil {
		t.Fatalf("count FTS rows: %v", err)
	}
	if ftsRows != 1 {
		t.Fatalf("FTS rows for revisited URL = %d, want 1", ftsRows)
	}
	var title string
	if err := db.QueryRow(`SELECT title FROM history_fts WHERE url='https://x.test/page'`).Scan(&title); err != nil {
		t.Fatalf("read FTS title: %v", err)
	}
	if title != "fresh kayak rental" {
		t.Fatalf("FTS title = %q, want newest title", title)
	}
	var matches int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_fts WHERE history_fts MATCH 'kayak'`).Scan(&matches); err != nil {
		t.Fatalf("count kayak FTS matches: %v", err)
	}
	if matches != 1 {
		t.Fatalf("kayak FTS matches = %d, want 1", matches)
	}
}

func TestMetaPPSingletonEnforced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		t.Fatalf("InitArchiveSchema: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin rollback-only duplicate probe: %v", err)
	}
	_, err = tx.Exec(`INSERT INTO meta_pp(id, archive_enabled, schema_version) VALUES (1, 1, 2)`)
	if rbErr := tx.Rollback(); rbErr != nil {
		t.Fatalf("rollback duplicate probe: %v", rbErr)
	}
	if err == nil {
		t.Errorf("second meta_pp insert succeeded, want constraint error")
	}
	if _, err := db.Exec(`INSERT INTO meta_pp(archive_enabled, schema_version) VALUES (1, 2)`); err == nil {
		t.Errorf("second meta_pp insert without id succeeded, want constraint error")
	}
	var rows int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM meta_pp`).Scan(&rows); err != nil {
		t.Fatalf("count meta_pp rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("meta_pp rows = %d, want 1", rows)
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM meta_pp`).Scan(&id); err != nil {
		t.Fatalf("read meta_pp id: %v", err)
	}
	if id != 1 {
		t.Fatalf("meta_pp id = %d, want 1", id)
	}
}

func TestMigrateArchiveFTSRebuildsIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		t.Fatalf("InitArchiveSchema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO history_archive(url, visit_time, title, visit_count) VALUES
		('https://example.test/a', 1, 'A old', 1),
		('https://example.test/a', 2, 'A new', 1),
		('https://example.test/b', 3, 'B', 1)`); err != nil {
		t.Fatalf("insert archive rows: %v", err)
	}
	if _, err := db.Exec(`UPDATE meta_pp SET schema_version=1`); err != nil {
		t.Fatalf("lower schema version: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM history_fts`); err != nil {
		t.Fatalf("empty FTS: %v", err)
	}
	if err := migrateArchiveFTS(db); err != nil {
		t.Fatalf("migrateArchiveFTS: %v", err)
	}
	var ftsRows, version int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_fts`).Scan(&ftsRows); err != nil {
		t.Fatalf("count FTS rows: %v", err)
	}
	if ftsRows != 2 {
		t.Fatalf("FTS rows = %d, want 2 distinct URLs", ftsRows)
	}
	if err := db.QueryRow(`SELECT schema_version FROM meta_pp LIMIT 1`).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != archiveSchemaVersion {
		t.Fatalf("schema_version = %d, want %d", version, archiveSchemaVersion)
	}
}

func TestArchiveCompatibilityViewsSupportCoreReads(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	source := newSourceFixtureWithTitle(t, []fixtureVisit{
		{N: 1, URL: "https://example.test/list-search-sql", Title: "compatibility needle"},
	})
	if _, err := AccumulateFromSource(archive, source, time.Now()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	st, err := Open(archive)
	if err != nil {
		t.Fatalf("Open archive: %v", err)
	}
	defer st.Close()
	src := chromeSource.New()
	visits, err := src.RecentVisits(st.DB(), archiveReadFilter())
	if err != nil {
		t.Fatalf("RecentVisits: %v", err)
	}
	if len(visits) != 1 || visits[0].URL != "https://example.test/list-search-sql" {
		t.Fatalf("RecentVisits = %+v, want archive row", visits)
	}
	matches, err := src.FullTextSearch(st.DB(), "needle", archiveReadFilter())
	if err != nil {
		t.Fatalf("FullTextSearch: %v", err)
	}
	if len(matches) != 1 || matches[0].Title != "compatibility needle" {
		t.Fatalf("FullTextSearch = %+v, want archive row", matches)
	}
	rows, err := st.RunSelect(`SELECT url, title FROM urls WHERE url LIKE '%list-search-sql%'`, 10)
	if err != nil {
		t.Fatalf("RunSelect: %v", err)
	}
	if len(rows) != 1 || rows[0]["url"] != "https://example.test/list-search-sql" {
		t.Fatalf("RunSelect = %+v, want archive url view row", rows)
	}
}

func TestStickyPlainAccumulateGrowsAfterEnable(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	firstSource := newSourceFixture(t, 1, 2)
	if _, err := AccumulateFromSource(archive, firstSource, time.Now()); err != nil {
		t.Fatalf("enable accumulate: %v", err)
	}
	status, err := ReadArchiveStatus()
	if err != nil {
		t.Fatalf("ReadArchiveStatus: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("archive status enabled = false, want true")
	}
	nextSource := newSourceFixture(t, 1, 3)
	if _, err := AccumulateFromSource(archive, nextSource, time.Now()); err != nil {
		t.Fatalf("sticky accumulate: %v", err)
	}
	if got := countRows(t, archive, "history_archive"); got != 3 {
		t.Fatalf("archive rows after sticky accumulate = %d, want 3", got)
	}
}

func TestReadArchiveStatusAbsent(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	status, err := ReadArchiveStatus()
	if err != nil {
		t.Fatalf("ReadArchiveStatus: %v", err)
	}
	if status.Enabled {
		t.Fatalf("status enabled = true, want false")
	}
}

func TestEnableArchiveFromSourceIdempotent(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	source := newSourceFixture(t, 1, 3)
	first, alreadyEnabled, err := EnableArchiveFromSource(source, time.Date(2026, 6, 12, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("first enable: %v", err)
	}
	if alreadyEnabled || first.Appended != 3 || first.Total != 3 {
		t.Fatalf("first enable counts=%+v already=%v, want appended=3 total=3 already=false", first, alreadyEnabled)
	}
	second, alreadyEnabled, err := EnableArchiveFromSource(source, time.Date(2026, 6, 12, 1, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("second enable: %v", err)
	}
	if !alreadyEnabled || second.Appended != 0 || second.Total != 3 {
		t.Fatalf("second enable counts=%+v already=%v, want appended=0 total=3 already=true", second, alreadyEnabled)
	}
	status, err := ReadArchiveStatus()
	if err != nil {
		t.Fatalf("ReadArchiveStatus: %v", err)
	}
	if !status.Enabled || status.ArchiveVisits != 3 {
		t.Fatalf("status=%+v, want enabled with 3 visits", status)
	}
}

func TestDisableArchiveRoutesToSnapshotAndReenableRoutesToArchive(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	snapshot, err := SnapshotPath()
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	source := newSourceFixture(t, 1, 2)
	copyFile(t, source, snapshot)
	if _, _, err := EnableArchiveFromSource(snapshot, time.Now()); err != nil {
		t.Fatalf("enable: %v", err)
	}
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	if _, err := DisableArchive(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	path, isArchive, err := ActiveStorePath()
	if err != nil {
		t.Fatalf("ActiveStorePath disabled: %v", err)
	}
	if path != snapshot || isArchive {
		t.Fatalf("disabled path=%q isArchive=%v, want snapshot %q false", path, isArchive, snapshot)
	}
	if _, _, err := EnableArchiveFromSource(snapshot, time.Now()); err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	path, isArchive, err = ActiveStorePath()
	if err != nil {
		t.Fatalf("ActiveStorePath re-enabled: %v", err)
	}
	if path != archive || !isArchive {
		t.Fatalf("re-enabled path=%q isArchive=%v, want archive %q true", path, isArchive, archive)
	}
}

func TestClobberArchiveDropsOldRowsAndRefreshesFTS(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	oldSource := newSourceFixtureWithTitle(t, []fixtureVisit{
		{N: 1, URL: "https://example.test/old", Title: "oldonly needle"},
		{N: 2, URL: "https://example.test/current", Title: "current needle"},
	})
	if _, err := AccumulateFromSource(archive, oldSource, time.Now()); err != nil {
		t.Fatalf("accumulate old: %v", err)
	}
	currentSource := newSourceFixtureWithTitle(t, []fixtureVisit{
		{N: 2, URL: "https://example.test/current", Title: "current needle"},
	})
	result, err := ClobberArchiveFromSource(currentSource, time.Date(2026, 6, 12, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("clobber: %v", err)
	}
	if result.OldVisits != 2 || result.NewVisits != 1 {
		t.Fatalf("clobber result=%+v, want old=2 new=1", result)
	}
	status, err := ReadArchiveStatus()
	if err != nil {
		t.Fatalf("ReadArchiveStatus: %v", err)
	}
	if !status.Enabled || status.ArchiveVisits != 1 {
		t.Fatalf("status=%+v, want enabled with 1 visit", status)
	}
	st, err := Open(archive)
	if err != nil {
		t.Fatalf("Open archive: %v", err)
	}
	defer st.Close()
	var oldFTSRows int64
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM history_fts WHERE url LIKE '%/old'`).Scan(&oldFTSRows); err != nil {
		t.Fatalf("count old FTS rows: %v", err)
	}
	if oldFTSRows != 0 {
		t.Fatalf("old FTS rows = %d, want 0 after clobber", oldFTSRows)
	}
	var ftsRows, distinctURLs int64
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM history_fts`).Scan(&ftsRows); err != nil {
		t.Fatalf("count FTS rows: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(DISTINCT url) FROM history_archive`).Scan(&distinctURLs); err != nil {
		t.Fatalf("count distinct archive URLs: %v", err)
	}
	if ftsRows != distinctURLs {
		t.Fatalf("FTS rows = %d, want distinct archive URLs %d", ftsRows, distinctURLs)
	}
	src := chromeSource.New()
	oldMatches, err := src.FullTextSearch(st.DB(), "oldonly", archiveReadFilter())
	if err != nil {
		t.Fatalf("FullTextSearch oldonly: %v", err)
	}
	if len(oldMatches) != 0 {
		t.Fatalf("oldonly matches=%+v, want none after clobber", oldMatches)
	}
	currentMatches, err := src.FullTextSearch(st.DB(), "current", archiveReadFilter())
	if err != nil {
		t.Fatalf("FullTextSearch current: %v", err)
	}
	if len(currentMatches) != 1 || currentMatches[0].URL != "https://example.test/current" {
		t.Fatalf("current matches=%+v, want current row", currentMatches)
	}
}

func TestResetArchiveRequiresForcePlanDoesNotMutate(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	source := newSourceFixture(t, 1, 2)
	if _, err := AccumulateFromSource(archive, source, time.Now()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	plan, err := PlanArchiveReset()
	if err != nil {
		t.Fatalf("PlanArchiveReset: %v", err)
	}
	if !plan.WouldDestroy || plan.ArchiveVisits != 2 || plan.ArchivePath != archive {
		t.Fatalf("plan=%+v, want would_destroy with 2 visits at archive path", plan)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("archive stat after plan: %v", err)
	}
	if got := countRows(t, archive, "history_archive"); got != 2 {
		t.Fatalf("archive rows after plan = %d, want 2", got)
	}
}

func TestResetArchiveForceMovesBackupAndRoutesSnapshot(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	snapshot, err := SnapshotPath()
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	source := newSourceFixture(t, 1, 1)
	copyFile(t, source, snapshot)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	if _, err := AccumulateFromSource(archive, source, time.Now()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	result, err := ResetArchive(false, time.Date(2026, 6, 12, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ResetArchive: %v", err)
	}
	if result.BackupPath == "" || result.Purged || result.NoOp {
		t.Fatalf("reset result=%+v, want backup move", result)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("archive stat err=%v, want not exist", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(archive), "archive.db.reset-*.bak"))
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("backup matches=%v, want one backup", matches)
	}
	path, isArchive, err := ActiveStorePath()
	if err != nil {
		t.Fatalf("ActiveStorePath: %v", err)
	}
	if path != snapshot || isArchive {
		t.Fatalf("active path=%q isArchive=%v, want snapshot %q false", path, isArchive, snapshot)
	}
}

func TestResetArchiveForceBackupNameCollisionKeepsBothBackups(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	sameSecond := time.Date(2026, 6, 12, 3, 0, 0, 0, time.UTC)
	firstSource := newSourceFixture(t, 1, 1)
	if _, err := AccumulateFromSource(archive, firstSource, time.Now()); err != nil {
		t.Fatalf("first accumulate: %v", err)
	}
	first, err := ResetArchive(false, sameSecond)
	if err != nil {
		t.Fatalf("first ResetArchive: %v", err)
	}
	secondSource := newSourceFixture(t, 1, 2)
	if _, err := AccumulateFromSource(archive, secondSource, time.Now()); err != nil {
		t.Fatalf("second accumulate: %v", err)
	}
	second, err := ResetArchive(false, sameSecond)
	if err != nil {
		t.Fatalf("second ResetArchive: %v", err)
	}
	if first.BackupPath == "" || second.BackupPath == "" || first.BackupPath == second.BackupPath {
		t.Fatalf("backup paths first=%q second=%q, want distinct non-empty paths", first.BackupPath, second.BackupPath)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(archive), "archive.db.reset-*.bak"))
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("backup matches=%v, want two surviving backups", matches)
	}
	if got := countRows(t, first.BackupPath, "history_archive"); got != 1 {
		t.Fatalf("first backup rows = %d, want 1", got)
	}
	if got := countRows(t, second.BackupPath, "history_archive"); got != 2 {
		t.Fatalf("second backup rows = %d, want 2", got)
	}
}

func TestResetArchiveForcePurgeDeletesWithoutBackup(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	source := newSourceFixture(t, 1, 1)
	if _, err := AccumulateFromSource(archive, source, time.Now()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	result, err := ResetArchive(true, time.Date(2026, 6, 12, 3, 5, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ResetArchive purge: %v", err)
	}
	if !result.Purged || result.BackupPath != "" || result.NoOp {
		t.Fatalf("purge result=%+v, want purged without backup", result)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("archive stat err=%v, want not exist", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(archive), "archive.db.reset-*.bak"))
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("backup matches=%v, want none after purge", matches)
	}
}

func TestResetArchiveForceAbsentIsNoop(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	result, err := ResetArchive(false, time.Now())
	if err != nil {
		t.Fatalf("ResetArchive absent: %v", err)
	}
	if !result.NoOp {
		t.Fatalf("reset absent result=%+v, want noop", result)
	}
}

func TestVacuumArchivePopulated(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	archive, err := ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	source := newSourceFixture(t, 1, 10)
	if _, err := AccumulateFromSource(archive, source, time.Now()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	result, err := VacuumArchive()
	if err != nil {
		t.Fatalf("VacuumArchive: %v", err)
	}
	if result.NoOp || result.SizeBeforeBytes == 0 || result.SizeAfterBytes == 0 {
		t.Fatalf("vacuum result=%+v, want non-noop sizes", result)
	}
	if got := countRows(t, archive, "history_archive"); got != 10 {
		t.Fatalf("archive rows after vacuum = %d, want 10", got)
	}
}

func archiveReadFilter() source.VisitFilter {
	return source.VisitFilter{Limit: 10, MinVisits: 0, Device: "all"}
}

type fixtureVisit struct {
	N     int
	URL   string
	Title string
}

func newSourceFixture(t *testing.T, start, end int) string {
	t.Helper()
	visits := make([]fixtureVisit, 0, end-start+1)
	for i := start; i <= end; i++ {
		visits = append(visits, fixtureVisit{
			N:     i,
			URL:   fmt.Sprintf("https://example.test/v%d", i),
			Title: fmt.Sprintf("Visit %d", i),
		})
	}
	return newSourceFixtureWithTitle(t, visits)
}

func newSourceFixtureWithTitle(t *testing.T, visits []fixtureVisit) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE urls (
		id INTEGER PRIMARY KEY,
		url TEXT,
		title TEXT,
		visit_count INTEGER,
		last_visit_time INTEGER,
		typed_count INTEGER DEFAULT 0,
		hidden INTEGER DEFAULT 0
	);
	CREATE TABLE visits (
		id INTEGER PRIMARY KEY,
		url INTEGER,
		visit_time INTEGER,
		from_visit INTEGER DEFAULT 0,
		transition INTEGER DEFAULT 0,
		visit_duration INTEGER DEFAULT 0
	);
	CREATE TABLE meta(key TEXT, value TEXT);`)
	if err != nil {
		t.Fatalf("create fixture schema: %v", err)
	}
	for _, v := range visits {
		visitTime := int64(13200000000000000 + v.N)
		if _, err := db.Exec(`INSERT INTO urls(id, url, title, visit_count, last_visit_time) VALUES(?,?,?,?,?)`, v.N, v.URL, v.Title, 1, visitTime); err != nil {
			t.Fatalf("insert url: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO visits(id, url, visit_time) VALUES(?,?,?)`, v.N, v.N, visitTime); err != nil {
			t.Fatalf("insert visit: %v", err)
		}
	}
	return path
}

func countRows(t *testing.T, path, table string) int64 {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer db.Close()
	var n int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}
