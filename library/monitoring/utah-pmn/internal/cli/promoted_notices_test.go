// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/internal/store"
)

func TestNoticesListsCachedPMNRowsInLocalMode(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("UTAH_PMN_DATA_DIR", dataDir)
	t.Setenv("UTAH_PMN_CONFIG", filepath.Join(t.TempDir(), "missing-config.json"))

	db, err := store.Open(filepath.Join(dataDir, "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	stored, skipped, err := db.UpsertBatch("notices", []json.RawMessage{
		json.RawMessage(`{"noticeId": 42, "meetingCity": "Delta", "meetingTitle": "Land use hearing"}`),
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
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute local notices: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, `"noticeId": 42`) {
		t.Fatalf("local notices output %q does not contain cached notice", got)
	}
}

func TestNoticesFiltersCachedPMNRows(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("UTAH_PMN_DATA_DIR", dataDir)
	t.Setenv("UTAH_PMN_CONFIG", filepath.Join(t.TempDir(), "missing-config.json"))

	db, err := store.Open(filepath.Join(dataDir, "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_, _, err = db.UpsertBatch("notices", []json.RawMessage{
		json.RawMessage(`{"noticeId":101,"meetingCity":"Delta","meetingZip":"84624","meetingStartTime":"2026-06-15","meetingTitle":"wanted"}`),
		json.RawMessage(`{"noticeId":202,"meetingCity":"Fillmore","meetingZip":"84631","meetingStartTime":"2026-06-20","meetingTitle":"wrong city"}`),
		json.RawMessage(`{"noticeId":303,"meetingCity":"Delta","meetingZip":"84624","meetingStartTime":"2026-07-01","meetingTitle":"wrong date"}`),
		json.RawMessage(`{"noticeId":404,"meetingCity":"Delta","meetingZip":"84624","meetingStartTime":"2026-06-16","meetingTitle":"over limit"}`),
	})
	if err != nil {
		t.Fatalf("seed notices: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed store: %v", err)
	}

	cmd := RootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--json", "--data-source", "local", "notices", "--location", "Delta", "--start", "2026-06-01", "--end", "2026-06-30", "--limit", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute local notices: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, `"noticeId": 101`) || strings.Contains(got, `"noticeId": 202`) || strings.Contains(got, `"noticeId": 303`) || strings.Contains(got, `"noticeId": 404`) {
		t.Fatalf("filtered local notices output = %s", got)
	}
}

func TestNoticesPreservesNumericMeetingTimesInLocalMode(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("UTAH_PMN_DATA_DIR", dataDir)
	t.Setenv("UTAH_PMN_CONFIG", filepath.Join(t.TempDir(), "missing-config.json"))

	db, err := store.Open(filepath.Join(dataDir, "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_, _, err = db.UpsertBatch("notices", []json.RawMessage{
		json.RawMessage(`{"noticeId":901,"meetingCity":"Delta","meetingZip":"84624","meetingStartTime":1781481600000,"meetingTitle":"numeric date"}`),
		json.RawMessage(`{"noticeId":902,"meetingCity":"Delta","meetingZip":"84624","meetingStartTime":"2026-06-16","meetingTitle":"formatted date"}`),
	})
	if err != nil {
		t.Fatalf("seed notices: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed store: %v", err)
	}

	cmd := RootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--json", "--data-source", "local", "notices", "--location", "Delta", "--start", "2026-06-15", "--end", "2026-06-16"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute local notices: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, `"noticeId": 901`) || !strings.Contains(got, `"noticeId": 902`) {
		t.Fatalf("local notices output dropped a compatible cached date shape: %s", got)
	}
}
