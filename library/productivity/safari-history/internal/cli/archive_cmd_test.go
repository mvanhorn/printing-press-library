package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/store"
	_ "modernc.org/sqlite"
)

func writeArchiveCommandSnapshot(t *testing.T, path string, rows []struct {
	id    int
	url   string
	title string
	when  float64
}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	_ = os.Remove(path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE history_items (id INTEGER PRIMARY KEY, url TEXT, domain_expansion TEXT, visit_count INTEGER)`,
		`CREATE TABLE history_visits (id INTEGER PRIMARY KEY, history_item INTEGER, visit_time REAL, title TEXT, origin INTEGER, redirect_source INTEGER)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO history_items(id, url, domain_expansion, visit_count) VALUES(?, ?, ?, 1)`, row.id, row.url, row.url); err != nil {
			t.Fatalf("insert item: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO history_visits(id, history_item, visit_time, title, origin, redirect_source) VALUES(?, ?, ?, ?, 0, 0)`, row.id, row.id, row.when, row.title); err != nil {
			t.Fatalf("insert visit: %v", err)
		}
	}
}

func runArchiveCmdJSON(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	runErr := cmd.Execute()
	_ = w.Close()
	os.Stdout = orig
	data, _ := io.ReadAll(r)
	return string(data), runErr
}

func decodeObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatalf("decode object %q: %v", raw, err)
	}
	return obj
}

func decodeRows(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode rows %q: %v", raw, err)
	}
	return rows
}

// TestArchiveStatusCachedStoreFields asserts the two steering branches of
// `archive status`: queryable (enabled + rows) and present_disabled (disabled +
// rows).  Both branches must set cached_store correctly and emit a non-empty
// note so agents know whether the archive is usable offline.
func TestArchiveStatusCachedStoreFields(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	snapshot, err := snapshotPath()
	if err != nil {
		t.Fatalf("snapshot path: %v", err)
	}

	writeArchiveCommandSnapshot(t, snapshot, []struct {
		id    int
		url   string
		title string
		when  float64
	}{
		{1, "https://steer-test.example/a", "SteerA", 101},
		{2, "https://steer-test.example/b", "SteerB", 202},
	})

	// Enable archive — seeds rows from the snapshot.
	raw, err := runArchiveCmdJSON(t, "--json", "archive", "enable")
	if err != nil {
		t.Fatalf("enable: %v\n%s", err, raw)
	}

	// Branch 1: enabled + rows → cached_store="queryable", note non-empty.
	raw, err = runArchiveCmdJSON(t, "--json", "archive", "status")
	if err != nil {
		t.Fatalf("status after enable: %v\n%s", err, raw)
	}
	status := decodeObject(t, raw)
	if status["cached_store"] != "queryable" {
		t.Fatalf("archive status cached_store = %#v, want queryable (enabled with rows); full=%#v", status["cached_store"], status)
	}
	note, _ := status["note"].(string)
	if note == "" {
		t.Fatalf("archive status note = %q, want non-empty offline guidance when queryable", note)
	}

	// Disable archive — keeps the rows but stops accumulation.
	raw, err = runArchiveCmdJSON(t, "--json", "archive", "disable")
	if err != nil {
		t.Fatalf("disable: %v\n%s", err, raw)
	}

	// Branch 2: disabled + rows → cached_store="present_disabled", note non-empty.
	raw, err = runArchiveCmdJSON(t, "--json", "archive", "status")
	if err != nil {
		t.Fatalf("status after disable: %v\n%s", err, raw)
	}
	status = decodeObject(t, raw)
	if status["cached_store"] != "present_disabled" {
		t.Fatalf("archive status cached_store = %#v, want present_disabled (disabled with rows); full=%#v", status["cached_store"], status)
	}
	disabledNote, _ := status["note"].(string)
	if disabledNote == "" {
		t.Fatalf("archive status note = %q, want non-empty guidance when present_disabled", disabledNote)
	}
}

