// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/internal/store"
)

func TestWorkflowArchiveRequiresLocationBeforeClientCreation(t *testing.T) {
	cmd := newWorkflowArchiveCmd(&rootFlags{})
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--postal-code is required") {
		t.Fatalf("archive error = %v, want required postal-code error", err)
	}
}

func TestWorkflowArchiveFetchesBothResourcesForSelectedMarket(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("postal_code") != "10001" || r.URL.Query().Get("locale") != "en-us" {
			t.Errorf("archive request %s has wrong market query: %s", r.URL.Path, r.URL.RawQuery)
		}
		requests = append(requests, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/flyers":
			_, _ = fmt.Fprint(w, `[{"id":"f1","name":"Weekly flyer"}]`)
		case "/merchants":
			_, _ = fmt.Fprint(w, `[{"id":"m1","name":"Corner shop"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("FLIPP_BASE_URL", server.URL)

	dbPath := filepath.Join(t.TempDir(), "archive.db")
	flags := &rootFlags{configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newWorkflowArchiveCmd(flags)
	cmd.SetArgs([]string{"--postal-code", "10001", "--db", dbPath})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("archive: %v (%s)", err, output.String())
	}
	if len(requests) != 2 || requests[0] != "/flyers" || requests[1] != "/merchants" {
		t.Fatalf("archive requests = %v, want flyers then merchants", requests)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, resource := range defaultSyncResources() {
		rows, err := db.ListScoped(resource, "10001", "en-us", 0)
		if err != nil || len(rows) != 1 {
			t.Fatalf("%s archived rows = %d, %v; want 1", resource, len(rows), err)
		}
		otherRows, err := db.ListScoped(resource, "94105", "en-us", 0)
		if err != nil || len(otherRows) != 0 {
			t.Fatalf("%s other-market rows = %d, %v; want 0", resource, len(otherRows), err)
		}
	}
}

func TestWorkflowArchiveRejectsIncompleteResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/merchants" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, "<html>temporary response</html>")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[{"id":"f1","name":"Weekly flyer"}]`)
	}))
	defer server.Close()
	t.Setenv("FLIPP_BASE_URL", server.URL)

	flags := &rootFlags{configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newWorkflowArchiveCmd(flags)
	cmd.SetArgs([]string{"--postal-code", "10001", "--db", filepath.Join(t.TempDir(), "archive.db")})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "archive incomplete") {
		t.Fatalf("archive error = %v, want incomplete-resource error (%s)", err, output.String())
	}
}

func TestWorkflowArchiveRejectsUnexpectedNextPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/flyers" {
			_, _ = fmt.Fprint(w, `{"items":[{"id":"f1","name":"First page"}],"has_more":true,"next_cursor":"page-2"}`)
			return
		}
		_, _ = fmt.Fprint(w, `[{"id":"m1","name":"Merchant"}]`)
	}))
	defer server.Close()
	t.Setenv("FLIPP_BASE_URL", server.URL)

	flags := &rootFlags{configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newWorkflowArchiveCmd(flags)
	cmd.SetArgs([]string{"--postal-code", "10001", "--db", filepath.Join(t.TempDir(), "archive.db")})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "pagination_unhandled") {
		t.Fatalf("archive next-page error = %v, want incomplete enumeration (%s)", err, output.String())
	}
}

func TestWorkflowArchiveDryRunPreservesCursor(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "archive.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	seedLastSynced(t, db, flippSyncStateKey("flyers", "10001", "en-us"), old, "keep-me")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{configPath: filepath.Join(t.TempDir(), "missing.toml"), dryRun: true}
	cmd := newWorkflowArchiveCmd(flags)
	cmd.SetArgs([]string{"--postal-code", "10001", "--db", dbPath, "--full"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run archive: %v (%s)", err, output.String())
	}
	db, err = store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cursor, syncedAt, _, err := db.GetSyncState(flippSyncStateKey("flyers", "10001", "en-us"))
	if err != nil || cursor != "keep-me" || !syncedAt.Equal(old) {
		t.Fatalf("dry-run changed cursor or freshness: %q, %v, %v", cursor, syncedAt, err)
	}
	if !strings.Contains(output.String(), "Dry run: would archive") {
		t.Fatalf("dry-run output = %q", output.String())
	}
}

func TestFlippDefaultSyncResourcesPopulateArchive(t *testing.T) {
	resources := defaultSyncResources()
	if len(resources) != 2 || resources[0] != "flyers" || resources[1] != "merchants" {
		t.Fatalf("default archive resources = %v, want flyers and merchants", resources)
	}
}

func TestFullArchiveResetsOnlySelectedLocationCursors(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, resource := range defaultSyncResources() {
		seedLastSynced(t, db, flippSyncStateKey(resource, "10001", "en-us"), old, "east-cursor")
		seedLastSynced(t, db, flippSyncStateKey(resource, "94105", "en-us"), old, "west-cursor")
	}
	if err := resetFlippArchiveCursors(db, defaultSyncResources(), flippSyncLocationParams("10001", "en-us")); err != nil {
		t.Fatal(err)
	}
	for _, resource := range defaultSyncResources() {
		east, eastTime, _, err := db.GetSyncState(flippSyncStateKey(resource, "10001", "en-us"))
		if err != nil || east != "" || !eastTime.Equal(old) {
			t.Fatalf("%s east state = %q, %v, %v; want cursor reset and old freshness", resource, east, eastTime, err)
		}
		west, westTime, _, err := db.GetSyncState(flippSyncStateKey(resource, "94105", "en-us"))
		if err != nil || west != "west-cursor" || !westTime.Equal(old) {
			t.Fatalf("%s west state = %q, %v, %v; want untouched", resource, west, westTime, err)
		}
	}
}
