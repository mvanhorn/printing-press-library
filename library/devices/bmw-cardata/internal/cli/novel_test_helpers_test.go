// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/store"
)

const testCardataVIN = "WBA00000000000000"

func newTestCardataStore(t *testing.T) (string, *store.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cardata.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	return path, db
}

func insertTestSnapshot(t *testing.T, db *store.Store, descriptor, value, unit, timestamp, fetchedAt string) {
	t.Helper()
	if _, err := db.DB().Exec(
		`INSERT INTO cardata_telematic_snapshots(vin, descriptor, value, unit, ts, fetched_at)
		 VALUES(?,?,?,?,?,?)`, testCardataVIN, descriptor, value, unit, timestamp, fetchedAt,
	); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
}

func executeTestJSON(t *testing.T, args ...string) any {
	t.Helper()
	cmd := RootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append([]string{"--json"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v\nstderr: %s\nstdout: %s", args, err, stderr.String(), stdout.String())
	}
	var result any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode command JSON %q: %v", stdout.String(), err)
	}
	return result
}

func resultObject(t *testing.T, value any) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want object", value)
	}
	return result
}
