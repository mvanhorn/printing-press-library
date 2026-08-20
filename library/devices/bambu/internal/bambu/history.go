package bambu

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/store"

	_ "modernc.org/sqlite"
)

type History struct{ db *sql.DB }

const BambuHistorySchemaVersion = 6

type historyMigrationDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type historyCompatibilityDB interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type historyBusyObserverKey struct{}
type historyBusyTimeoutKey struct{}

type StoredJob struct {
	JobKey                  string     `json:"job_key"`
	Name                    string     `json:"name"`
	TaskID                  string     `json:"task_id,omitempty"`
	SubtaskID               string     `json:"subtask_id,omitempty"`
	File                    string     `json:"file,omitempty"`
	StartedAt               *time.Time `json:"started_at,omitempty"`
	FinishedAt              *time.Time `json:"finished_at,omitempty"`
	Outcome                 string     `json:"outcome,omitempty"`
	DurationSeconds         *int64     `json:"duration_seconds,omitempty"`
	InitialRemainingMinutes *int       `json:"initial_remaining_minutes,omitempty"`
	WeightGrams             *float64   `json:"weight_grams,omitempty"`
}

type StoredEvent struct {
	ID         int64          `json:"id"`
	JobKey     string         `json:"job_key"`
	EventType  string         `json:"event_type"`
	OccurredAt time.Time      `json:"occurred_at"`
	Data       map[string]any `json:"data"`
}

func OpenHistory(ctx context.Context, dbPath string) (*History, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, err
	}
	busyTimeout := 5000
	if configured, ok := ctx.Value(historyBusyTimeoutKey{}).(int); ok && configured > 0 {
		busyTimeout = configured
	}
	if _, err := os.Stat(dbPath); err == nil {
		probe, openErr := sql.Open("sqlite", fmt.Sprintf("%s?_pragma=busy_timeout(%d)", dbPath, busyTimeout))
		if openErr != nil {
			return nil, openErr
		}
		compatErr := retryHistoryBusy(ctx, "probe history compatibility", func() error { return checkHistoryCompatibility(ctx, probe) })
		_ = probe.Close()
		if compatErr != nil {
			return nil, compatErr
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath, busyTimeout))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	h := &History{db: db}
	if err := retryHistoryBusy(ctx, "check history compatibility", func() error { return checkHistoryCompatibility(ctx, db) }); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := h.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(dbPath, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure Bambu database permissions: %w", err)
	}
	return h, nil
}

func (h *History) Close() error { return h.db.Close() }

func checkHistoryCompatibility(ctx context.Context, db historyCompatibilityDB) error {
	var sharedVersion int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&sharedVersion); err != nil {
		return fmt.Errorf("read shared schema version: %w", err)
	}
	if sharedVersion > store.StoreSchemaVersion {
		return fmt.Errorf("database shared schema version %d is newer than supported version %d", sharedVersion, store.StoreSchemaVersion)
	}
	var hasLedger int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='bambu_history_migrations'`).Scan(&hasLedger); err != nil {
		return fmt.Errorf("inspect Bambu history schema: %w", err)
	}
	if hasLedger == 0 {
		return nil
	}
	var historyVersion sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM bambu_history_migrations`).Scan(&historyVersion); err != nil {
		return fmt.Errorf("read Bambu history schema version: %w", err)
	}
	if historyVersion.Valid && historyVersion.Int64 > BambuHistorySchemaVersion {
		return fmt.Errorf("Bambu history schema version %d is newer than supported version %d", historyVersion.Int64, BambuHistorySchemaVersion)
	}
	return nil
}

func retryHistoryBusy(ctx context.Context, label string, operation func() error) error {
	deadline := time.Now().Add(5 * time.Second)
	backoff := 5 * time.Millisecond
	for {
		err := operation()
		if err == nil {
			return nil
		}
		message := err.Error()
		if !strings.Contains(message, "SQLITE_BUSY") && !strings.Contains(message, "SQLITE_LOCKED") && !strings.Contains(message, "database is locked") && !strings.Contains(message, "database table is locked") {
			return err
		}
		if observer, ok := ctx.Value(historyBusyObserverKey{}).(func(string)); ok {
			observer(label)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s: timed out under SQLite contention: %w", label, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 100*time.Millisecond {
			backoff *= 2
		}
	}
}

