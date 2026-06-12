package store

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const archiveSchemaVersion = 2

// ArchiveCounts reports how many source visits were appended and how many are
// present after an accumulation run.
type ArchiveCounts struct {
	Appended int64 `json:"appended"`
	Total    int64 `json:"total"`
}

// ArchiveStatus describes the read-only state shown by `archive status`.
type ArchiveStatus struct {
	Enabled          bool   `json:"archive_enabled"`
	BaselineAt       string `json:"baseline_at,omitempty"`
	ArchiveVisits    int64  `json:"archive_visits,omitempty"`
	ArchivePath      string `json:"archive_path,omitempty"`
	ArchiveSizeBytes int64  `json:"archive_size_bytes,omitempty"`
}

// ArchiveClobberResult reports the before/after counts for a reset baseline.
type ArchiveClobberResult struct {
	OldVisits  int64  `json:"old_archive_visits"`
	NewVisits  int64  `json:"new_archive_visits"`
	BaselineAt string `json:"baseline_at"`
}

// ArchiveResetPlan describes the guarded data that archive reset would remove.
type ArchiveResetPlan struct {
	ArchiveVisits int64  `json:"archive_visits"`
	BaselineAt    string `json:"baseline_at,omitempty"`
	ArchivePath   string `json:"archive_path,omitempty"`
	WouldDestroy  bool   `json:"would_destroy"`
}

// ArchiveResetResult reports the outcome of archive reset.
type ArchiveResetResult struct {
	ArchivePath string `json:"archive_path,omitempty"`
	BackupPath  string `json:"backup_path,omitempty"`
	Purged      bool   `json:"purged"`
	NoOp        bool   `json:"noop"`
}

// ArchiveVacuumResult reports compaction sizes for archive vacuum.
type ArchiveVacuumResult struct {
	ArchivePath     string `json:"archive_path,omitempty"`
	SizeBeforeBytes int64  `json:"size_before_bytes"`
	SizeAfterBytes  int64  `json:"size_after_bytes"`
	NoOp            bool   `json:"noop"`
}

// SnapshotPath returns the generated CLI's snapshot location.
func SnapshotPath() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "snapshot.db"), nil
}

// ArchivePath returns the generated CLI's accumulating archive location.
func ArchivePath() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "archive.db"), nil
}

