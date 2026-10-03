// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
)

func tripCacheRecord(t *testing.T, name string) trip.Record {
	t.Helper()
	raw, err := os.ReadFile("../trip/testdata/spots-8220.html")
	if err != nil {
		t.Fatal(err)
	}
	record, err := trip.ParseDetail(raw, "spots/8220", time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	record.Name = name
	return record
}

func seedTripCache(t *testing.T, path string, record trip.Record) {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	err = db.SaveTripRecords(context.Background(), []trip.Record{record})
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("seed: %v close: %v", err, closeErr)
	}
}

func TestTripLazyReaderUsesPinnedSnapshotRows(t *testing.T) {
	testenv.Isolate(t)
	selected := defaultDBPath("iko-yo-pp-cli")
	original := tripCacheRecord(t, "Synthetic verified original")
	seedTripCache(t, selected, original)
	substitute := filepath.Join(t.TempDir(), "substitute.db")
	seedTripCache(t, substitute, tripCacheRecord(t, "Synthetic substituted wrong rows"))
	db, guard, err := tripOpenStoreForRead(context.Background())
	if err != nil || db == nil {
		t.Fatalf("open snapshot: %v", err)
	}
	if db.DB().Stats().OpenConnections != 0 {
		t.Fatal("test must exercise SQLite's lazy first connection")
	}
	backup := selected + ".verified"
	if err := os.Rename(selected, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(substitute, selected); err != nil {
		t.Fatal(err)
	}
	record, queryErr := db.TripRecord(context.Background(), original.Ref)
	if err := os.Rename(selected, substitute); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, selected); err != nil {
		t.Fatal(err)
	}
	if err := tripFinishStoreRead(db, guard, queryErr); err != nil {
		t.Fatal(err)
	}
	if record.Name != original.Name {
		t.Fatalf("lazy query read substituted rows: %q", record.Name)
	}
}

