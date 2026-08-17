package cli

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/store"
	_ "modernc.org/sqlite"
)

func TestRichDataCommandReadsSnapshotWhenArchiveEnabled(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	snapshot, err := store.SnapshotPath()
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	db, err := sql.Open("sqlite", snapshot)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE downloads (
		target_path TEXT,
		received_bytes INTEGER,
		mime_type TEXT,
		original_mime_type TEXT,
		site_url TEXT,
		referrer TEXT,
		start_time INTEGER,
		state INTEGER
	)`); err != nil {
		t.Fatalf("create downloads: %v", err)
	}
	when := (time.Now().UTC().Unix() + 11644473600) * 1_000_000
	if _, err := db.Exec(`INSERT INTO downloads(target_path, received_bytes, mime_type, site_url, start_time, state) VALUES(?,?,?,?,?,?)`,
		filepath.Join(t.TempDir(), "snapshot-file.txt"), 42, "text/plain", "https://example.test/file", when, 1); err != nil {
		t.Fatalf("insert download: %v", err)
	}
	db.Close()

	archive, err := store.ArchivePath()
	if err != nil {
		t.Fatalf("ArchivePath: %v", err)
	}
	adb, err := sql.Open("sqlite", archive)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	if err := store.InitArchiveSchema(adb); err != nil {
		t.Fatalf("InitArchiveSchema: %v", err)
	}
	if _, err := adb.Exec(`UPDATE meta_pp SET archive_enabled=1`); err != nil {
		t.Fatalf("enable archive: %v", err)
	}
	adb.Close()

	out := captureStdout(t, func() {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"downloads", "--json", "--since", "3650d"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("downloads command: %v", err)
		}
	})
	if !strings.Contains(out, "snapshot-file.txt") {
		t.Fatalf("downloads output = %s, want snapshot download row", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(data)
}
