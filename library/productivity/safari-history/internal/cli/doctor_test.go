package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/store"
)

func TestDoctorReportsQueryableCacheWhenSourceMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	snapshot, err := snapshotPath()
	if err != nil {
		t.Fatalf("snapshot path: %v", err)
	}
	writeRoutingSnapshotRows(t, snapshot, []routingVisit{
		{ID: 1, URL: "https://example.com/cached", Title: "Cached", Origin: 0, When: time.Now().UTC()},
	})

	rows := runRootJSONRows(t, "--json", "doctor")
	if len(rows) != 1 {
		t.Fatalf("doctor rows = %#v, want one row", rows)
	}
	row := rows[0]
	if row["source_db"] != "missing" {
		t.Fatalf("source_db = %#v, want missing", row["source_db"])
	}
	if row["cached_store"] != "queryable" {
		t.Fatalf("cached_store = %#v, want queryable; row=%#v", row["cached_store"], row)
	}
	if row["cached_store_source"] != "snapshot" {
		t.Fatalf("cached_store_source = %#v, want snapshot", row["cached_store_source"])
	}
	if row["healthy"] != false {
		t.Fatalf("healthy = %#v, want false because live refresh source is missing", row["healthy"])
	}
	note, _ := row["note"].(string)
	if !strings.Contains(note, "cached history is queryable offline") || !strings.Contains(note, "no sync required") {
		t.Fatalf("note = %q, want offline cached-read guidance", note)
	}
}

// TestDoctorReportsQueryableCacheFromArchiveEnabled covers the archive-enabled
// branch: snapshot is gone (no live Safari source, no snapshot visits), but the
// accumulating archive is enabled and has rows.  The doctor must report
// cached_store=="queryable", cached_store_source=="archive", and an offline note.
func TestDoctorReportsQueryableCacheFromArchiveEnabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	snapshot, err := snapshotPath()
	if err != nil {
		t.Fatalf("snapshot path: %v", err)
	}
	archivePath, err := store.ArchivePath()
	if err != nil {
		t.Fatalf("archive path: %v", err)
	}

	// Seed the snapshot with rows so AccumulateFromSource has something to copy
	// into the archive.
	writeRoutingSnapshotRows(t, snapshot, []routingVisit{
		{ID: 1, URL: "https://archive-doctor.example/page", Title: "ArchiveDoc", Origin: 0, When: time.Now().UTC().Add(-time.Hour)},
	})
	if err := store.EnableArchiveFromSource(archivePath, snapshot, time.Now().UTC()); err != nil {
		t.Fatalf("enable archive: %v", err)
	}

	// Remove the snapshot so the doctor sees no live snapshot visits — archive
	// must be the sole queryable store.
	if err := os.Remove(snapshot); err != nil {
		t.Fatalf("remove snapshot: %v", err)
	}

	rows := runRootJSONRows(t, "--json", "doctor")
	if len(rows) != 1 {
		t.Fatalf("doctor rows = %#v, want one row", rows)
	}
	row := rows[0]
	if row["cached_store"] != "queryable" {
		t.Fatalf("cached_store = %#v, want queryable (archive enabled with rows); row=%#v", row["cached_store"], row)
	}
	if row["cached_store_source"] != "archive" {
		t.Fatalf("cached_store_source = %#v, want archive", row["cached_store_source"])
	}
	note, _ := row["note"].(string)
	if note == "" {
		t.Fatalf("note = %q, want offline cached-read guidance note to be present", note)
	}
}

func TestSyncSourceMissingErrorSteersToCachedReads(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"sync"})
	err := cmd.Execute()
	if !errors.Is(err, ErrSourceDBMissing) {
		t.Fatalf("sync err = %v, want ErrSourceDBMissing", err)
	}
	if got := ExitCodeForError(err); got != ExitSourceDBMissing {
		t.Fatalf("sync exit code = %d, want %d", got, ExitSourceDBMissing)
	}
	msg := err.Error()
	for _, want := range []string{
		"Cannot refresh live Safari history right now",
		"does not mean there is no data",
		"query it directly with search, sql, domains, list, or report",
		"Full Disk Access",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("sync error missing %q:\n%s", want, msg)
		}
	}
}
