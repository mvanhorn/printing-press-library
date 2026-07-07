package store

import (
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

const archiveSchemaVersion = 1

// ArchiveCounts reports archive row counts before and after an accumulation run.
type ArchiveCounts struct {
	Before   int64 `json:"before"`
	After    int64 `json:"after"`
	Inserted int64 `json:"inserted"`
}

type ArchiveStatus struct {
	Enabled    bool   `json:"enabled"`
	BaselineAt string `json:"baseline_at"`
	URLCount   int64  `json:"url_count"`
	VisitCount int64  `json:"visit_count"`
	Path       string `json:"path"`
	SizeBytes  int64  `json:"size_bytes"`
}

// ArchivePath returns the generated CLI's accumulating archive location.
func ArchivePath() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "archive.db"), nil
}

// ActiveStorePath returns archive.db when it exists and archive mode is
// enabled; otherwise it falls back to snapshot.db.
func ActiveStorePath() (string, bool, error) {
	archive, err := ArchivePath()
	if err != nil {
		return "", false, err
	}
	snapshot, err := SnapshotPath()
	if err != nil {
		return "", false, err
	}
	if _, err := os.Stat(archive); err != nil {
		if os.IsNotExist(err) {
			return snapshot, false, nil
		}
		return "", false, err
	}
	db, err := openReadOnlyDB(archive)
	if err != nil {
		return "", false, err
	}
	defer db.Close()
	enabled, err := readArchiveEnabled(db)
	if err != nil {
		return "", false, err
	}
	if enabled {
		return archive, true, nil
	}
	return snapshot, false, nil
}

// OpenActiveStore opens the currently active read store.
func OpenActiveStore() (*Store, bool, error) {
	path, isArchive, err := ActiveStorePath()
	if err != nil {
		return nil, false, err
	}
	st, err := OpenExisting(path)
	if err != nil {
		return nil, false, err
	}
	return st, isArchive, nil
}