func cacheDir() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if strings.TrimSpace(base) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(base, "chrome-history")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ActiveStorePath returns the archive when it exists and has archive mode
// enabled; otherwise it falls back to the current snapshot.
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
// existing data.
func InitArchiveSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta_pp (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			archive_enabled INTEGER NOT NULL DEFAULT 0,
			baseline_at TEXT,
			schema_version INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS history_archive (
			url TEXT NOT NULL,
			visit_time INTEGER NOT NULL,
			title TEXT,
			visit_count INTEGER,
			UNIQUE(url, visit_time)
		)`,
		`DROP TRIGGER IF EXISTS history_archive_ai`,
		`DROP TRIGGER IF EXISTS history_archive_ad`,
		`DROP TRIGGER IF EXISTS history_archive_au`,
		`DROP TABLE IF EXISTS history_archive_fts`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS history_fts USING fts5(
			url,
			title,
			search_terms
		)`,
		`CREATE TRIGGER IF NOT EXISTS history_archive_ai AFTER INSERT ON history_archive BEGIN
			DELETE FROM history_fts WHERE url = new.url;
			INSERT INTO history_fts(rowid, url, title, search_terms)
				SELECT MIN(rowid), url, COALESCE((SELECT title FROM history_archive h2 WHERE h2.url = history_archive.url ORDER BY visit_time DESC, rowid DESC LIMIT 1), ''), ''
				FROM history_archive WHERE url = new.url GROUP BY url;
		END`,
		`CREATE TRIGGER IF NOT EXISTS history_archive_ad AFTER DELETE ON history_archive BEGIN
			DELETE FROM history_fts WHERE url = old.url AND NOT EXISTS (SELECT 1 FROM history_archive WHERE url = old.url);
		END`,
		`CREATE TRIGGER IF NOT EXISTS history_archive_au AFTER UPDATE ON history_archive BEGIN
			DELETE FROM history_fts WHERE url = old.url;
			DELETE FROM history_fts WHERE url = new.url;
			INSERT INTO history_fts(rowid, url, title, search_terms)
				SELECT MIN(rowid), url, COALESCE((SELECT title FROM history_archive h2 WHERE h2.url = history_archive.url ORDER BY visit_time DESC, rowid DESC LIMIT 1), ''), ''
				FROM history_archive WHERE url IN (old.url, new.url) GROUP BY url;
		END`,
		`CREATE VIEW IF NOT EXISTS urls AS
			SELECT
				MIN(rowid) AS id,
				url,
				COALESCE((SELECT title FROM history_archive h2 WHERE h2.url = h.url ORDER BY visit_time DESC, rowid DESC LIMIT 1), '') AS title,
				COUNT(*) AS visit_count,
				MAX(visit_time) AS last_visit_time,
				0 AS typed_count,
				0 AS hidden
			FROM history_archive h
			GROUP BY url`,
		`CREATE VIEW IF NOT EXISTS visits AS
			SELECT
				h.rowid AS id,
				(SELECT MIN(h2.rowid) FROM history_archive h2 WHERE h2.url = h.url) AS url,
				h.visit_time AS visit_time,
				0 AS from_visit,
				0 AS transition,
				0 AS visit_duration,
				'' AS originator_cache_guid
			FROM history_archive h`,
		`CREATE VIEW IF NOT EXISTS visit_source AS
			SELECT rowid AS id, 1 AS source, '' AS originator_cache_guid FROM history_archive`,
		`CREATE TABLE IF NOT EXISTS keyword_search_terms (
			url_id INTEGER,
			term TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS downloads (
			target_path TEXT,
			received_bytes INTEGER,
			mime_type TEXT,
			original_mime_type TEXT,
			site_url TEXT,
			referrer TEXT,
			start_time INTEGER,
			state INTEGER
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	_, err := db.Exec(`INSERT INTO meta_pp(id, archive_enabled, schema_version)
		SELECT 1, 0, ? WHERE NOT EXISTS (SELECT 1 FROM meta_pp)`, archiveSchemaVersion)
	if err != nil {
		return err
	}
	return migrateArchiveFTS(db)
}

// IsArchiveEnabled reads the sticky archive mode flag.
func IsArchiveEnabled(db *sql.DB) (bool, error) {
	if err := InitArchiveSchema(db); err != nil {
		return false, err
	}
	return readArchiveEnabled(db)
}

func readArchiveEnabled(db *sql.DB) (bool, error) {
	if !tableExists(db, "meta_pp") {
		return false, nil
	}
	var enabled int
	err := db.QueryRow(`SELECT archive_enabled FROM meta_pp LIMIT 1`).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return enabled != 0, err
}

func openReadOnlyDB(path string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
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

func migrateArchiveFTS(db *sql.DB) error {
	var version sql.NullInt64
	if err := db.QueryRow(`SELECT schema_version FROM meta_pp LIMIT 1`).Scan(&version); err != nil {
		return err
	}
	if version.Valid && version.Int64 >= archiveSchemaVersion {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM history_fts`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO history_fts(rowid, url, title, search_terms)
		SELECT MIN(rowid), url, COALESCE((SELECT title FROM history_archive h2 WHERE h2.url = history_archive.url ORDER BY visit_time DESC, rowid DESC LIMIT 1), ''), ''
		FROM history_archive GROUP BY url`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE meta_pp SET schema_version=?`, archiveSchemaVersion); err != nil {
		return err
	}
	return tx.Commit()
}

