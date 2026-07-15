package bambu

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/store"
)

func TestHistoryQueriesRemainCorrectPastCaps(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	tx, err := history.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO bambu_snapshots(job_key,observed_at,state,data) VALUES(?,?,?,?)`)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for index := 0; index < 10005; index++ {
		jobKey := "other"
		if index == 10004 {
			jobKey = "latest-job"
		}
		snapshot := Snapshot{ObservedAt: base.Add(time.Duration(index) * time.Second), State: "RUNNING", TaskID: jobKey, Raw: map[string]any{"sequence": index}}
		payload, _ := json.Marshal(snapshot)
		if _, err := stmt.ExecContext(ctx, jobKey, snapshot.ObservedAt.Format(time.RFC3339Nano), snapshot.State, string(payload)); err != nil {
			t.Fatal(err)
		}
	}
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	first, last, count, err := history.SnapshotEndpoints(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if count != 10005 || first.Raw["sequence"] != float64(0) || last.Raw["sequence"] != float64(10004) {
		t.Fatalf("unexpected endpoints: count=%d first=%v last=%v", count, first.Raw["sequence"], last.Raw["sequence"])
	}
	page, err := history.SnapshotsPage(ctx, time.Time{}, "", "latest-job", 100, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || JobKey(page[0]) != "latest-job" {
		t.Fatalf("job page = %#v", page)
	}

	jobTx, err := history.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobStmt, err := jobTx.PrepareContext(ctx, `INSERT INTO bambu_jobs(job_key,name,finished_at,outcome,duration_seconds,data) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 1005; index++ {
		if _, err := jobStmt.ExecContext(ctx, fmt.Sprintf("job-%d", index), "job", base.Format(time.RFC3339Nano), "finished", 60, `{}`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := jobStmt.ExecContext(ctx, "failed-job", "job", base.Format(time.RFC3339Nano), "failed", 9999, `{}`); err != nil {
		t.Fatal(err)
	}
	_ = jobStmt.Close()
	if err := jobTx.Commit(); err != nil {
		t.Fatal(err)
	}
	seconds, err := history.CompletedPrintSeconds(ctx, base.Add(-time.Second), "")
	if err != nil {
		t.Fatal(err)
	}
	if seconds != 1005*60 {
		t.Fatalf("completed seconds = %d, want %d", seconds, 1005*60)
	}
}

func TestHistoryScopesSameTaskByPrinter(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "scoped.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	base := time.Now().UTC()
	for index, serial := range []string{"PRINTERSERIAL001", "PRINTERSERIAL002"} {
		snapshot := Snapshot{ObservedAt: base.Add(time.Duration(index) * time.Second), Serial: serial, State: "RUNNING", TaskID: "same-task", JobName: "same job", Raw: map[string]any{}}
		if err := history.RecordSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	firstKey, secondKey := PrinterKey("PRINTERSERIAL001"), PrinterKey("PRINTERSERIAL002")
	first, err := history.JobsForPrinter(ctx, time.Time{}, firstKey, 10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := history.JobsForPrinter(ctx, time.Time{}, secondKey, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].JobKey == second[0].JobKey {
		t.Fatalf("printer scopes collided: %#v %#v", first, second)
	}
	if err := history.CompleteMaintenance(ctx, firstKey, "nozzle-clean", "first"); err != nil {
		t.Fatal(err)
	}
	if err := history.CompleteMaintenance(ctx, secondKey, "nozzle-clean", "second"); err != nil {
		t.Fatal(err)
	}
	items, err := history.Maintenance(ctx, firstKey)
	if err != nil || len(items) != 1 || items[0]["note"] != "first" {
		t.Fatalf("scoped maintenance = %#v err=%v", items, err)
	}
}

func TestLegacySameTaskMigrationRebuildsBothPrinters(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE bambu_snapshots(id INTEGER PRIMARY KEY AUTOINCREMENT,job_key TEXT NOT NULL,observed_at TEXT NOT NULL,state TEXT NOT NULL,data TEXT NOT NULL);
CREATE TABLE bambu_events(id INTEGER PRIMARY KEY AUTOINCREMENT,job_key TEXT NOT NULL,event_type TEXT NOT NULL,occurred_at TEXT NOT NULL,data TEXT NOT NULL);
CREATE TABLE bambu_jobs(job_key TEXT PRIMARY KEY,name TEXT NOT NULL,task_id TEXT,subtask_id TEXT,file TEXT,started_at TEXT,finished_at TEXT,outcome TEXT,duration_seconds INTEGER,initial_remaining_minutes INTEGER,weight_grams REAL,data TEXT NOT NULL DEFAULT '{}');
CREATE TABLE bambu_maintenance(task TEXT PRIMARY KEY,completed_at TEXT NOT NULL,note TEXT NOT NULL DEFAULT '');`)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	keys := []string{"printer-aaaaaaaa", "printer-bbbbbbbb"}
	for index, key := range keys {
		remaining := 100 + index*100
		snapshot := Snapshot{ObservedAt: base.Add(time.Duration(index) * time.Second), PrinterKey: key, State: "RUNNING", TaskID: "same-task", JobName: "legacy job", RemainingMinutes: &remaining, Raw: map[string]any{}}
		payload, _ := json.Marshal(snapshot)
		if _, err := db.Exec(`INSERT INTO bambu_snapshots(job_key,observed_at,state,data) VALUES(?,?,?,?)`, "same-task", snapshot.ObservedAt.Format(time.RFC3339Nano), snapshot.State, string(payload)); err != nil {
			t.Fatal(err)
		}
		if index == 1 {
			if _, err := db.Exec(`INSERT INTO bambu_jobs(job_key,name,task_id,data) VALUES(?,?,?,?)`, "same-task", snapshot.JobName, snapshot.TaskID, string(payload)); err != nil {
				t.Fatal(err)
			}
		}
		if index == 0 {
			laterRemaining := 50
			later := snapshot
			later.ObservedAt = base.Add(10 * time.Second)
			later.RemainingMinutes = &laterRemaining
			laterPayload, _ := json.Marshal(later)
			if _, err := db.Exec(`INSERT INTO bambu_snapshots(job_key,observed_at,state,data) VALUES(?,?,?,?)`, "same-task", later.ObservedAt.Format(time.RFC3339Nano), later.State, string(laterPayload)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := db.Exec(`INSERT INTO bambu_maintenance(task,completed_at,note) VALUES('legacy-task',?,'legacy')`, base.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	for pass := 0; pass < 2; pass++ {
		history, err := OpenHistory(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		jobs, err := history.Jobs(ctx, time.Time{}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(jobs) != 2 || jobs[0].JobKey == jobs[1].JobKey {
			t.Fatalf("pass %d migrated jobs = %#v", pass, jobs)
		}
		remainingValues := map[int]bool{}
		for _, job := range jobs {
			if job.InitialRemainingMinutes != nil {
				remainingValues[*job.InitialRemainingMinutes] = true
			}
		}
		if !remainingValues[100] || !remainingValues[200] || remainingValues[50] {
			t.Fatalf("migration did not preserve earliest remaining estimates: %#v", jobs)
		}
		firstScope := strings.SplitN(jobs[0].JobKey, "/", 2)[0]
		if len(firstScope) != len("printer-")+32 {
			t.Fatalf("short scope was not upgraded: %q", firstScope)
		}
		maintenance, err := history.Maintenance(ctx, firstScope)
		if err != nil || len(maintenance) != 1 || maintenance[0]["legacy_unscoped"] != true {
			t.Fatalf("legacy maintenance hidden: %#v err=%v", maintenance, err)
		}
		_ = history.Close()
	}
}

func TestEventsBetweenCoversGapAfterPreviousSnapshot(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	base := time.Now().UTC()
	snapshot := Snapshot{Serial: "PRINTERSERIAL001", TaskID: "task", State: "RUNNING", Raw: map[string]any{}}
	for _, offset := range []time.Duration{0, 2 * time.Second} {
		snapshot.ObservedAt = base.Add(offset)
		if err := history.RecordSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	snapshot.ObservedAt = base.Add(time.Second)
	if err := history.RecordEvent(ctx, Event{Kind: "paused", Snapshot: snapshot}); err != nil {
		t.Fatal(err)
	}
	events, err := history.EventsBetween(ctx, JobKey(SanitizeSnapshot(snapshot)), base, base.Add(2*time.Second), false, 10)
	if err != nil || len(events) != 1 || events[0].EventType != "paused" {
		t.Fatalf("gap events = %#v err=%v", events, err)
	}
}

func TestRecordSnapshotRollsBackWhenJobWriteFails(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "snapshot-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	if _, err := history.db.Exec(`CREATE TRIGGER fail_bambu_job_insert BEFORE INSERT ON bambu_jobs BEGIN SELECT RAISE(ABORT, 'forced job failure'); END`); err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{ObservedAt: time.Now().UTC(), Serial: "PRINTERSERIAL004", State: "RUNNING", TaskID: "rollback-task", JobName: "rollback", Raw: map[string]any{}}
	if err := history.RecordSnapshot(ctx, snapshot); err == nil {
		t.Fatal("RecordSnapshot succeeded despite forced job failure")
	}
	var snapshots, jobs int
	if err := history.db.QueryRow(`SELECT COUNT(*) FROM bambu_snapshots`).Scan(&snapshots); err != nil {
		t.Fatal(err)
	}
	if err := history.db.QueryRow(`SELECT COUNT(*) FROM bambu_jobs`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if snapshots != 0 || jobs != 0 {
		t.Fatalf("partial snapshot transaction persisted: snapshots=%d jobs=%d", snapshots, jobs)
	}
}

func TestRecordLifecycleEventRollsBackAsOneTransaction(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "event-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	if _, err := history.db.Exec(`CREATE TRIGGER fail_bambu_event_insert BEFORE INSERT ON bambu_events BEGIN SELECT RAISE(ABORT, 'forced event failure'); END`); err != nil {
		t.Fatal(err)
	}
	event := Event{Kind: "started", Snapshot: Snapshot{ObservedAt: time.Now().UTC(), Serial: "PRINTERSERIAL005", State: "RUNNING", TaskID: "rollback-event", JobName: "rollback", Raw: map[string]any{}}}
	if err := history.RecordLifecycleEvent(ctx, event); err == nil {
		t.Fatal("RecordLifecycleEvent succeeded despite forced event failure")
	}
	for _, table := range []string{"bambu_snapshots", "bambu_jobs", "bambu_events"} {
		var count int
		if err := history.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("partial lifecycle transaction persisted in %s: %d rows", table, count)
		}
	}
}

func TestClaimLegacyPrinterKeyMergesIntoCanonicalScope(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "claim.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	serial := "PRINTERSERIAL003"
	legacy := LegacyPrinterKeys(serial)[0]
	snapshot := Snapshot{ObservedAt: time.Now().UTC(), PrinterKey: legacy, State: "RUNNING", TaskID: "task", JobName: "legacy", Raw: map[string]any{}}
	if err := history.RecordSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	canonical := PrinterKey(serial)
	if err := history.ClaimLegacyPrinterKeys(ctx, canonical, LegacyPrinterKeys(serial)...); err != nil {
		t.Fatal(err)
	}
	jobs, err := history.JobsForPrinter(ctx, time.Time{}, canonical, 10)
	if err != nil || len(jobs) != 1 || !strings.HasPrefix(jobs[0].JobKey, canonical+"/") {
		t.Fatalf("claimed jobs = %#v err=%v", jobs, err)
	}
}

func TestJobsOrderUnknownStartsByLatestObservationDeterministically(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "ordering.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	base := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	printerKey := PrinterKey("PRINTERSERIAL004")
	record := func(task string, observed time.Time) Snapshot {
		snapshot := Snapshot{ObservedAt: observed, PrinterKey: printerKey, State: "RUNNING", TaskID: task, JobName: task, Raw: map[string]any{}}
		if err := history.RecordSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	record("old-unknown", base.Add(time.Hour))
	completed := record("completed", base.Add(4*time.Hour))
	completed.ObservedAt = base.Add(2 * time.Hour)
	if err := history.RecordEvent(ctx, Event{Kind: "started", Snapshot: completed}); err != nil {
		t.Fatal(err)
	}
	completed.ObservedAt = base.Add(3 * time.Hour)
	if err := history.RecordEvent(ctx, Event{Kind: "finished", Snapshot: completed}); err != nil {
		t.Fatal(err)
	}
	tiedAt := base.Add(5 * time.Hour)
	record("new-unknown-a", tiedAt)
	record("new-unknown-b", tiedAt)

	jobs, err := history.JobsForPrinter(ctx, time.Time{}, printerKey, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{printerKey + "/new-unknown-b", printerKey + "/new-unknown-a", printerKey + "/completed", printerKey + "/old-unknown"}
	if len(jobs) != len(want) {
		t.Fatalf("jobs = %#v", jobs)
	}
	for index := range want {
		if jobs[index].JobKey != want[index] {
			t.Fatalf("jobs[%d] = %q, want %q", index, jobs[index].JobKey, want[index])
		}
	}
}

func TestMigrationBackfillsUnknownStartFromLatestSnapshot(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sort-migration.db")
	history, err := OpenHistory(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	printerKey := PrinterKey("PRINTERSERIAL005")
	snapshot := Snapshot{PrinterKey: printerKey, State: "RUNNING", TaskID: "unknown-start", JobName: "unknown", Raw: map[string]any{}}
	for _, offset := range []time.Duration{time.Hour, 3 * time.Hour} {
		snapshot.ObservedAt = base.Add(offset)
		if err := history.RecordSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE bambu_jobs SET sort_started_at='9999-12-31T23:59:59.999999999Z'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM bambu_history_migrations WHERE version=2`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	history, err = OpenHistory(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	var sortStarted string
	if err := history.db.QueryRowContext(ctx, `SELECT sort_started_at FROM bambu_jobs WHERE job_key=?`, JobKey(snapshot)).Scan(&sortStarted); err != nil {
		t.Fatal(err)
	}
	want := base.Add(3 * time.Hour).Format(time.RFC3339Nano)
	if sortStarted != want {
		t.Fatalf("sort_started_at = %q, want latest snapshot %q", sortStarted, want)
	}
	jobs, err := history.JobsForPrinter(ctx, base.Add(2*time.Hour), printerKey, 10)
	if err != nil || len(jobs) != 1 || jobs[0].JobKey != JobKey(snapshot) {
		t.Fatalf("migrated jobs = %#v err=%v", jobs, err)
	}
}

func TestSecondOpenDoesNotRewriteHistory(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "repeat-open.db")
	history, err := OpenHistory(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{ObservedAt: time.Now().UTC(), PrinterKey: PrinterKey("PRINTERSERIAL006"), State: "RUNNING", TaskID: "task", JobName: "job", Raw: map[string]any{}}
	if err := history.RecordSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}

	observer, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close()
	var before, after int
	if err := observer.QueryRow(`PRAGMA data_version`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	second, err := OpenHistory(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if err := observer.QueryRow(`PRAGMA data_version`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("second open changed data_version from %d to %d", before, after)
	}
}

func TestOpenHistoryRejectsFutureSharedSchemaWithoutMutation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "future-shared.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf(`CREATE TABLE future_marker(value TEXT); INSERT INTO future_marker VALUES('preserved'); PRAGMA user_version=%d`, store.StoreSchemaVersion+1)); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if history, err := OpenHistory(ctx, path); err == nil {
		_ = history.Close()
		t.Fatal("future shared schema was accepted")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var marker string
	if err := db.QueryRow(`SELECT value FROM future_marker`).Scan(&marker); err != nil || marker != "preserved" {
		t.Fatalf("future marker = %q err=%v", marker, err)
	}
	var bambuTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name LIKE 'bambu_%'`).Scan(&bambuTables); err != nil || bambuTables != 0 {
		t.Fatalf("future shared DB mutated: tables=%d err=%v", bambuTables, err)
	}
}

func TestOpenHistoryRejectsFutureBambuSchemaWithoutMutation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "future-bambu.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	futureVersion := BambuHistorySchemaVersion + 1
	if _, err := db.Exec(fmt.Sprintf(`CREATE TABLE bambu_history_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL); INSERT INTO bambu_history_migrations VALUES(%d,'future'); CREATE TABLE future_marker(value TEXT); INSERT INTO future_marker VALUES('preserved')`, futureVersion)); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if history, err := OpenHistory(ctx, path); err == nil {
		_ = history.Close()
		t.Fatal("future Bambu schema was accepted")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var marker string
	if err := db.QueryRow(`SELECT value FROM future_marker`).Scan(&marker); err != nil || marker != "preserved" {
		t.Fatalf("future marker = %q err=%v", marker, err)
	}
	var snapshots int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='bambu_snapshots'`).Scan(&snapshots); err != nil || snapshots != 0 {
		t.Fatalf("future Bambu DB mutated: snapshots=%d err=%v", snapshots, err)
	}
}

func TestLegacyMigrationRollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rollback.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE bambu_snapshots(id INTEGER PRIMARY KEY AUTOINCREMENT,job_key TEXT NOT NULL,observed_at TEXT NOT NULL,state TEXT NOT NULL,data TEXT NOT NULL);
CREATE TABLE bambu_events(id INTEGER PRIMARY KEY AUTOINCREMENT,job_key TEXT NOT NULL,event_type TEXT NOT NULL,occurred_at TEXT NOT NULL,data TEXT NOT NULL);
CREATE TABLE bambu_jobs(job_key TEXT PRIMARY KEY,name TEXT NOT NULL,task_id TEXT,subtask_id TEXT,file TEXT,started_at TEXT,finished_at TEXT,outcome TEXT,duration_seconds INTEGER,initial_remaining_minutes INTEGER,weight_grams REAL,data TEXT NOT NULL DEFAULT '{}');
CREATE TABLE bambu_maintenance(printer_key TEXT NOT NULL DEFAULT '',task TEXT NOT NULL,completed_at TEXT NOT NULL,note TEXT NOT NULL DEFAULT '',PRIMARY KEY(printer_key,task));
INSERT INTO bambu_jobs(job_key,name,data) VALUES('broken','broken','{not-json');`)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if history, err := OpenHistory(ctx, path); err == nil {
		_ = history.Close()
		t.Fatal("malformed legacy migration unexpectedly succeeded")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`PRAGMA table_info(bambu_snapshots)`)
	if err != nil {
		t.Fatal(err)
	}
	hasPrinterKey := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		hasPrinterKey = hasPrinterKey || name == "printer_key"
	}
	_ = rows.Close()
	if hasPrinterKey {
		t.Fatal("failed v1 migration left printer_key column behind")
	}
	var ledgerTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='bambu_history_migrations'`).Scan(&ledgerTables); err != nil {
		t.Fatal(err)
	}
	if ledgerTables != 0 {
		t.Fatalf("failed v1 migration left %d ledger tables", ledgerTables)
	}
	var baseIndexes int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_bambu_snapshots_job_time'`).Scan(&baseIndexes); err != nil || baseIndexes != 0 {
		t.Fatalf("failed v1 migration left base indexes=%d err=%v", baseIndexes, err)
	}
}

