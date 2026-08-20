// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the backup feature.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/store"
)

func TestReadBackupItems(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w := `{"id":555,"name":"Ride","starts":"2026-01-02T07:00:00Z",
		"workout_summary":{"file":{"url":"https://example.com/555.fit"}}}`
	if err := db.UpsertWorkouts(json.RawMessage(w)); err != nil {
		t.Fatal(err)
	}
	items, err := readBackupItems(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%d want 1", len(items))
	}
	if items[0].id != "555" {
		t.Errorf("id=%q want 555", items[0].id)
	}
	if items[0].fileURL != "https://example.com/555.fit" {
		t.Errorf("fileURL=%q", items[0].fileURL)
	}
	if !items[0].hasStarts {
		t.Error("hasStarts false")
	}
}

func TestDownloadTo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("FITDATA"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "x.fit")
	if err := downloadTo(context.Background(), srv.Client(), srv.URL, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dest)
	if err != nil || string(b) != "FITDATA" {
		t.Errorf("content=%q err=%v", string(b), err)
	}

	// A non-200 response is an error and leaves no partial file behind.
	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv404.Close()
	bad := filepath.Join(dir, "y.fit")
	if err := downloadTo(context.Background(), srv404.Client(), srv404.URL, bad); err == nil {
		t.Error("expected error on HTTP 404")
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Error("404 download should not leave a file")
	}
}