// AccumulateFromSource appends current source visits to archivePath, deduping
// by URL and Chrome-native visit_time.
func AccumulateFromSource(archivePath, sourcePath string, now time.Time) (ArchiveCounts, error) {
	var counts ArchiveCounts
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
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
	var enabled int
	var baseline sql.NullString
	if err := tx.QueryRow(`SELECT archive_enabled, baseline_at FROM meta_pp LIMIT 1`).Scan(&enabled, &baseline); err != nil {
		return counts, err
	}
	appended, err := accumulateFromSourceTx(tx, sourcePath)
	if err != nil {
		return counts, err
	}
	counts.Appended = appended
	if enabled == 0 {
		ts := now.UTC().Format(time.RFC3339)
		if _, err := tx.Exec(`UPDATE meta_pp SET archive_enabled=1, baseline_at=COALESCE(NULLIF(baseline_at,''), ?), schema_version=?`, ts, archiveSchemaVersion); err != nil {
			return counts, err
		}
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM history_archive`).Scan(&counts.Total); err != nil {
		return counts, err
	}
	if err := tx.Commit(); err != nil {
		return counts, err
	}
	_, err = db.Exec(`PRAGMA optimize`)
	return counts, err
}

func accumulateFromSourceTx(tx *sql.Tx, sourcePath string) (int64, error) {
	if _, err := os.Stat(sourcePath); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`ATTACH DATABASE ? AS source_db`, sourcePath); err != nil {
		return 0, err
	}
	defer tx.Exec(`DETACH DATABASE source_db`)
	res, err := tx.Exec(`INSERT OR IGNORE INTO history_archive(url, visit_time, title, visit_count)
		SELECT COALESCE(u.url,''), COALESCE(v.visit_time,0), COALESCE(u.title,''), COALESCE(u.visit_count,0)
		FROM source_db.visits v JOIN source_db.urls u ON u.id = v.url
		WHERE COALESCE(u.url,'') <> '' AND COALESCE(v.visit_time,0) > 0`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// EnableArchiveFromSource baselines a disabled or absent archive from sourcePath.
// Already-enabled archives are left unchanged so explicit enable is idempotent.
func EnableArchiveFromSource(sourcePath string, now time.Time) (ArchiveCounts, bool, error) {
	var counts ArchiveCounts
	if _, err := os.Stat(sourcePath); err != nil {
		return counts, false, err
	}
	status, err := ReadArchiveStatus()
	if err != nil {
		return counts, false, err
	}
	if status.Enabled {
		counts.Total = status.ArchiveVisits
		return counts, true, nil
	}
	archivePath, err := ArchivePath()
	if err != nil {
		return counts, false, err
	}
	counts, err = AccumulateFromSource(archivePath, sourcePath, now)
	return counts, false, err
}

// DisableArchive turns archive mode off while preserving the archive file.
func DisableArchive() (ArchiveStatus, error) {
	path, err := ArchivePath()
	if err != nil {
		return ArchiveStatus{}, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return ArchiveStatus{Enabled: false}, nil
		}
		return ArchiveStatus{}, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return ArchiveStatus{}, err
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		return ArchiveStatus{}, err
	}
	if _, err := db.Exec(`UPDATE meta_pp SET archive_enabled=0, schema_version=?`, archiveSchemaVersion); err != nil {
		return ArchiveStatus{}, err
	}
	return ReadArchiveStatus()
}

// ClobberArchiveFromSource keeps archive mode enabled while replacing archive
// contents with a fresh baseline from sourcePath in one transaction.
func ClobberArchiveFromSource(sourcePath string, now time.Time) (ArchiveClobberResult, error) {
	var out ArchiveClobberResult
	if _, err := os.Stat(sourcePath); err != nil {
		return out, err
	}
	archivePath, err := ArchivePath()
	if err != nil {
		return out, err
	}
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return out, err
	}
	db, err := sql.Open("sqlite", archivePath)
	if err != nil {
		return out, err
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		return out, err
	}
	tx, err := db.Begin()
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err := tx.QueryRow(`SELECT COUNT(*) FROM history_archive`).Scan(&out.OldVisits); err != nil {
		return out, err
	}
	if _, err := tx.Exec(`DELETE FROM history_archive`); err != nil {
		return out, err
	}
	if _, err := accumulateFromSourceTx(tx, sourcePath); err != nil {
		return out, err
	}
	out.BaselineAt = now.UTC().Format(time.RFC3339)
	if _, err := tx.Exec(`UPDATE meta_pp SET archive_enabled=1, baseline_at=?, schema_version=?`, out.BaselineAt, archiveSchemaVersion); err != nil {
		return out, err
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM history_archive`).Scan(&out.NewVisits); err != nil {
		return out, err
	}
	if err := tx.Commit(); err != nil {
		return out, err
	}
	_, err = db.Exec(`PRAGMA optimize`)
	return out, err
}