func TestConcurrentFirstOpenSerializesHistoryMigrations(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "concurrent.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE bambu_snapshots(id INTEGER PRIMARY KEY AUTOINCREMENT,job_key TEXT NOT NULL,observed_at TEXT NOT NULL,state TEXT NOT NULL,data TEXT NOT NULL);
CREATE TABLE bambu_events(id INTEGER PRIMARY KEY AUTOINCREMENT,job_key TEXT NOT NULL,event_type TEXT NOT NULL,occurred_at TEXT NOT NULL,data TEXT NOT NULL);
CREATE TABLE bambu_jobs(job_key TEXT PRIMARY KEY,name TEXT NOT NULL,task_id TEXT,subtask_id TEXT,file TEXT,started_at TEXT,finished_at TEXT,outcome TEXT,duration_seconds INTEGER,initial_remaining_minutes INTEGER,weight_grams REAL,data TEXT NOT NULL DEFAULT '{}');
CREATE TABLE bambu_maintenance(task TEXT PRIMARY KEY,completed_at TEXT NOT NULL,note TEXT NOT NULL DEFAULT '');`)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			history, err := OpenHistory(ctx, path)
			if err == nil {
				err = history.Close()
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent open: %v", err)
		}
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count, distinct int
	if err := db.QueryRow(`SELECT COUNT(*),COUNT(DISTINCT version) FROM bambu_history_migrations`).Scan(&count, &distinct); err != nil {
		t.Fatal(err)
	}
	if count != BambuHistorySchemaVersion || distinct != BambuHistorySchemaVersion {
		t.Fatalf("migration stamps count=%d distinct=%d", count, distinct)
	}
}

func TestMigrationRechecksFutureVersionAfterWaitingForLock(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "future-during-lock.db")
	history, err := OpenHistory(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.db.ExecContext(ctx, `DROP INDEX idx_bambu_snapshots_time`); err != nil {
		t.Fatal(err)
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}
	peer, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	peerConn, err := peer.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer peerConn.Close()
	if _, err := peerConn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	lockBusy := make(chan struct{})
	var busyOnce sync.Once
	openCtx := context.WithValue(ctx, historyBusyTimeoutKey{}, 1)
	openCtx = context.WithValue(openCtx, historyBusyObserverKey{}, func(label string) {
		if label == "begin Bambu history migration" {
			busyOnce.Do(func() { close(lockBusy) })
		}
	})
	result := make(chan error, 1)
	go func() {
		opened, err := OpenHistory(openCtx, path)
		if err == nil {
			err = opened.Close()
		}
		result <- err
	}()
	select {
	case <-lockBusy:
	case err := <-result:
		t.Fatalf("OpenHistory returned without observing migration-lock contention: %v", err)
	case <-time.After(6 * time.Second):
		t.Fatal("OpenHistory did not attempt the blocked migration lock")
	}
	futureVersion := BambuHistorySchemaVersion + 1
	if _, err := peerConn.ExecContext(ctx, `INSERT INTO bambu_history_migrations(version,applied_at) VALUES(?,'future')`, futureVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := peerConn.ExecContext(ctx, `COMMIT`); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "newer than supported") {
			t.Fatalf("migration result = %v, want future-version rejection", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("migration did not finish after peer released lock")
	}
	var recreatedIndex, futureVersionCount int
	if err := peer.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_bambu_snapshots_time'`).Scan(&recreatedIndex); err != nil {
		t.Fatal(err)
	}
	if err := peer.QueryRowContext(ctx, `SELECT COUNT(*) FROM bambu_history_migrations WHERE version=?`, futureVersion).Scan(&futureVersionCount); err != nil {
		t.Fatal(err)
	}
	if recreatedIndex != 0 || futureVersionCount != 1 {
		t.Fatalf("future-schema rejection recreated_index=%d future_version=%d", recreatedIndex, futureVersionCount)
	}
}

