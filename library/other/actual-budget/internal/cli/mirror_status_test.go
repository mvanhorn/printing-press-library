// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func TestMirrorStatusReportsBudgetType(t *testing.T) {
	s3Fixture(t)
	dir, err := actual.MirrorDir(s3SyncID)
	if err != nil {
		t.Fatal(err)
	}
	mj, _ := json.Marshal(actual.Manifest{SyncID: s3SyncID, GroupID: s3SyncID, Name: "Fixture", PulledAt: time.Now().UTC(), BudgetType: "envelope"})
	if err := os.WriteFile(filepath.Join(dir, "pull.json"), mj, 0o600); err != nil {
		t.Fatal(err)
	}
	var st map[string]any
	s3RunJSON(t, &st, "mirror", "status")
	if st["mirrored"] != true || st["budget_type"] != "envelope" || st["budget"] != "Fixture" {
		t.Fatalf("status = %v, want mirrored envelope budget", st)
	}
}

func TestMirrorStatusNotPulledKeepsWouldBePath(t *testing.T) {
	s3NoMirror(t)
	want, err := actual.DBPath(s3SyncID)
	if err != nil {
		t.Fatal(err)
	}
	var st map[string]any
	s3RunJSON(t, &st, "mirror", "status")
	if st["mirrored"] != false || st["db_path"] != want {
		t.Fatalf("status = %v, want unmirrored with db_path %s", st, want)
	}
	if _, ok := st["budget_type"]; ok {
		t.Fatalf("budget_type present without a manifest: %v", st)
	}
}
