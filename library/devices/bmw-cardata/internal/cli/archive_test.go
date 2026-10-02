// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveReadReportsPersistenceFailure(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "archive.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	zw := zip.NewWriter(file)
	entry, err := zw.Create("telematic.json")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write([]byte(`{"telematicData":{"vehicle.test":{"value":"1","unit":"x"}}}`)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	// SQLite cannot open an existing directory as a database file.
	unwritableDB := t.TempDir()
	cmd := RootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json", "archive", "read", archivePath,
		"--vin", "WBA00000000000000", "--db", unwritableDB})
	if err := cmd.Execute(); err == nil {
		t.Fatal("archive read succeeded despite persistence failure")
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON report %q: %v", stdout.String(), err)
	}
	if report["failed_files"] != float64(1) || report["descriptors"] != float64(0) {
		t.Fatalf("persistence failure was counted as an import: %#v", report)
	}
}