func TestClaimLegacyPrinterKeyMergesCollidingTimeline(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "claim-collision.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	serial := "PRINTERSERIAL007"
	canonical := PrinterKey(serial)
	legacy := LegacyPrinterKeys(serial)[0]
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	recordTimeline := func(printerKey string, start, finish time.Time, outcome string) {
		snapshot := Snapshot{ObservedAt: start, PrinterKey: printerKey, State: "RUNNING", TaskID: "shared-task", JobName: "shared", Raw: map[string]any{}}
		if err := history.RecordSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
		if err := history.RecordEvent(ctx, Event{Kind: "started", Snapshot: snapshot}); err != nil {
			t.Fatal(err)
		}
		snapshot.ObservedAt = finish
		if err := history.RecordEvent(ctx, Event{Kind: outcome, Snapshot: snapshot}); err != nil {
			t.Fatal(err)
		}
	}
	recordTimeline(legacy, base.Add(time.Hour), base.Add(5*time.Hour), "failed")
	recordTimeline(canonical, base.Add(2*time.Hour), base.Add(4*time.Hour), "finished")
	if err := history.ClaimLegacyPrinterKeys(ctx, canonical, legacy); err != nil {
		t.Fatal(err)
	}
	jobs, err := history.JobsForPrinter(ctx, time.Time{}, canonical, 10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("merged jobs = %#v err=%v", jobs, err)
	}
	job := jobs[0]
	if job.StartedAt == nil || !job.StartedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("merged start = %v", job.StartedAt)
	}
	if job.FinishedAt == nil || !job.FinishedAt.Equal(base.Add(5*time.Hour)) || job.Outcome != "failed" {
		t.Fatalf("merged terminal = %v %q", job.FinishedAt, job.Outcome)
	}
	if job.DurationSeconds == nil || *job.DurationSeconds != int64((4*time.Hour)/time.Second) {
		t.Fatalf("merged duration = %v", job.DurationSeconds)
	}
	var sortStarted string
	if err := history.db.QueryRowContext(ctx, `SELECT sort_started_at FROM bambu_jobs WHERE job_key=?`, canonical+"/shared-task").Scan(&sortStarted); err != nil {
		t.Fatal(err)
	}
	if sortStarted != base.Add(time.Hour).Format(time.RFC3339Nano) {
		t.Fatalf("merged sort start = %q", sortStarted)
	}
	events, err := history.Events(ctx, canonical+"/shared-task", 10)
	if err != nil || len(events) != 4 {
		t.Fatalf("merged events = %#v err=%v", events, err)
	}
}