// PlanArchiveReset reports what guarded archive reset would destroy.
func PlanArchiveReset() (ArchiveResetPlan, error) {
	status, err := ReadArchiveStatus()
	if err != nil {
		return ArchiveResetPlan{}, err
	}
	return ArchiveResetPlan{
		ArchiveVisits: status.ArchiveVisits,
		BaselineAt:    status.BaselineAt,
		ArchivePath:   status.ArchivePath,
		WouldDestroy:  status.ArchivePath != "",
	}, nil
}

// ResetArchive disables archive mode and then moves archive.db aside by
// default. purge=true deletes it outright.
func ResetArchive(purge bool, now time.Time) (ArchiveResetResult, error) {
	var out ArchiveResetResult
	path, err := ArchivePath()
	if err != nil {
		return out, err
	}
	out.ArchivePath = path
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			out.NoOp = true
			return out, nil
		}
		return out, err
	}
	if _, err := DisableArchive(); err != nil {
		return out, err
	}
	if purge {
		if err := os.Remove(path); err != nil {
			return out, err
		}
		out.Purged = true
		return out, nil
	}
	out.BackupPath, err = resetBackupPath(path, now)
	if err != nil {
		return out, err
	}
	if err := os.Rename(path, out.BackupPath); err != nil {
		return out, err
	}
	return out, nil
}

func resetBackupPath(path string, now time.Time) (string, error) {
	stamp := now.UTC().Format("2006-01-02T15-04-05Z")
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.reset-%s.bak", path, stamp)
		if i > 1 {
			candidate = fmt.Sprintf("%s.reset-%s-%d.bak", path, stamp, i)
		}
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				return candidate, nil
			}
			return "", err
		}
	}
}

// VacuumArchive compacts archive.db and reports file sizes before and after.
func VacuumArchive() (ArchiveVacuumResult, error) {
	var out ArchiveVacuumResult
	path, err := ArchivePath()
	if err != nil {
		return out, err
	}
	out.ArchivePath = path
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			out.NoOp = true
			return out, nil
		}
		return out, err
	}
	out.SizeBeforeBytes = info.Size()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return out, err
	}
	defer db.Close()
	if err := InitArchiveSchema(db); err != nil {
		return out, err
	}
	if _, err := db.Exec(`VACUUM`); err != nil {
		return out, err
	}
	if _, err := db.Exec(`PRAGMA optimize`); err != nil {
		return out, err
	}
	info, err = os.Stat(path)
	if err != nil {
		return out, err
	}
	out.SizeAfterBytes = info.Size()
	return out, nil
}

// ReadArchiveStatus returns archive status without creating an archive file.
func ReadArchiveStatus() (ArchiveStatus, error) {
	path, err := ArchivePath()
	if err != nil {
		return ArchiveStatus{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ArchiveStatus{Enabled: false}, nil
		}
		return ArchiveStatus{}, err
	}
	db, err := openReadOnlyDB(path)
	if err != nil {
		return ArchiveStatus{}, err
	}
	defer db.Close()
	if !tableExists(db, "meta_pp") {
		return ArchiveStatus{Enabled: false, ArchivePath: path, ArchiveSizeBytes: info.Size()}, nil
	}
	var enabled int
	var baseline sql.NullString
	var visits int64
	if err := db.QueryRow(`SELECT archive_enabled, baseline_at FROM meta_pp LIMIT 1`).Scan(&enabled, &baseline); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ArchiveStatus{Enabled: false, ArchivePath: path, ArchiveSizeBytes: info.Size()}, nil
		}
		return ArchiveStatus{}, err
	}
	if tableExists(db, "history_archive") {
		if err := db.QueryRow(`SELECT COUNT(*) FROM history_archive`).Scan(&visits); err != nil {
			return ArchiveStatus{}, err
		}
	}
	status := ArchiveStatus{
		Enabled:          enabled != 0,
		ArchiveVisits:    visits,
		ArchivePath:      path,
		ArchiveSizeBytes: info.Size(),
	}
	if baseline.Valid {
		status.BaselineAt = baseline.String
	}
	return status, nil
}
