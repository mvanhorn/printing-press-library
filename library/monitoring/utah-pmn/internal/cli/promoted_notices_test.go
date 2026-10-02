// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/internal/store"
)

func TestNoticesRefusesOfflineLocationSearch(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("UTAH_PMN_DATA_DIR", dataDir)
	t.Setenv("UTAH_PMN_CONFIG", filepath.Join(t.TempDir(), "missing-config.json"))

	db, err := store.Open(filepath.Join(dataDir, "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	stored, skipped, err := db.UpsertBatch("notices", []json.RawMessage{
		json.RawMessage(`{"noticeId": 42, "meetingCity": "Hinckley", "meetingTitle": "Nearby land use hearing"}`),
	})
	if err != nil {
		t.Fatalf("seed notices: %v", err)
	}
	if stored != 1 || skipped != 0 {
		t.Fatalf("stored, skipped = %d, %d; want 1, 0", stored, skipped)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed store: %v", err)
	}

	cmd := RootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--json", "--data-source", "local", "notices", "--location", "Delta"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "PMN searches nearby locations") {
		t.Fatalf("local notices error = %v, want nearby-search refusal", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("local notices unexpectedly returned %q", stdout.String())
	}
}

func TestFilterCachedNoticesUsesUtahCalendarDatesAndDefaultLimit(t *testing.T) {
	items := []json.RawMessage{
		json.RawMessage(`{"noticeId":1,"meetingStartTime":1781577000000}`), // June 15, 20:30 in Utah; June 16 UTC.
		json.RawMessage(`{"noticeId":2,"meetingStartTime":"2026-06-15"}`),
		json.RawMessage(`{"noticeId":3,"meetingStartTime":"2026-06-16"}`),
	}
	got, err := filterCachedNotices(items, map[string]string{"startDate": "2026-06-15", "endDate": "2026-06-15"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !bytes.Contains(got[0], []byte(`"noticeId":2`)) || !bytes.Contains(got[1], []byte(`"noticeId":1`)) {
		t.Fatalf("Utah June 15 results = %s, want formatted and late-night numeric notices", got)
	}

	items = make([]json.RawMessage, 51)
	for i := range items {
		items[i] = json.RawMessage(`{"noticeId":1,"meetingStartTime":"2026-06-15"}`)
	}
	got, err = filterCachedNotices(items, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 50 {
		t.Fatalf("default local notice limit = %d, want 50", len(got))
	}
}

func TestResolveLocalNoticesDistinguishesEmptyCacheFromEmptyFilter(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("UTAH_PMN_DATA_DIR", dataDir)
	db, err := store.Open(filepath.Join(dataDir, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{dataSource: "local"}
	if _, _, err := resolveLocal(context.Background(), flags, io.Discard, "notices", true, "/getUpcomingNotices.json", nil, "user_requested"); err == nil || !strings.Contains(err.Error(), "no local data") {
		t.Fatalf("empty cache error = %v, want no local data", err)
	}
	db, err = store.Open(filepath.Join(dataDir, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.UpsertBatch("notices", []json.RawMessage{json.RawMessage(`{"noticeId":1,"meetingStartTime":"2026-06-15"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	data, _, err := resolveLocal(context.Background(), flags, io.Discard, "notices", true, "/getUpcomingNotices.json", map[string]string{"startDate": "2026-07-01"}, "user_requested")
	if err != nil || string(data) != "[]" {
		t.Fatalf("filtered cache result = %s, %v; want [] and nil", data, err)
	}
}
