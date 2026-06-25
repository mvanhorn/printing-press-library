package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/store"
	_ "modernc.org/sqlite"
)

func TestArchiveResetCommandWithoutForceRefusesAndDoesNotMutate(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	source := newCLIArchiveSourceFixture(t, 1, 2)
	archive, err := store.ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	if _, err := store.AccumulateFromSource(archive, source, time.Now()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"archive", "reset", "--json"})
		cmdErr = cmd.Execute()
	})
	if !errors.Is(cmdErr, ErrUsage) {
		t.Fatalf("archive reset err=%v, want ErrUsage", cmdErr)
	}
	if !strings.Contains(out, `"would_destroy": true`) || !strings.Contains(out, `"archive_visits": 2`) {
		t.Fatalf("reset output=%s, want guarded destruction plan", out)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("archive stat after refused reset: %v", err)
	}
	if got := countCLIArchiveRows(t, archive); got != 2 {
		t.Fatalf("archive rows after refused reset = %d, want 2", got)
	}
}

func TestArchiveEnableCommandIdempotent(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	source := newCLIArchiveSourceFixture(t, 1, 2)
	snapshot, err := store.SnapshotPath()
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	copyCLIArchiveFile(t, source, snapshot)
	for i := 0; i < 2; i++ {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"archive", "enable", "--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("archive enable %d: %v", i+1, err)
		}
	}
	archive, err := store.ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	if got := countCLIArchiveRows(t, archive); got != 2 {
		t.Fatalf("archive rows after enable twice = %d, want 2", got)
	}
	status, err := store.ReadArchiveStatus()
	if err != nil {
		t.Fatalf("ReadArchiveStatus: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("archive enabled = false, want true")
	}
}

func TestArchiveEnableAndClobberNoSnapshotReturnTypedError(t *testing.T) {
	for _, args := range [][]string{
		{"archive", "enable", "--json"},
		{"archive", "clobber", "--json"},
	} {
		t.Run(strings.Join(args[:2], " "), func(t *testing.T) {
			cache := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", cache)
			cmd := NewRootCmd()
			cmd.SetArgs(args)
			if err := cmd.Execute(); !errors.Is(err, ErrNoSnapshot) {
				t.Fatalf("%v err=%v, want ErrNoSnapshot", args, err)
			}
		})
	}
}

func TestArchiveLifecycleCommandSmoke(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	source := newCLIArchiveSourceFixture(t, 1, 2)
	snapshot, err := store.SnapshotPath()
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	copyCLIArchiveFile(t, source, snapshot)
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"archive", "enable", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("archive enable: %v", err)
	}
	out := captureStdout(t, func() {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"archive", "disable", "--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("archive disable: %v", err)
		}
	})
	if !strings.Contains(out, `"archive_enabled": false`) {
		t.Fatalf("disable output=%s, want archive_enabled false", out)
	}
	out = captureStdout(t, func() {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"archive", "clobber", "--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("archive clobber: %v", err)
		}
	})
	if !strings.Contains(out, `"new_archive_visits": 2`) {
		t.Fatalf("clobber output=%s, want new_archive_visits 2", out)
	}
	out = captureStdout(t, func() {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"archive", "vacuum", "--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("archive vacuum: %v", err)
		}
	})
	if !strings.Contains(out, `"size_before_bytes":`) || !strings.Contains(out, `"size_after_bytes":`) {
		t.Fatalf("vacuum output=%s, want size fields", out)
	}
}

func newCLIArchiveSourceFixture(t *testing.T, start, end int) string {
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
	);`)
	if err != nil {
		t.Fatalf("create fixture schema: %v", err)
	}
	for i := start; i <= end; i++ {
		visitTime := int64(13200000000000000 + i)
		if _, err := db.Exec(`INSERT INTO urls(id, url, title, visit_count, last_visit_time) VALUES(?,?,?,?,?)`, i, fmt.Sprintf("https://example.test/cmd-v%d", i), fmt.Sprintf("Visit %d", i), 1, visitTime); err != nil {
			t.Fatalf("insert url: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO visits(id, url, visit_time) VALUES(?,?,?)`, i, i, visitTime); err != nil {
			t.Fatalf("insert visit: %v", err)
		}
	}
	return path
}

func countCLIArchiveRows(t *testing.T, path string) int64 {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer db.Close()
	var n int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_archive`).Scan(&n); err != nil {
		t.Fatalf("count archive: %v", err)
	}
	return n
}

func copyCLIArchiveFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}