func TestArchiveLifecycleCommands(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	snapshot, err := snapshotPath()
	if err != nil {
		t.Fatalf("snapshot path: %v", err)
	}
	archivePath, err := store.ArchivePath()
	if err != nil {
		t.Fatalf("archive path: %v", err)
	}
	writeArchiveCommandSnapshot(t, snapshot, []struct {
		id    int
		url   string
		title string
		when  float64
	}{
		{1, "https://example.com/a", "Example", 101},
		{2, "https://openai.com/docs", "Docs", 202},
	})

	raw, err := runArchiveCmdJSON(t, "--json", "archive", "reset", "--purge")
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("reset without --force err = %v, want ErrUsage; out=%s", err, raw)
	}
	if got := ExitCodeForError(err); got != ExitUsage {
		t.Fatalf("reset without --force exit = %d, want %d", got, ExitUsage)
	}
	plan := decodeRows(t, raw)
	if len(plan) != 1 || plan[0]["requires_force"] != true || plan[0]["path"] != archivePath {
		t.Fatalf("reset plan = %#v, want guarded plan for %s", plan, archivePath)
	}

	raw, err = runArchiveCmdJSON(t, "--json", "archive", "enable")
	if err != nil {
		t.Fatalf("enable: %v\n%s", err, raw)
	}
	enableRows := decodeRows(t, raw)
	if len(enableRows) != 1 || enableRows[0]["enabled"] != true || enableRows[0]["visit_count"] != float64(2) {
		t.Fatalf("enable rows = %#v, want enabled with 2 visits", enableRows)
	}

	raw, err = runArchiveCmdJSON(t, "--json", "archive", "status")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, raw)
	}
	status := decodeObject(t, raw)
	if status["enabled"] != true || status["url_count"] != float64(2) || status["visit_count"] != float64(2) || status["path"] != archivePath {
		t.Fatalf("status = %#v, want enabled counts/path", status)
	}

	raw, err = runArchiveCmdJSON(t, "--json", "archive", "vacuum")
	if err != nil {
		t.Fatalf("vacuum: %v\n%s", err, raw)
	}
	vacuumRows := decodeRows(t, raw)
	if len(vacuumRows) != 1 || vacuumRows[0]["path"] != archivePath {
		t.Fatalf("vacuum rows = %#v, want one row for %s", vacuumRows, archivePath)
	}
	if _, ok := vacuumRows[0]["size_bytes_before"]; !ok {
		t.Fatalf("vacuum rows missing size_bytes_before: %#v", vacuumRows)
	}
	if _, ok := vacuumRows[0]["size_bytes_after"]; !ok {
		t.Fatalf("vacuum rows missing size_bytes_after: %#v", vacuumRows)
	}

	raw, err = runArchiveCmdJSON(t, "--json", "archive", "disable")
	if err != nil {
		t.Fatalf("disable: %v\n%s", err, raw)
	}
	status = decodeObject(t, raw)
	if status["enabled"] != false || status["visit_count"] != float64(2) {
		t.Fatalf("disable status = %#v, want disabled with data kept", status)
	}

	writeArchiveCommandSnapshot(t, snapshot, []struct {
		id    int
		url   string
		title string
		when  float64
	}{
		{1, "https://refreshed.example/new", "Refreshed", 303},
	})
	raw, err = runArchiveCmdJSON(t, "--json", "archive", "clobber")
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("clobber without --force err = %v, want ErrUsage; out=%s", err, raw)
	}
	if got := ExitCodeForError(err); got != ExitUsage {
		t.Fatalf("clobber without --force exit = %d, want %d", got, ExitUsage)
	}
	clobberPlan := decodeRows(t, raw)
	if len(clobberPlan) != 1 || clobberPlan[0]["requires_force"] != true || clobberPlan[0]["path"] != archivePath {
		t.Fatalf("clobber plan = %#v, want guarded plan for %s", clobberPlan, archivePath)
	}

	raw, err = runArchiveCmdJSON(t, "--json", "archive", "clobber", "--force")
	if err != nil {
		t.Fatalf("clobber --force: %v\n%s", err, raw)
	}
	status = decodeObject(t, raw)
	if status["enabled"] != true || status["url_count"] != float64(1) || status["visit_count"] != float64(1) {
		t.Fatalf("clobber --force status = %#v, want refreshed enabled archive", status)
	}

	raw, err = runArchiveCmdJSON(t, "--json", "archive", "reset", "--force", "--purge")
	if err != nil {
		t.Fatalf("reset force purge: %v\n%s", err, raw)
	}
	resetRows := decodeRows(t, raw)
	if len(resetRows) != 1 || resetRows[0]["reset"] != true || resetRows[0]["purged"] != true || resetRows[0]["file_exists"] != false {
		t.Fatalf("reset rows = %#v, want purged archive", resetRows)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive exists after purge or stat err = %v", err)
	}
}