func (h *History) migrate(ctx context.Context) error {
	conn, err := h.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := retryHistoryBusy(ctx, "begin Bambu history migration", func() error {
		_, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`)
		return err
	}); err != nil {
		return fmt.Errorf("begin Bambu history migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	if err := checkHistoryCompatibility(ctx, conn); err != nil {
		return fmt.Errorf("recheck compatibility inside Bambu history migration: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS bambu_snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  printer_key TEXT NOT NULL DEFAULT '',
  job_key TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  state TEXT NOT NULL,
  data TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_bambu_snapshots_job_time ON bambu_snapshots(job_key, observed_at);
CREATE INDEX IF NOT EXISTS idx_bambu_snapshots_time ON bambu_snapshots(observed_at);
CREATE TABLE IF NOT EXISTS bambu_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  printer_key TEXT NOT NULL DEFAULT '',
  job_key TEXT NOT NULL,
  event_type TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  data TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_bambu_events_job_time ON bambu_events(job_key, occurred_at);
CREATE TABLE IF NOT EXISTS bambu_jobs (
  job_key TEXT PRIMARY KEY,
  printer_key TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  task_id TEXT,
  subtask_id TEXT,
  file TEXT,
  started_at TEXT,
  sort_started_at TEXT NOT NULL DEFAULT '0001-01-01T00:00:00Z',
  finished_at TEXT,
  outcome TEXT,
  duration_seconds INTEGER,
  initial_remaining_minutes INTEGER,
  weight_grams REAL,
  data TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_bambu_jobs_name ON bambu_jobs(name, started_at);
CREATE INDEX IF NOT EXISTS idx_bambu_jobs_started ON bambu_jobs(started_at);
CREATE TABLE IF NOT EXISTS bambu_history_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS bambu_maintenance (
  printer_key TEXT NOT NULL DEFAULT '',
  task TEXT NOT NULL,
  completed_at TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  PRIMARY KEY(printer_key,task)
);`); err != nil {
		return fmt.Errorf("migrate Bambu history: %w", err)
	}
	for version := 1; version <= BambuHistorySchemaVersion; version++ {
		var found int
		err := conn.QueryRowContext(ctx, `SELECT 1 FROM bambu_history_migrations WHERE version=?`, version).Scan(&found)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == sql.ErrNoRows {
			switch version {
			case 1:
				err = h.migrateLegacyPrinterScope(ctx, conn)
			case 2:
				err = h.migrateStableJobOrdering(ctx, conn)
			case 3:
				err = h.migrateStableTaskJobIdentity(ctx, conn)
			case 4:
				err = h.migratePayloadTaskOwnership(ctx, conn)
			case 5:
				err = h.migrateUnscopedJobOwnership(ctx, conn)
			case 6:
				err = h.recalculateJobDurations(ctx, conn)
			default:
				err = fmt.Errorf("unsupported Bambu history migration %d", version)
			}
			if err != nil {
				return err
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO bambu_history_migrations(version,applied_at) VALUES(?,?)`, version, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				return fmt.Errorf("stamp Bambu history migration %d: %w", version, err)
			}
		}
	}
	if err := retryHistoryBusy(ctx, "commit Bambu history migration", func() error {
		_, err := conn.ExecContext(ctx, `COMMIT`)
		return err
	}); err != nil {
		return fmt.Errorf("commit Bambu history migration: %w", err)
	}
	committed = true
	return nil
}

func (h *History) migrateLegacyPrinterScope(ctx context.Context, db historyMigrationDB) error {
	for _, migration := range []historyColumnMigration{snapshotPrinterKey, eventPrinterKey, jobPrinterKey, jobSortStartedAt} {
		if err := h.ensureHistoryColumn(ctx, db, migration); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_bambu_snapshots_printer_time ON bambu_snapshots(printer_key,observed_at); CREATE INDEX IF NOT EXISTS idx_bambu_events_printer_time ON bambu_events(printer_key,occurred_at);`); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(bambu_maintenance)`)
	if err != nil {
		return err
	}
	hasPrinterKey := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		if name == "printer_key" {
			hasPrinterKey = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !hasPrinterKey {
		if _, err := db.ExecContext(ctx, `CREATE TABLE bambu_maintenance_scoped (printer_key TEXT NOT NULL DEFAULT '',task TEXT NOT NULL,completed_at TEXT NOT NULL,note TEXT NOT NULL DEFAULT '',PRIMARY KEY(printer_key,task));
INSERT INTO bambu_maintenance_scoped(printer_key,task,completed_at,note) SELECT '',task,completed_at,note FROM bambu_maintenance;
DROP TABLE bambu_maintenance;
ALTER TABLE bambu_maintenance_scoped RENAME TO bambu_maintenance;`); err != nil {
			return fmt.Errorf("migrate maintenance printer scope: %w", err)
		}
	}
	_, err = db.ExecContext(ctx, `UPDATE bambu_snapshots SET printer_key=COALESCE(json_extract(data,'$.printer_key'),''),job_key=json_extract(data,'$.printer_key')||'/'||job_key WHERE COALESCE(json_extract(data,'$.printer_key'),'')<>'' AND job_key NOT LIKE json_extract(data,'$.printer_key')||'/%';
	UPDATE bambu_events SET printer_key=COALESCE(json_extract(data,'$.printer_key'),''),job_key=json_extract(data,'$.printer_key')||'/'||job_key WHERE COALESCE(json_extract(data,'$.printer_key'),'')<>'' AND job_key NOT LIKE json_extract(data,'$.printer_key')||'/%';
	UPDATE bambu_jobs SET printer_key=COALESCE(json_extract(data,'$.printer_key'),''),job_key=json_extract(data,'$.printer_key')||'/'||job_key WHERE COALESCE(json_extract(data,'$.printer_key'),'')<>'' AND job_key NOT LIKE json_extract(data,'$.printer_key')||'/%';
	UPDATE bambu_snapshots SET printer_key=COALESCE(json_extract(data,'$.printer_key'),'') WHERE printer_key='';
	UPDATE bambu_events SET printer_key=COALESCE(json_extract(data,'$.printer_key'),'') WHERE printer_key='';
	UPDATE bambu_jobs SET printer_key=COALESCE(json_extract(data,'$.printer_key'),'') WHERE printer_key='';
		INSERT OR IGNORE INTO bambu_jobs(job_key,printer_key,name,task_id,subtask_id,file,initial_remaining_minutes,data)
		SELECT s.job_key,s.printer_key,COALESCE(json_extract(s.data,'$.job_name'),'unnamed print'),COALESCE(json_extract(s.data,'$.task_id'),''),COALESCE(json_extract(s.data,'$.subtask_id'),''),COALESCE(json_extract(s.data,'$.gcode_file'),''),CAST(json_extract(s.data,'$.remaining_minutes') AS INTEGER),s.data FROM bambu_snapshots s WHERE s.printer_key<>'' AND s.id=(SELECT first.id FROM bambu_snapshots first WHERE first.job_key=s.job_key ORDER BY first.observed_at ASC,first.id ASC LIMIT 1);
		UPDATE bambu_jobs SET initial_remaining_minutes=COALESCE(initial_remaining_minutes,(SELECT CAST(json_extract(first.data,'$.remaining_minutes') AS INTEGER) FROM bambu_snapshots first WHERE first.job_key=bambu_jobs.job_key ORDER BY first.observed_at ASC,first.id ASC LIMIT 1));
	UPDATE bambu_maintenance SET printer_key=(SELECT json_extract(data,'$.printer_key') FROM bambu_snapshots WHERE COALESCE(json_extract(data,'$.printer_key'),'')<>'' LIMIT 1) WHERE printer_key='' AND (SELECT COUNT(DISTINCT json_extract(data,'$.printer_key')) FROM bambu_snapshots WHERE COALESCE(json_extract(data,'$.printer_key'),'')<>'')=1;`)
	if err != nil {
		return fmt.Errorf("migrate Bambu job printer scope: %w", err)
	}
	if err := h.rebuildJobTerminals(ctx, db); err != nil {
		return err
	}
	if err := h.upgradeShortPrinterKeys(ctx, db); err != nil {
		return err
	}
	return nil
}

func (h *History) migrateStableJobOrdering(ctx context.Context, db historyMigrationDB) error {
	if _, err := db.ExecContext(ctx, `UPDATE bambu_jobs SET sort_started_at=COALESCE(
		NULLIF(started_at,''),
		(SELECT MAX(observed_at) FROM bambu_snapshots s WHERE s.job_key=bambu_jobs.job_key),
		(SELECT MAX(occurred_at) FROM bambu_events e WHERE e.job_key=bambu_jobs.job_key),
		NULLIF(json_extract(data,'$.observed_at'),''),
		NULLIF(finished_at,''),
		'0001-01-01T00:00:00Z');
		DROP INDEX IF EXISTS idx_bambu_jobs_printer_started;
		CREATE INDEX idx_bambu_jobs_printer_started ON bambu_jobs(printer_key,sort_started_at DESC,job_key DESC);
		DROP INDEX IF EXISTS idx_bambu_jobs_sort_started;
		CREATE INDEX idx_bambu_jobs_sort_started ON bambu_jobs(sort_started_at DESC,job_key DESC);`); err != nil {
		return fmt.Errorf("migrate stable Bambu job ordering: %w", err)
	}
	return nil
}

func (h *History) migrateStableTaskJobIdentity(ctx context.Context, db historyMigrationDB) error {
	rows, err := db.QueryContext(ctx, `SELECT job_key,printer_key,task_id FROM bambu_jobs WHERE TRIM(COALESCE(task_id,''))<>'' AND TRIM(task_id)<>'0' ORDER BY job_key`)
	if err != nil {
		return err
	}
	type rekey struct{ oldKey, canonical string }
	rekeys := make([]rekey, 0)
	for rows.Next() {
		var oldKey, printerKey, taskID string
		if err := rows.Scan(&oldKey, &printerKey, &taskID); err != nil {
			_ = rows.Close()
			return err
		}
		canonical := strings.TrimSpace(taskID)
		if printerKey != "" {
			canonical = printerKey + "/" + canonical
		}
		if oldKey != canonical {
			rekeys = append(rekeys, rekey{oldKey: oldKey, canonical: canonical})
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, item := range rekeys {
		var canonicalExists int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bambu_jobs WHERE job_key=?`, item.canonical).Scan(&canonicalExists); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `UPDATE bambu_snapshots SET job_key=? WHERE job_key=?; UPDATE bambu_events SET job_key=? WHERE job_key=?;`, item.canonical, item.oldKey, item.canonical, item.oldKey); err != nil {
			return err
		}
		if canonicalExists == 0 {
			if _, err := db.ExecContext(ctx, `UPDATE bambu_jobs SET job_key=? WHERE job_key=?`, item.canonical, item.oldKey); err != nil {
				return err
			}
			continue
		}
		if _, err := db.ExecContext(ctx, `UPDATE bambu_jobs SET
			started_at=CASE
				WHEN COALESCE(started_at,'')='' THEN (SELECT started_at FROM bambu_jobs WHERE job_key=?)
				WHEN COALESCE((SELECT started_at FROM bambu_jobs WHERE job_key=?),'')='' THEN started_at
				WHEN (SELECT started_at FROM bambu_jobs WHERE job_key=?)<started_at THEN (SELECT started_at FROM bambu_jobs WHERE job_key=?)
				ELSE started_at END,
			finished_at=CASE
				WHEN COALESCE(finished_at,'')='' THEN (SELECT finished_at FROM bambu_jobs WHERE job_key=?)
				WHEN COALESCE((SELECT finished_at FROM bambu_jobs WHERE job_key=?),'')='' THEN finished_at
				WHEN (SELECT finished_at FROM bambu_jobs WHERE job_key=?)>finished_at THEN (SELECT finished_at FROM bambu_jobs WHERE job_key=?)
				ELSE finished_at END,
			outcome=CASE WHEN COALESCE((SELECT outcome FROM bambu_jobs WHERE job_key=?),'')<>'' THEN (SELECT outcome FROM bambu_jobs WHERE job_key=?) ELSE outcome END,
			duration_seconds=NULL
			WHERE job_key=?`,
			item.oldKey, item.oldKey, item.oldKey, item.oldKey,
			item.oldKey, item.oldKey, item.oldKey, item.oldKey,
			item.oldKey, item.oldKey, item.canonical); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM bambu_jobs WHERE job_key=?`, item.oldKey); err != nil {
			return err
		}
	}

	if _, err := db.ExecContext(ctx, `UPDATE bambu_snapshots SET data=json_set(data,'$.plate_number',CAST(COALESCE(json_extract(data,'$.raw.plate_idx'),json_extract(data,'$.raw.plate_id')) AS INTEGER)) WHERE CAST(COALESCE(json_extract(data,'$.raw.plate_idx'),json_extract(data,'$.raw.plate_id')) AS INTEGER) BETWEEN 1 AND 999;
		UPDATE bambu_events SET data=json_set(data,'$.plate_number',CAST(COALESCE(json_extract(data,'$.raw.plate_idx'),json_extract(data,'$.raw.plate_id')) AS INTEGER)) WHERE CAST(COALESCE(json_extract(data,'$.raw.plate_idx'),json_extract(data,'$.raw.plate_id')) AS INTEGER) BETWEEN 1 AND 999;
		UPDATE bambu_jobs SET data=json_set(data,'$.plate_number',CAST(COALESCE(json_extract(data,'$.raw.plate_idx'),json_extract(data,'$.raw.plate_id')) AS INTEGER)) WHERE CAST(COALESCE(json_extract(data,'$.raw.plate_idx'),json_extract(data,'$.raw.plate_id')) AS INTEGER) BETWEEN 1 AND 999;`); err != nil {
		return fmt.Errorf("repair Bambu plate numbering: %w", err)
	}
	if err := h.rebuildJobTerminals(ctx, db); err != nil {
		return err
	}
	return h.migrateStableJobOrdering(ctx, db)
}

func (h *History) migratePayloadTaskOwnership(ctx context.Context, db historyMigrationDB) error {
	if _, err := db.ExecContext(ctx, `UPDATE bambu_snapshots
		SET job_key=printer_key||'/'||TRIM(CAST(json_extract(data,'$.task_id') AS TEXT))
		WHERE printer_key<>''
		AND TRIM(COALESCE(CAST(json_extract(data,'$.task_id') AS TEXT),'')) NOT IN ('','0')
		AND job_key<>printer_key||'/'||TRIM(CAST(json_extract(data,'$.task_id') AS TEXT));
		UPDATE bambu_events
		SET job_key=printer_key||'/'||TRIM(CAST(json_extract(data,'$.task_id') AS TEXT))
		WHERE printer_key<>''
		AND TRIM(COALESCE(CAST(json_extract(data,'$.task_id') AS TEXT),'')) NOT IN ('','0')
		AND job_key<>printer_key||'/'||TRIM(CAST(json_extract(data,'$.task_id') AS TEXT));

		INSERT OR IGNORE INTO bambu_jobs(job_key,printer_key,name,task_id,subtask_id,file,sort_started_at,initial_remaining_minutes,data)
		SELECT s.job_key,s.printer_key,
			COALESCE(NULLIF(json_extract(s.data,'$.job_name'),''),'unnamed print'),
			COALESCE(CAST(json_extract(s.data,'$.task_id') AS TEXT),''),
			COALESCE(CAST(json_extract(s.data,'$.subtask_id') AS TEXT),''),
			COALESCE(json_extract(s.data,'$.gcode_file'),''),
			s.observed_at,
			CAST(json_extract(s.data,'$.remaining_minutes') AS INTEGER),
			s.data
		FROM bambu_snapshots s
		WHERE s.id=(SELECT first.id FROM bambu_snapshots first WHERE first.job_key=s.job_key ORDER BY first.observed_at,first.id LIMIT 1);

		INSERT OR IGNORE INTO bambu_jobs(job_key,printer_key,name,task_id,subtask_id,file,sort_started_at,data)
		SELECT e.job_key,e.printer_key,
			COALESCE(NULLIF(json_extract(e.data,'$.job_name'),''),'unnamed print'),
			COALESCE(CAST(json_extract(e.data,'$.task_id') AS TEXT),''),
			COALESCE(CAST(json_extract(e.data,'$.subtask_id') AS TEXT),''),
			COALESCE(json_extract(e.data,'$.gcode_file'),''),
			e.occurred_at,
			e.data
		FROM bambu_events e
		WHERE e.id=(SELECT first.id FROM bambu_events first WHERE first.job_key=e.job_key ORDER BY first.occurred_at,first.id LIMIT 1);`); err != nil {
		return fmt.Errorf("migrate Bambu payload task ownership: %w", err)
	}
	if err := h.rebuildJobTerminals(ctx, db); err != nil {
		return err
	}
	return h.migrateStableJobOrdering(ctx, db)
}

func (h *History) migrateUnscopedJobOwnership(ctx context.Context, db historyMigrationDB) error {
	rows, err := db.QueryContext(ctx, `SELECT job_key,TRIM(COALESCE(task_id,'')) FROM bambu_jobs WHERE printer_key='' AND TRIM(COALESCE(task_id,'')) NOT IN ('','0')`)
	if err != nil {
		return err
	}
	type unscopedJob struct{ jobKey, taskID string }
	jobs := make([]unscopedJob, 0)
	for rows.Next() {
		var job unscopedJob
		if err := rows.Scan(&job.jobKey, &job.taskID); err != nil {
			_ = rows.Close()
			return err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, job := range jobs {
		var count int
		var targetKey, printerKey string
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(MIN(job_key),''),COALESCE(MIN(printer_key),'') FROM bambu_jobs WHERE printer_key<>'' AND task_id=?`, job.taskID).Scan(&count, &targetKey, &printerKey); err != nil {
			return err
		}
		if count != 1 {
			continue
		}
		if _, err := db.ExecContext(ctx, `UPDATE bambu_jobs SET
			sort_started_at=MIN(sort_started_at,(SELECT sort_started_at FROM bambu_jobs WHERE job_key=?)),
			initial_remaining_minutes=CASE WHEN (SELECT sort_started_at FROM bambu_jobs WHERE job_key=?)<sort_started_at THEN COALESCE((SELECT initial_remaining_minutes FROM bambu_jobs WHERE job_key=?),initial_remaining_minutes) ELSE initial_remaining_minutes END
			WHERE job_key=?`, job.jobKey, job.jobKey, job.jobKey, targetKey); err != nil {
			return err
		}
		for _, statement := range []struct {
			query string
			args  []any
		}{
			{`UPDATE bambu_snapshots SET printer_key=?,job_key=?,data=json_set(data,'$.printer_key',?) WHERE job_key=?`, []any{printerKey, targetKey, printerKey, job.jobKey}},
			{`UPDATE bambu_events SET printer_key=?,job_key=?,data=json_set(data,'$.printer_key',?) WHERE job_key=?`, []any{printerKey, targetKey, printerKey, job.jobKey}},
			{`DELETE FROM bambu_jobs WHERE job_key=?`, []any{job.jobKey}},
		} {
			if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
				return err
			}
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE bambu_jobs SET started_at=(SELECT MIN(observed_at) FROM bambu_snapshots s WHERE s.job_key=bambu_jobs.job_key AND s.state='RUNNING'),duration_seconds=NULL WHERE started_at IS NULL AND finished_at IS NOT NULL AND EXISTS(SELECT 1 FROM bambu_snapshots s WHERE s.job_key=bambu_jobs.job_key AND s.state='RUNNING')`); err != nil {
		return fmt.Errorf("repair unscoped Bambu job starts: %w", err)
	}
	if err := h.rebuildJobTerminals(ctx, db); err != nil {
		return err
	}
	return h.migrateStableJobOrdering(ctx, db)
}

type historyColumnMigration int

const (
	snapshotPrinterKey historyColumnMigration = iota
	eventPrinterKey
	jobPrinterKey
	jobSortStartedAt
)

func (h *History) ensureHistoryColumn(ctx context.Context, db historyMigrationDB, migration historyColumnMigration) error {
	var pragma, column, alter string
	switch migration {
	case snapshotPrinterKey:
		pragma, column, alter = `PRAGMA table_info(bambu_snapshots)`, "printer_key", `ALTER TABLE bambu_snapshots ADD COLUMN printer_key TEXT NOT NULL DEFAULT ''`
	case eventPrinterKey:
		pragma, column, alter = `PRAGMA table_info(bambu_events)`, "printer_key", `ALTER TABLE bambu_events ADD COLUMN printer_key TEXT NOT NULL DEFAULT ''`
	case jobPrinterKey:
		pragma, column, alter = `PRAGMA table_info(bambu_jobs)`, "printer_key", `ALTER TABLE bambu_jobs ADD COLUMN printer_key TEXT NOT NULL DEFAULT ''`
	case jobSortStartedAt:
		pragma, column, alter = `PRAGMA table_info(bambu_jobs)`, "sort_started_at", `ALTER TABLE bambu_jobs ADD COLUMN sort_started_at TEXT NOT NULL DEFAULT '0001-01-01T00:00:00Z'`
	default:
		return fmt.Errorf("unsupported Bambu history column migration %d", migration)
	}
	rows, err := db.QueryContext(ctx, pragma)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	_ = rows.Close()
	if found {
		return nil
	}
	_, err = db.ExecContext(ctx, alter)
	return err
}

func (h *History) rebuildJobTerminals(ctx context.Context, db historyMigrationDB) error {
	_, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO bambu_jobs(job_key,printer_key,name,task_id,subtask_id,file,initial_remaining_minutes,data)
	SELECT e.job_key,e.printer_key,COALESCE(json_extract(e.data,'$.job_name'),'unnamed print'),COALESCE(json_extract(e.data,'$.task_id'),''),COALESCE(json_extract(e.data,'$.subtask_id'),''),COALESCE(json_extract(e.data,'$.gcode_file'),''),CAST(json_extract(e.data,'$.remaining_minutes') AS INTEGER),e.data FROM bambu_events e WHERE e.printer_key<>'' AND e.id=(SELECT first.id FROM bambu_events first WHERE first.job_key=e.job_key ORDER BY first.occurred_at ASC,first.id ASC LIMIT 1);
UPDATE bambu_jobs SET
started_at=COALESCE(started_at,(SELECT MIN(occurred_at) FROM bambu_events e WHERE e.job_key=bambu_jobs.job_key AND e.event_type='started')),
finished_at=COALESCE(finished_at,(SELECT MAX(occurred_at) FROM bambu_events e WHERE e.job_key=bambu_jobs.job_key AND e.event_type IN ('finished','failed','canceled'))),
outcome=COALESCE(NULLIF(outcome,''),(SELECT event_type FROM bambu_events e WHERE e.job_key=bambu_jobs.job_key AND e.event_type IN ('finished','failed','canceled') ORDER BY occurred_at DESC LIMIT 1));
UPDATE bambu_jobs SET duration_seconds=CAST(strftime('%s',finished_at)-strftime('%s',started_at) AS INTEGER) WHERE duration_seconds IS NULL AND started_at IS NOT NULL AND finished_at IS NOT NULL;`)
	if err != nil {
		return fmt.Errorf("rebuild printer-scoped jobs: %w", err)
	}
	return h.recalculateJobDurations(ctx, db)
}