func TestTripSnapshotWritesUseCanonicalSymlinkWithOpenWriter(t *testing.T) {
	testenv.Isolate(t)
	canonical := filepath.Join(t.TempDir(), "canonical.db")
	seedTripCache(t, canonical, tripCacheRecord(t, "Synthetic baseline"))
	writer, err := store.Open(canonical)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	selected := defaultDBPath("iko-yo-pp-cli")
	if err := os.MkdirAll(filepath.Dir(selected), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(canonical, selected); err != nil {
		t.Skip(err)
	}
	fresh := tripCacheRecord(t, "Synthetic canonical save")
	if err := tripSave(context.Background(), &rootFlags{}, []trip.Record{fresh}); err != nil {
		t.Fatal(err)
	}
	held, err := writer.TripRecord(context.Background(), fresh.Ref)
	if err != nil || held.Name != fresh.Name {
		t.Fatalf("open writer did not see canonical save: %+v %v", held, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	saved, err := tripLocalRecord(context.Background(), fresh.Ref)
	if err != nil || saved.Name != fresh.Name {
		t.Fatalf("writer close lost newer observation: %+v %v", saved, err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if info, err := os.Stat(selected + suffix); err == nil && info.Size() > 0 {
			t.Fatalf("writer used alias sidecars %s", suffix)
		}
	}
}

type tripCacheTransport func(*http.Request) (*http.Response, error)

func (f tripCacheTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func mockTripLiveDetail(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile("../trip/testdata/spots-8220.html")
	if err != nil {
		t.Fatal(err)
	}
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = tripCacheTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != trip.BaseURL+"/spots/8220" {
			t.Fatalf("unexpected source request: %s", request.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(bytes.NewReader(raw)), Request: request}, nil
	})
}

func TestTripLiveInspectRejectsHardLinkBeforeSnapshotWrite(t *testing.T) {
	testenv.Isolate(t)
	mockTripLiveDetail(t)
	canonical := filepath.Join(t.TempDir(), "canonical.db")
	seedTripCache(t, canonical, tripCacheRecord(t, "Synthetic checkpoint baseline"))
	writer, err := store.Open(canonical)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	fresh := tripCacheRecord(t, "Synthetic committed canonical writer")
	if err := writer.SaveTripRecords(context.Background(), []trip.Record{fresh}); err != nil {
		t.Fatal(err)
	}
	selected := defaultDBPath("iko-yo-pp-cli")
	if err := os.MkdirAll(filepath.Dir(selected), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(canonical, selected); err != nil {
		t.Skip(err)
	}
	output, err := tripRun(t, "trip", "inspect", "spots/8220", "--data-source", "live", "--json")
	if err == nil || len(output) != 0 || !strings.Contains(err.Error(), "hard link") {
		t.Fatalf("unsafe live save reported success: bytes=%d error=%v", len(output), err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(selected); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenReadOnly(canonical)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	saved, err := db.TripRecord(context.Background(), fresh.Ref)
	if err != nil || saved.Name != fresh.Name {
		t.Fatalf("rejected alias damaged canonical writer observation: %+v %v", saved, err)
	}
}

func TestTripURIWriterTrapDoesNotTouchAnotherCache(t *testing.T) {
	for _, tc := range []struct{ name, token string }{{"question", "?"}, {"fragment", "#"}, {"percent", "%"}} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			restore, err := cliutil.SetHomeOverride("")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(restore)
			mockTripLiveDetail(t)
			root := t.TempDir()
			danger := filepath.Join(root, "new"+tc.token+"home")
			// A second valid cache must remain unchanged if a URI is truncated.
			sentinel := filepath.Join(root, "new")
			seedTripCache(t, sentinel, tripCacheRecord(t, "Synthetic unrelated sentinel"))
			output, err := tripRun(t, "trip", "inspect", "spots/8220", "--data-source", "live", "--home", danger, "--json")
			if err == nil || len(output) != 0 || !strings.Contains(err.Error(), "URI") {
				t.Fatalf("unsafe home reached writer: bytes=%d error=%v", len(output), err)
			}
			if _, err := os.Stat(danger); !os.IsNotExist(err) {
				walkErr := filepath.WalkDir(danger, func(path string, entry os.DirEntry, err error) error {
					if err == nil && entry.Name() == "data.db" {
						t.Errorf("unsafe home created a database")
					}
					return err
				})
				if walkErr != nil {
					t.Fatal(walkErr)
				}
			}
			db, err := store.OpenReadOnly(sentinel)
			if err != nil {
				t.Fatal(err)
			}
			got, readErr := db.TripRecord(context.Background(), "spots/8220")
			closeErr := db.Close()
			if readErr != nil || closeErr != nil || got.Name != "Synthetic unrelated sentinel" {
				t.Fatalf("writer trap modified other cache: %+v %v %v", got, readErr, closeErr)
			}
		})
	}
}

func TestTripSQLiteRuntimePatched(t *testing.T) {
	testenv.Isolate(t)
	db, err := store.Open(defaultDBPath("iko-yo-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version string
	if err := db.DB().QueryRow("SELECT sqlite_version()").Scan(&version); err != nil || version != "3.51.3" {
		t.Fatalf("SQLite runtime=%q error=%v", version, err)
	}
}

func TestTripSavedCommandsRejectActiveCommittedWAL(t *testing.T) {
	testenv.Isolate(t)
	path := defaultDBPath("iko-yo-pp-cli")
	raw, err := os.ReadFile("../trip/testdata/spots-8220.html")
	if err != nil {
		t.Fatal(err)
	}
	record, err := trip.ParseDetail(raw, "spots/8220", time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = baseline.SaveTripRecords(context.Background(), []trip.Record{record}); err != nil {
		t.Fatal(err)
	}
	if err = baseline.Close(); err != nil {
		t.Fatal(err)
	}
	writer, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err = writer.DB().Exec("PRAGMA wal_autocheckpoint(0)"); err != nil {
		t.Fatal(err)
	}
	record.Name = "Synthetic committed WAL update"
	record.ObservedAt = "2026-10-03T01:00:00Z"
	if err = writer.SaveTripRecords(context.Background(), []trip.Record{record}); err != nil {
		t.Fatal(err)
	}
	wal, err := os.Stat(path + "-wal")
	if err != nil || wal.Size() == 0 {
		t.Fatalf("committed open-writer WAL missing: %v", err)
	}
	for _, args := range [][]string{
		{"trip", "cached", "--kind", "spots", "--json"},
		{"trip", "inspect", "spots/8220", "--data-source", "local", "--json"},
		{"trip", "compare", "spots/8220", "--data-source", "local", "--json"},
	} {
		t.Run(args[1], func(t *testing.T) {
			output, err := tripRun(t, args...)
			if err == nil {
				var data any
				if decode := json.Unmarshal(output, &data); decode != nil {
					t.Fatal(decode)
				}
				t.Fatalf("active committed WAL produced successful facts instead of refusing unsafe snapshot: fresh name present=%t; bytes=%d", strings.Contains(string(output), record.Name), len(output))
			}
			if len(output) != 0 || !strings.Contains(err.Error(), "cache") {
				t.Fatalf("unsafe cache must fail without facts: bytes=%d error=%v", len(output), err)
			}
		})
	}
}