func TestArchiveVacuumAbsentReturnsError(t *testing.T) {
	// VacuumArchive must return an archive-missing error (exit 3) and must NOT
	// create archive.db when it does not exist yet.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	archivePath, err := store.ArchivePath()
	if err != nil {
		t.Fatalf("archive path: %v", err)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("precondition: archive.db already exists at %s", archivePath)
	}
	_, runErr := runArchiveCmdJSON(t, "--json", "archive", "vacuum")
	if runErr == nil {
		t.Fatal("archive vacuum on absent archive: got nil error, want non-nil")
	}
	if !errors.Is(runErr, ErrNoSnapshot) {
		t.Fatalf("archive vacuum on absent archive: got %v, want ErrNoSnapshot wrapped error", runErr)
	}
	if got := ExitCodeForError(runErr); got != ExitNoSnapshot {
		t.Fatalf("archive vacuum on absent archive: exit = %d, want %d (ExitNoSnapshot)", got, ExitNoSnapshot)
	}
	// The file must NOT have been created.
	if _, statErr := os.Stat(archivePath); !os.IsNotExist(statErr) {
		t.Fatal("archive vacuum on absent archive: archive.db was created (should not be)")
	}
}

func TestArchiveClobberForceGuard(t *testing.T) {
	// Without --force, clobber must return ErrUsage and leave accumulated rows intact.
	// With --force, it must proceed and replace the archive.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	snapshot, err := snapshotPath()
	if err != nil {
		t.Fatalf("snapshot path: %v", err)
	}
	archivePath, err := store.ArchivePath()
	if err != nil {
		t.Fatalf("archive path: %v", err)
	}

	// Seed an archive with 2 rows via enable.
	writeArchiveCommandSnapshot(t, snapshot, []struct {
		id    int
		url   string
		title string
		when  float64
	}{
		{1, "https://alpha.test/page", "Alpha", 101},
		{2, "https://beta.test/page", "Beta", 202},
	})
	if _, err := runArchiveCmdJSON(t, "--json", "archive", "enable"); err != nil {
		t.Fatalf("enable: %v", err)
	}

	// Now change the snapshot to a single new row.
	writeArchiveCommandSnapshot(t, snapshot, []struct {
		id    int
		url   string
		title string
		when  float64
	}{
		{3, "https://gamma.test/page", "Gamma", 303},
	})

	// Without --force: must return ErrUsage and the archive rows must be untouched.
	raw, clobberErr := runArchiveCmdJSON(t, "--json", "archive", "clobber")
	if !errors.Is(clobberErr, ErrUsage) {
		t.Fatalf("clobber without --force err = %v, want ErrUsage; out=%s", clobberErr, raw)
	}
	if got := ExitCodeForError(clobberErr); got != ExitUsage {
		t.Fatalf("clobber without --force exit = %d, want %d", got, ExitUsage)
	}
	plan := decodeRows(t, raw)
	if len(plan) != 1 || plan[0]["requires_force"] != true {
		t.Fatalf("clobber plan = %#v, want requires_force=true", plan)
	}
	// The original 2 accumulated rows must still be present.
	raw, err = runArchiveCmdJSON(t, "--json", "archive", "status")
	if err != nil {
		t.Fatalf("status after blocked clobber: %v\n%s", err, raw)
	}
	st := decodeObject(t, raw)
	if st["visit_count"] != float64(2) {
		t.Fatalf("after blocked clobber visit_count = %v, want 2 (rows must be intact)", st["visit_count"])
	}

	// With --force: archive must be replaced with the single new row.
	raw, err = runArchiveCmdJSON(t, "--json", "archive", "clobber", "--force")
	if err != nil {
		t.Fatalf("clobber --force: %v\n%s", err, raw)
	}
	st = decodeObject(t, raw)
	if st["enabled"] != true || st["url_count"] != float64(1) || st["visit_count"] != float64(1) {
		t.Fatalf("clobber --force status = %#v, want enabled with 1 url/1 visit", st)
	}
	_ = archivePath
}