func (h *History) recalculateJobDurations(ctx context.Context, db historyMigrationDB) error {
	rows, err := db.QueryContext(ctx, `SELECT job_key,started_at,finished_at FROM bambu_jobs WHERE started_at IS NOT NULL AND finished_at IS NOT NULL`)
	if err != nil {
		return err
	}
	type jobDuration struct {
		jobKey     string
		startedAt  time.Time
		finishedAt time.Time
	}
	durations := make([]jobDuration, 0)
	for rows.Next() {
		var jobKey, startedText, finishedText string
		if err := rows.Scan(&jobKey, &startedText, &finishedText); err != nil {
			_ = rows.Close()
			return err
		}
		startedAt, startErr := time.Parse(time.RFC3339Nano, startedText)
		finishedAt, finishErr := time.Parse(time.RFC3339Nano, finishedText)
		if startErr == nil && finishErr == nil && !finishedAt.Before(startedAt) {
			durations = append(durations, jobDuration{jobKey: jobKey, startedAt: startedAt, finishedAt: finishedAt})
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, duration := range durations {
		if _, err := db.ExecContext(ctx, `UPDATE bambu_jobs SET duration_seconds=? WHERE job_key=?`, int64(duration.finishedAt.Sub(duration.startedAt)/time.Second), duration.jobKey); err != nil {
			return err
		}
	}
	return nil
}

func (h *History) upgradeShortPrinterKeys(ctx context.Context, db historyMigrationDB) error {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT printer_key FROM (SELECT printer_key FROM bambu_snapshots UNION SELECT printer_key FROM bambu_events UNION SELECT printer_key FROM bambu_jobs UNION SELECT printer_key FROM bambu_maintenance) WHERE printer_key GLOB 'printer-[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]' AND length(printer_key)=16`)
	if err != nil {
		return err
	}
	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			_ = rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	_ = rows.Close()
	for _, oldKey := range keys {
		sum := sha256.Sum256([]byte(oldKey))
		newKey := fmt.Sprintf("printer-%x", sum[:16])
		statements := []string{
			`UPDATE bambu_snapshots SET printer_key=?,job_key=?||substr(job_key,length(?)+1),data=json_set(data,'$.printer_key',?) WHERE printer_key=?`,
			`UPDATE bambu_events SET printer_key=?,job_key=?||substr(job_key,length(?)+1),data=json_set(data,'$.printer_key',?) WHERE printer_key=?`,
			`UPDATE bambu_jobs SET printer_key=?,job_key=?||substr(job_key,length(?)+1),data=json_set(data,'$.printer_key',?) WHERE printer_key=?`,
			`UPDATE bambu_maintenance SET printer_key=? WHERE printer_key=?`,
		}
		for index, statement := range statements {
			var execErr error
			if index < 3 {
				_, execErr = db.ExecContext(ctx, statement, newKey, newKey, oldKey, newKey, oldKey)
			} else {
				_, execErr = db.ExecContext(ctx, statement, newKey, oldKey)
			}
			if execErr != nil {
				return fmt.Errorf("upgrade short printer key: %w", execErr)
			}
		}
	}
	return nil
}

func (h *History) ClaimLegacyPrinterKeys(ctx context.Context, canonical string, aliases ...string) error {
	if canonical == "" {
		return fmt.Errorf("canonical printer key is required")
	}
	for _, alias := range aliases {
		if alias == "" || alias == canonical {
			continue
		}
		tx, err := h.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT job_key FROM bambu_jobs WHERE printer_key=?`, alias)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		jobKeys := make([]string, 0)
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				_ = rows.Close()
				_ = tx.Rollback()
				return err
			}
			jobKeys = append(jobKeys, key)
		}
		_ = rows.Close()
		for _, oldJobKey := range jobKeys {
			_, suffix, found := strings.Cut(oldJobKey, "/")
			if !found {
				suffix = oldJobKey
			}
			newJobKey := canonical + "/" + suffix
			_, err = tx.ExecContext(ctx, `INSERT INTO bambu_jobs(job_key,printer_key,name,task_id,subtask_id,file,started_at,sort_started_at,finished_at,outcome,duration_seconds,initial_remaining_minutes,weight_grams,data)
	SELECT ?,?,name,task_id,subtask_id,file,started_at,sort_started_at,finished_at,outcome,duration_seconds,initial_remaining_minutes,weight_grams,json_set(data,'$.printer_key',?) FROM bambu_jobs WHERE job_key=?
			ON CONFLICT(job_key) DO UPDATE SET
			started_at=CASE WHEN bambu_jobs.started_at IS NULL THEN excluded.started_at WHEN excluded.started_at IS NULL THEN bambu_jobs.started_at ELSE MIN(bambu_jobs.started_at,excluded.started_at) END,
			sort_started_at=CASE WHEN bambu_jobs.started_at IS NOT NULL AND excluded.started_at IS NOT NULL THEN MIN(bambu_jobs.started_at,excluded.started_at) WHEN bambu_jobs.started_at IS NOT NULL THEN bambu_jobs.started_at WHEN excluded.started_at IS NOT NULL THEN excluded.started_at ELSE MAX(bambu_jobs.sort_started_at,excluded.sort_started_at) END,
			finished_at=CASE WHEN bambu_jobs.finished_at IS NULL THEN excluded.finished_at WHEN excluded.finished_at IS NULL THEN bambu_jobs.finished_at ELSE MAX(bambu_jobs.finished_at,excluded.finished_at) END,
			outcome=CASE WHEN excluded.finished_at IS NOT NULL AND (bambu_jobs.finished_at IS NULL OR excluded.finished_at>=bambu_jobs.finished_at) THEN excluded.outcome ELSE bambu_jobs.outcome END,
			duration_seconds=CASE WHEN (bambu_jobs.started_at IS NOT NULL OR excluded.started_at IS NOT NULL) AND (bambu_jobs.finished_at IS NOT NULL OR excluded.finished_at IS NOT NULL) THEN CAST(strftime('%s',CASE WHEN bambu_jobs.finished_at IS NULL THEN excluded.finished_at WHEN excluded.finished_at IS NULL THEN bambu_jobs.finished_at ELSE MAX(bambu_jobs.finished_at,excluded.finished_at) END)-strftime('%s',CASE WHEN bambu_jobs.started_at IS NULL THEN excluded.started_at WHEN excluded.started_at IS NULL THEN bambu_jobs.started_at ELSE MIN(bambu_jobs.started_at,excluded.started_at) END) AS INTEGER) ELSE COALESCE(bambu_jobs.duration_seconds,excluded.duration_seconds) END,
			initial_remaining_minutes=COALESCE(bambu_jobs.initial_remaining_minutes,excluded.initial_remaining_minutes),weight_grams=COALESCE(bambu_jobs.weight_grams,excluded.weight_grams)`, newJobKey, canonical, canonical, oldJobKey)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM bambu_jobs WHERE job_key=?`, oldJobKey); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE bambu_snapshots SET printer_key=?,job_key=?||substr(job_key,length(?)+1),data=json_set(data,'$.printer_key',?) WHERE printer_key=?`, canonical, canonical, alias, canonical, alias); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE bambu_events SET printer_key=?,job_key=?||substr(job_key,length(?)+1),data=json_set(data,'$.printer_key',?) WHERE printer_key=?`, canonical, canonical, alias, canonical, alias); err != nil {
			_ = tx.Rollback()
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO bambu_maintenance(printer_key,task,completed_at,note) SELECT ?,task,completed_at,note FROM bambu_maintenance WHERE printer_key=? ON CONFLICT(printer_key,task) DO UPDATE SET completed_at=MAX(bambu_maintenance.completed_at,excluded.completed_at),note=CASE WHEN excluded.completed_at>=bambu_maintenance.completed_at THEN excluded.note ELSE bambu_maintenance.note END`, canonical, alias)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM bambu_maintenance WHERE printer_key=?`, alias); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func JobKey(snapshot Snapshot) string {
	base := ""
	for _, value := range []string{snapshot.TaskID, snapshot.SubtaskID} {
		if candidate := strings.TrimSpace(value); candidate != "" && candidate != "0" {
			base = candidate
			break
		}
	}
	if base == "" {
		base = strings.ToLower(strings.TrimSpace(snapshot.JobName + "|" + snapshot.GCodeFile))
	}
	if snapshot.PrinterKey != "" {
		return snapshot.PrinterKey + "/" + base
	}
	return base
}

func (h *History) RecordSnapshot(ctx context.Context, snapshot Snapshot) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := recordSnapshot(ctx, tx, snapshot); err != nil {
		return err
	}
	return tx.Commit()
}

type historyExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func recordSnapshot(ctx context.Context, executor historyExecutor, snapshot Snapshot) error {
	snapshot = SanitizeSnapshot(snapshot)
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	jobKey := JobKey(snapshot)
	_, err = executor.ExecContext(ctx, `INSERT INTO bambu_snapshots(printer_key,job_key,observed_at,state,data) VALUES(?,?,?,?,?)`, snapshot.PrinterKey, jobKey, snapshot.ObservedAt.Format(time.RFC3339Nano), snapshot.State, string(payload))
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `INSERT INTO bambu_jobs(job_key,printer_key,name,task_id,subtask_id,file,sort_started_at,initial_remaining_minutes,data)
	VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(job_key) DO UPDATE SET printer_key=excluded.printer_key,name=excluded.name,task_id=excluded.task_id,subtask_id=excluded.subtask_id,file=excluded.file,sort_started_at=CASE WHEN bambu_jobs.started_at IS NULL THEN MAX(bambu_jobs.sort_started_at,excluded.sort_started_at) ELSE bambu_jobs.started_at END,initial_remaining_minutes=COALESCE(bambu_jobs.initial_remaining_minutes,excluded.initial_remaining_minutes),data=excluded.data`,
		jobKey, snapshot.PrinterKey, snapshot.JobName, snapshot.TaskID, snapshot.SubtaskID, snapshot.GCodeFile, snapshot.ObservedAt.Format(time.RFC3339Nano), validInitialRemaining(snapshot), string(payload))
	return err
}

func validInitialRemaining(snapshot Snapshot) *int {
	if snapshot.State != "RUNNING" || snapshot.RemainingMinutes == nil || *snapshot.RemainingMinutes <= 0 {
		return nil
	}
	if snapshot.Percent != nil && *snapshot.Percent == 0 && snapshot.CurrentLayer != nil && *snapshot.CurrentLayer > 1 {
		return nil
	}
	return snapshot.RemainingMinutes
}

func (h *History) RecordMetadata(ctx context.Context, snapshot Snapshot, metadata Metadata) error {
	snapshot = SanitizeSnapshot(snapshot)
	_, err := h.db.ExecContext(ctx, `UPDATE bambu_jobs SET weight_grams=? WHERE job_key=?`, metadata.WeightGrams, JobKey(snapshot))
	return err
}

func (h *History) RecordEvent(ctx context.Context, event Event) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := recordEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (h *History) RecordLifecycleEvent(ctx context.Context, event Event) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := recordSnapshot(ctx, tx, event.Snapshot); err != nil {
		return err
	}
	if err := recordEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func recordEvent(ctx context.Context, executor historyExecutor, event Event) error {
	event.Snapshot = SanitizeSnapshot(event.Snapshot)
	payload, err := json.Marshal(event.Snapshot)
	if err != nil {
		return err
	}
	jobKey := JobKey(event.Snapshot)
	occurred := event.Snapshot.ObservedAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}
	if _, err := executor.ExecContext(ctx, `INSERT INTO bambu_events(printer_key,job_key,event_type,occurred_at,data) VALUES(?,?,?,?,?)`, event.Snapshot.PrinterKey, jobKey, event.Kind, occurred.Format(time.RFC3339Nano), string(payload)); err != nil {
		return err
	}
	switch event.Kind {
	case "started":
		_, err = executor.ExecContext(ctx, `UPDATE bambu_jobs SET started_at=COALESCE(started_at,?),sort_started_at=COALESCE(started_at,?),outcome='' WHERE job_key=?`, occurred.Format(time.RFC3339Nano), occurred.Format(time.RFC3339Nano), jobKey)
	case "finished", "failed", "canceled":
		_, err = executor.ExecContext(ctx, `UPDATE bambu_jobs SET finished_at=?, outcome=?, duration_seconds=CASE WHEN started_at IS NOT NULL THEN CAST(strftime('%s',?)-strftime('%s',started_at) AS INTEGER) ELSE duration_seconds END WHERE job_key=?`, occurred.Format(time.RFC3339Nano), event.Kind, occurred.Format(time.RFC3339Nano), jobKey)
	}
	return err
}

func (h *History) Jobs(ctx context.Context, since time.Time, limit int) ([]StoredJob, error) {
	return h.JobsForPrinter(ctx, since, "", limit)
}

func (h *History) JobsForPrinter(ctx context.Context, since time.Time, printerKey string, limit int) ([]StoredJob, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	query := `SELECT job_key,name,task_id,subtask_id,file,started_at,finished_at,outcome,duration_seconds,initial_remaining_minutes,weight_grams FROM bambu_jobs WHERE sort_started_at>=?`
	args := []any{since.Format(time.RFC3339Nano)}
	if printerKey != "" {
		query += ` AND printer_key=?`
		args = append(args, printerKey)
	}
	query += ` ORDER BY sort_started_at DESC,job_key DESC LIMIT ?`
	args = append(args, limit)
	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]StoredJob, 0)
	for rows.Next() {
		var job StoredJob
		var taskID, subtaskID, file, started, finished sql.NullString
		var outcome sql.NullString
		var duration, remaining sql.NullInt64
		var weight sql.NullFloat64
		if err := rows.Scan(&job.JobKey, &job.Name, &taskID, &subtaskID, &file, &started, &finished, &outcome, &duration, &remaining, &weight); err != nil {
			return nil, err
		}
		job.TaskID, job.SubtaskID, job.File = taskID.String, subtaskID.String, file.String
		job.Outcome = outcome.String
		job.StartedAt = parseNullTime(started)
		job.FinishedAt = parseNullTime(finished)
		if duration.Valid {
			v := duration.Int64
			job.DurationSeconds = &v
		}
		if remaining.Valid {
			v := int(remaining.Int64)
			job.InitialRemainingMinutes = &v
		}
		if weight.Valid {
			v := weight.Float64
			job.WeightGrams = &v
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (h *History) JobsPage(ctx context.Context, since time.Time, printerKey string, limit, offset int) ([]StoredJob, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT job_key,name,task_id,subtask_id,file,started_at,finished_at,outcome,duration_seconds,initial_remaining_minutes,weight_grams FROM bambu_jobs WHERE sort_started_at>=?`
	args := []any{since.Format(time.RFC3339Nano)}
	if printerKey != "" {
		query += ` AND printer_key=?`
		args = append(args, printerKey)
	}
	query += ` ORDER BY sort_started_at DESC,job_key DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]StoredJob, 0, limit)
	for rows.Next() {
		var job StoredJob
		var taskID, subtaskID, file, started, finished sql.NullString
		var outcome sql.NullString
		var duration, remaining sql.NullInt64
		var weight sql.NullFloat64
		if err := rows.Scan(&job.JobKey, &job.Name, &taskID, &subtaskID, &file, &started, &finished, &outcome, &duration, &remaining, &weight); err != nil {
			return nil, err
		}
		job.TaskID, job.SubtaskID, job.File = taskID.String, subtaskID.String, file.String
		job.Outcome = outcome.String
		job.StartedAt = parseNullTime(started)
		job.FinishedAt = parseNullTime(finished)
		if duration.Valid {
			value := duration.Int64
			job.DurationSeconds = &value
		}
		if remaining.Valid {
			value := int(remaining.Int64)
			job.InitialRemainingMinutes = &value
		}
		if weight.Valid {
			value := weight.Float64
			job.WeightGrams = &value
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (h *History) Events(ctx context.Context, jobKey string, limit int) ([]StoredEvent, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	rows, err := h.db.QueryContext(ctx, `SELECT id,job_key,event_type,occurred_at,data FROM bambu_events WHERE (?='' OR job_key=?) ORDER BY occurred_at ASC LIMIT ?`, jobKey, jobKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]StoredEvent, 0)
	for rows.Next() {
		var event StoredEvent
		var occurred, data string
		if err := rows.Scan(&event.ID, &event.JobKey, &event.EventType, &occurred, &data); err != nil {
			return nil, err
		}
		event.OccurredAt, _ = time.Parse(time.RFC3339Nano, occurred)
		_ = json.Unmarshal([]byte(data), &event.Data)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (h *History) EventsBetween(ctx context.Context, jobKey string, start, end time.Time, includeStart bool, limit int) ([]StoredEvent, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	operator := ">"
	if includeStart {
		operator = ">="
	}
	rows, err := h.db.QueryContext(ctx, `SELECT id,job_key,event_type,occurred_at,data FROM bambu_events WHERE job_key=? AND occurred_at`+operator+`? AND occurred_at<=? ORDER BY occurred_at ASC LIMIT ?`, jobKey, start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]StoredEvent, 0)
	for rows.Next() {
		var event StoredEvent
		var occurred, data string
		if err := rows.Scan(&event.ID, &event.JobKey, &event.EventType, &occurred, &data); err != nil {
			return nil, err
		}
		event.OccurredAt, _ = time.Parse(time.RFC3339Nano, occurred)
		_ = json.Unmarshal([]byte(data), &event.Data)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (h *History) Snapshots(ctx context.Context, since time.Time, limit int) ([]Snapshot, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	rows, err := h.db.QueryContext(ctx, `SELECT DISTINCT data FROM bambu_snapshots WHERE observed_at>=? ORDER BY observed_at ASC LIMIT ?`, since.Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Snapshot, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var snapshot Snapshot
		if json.Unmarshal([]byte(data), &snapshot) == nil {
			items = append(items, snapshot)
		}
	}
	return items, rows.Err()
}

func (h *History) SnapshotEndpoints(ctx context.Context, since time.Time) (Snapshot, Snapshot, int, error) {
	return h.SnapshotEndpointsForPrinter(ctx, since, "")
}

func (h *History) SnapshotEndpointsForPrinter(ctx context.Context, since time.Time, printerKey string) (Snapshot, Snapshot, int, error) {
	var first, last Snapshot
	var firstData, lastData string
	filter, args := ``, []any{since.Format(time.RFC3339Nano)}
	if printerKey != "" {
		filter = ` AND printer_key=?`
		args = append(args, printerKey)
	}
	if err := h.db.QueryRowContext(ctx, `SELECT data FROM bambu_snapshots WHERE observed_at>=?`+filter+` ORDER BY observed_at ASC LIMIT 1`, args...).Scan(&firstData); err != nil && err != sql.ErrNoRows {
		return first, last, 0, err
	}
	if firstData == "" {
		return first, last, 0, nil
	}
	if err := h.db.QueryRowContext(ctx, `SELECT data FROM bambu_snapshots WHERE observed_at>=?`+filter+` ORDER BY observed_at DESC LIMIT 1`, args...).Scan(&lastData); err != nil {
		return first, last, 0, err
	}
	var count int
	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bambu_snapshots WHERE observed_at>=?`+filter, args...).Scan(&count); err != nil {
		return first, last, 0, err
	}
	if err := json.Unmarshal([]byte(firstData), &first); err != nil {
		return first, last, 0, err
	}
	if err := json.Unmarshal([]byte(lastData), &last); err != nil {
		return first, last, 0, err
	}
	return first, last, count, nil
}