// InitArchiveSchema creates the accumulating archive schema without dropping
// existing archive data.
func InitArchiveSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS archive_meta (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			archive_enabled INTEGER NOT NULL DEFAULT 0,
			baseline_at TEXT,
			schema_version INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS history_archive (
			url TEXT NOT NULL,
			visit_time REAL NOT NULL,
			title TEXT,
			visit_count INTEGER,
			domain_expansion TEXT,
			origin INTEGER,
			UNIQUE(url, visit_time)
		)`,
		`CREATE VIEW IF NOT EXISTS history_items AS
			SELECT MIN(rowid) AS id,
				   url,
				   COALESCE((SELECT domain_expansion FROM history_archive h2 WHERE h2.url = h.url AND COALESCE(h2.domain_expansion,'')<>'' LIMIT 1), '') AS domain_expansion,
				   COUNT(*) AS visit_count
			FROM history_archive h
			GROUP BY url`,
		`CREATE VIEW IF NOT EXISTS history_visits AS
			SELECT h.rowid AS id,
				   (SELECT MIN(h2.rowid) FROM history_archive h2 WHERE h2.url = h.url) AS history_item,
				   h.visit_time AS visit_time,
				   COALESCE(h.title,'') AS title,
				   COALESCE(h.origin,0) AS origin,
				   0 AS redirect_source
			FROM history_archive h`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	_, err := db.Exec(`INSERT INTO archive_meta(id, archive_enabled, schema_version)
		SELECT 1, 0, ? WHERE NOT EXISTS (SELECT 1 FROM archive_meta WHERE id = 1)`, archiveSchemaVersion)
	return err
}

type sqlExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func rebuildArchiveFTS(db sqlExecer) error {
	stmts := []string{
		`DROP TABLE IF EXISTS history_fts`,
		`CREATE VIRTUAL TABLE history_fts USING fts5(url, title, search_terms)`,
		`INSERT INTO history_fts(url, title, search_terms)
			SELECT url, COALESCE((SELECT title FROM history_archive h2 WHERE h2.url = h.url ORDER BY visit_time DESC, rowid DESC LIMIT 1), ''), ''
			FROM history_archive h GROUP BY url`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func readArchiveEnabled(db *sql.DB) (bool, error) {
	if !tableExists(db, "archive_meta") {
		return false, nil
	}
	var enabled int
	err := db.QueryRow(`SELECT archive_enabled FROM archive_meta WHERE id = 1`).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return enabled != 0, err
}

// IsArchiveEnabled reads the sticky archive mode flag without creating the
// archive file.
func IsArchiveEnabled() (bool, error) {
	path, err := ArchivePath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	db, err := openReadOnlyDB(path)
	if err != nil {
		return false, err
	}
	defer db.Close()
	return readArchiveEnabled(db)
}

// ReadArchiveStatus reports archive metadata and counts without creating the
// archive file.
func ReadArchiveStatus() (ArchiveStatus, error) {
	path, err := ArchivePath()
	if err != nil {
		return ArchiveStatus{}, err
	}
	status := ArchiveStatus{Path: path}
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return status, nil
		}
		return status, err
	}
	status.SizeBytes = st.Size()
	db, err := openReadOnlyDB(path)
	if err != nil {
		return status, err
	}
	defer db.Close()
	if tableExists(db, "archive_meta") {
		var enabled int
		var baseline sql.NullString
		err := db.QueryRow(`SELECT archive_enabled, baseline_at FROM archive_meta WHERE id = 1`).Scan(&enabled, &baseline)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return status, err
		}
		status.Enabled = enabled != 0
		if baseline.Valid {
			status.BaselineAt = baseline.String
		}
	}
	if tableExists(db, "history_archive") {
		if err := db.QueryRow(`SELECT COUNT(DISTINCT url), COUNT(*) FROM history_archive`).Scan(&status.URLCount, &status.VisitCount); err != nil {
			return status, err
		}
	}
	return status, nil
}

func openReadOnlyDB(path string) (*sql.DB, error) {
	u := sqliteReadOnlyURI(path)
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA query_only=ON`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func sqliteReadOnlyURI(path string) url.URL {
	return url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
}

// AccumulateFromSource appends current snapshot visits to archivePath, deduping
// by URL and Safari-native visit_time.
func AccumulateFromSource(archivePath, sourcePath string, now time.Time) (ArchiveCounts, error) {
	var counts ArchiveCounts
	// 0o700: the archive holds the user's accumulated private browsing history.
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o700); err != nil {
		return counts, err
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return counts, err
	}
	db, err := sql.Open("sqlite", archivePath)
	if err != nil {
		return counts, err
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		return counts, err
	}
	tx, err := db.Begin()
	if err != nil {
		return counts, err
	}
	defer tx.Rollback()
	if err := tx.QueryRow(`SELECT COUNT(*) FROM history_archive`).Scan(&counts.Before); err != nil {
		return counts, err
	}
	enabled, err := readArchiveEnabledTx(tx)
	if err != nil {
		return counts, err
	}
	inserted, err := accumulateFromSourceTx(tx, sourcePath)
	if err != nil {
		return counts, err
	}
	counts.Inserted = inserted
	if enabled == 0 {
		ts := now.UTC().Format(time.RFC3339)
		if _, err := tx.Exec(`UPDATE archive_meta SET archive_enabled=1, baseline_at=COALESCE(NULLIF(baseline_at,''), ?), schema_version=? WHERE id=1`, ts, archiveSchemaVersion); err != nil {
			return counts, err
		}
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM history_archive`).Scan(&counts.After); err != nil {
		return counts, err
	}
	if err := rebuildArchiveFTS(tx); err != nil {
		return counts, err
	}
	if err := tx.Commit(); err != nil {
		return counts, err
	}
	_, err = db.Exec(`PRAGMA optimize`)
	return counts, err
}

func readArchiveEnabledTx(tx *sql.Tx) (int, error) {
	var enabled int
	err := tx.QueryRow(`SELECT archive_enabled FROM archive_meta WHERE id = 1`).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return enabled, err
}

func accumulateFromSourceTx(tx *sql.Tx, sourcePath string) (int64, error) {
	if _, err := tx.Exec(`ATTACH DATABASE 'file:' || ? || '?mode=ro' AS source_db`, sourcePath); err != nil {
		return 0, err
	}
	defer tx.Exec(`DETACH DATABASE source_db`)
	res, err := tx.Exec(`INSERT OR IGNORE INTO history_archive(url, visit_time, title, visit_count, domain_expansion, origin)
		SELECT hi.url, hv.visit_time, COALESCE(hv.title,''), COALESCE(hi.visit_count,0), COALESCE(hi.domain_expansion,''), COALESCE(hv.origin,0)
		FROM source_db.history_visits hv JOIN source_db.history_items hi ON hi.id = hv.history_item
		WHERE COALESCE(hi.url,'') <> '' AND COALESCE(hv.visit_time,0) > 0`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// EnableArchiveFromSource baselines a disabled or absent archive from snapshotPath.
func EnableArchiveFromSource(archivePath, snapshotPath string, now time.Time) error {
	_, err := AccumulateFromSource(archivePath, snapshotPath, now)
	return err
}

// ClobberArchiveFromSource replaces the archive contents with a fresh snapshot
// baseline and leaves archive mode enabled.
func ClobberArchiveFromSource(archivePath, snapshotPath string, now time.Time) error {
	// 0o700: the archive holds the user's accumulated private browsing history.
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(snapshotPath); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", archivePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM history_archive`); err != nil {
		return err
	}
	if _, err := accumulateFromSourceTx(tx, snapshotPath); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE archive_meta SET archive_enabled=1, baseline_at=?, schema_version=? WHERE id=1`, now.UTC().Format(time.RFC3339), archiveSchemaVersion); err != nil {
		return err
	}
	if err := rebuildArchiveFTS(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_, err = db.Exec(`PRAGMA optimize`)
	return err
}

// DisableArchive turns archive mode off while preserving the archive file.
func DisableArchive(archivePath string) error {
	if _, err := os.Stat(archivePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	db, err := sql.Open("sqlite", archivePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE archive_meta SET archive_enabled=0, schema_version=? WHERE id=1`, archiveSchemaVersion)
	return err
}

// VacuumArchive compacts archive.db when lifecycle commands are added later.
func VacuumArchive(archivePath string) error {
	if _, err := os.Stat(archivePath); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", archivePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		return err
	}
	_, err = db.Exec(`VACUUM`)
	return err
}

// ResetArchive disables archive mode, then either deletes archive.db and its
// sidecars or moves them aside to collision-safe .bak names.
func ResetArchive(archivePath string, purge bool) error {
	if _, err := os.Stat(archivePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := DisableArchive(archivePath); err != nil {
		return err
	}
	paths := []string{archivePath, archivePath + "-wal", archivePath + "-shm"}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if purge {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		dst, err := nextBackupPath(p)
		if err != nil {
			return err
		}
		if err := os.Rename(p, dst); err != nil {
			return err
		}
	}
	return nil
}

func nextBackupPath(path string) (string, error) {
	base := path + ".bak"
	if _, err := os.Stat(base); err != nil {
		if os.IsNotExist(err) {
			return base, nil
		}
		return "", err
	}
	for i := 1; ; i++ {
		candidate := base + "." + strconv.Itoa(i)
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				return candidate, nil
			}
			return "", err
		}
	}
}
