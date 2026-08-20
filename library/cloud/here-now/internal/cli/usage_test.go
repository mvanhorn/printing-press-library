// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored unit tests for the `usage` free-plan meter's graceful
// degradation: with no auth (nil client), buildUsageReport must report real
// local stats and mark drive metrics as requires-auth WITHOUT returning an
// error, so an unauthenticated `usage` run exits 0.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/here-now/internal/store"
)

// newUsageTestStore opens a fresh store in a temp dir with the novel-feature
// tables created. t.Cleanup handles Close.
func newUsageTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.EnsureHereNowTables(); err != nil {
		db.Close()
		t.Fatalf("ensure tables: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// seedPublish records one publish so both usage-meter reads observe it:
// Count("publishes") (the generic resource mirror) for the site count, and
// RecentPublishCount() (which counts publish_state rows by created_at) for the
// publish cadence.
func seedPublish(t *testing.T, db *store.Store, slug, publishedAt string) {
	t.Helper()
	doc, err := json.Marshal(map[string]string{
		"slug":        slug,
		"publishedAt": publishedAt,
		"updatedAt":   publishedAt,
	})
	if err != nil {
		t.Fatalf("marshal publish doc: %v", err)
	}
	if err := db.Upsert("publishes", slug, doc); err != nil {
		t.Fatalf("seed publish mirror %q: %v", slug, err)
	}
	if err := db.SavePublishState(store.PublishStateRecord{
		Slug:      slug,
		VersionID: "v1",
		Finalized: true,
		CreatedAt: publishedAt,
	}); err != nil {
		t.Fatalf("seed publish_state %q: %v", slug, err)
	}
}

// TestBuildUsageReportNoAuthDegradesGracefully proves the free-plan-first
// contract: with a nil client (no API key configured), buildUsageReport must
// NOT return an error. Local stats (site count and recent publishes) are real,
// while the drive metrics carry a note that mentions auth.
func TestBuildUsageReportNoAuthDegradesGracefully(t *testing.T) {
	db := newUsageTestStore(t)
	now := time.Now()
	seedPublish(t, db, "alpha", now.Add(-10*time.Minute).Format(time.RFC3339))

	report, err := buildUsageReport(context.Background(), nil, db, now)
	if err != nil {
		t.Fatalf("buildUsageReport with nil client returned error, want nil: %v", err)
	}

	// Local stats are real.
	if report.Sites.Used != 1 {
		t.Errorf("Sites.Used = %d, want 1 (from local mirror)", report.Sites.Used)
	}
	if report.PublishesLast1h.Used != 1 {
		t.Errorf("PublishesLast1h.Used = %d, want 1 (from local publish log)", report.PublishesLast1h.Used)
	}

	// Drive metrics degrade: zero bytes, and both notes mention auth.
	if report.DriveBytes.Used != 0 {
		t.Errorf("DriveBytes.Used = %d, want 0 without auth", report.DriveBytes.Used)
	}
	if !strings.Contains(report.Drives.Note, "auth") {
		t.Errorf("Drives.Note = %q, want a requires-auth note when client is nil", report.Drives.Note)
	}
	if !strings.Contains(report.DriveBytes.Note, "auth") {
		t.Errorf("DriveBytes.Note = %q, want a requires-auth note when client is nil", report.DriveBytes.Note)
	}
}

// TestUsageCommandRunsWithoutAuth is the end-to-end guard for the free-plan-first
// contract: invoking `usage --json --db <temp>` with no API key configured must
// exit 0 (RunE returns nil), emit valid JSON, and mark the drive metrics with a
// requires-auth note rather than failing. This is what keeps the unauthenticated
// `usage` path (the majority of here.now users) from regressing to a hard error.
func TestUsageCommandRunsWithoutAuth(t *testing.T) {
	t.Setenv("HERENOW_API_KEY", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	db := newUsageTestStore(t)
	seedPublish(t, db, "alpha", time.Now().Add(-10*time.Minute).Format(time.RFC3339))

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"usage", "--json", "--db", db.Path()})

	if err := root.Execute(); err != nil {
		t.Fatalf("usage with no auth returned error, want nil: %v\noutput: %s", err, out.String())
	}

	var report usageReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("usage --json did not emit valid JSON: %v\noutput: %s", err, out.String())
	}
	if report.Sites.Used != 1 {
		t.Errorf("Sites.Used = %d, want 1 (local mirror)", report.Sites.Used)
	}
	if !strings.Contains(report.Drives.Note, "auth") {
		t.Errorf("Drives.Note = %q, want a requires-auth note without a key", report.Drives.Note)
	}
}