func (h *History) SnapshotsPage(ctx context.Context, since time.Time, printerKey, jobKey string, limit, offset int, descending bool) ([]Snapshot, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	direction := "ASC"
	if descending {
		direction = "DESC"
	}
	query := `SELECT data FROM bambu_snapshots WHERE observed_at>=?`
	args := []any{since.Format(time.RFC3339Nano)}
	if printerKey != "" {
		query += ` AND printer_key=?`
		args = append(args, printerKey)
	}
	if jobKey != "" {
		query += ` AND job_key=?`
		args = append(args, jobKey)
	}
	query += ` ORDER BY observed_at ` + direction + ` LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Snapshot, 0, limit)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var snapshot Snapshot
		if json.Unmarshal([]byte(data), &snapshot) == nil {
			items = append(items, snapshot)
		}
	}
	return items, rows.Err()
}

func (h *History) CompletedPrintSeconds(ctx context.Context, since time.Time, printerKey string) (int64, error) {
	var seconds sql.NullInt64
	query, args := `SELECT SUM(duration_seconds) FROM bambu_jobs WHERE duration_seconds IS NOT NULL AND outcome='finished' AND finished_at>=?`, []any{since.Format(time.RFC3339Nano)}
	if printerKey != "" {
		query += ` AND printer_key=?`
		args = append(args, printerKey)
	}
	err := h.db.QueryRowContext(ctx, query, args...).Scan(&seconds)
	if err != nil || !seconds.Valid {
		return 0, err
	}
	return seconds.Int64, nil
}

func (h *History) HealthSnapshots(ctx context.Context, printerKey string, limit int) ([]Snapshot, error) {
	if limit <= 0 || limit > 1000 {
		limit = 25
	}
	query, args := `SELECT data FROM bambu_snapshots WHERE (CAST(COALESCE(json_extract(data,'$.print_error'),0) AS INTEGER)<>0 OR COALESCE(json_array_length(json_extract(data,'$.raw.hms')),0)>0)`, []any{}
	if printerKey != "" {
		query += ` AND printer_key=?`
		args = append(args, printerKey)
	}
	query += ` ORDER BY observed_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Snapshot, 0, limit)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var snapshot Snapshot
		if json.Unmarshal([]byte(data), &snapshot) == nil {
			items = append(items, snapshot)
		}
	}
	return items, rows.Err()
}