func TestScopedHistoryIndexesAreUsable(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "indexes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	queries := []struct{ sql, index string }{
		{`EXPLAIN QUERY PLAN SELECT data FROM bambu_snapshots WHERE printer_key=? AND observed_at>=? ORDER BY observed_at`, "idx_bambu_snapshots_printer_time"},
		{`EXPLAIN QUERY PLAN SELECT job_key FROM bambu_jobs WHERE sort_started_at>=? AND printer_key=? ORDER BY sort_started_at DESC,job_key DESC`, "idx_bambu_jobs_printer_started"},
	}
	for _, item := range queries {
		rows, err := history.db.QueryContext(ctx, item.sql, time.Time{}.Format(time.RFC3339Nano), "printer-key")
		if err != nil {
			t.Fatal(err)
		}
		detail := ""
		for rows.Next() {
			var id, parent, unused int
			var line string
			if err := rows.Scan(&id, &parent, &unused, &line); err != nil {
				t.Fatal(err)
			}
			detail += line
		}
		_ = rows.Close()
		if !strings.Contains(detail, item.index) || strings.Contains(detail, "TEMP B-TREE") {
			t.Fatalf("query plan %q does not use %s", detail, item.index)
		}
	}
}

func TestStableTaskIdentityMigrationMergesStaleSubtaskJob(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "task-identity.db")
	history, err := OpenHistory(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	printerKey := PrinterKey("PRINTERSERIAL009")
	canonical := printerKey + "/231"
	alias := printerKey + "/1082278740"
	if _, err := history.db.ExecContext(ctx, `DELETE FROM bambu_history_migrations WHERE version>=3`); err != nil {
		t.Fatal(err)
	}
	if _, err := history.db.ExecContext(ctx, `INSERT INTO bambu_jobs(job_key,printer_key,name,task_id,subtask_id,file,sort_started_at,initial_remaining_minutes,data) VALUES(?,?,?,?,?,?,?,?,?)`,
		canonical, printerKey, "3D Benchy", "231", "", "plate_1.gcode", "2026-07-11T17:19:00Z", 12, `{"task_id":"231","plate_number":1}`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := history.db.ExecContext(ctx, `INSERT INTO bambu_jobs(job_key,printer_key,name,task_id,subtask_id,file,started_at,sort_started_at,finished_at,outcome,duration_seconds,initial_remaining_minutes,weight_grams,data) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		alias, printerKey, "3D Benchy", "231", "1082278740", "plate_1.gcode", "2026-07-11T17:06:29Z", "2026-07-11T17:06:29Z", "2026-07-11T17:32:15Z", "finished", 1546, 462, 449.51, `{"task_id":"231","subtask_id":"1082278740","plate_number":2,"raw":{"plate_idx":1}}`); err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []string{"started", "finished"} {
		if _, err := history.db.ExecContext(ctx, `INSERT INTO bambu_events(printer_key,job_key,event_type,occurred_at,data) VALUES(?,?,?,?,?)`, printerKey, alias, eventType, "2026-07-11T17:06:29Z", `{"task_id":"231","subtask_id":"1082278740","plate_number":2,"raw":{"plate_idx":1}}`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := history.db.ExecContext(ctx, `INSERT INTO bambu_snapshots(printer_key,job_key,observed_at,state,data) VALUES(?,?,?,?,?)`, printerKey, alias, "2026-07-11T13:01:21Z", "FINISH", `{"task_id":"1082278740","subtask_id":"1082278740","job_name":"Previous print","gcode_file":"previous.gcode"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := history.db.ExecContext(ctx, `INSERT INTO bambu_events(printer_key,job_key,event_type,occurred_at,data) VALUES(?,?,?,?,?)`, printerKey, alias, "finished", "2026-07-11T13:01:21Z", `{"task_id":"1082278740","subtask_id":"1082278740","job_name":"Previous print","gcode_file":"previous.gcode"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := history.db.ExecContext(ctx, `INSERT INTO bambu_jobs(job_key,printer_key,name,task_id,subtask_id,file,sort_started_at,initial_remaining_minutes,data) VALUES(?,?,?,?,?,?,?,?,?)`, "1082278740", "", "Previous print", "1082278740", "1082278740", "previous.gcode", "2026-07-11T12:00:00Z", 120, `{}`); err != nil {
		t.Fatal(err)
	}
	if _, err := history.db.ExecContext(ctx, `INSERT INTO bambu_snapshots(printer_key,job_key,observed_at,state,data) VALUES(?,?,?,?,?)`, "", "1082278740", "2026-07-11T12:00:00Z", "RUNNING", `{"task_id":"1082278740","subtask_id":"1082278740","job_name":"Previous print","gcode_file":"previous.gcode"}`); err != nil {
		t.Fatal(err)
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}

	history, err = OpenHistory(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	jobs, err := history.JobsForPrinter(ctx, time.Time{}, printerKey, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("jobs after migration = %#v", jobs)
	}
	var job StoredJob
	for _, candidate := range jobs {
		if candidate.JobKey == canonical {
			job = candidate
		}
	}
	if job.JobKey != canonical || job.Outcome != "finished" || job.StartedAt == nil || job.FinishedAt == nil || job.InitialRemainingMinutes == nil || *job.InitialRemainingMinutes != 12 || job.WeightGrams != nil {
		t.Fatalf("merged job = %#v", job)
	}
	var eventKey string
	var plate int
	if err := history.db.QueryRowContext(ctx, `SELECT job_key,CAST(json_extract(data,'$.plate_number') AS INTEGER) FROM bambu_events WHERE json_extract(data,'$.task_id')='231' LIMIT 1`).Scan(&eventKey, &plate); err != nil {
		t.Fatal(err)
	}
	if eventKey != canonical || plate != 1 {
		t.Fatalf("migrated event key=%q plate=%d", eventKey, plate)
	}
	var previousJobs int
	if err := history.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bambu_jobs WHERE job_key=? AND task_id='1082278740'`, alias).Scan(&previousJobs); err != nil {
		t.Fatal(err)
	}
	if previousJobs != 1 {
		t.Fatalf("previous task jobs = %d, want 1", previousJobs)
	}
	var unscopedJobs int
	if err := history.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bambu_jobs WHERE printer_key='' AND task_id='1082278740'`).Scan(&unscopedJobs); err != nil {
		t.Fatal(err)
	}
	if unscopedJobs != 0 {
		rows, _ := history.db.QueryContext(ctx, `SELECT job_key,printer_key,task_id FROM bambu_jobs ORDER BY job_key`)
		var found []string
		for rows != nil && rows.Next() {
			var key, scopedPrinter, task string
			_ = rows.Scan(&key, &scopedPrinter, &task)
			found = append(found, fmt.Sprintf("%s|%s|%s", key, scopedPrinter, task))
		}
		if rows != nil {
			_ = rows.Close()
		}
		t.Fatalf("unscoped task jobs = %d, want 0; jobs=%v", unscopedJobs, found)
	}
}

func TestJobKeyPrefersStableTaskID(t *testing.T) {
	snapshot := Snapshot{PrinterKey: "printer-key", TaskID: "231", SubtaskID: "stale-subtask"}
	if got := JobKey(snapshot); got != "printer-key/231" {
		t.Fatalf("job key = %q", got)
	}
}

func TestInitialRemainingWaitsForValidRunningTelemetry(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "initial-remaining.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	staleRemaining, validRemaining := 462, 12
	percent := 1
	base := Snapshot{ObservedAt: time.Now().UTC(), PrinterKey: PrinterKey("PRINTERSERIAL010"), TaskID: "231", JobName: "Benchy", GCodeFile: "plate_1.gcode", Raw: map[string]any{}}
	stale := base
	stale.State = "PREPARE"
	stale.RemainingMinutes = &staleRemaining
	if err := history.RecordSnapshot(ctx, stale); err != nil {
		t.Fatal(err)
	}
	valid := base
	valid.ObservedAt = valid.ObservedAt.Add(time.Minute)
	valid.State = "RUNNING"
	valid.Percent = &percent
	valid.RemainingMinutes = &validRemaining
	if err := history.RecordSnapshot(ctx, valid); err != nil {
		t.Fatal(err)
	}
	var got int
	if err := history.db.QueryRowContext(ctx, `SELECT initial_remaining_minutes FROM bambu_jobs WHERE job_key=?`, JobKey(valid)).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != validRemaining {
		t.Fatalf("initial remaining = %d, want %d", got, validRemaining)
	}
}

func TestRebuildDurationUsesElapsedWholeSeconds(t *testing.T) {
	ctx := context.Background()
	history, err := OpenHistory(ctx, filepath.Join(t.TempDir(), "subsecond-duration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	if _, err := history.db.ExecContext(ctx, `INSERT INTO bambu_jobs(job_key,printer_key,name,task_id,started_at,sort_started_at,finished_at,data) VALUES(?,?,?,?,?,?,?,?)`,
		"printer/task", "printer", "Benchy", "task", "2026-07-11T17:06:29.758111Z", "2026-07-11T17:06:29.758111Z", "2026-07-11T17:32:15.513246Z", `{}`); err != nil {
		t.Fatal(err)
	}
	if err := history.rebuildJobTerminals(ctx, history.db); err != nil {
		t.Fatal(err)
	}
	var duration int64
	if err := history.db.QueryRowContext(ctx, `SELECT duration_seconds FROM bambu_jobs WHERE job_key='printer/task'`).Scan(&duration); err != nil {
		t.Fatal(err)
	}
	if duration != 1545 {
		t.Fatalf("duration = %d, want 1545", duration)
	}
}