func (h *History) CompleteMaintenance(ctx context.Context, printerKey, task, note string) error {
	_, err := h.db.ExecContext(ctx, `INSERT INTO bambu_maintenance(printer_key,task,completed_at,note) VALUES(?,?,?,?) ON CONFLICT(printer_key,task) DO UPDATE SET completed_at=excluded.completed_at,note=excluded.note`, printerKey, task, time.Now().UTC().Format(time.RFC3339Nano), note)
	return err
}

func (h *History) Maintenance(ctx context.Context, printerKey string) ([]map[string]any, error) {
	query, args := `SELECT printer_key,task,completed_at,note FROM bambu_maintenance`, []any{}
	if printerKey != "" {
		query += ` WHERE printer_key=? OR (printer_key='' AND NOT EXISTS (SELECT 1 FROM bambu_maintenance scoped WHERE scoped.printer_key=? AND scoped.task=bambu_maintenance.task))`
		args = append(args, printerKey, printerKey)
	}
	query += ` ORDER BY completed_at DESC`
	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var scope, task, at, note string
		if err := rows.Scan(&scope, &task, &at, &note); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"printer_key": scope, "task": task, "completed_at": at, "note": note, "legacy_unscoped": scope == ""})
	}
	return items, rows.Err()
}

func parseNullTime(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil
	}
	return &parsed
}
