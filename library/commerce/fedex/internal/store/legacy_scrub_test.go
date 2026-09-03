// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPhysicallyPurgesLegacyRawResponseBlobs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fedex.db")
	state, err := Open(path)
	if err != nil {
		t.Fatalf("Open initial: %v", err)
	}
	sentinel := bytes.Repeat([]byte("SENTINEL-LEGACY-RAW-RESPONSE-"), 7000)
	if _, err := state.db.Exec("INSERT INTO shipments (tracking_number, service_type, raw_response) VALUES (?, ?, ?)", "legacy-tracking", "FEDEX_GROUND", string(sentinel)); err != nil {
		state.Close()
		t.Fatalf("insert legacy blob: %v", err)
	}
	if err := state.Close(); err != nil {
		t.Fatalf("close initial: %v", err)
	}

	state, err = Open(path)
	if err != nil {
		t.Fatalf("Open scrub: %v", err)
	}
	if err := state.Close(); err != nil {
		t.Fatalf("close scrub: %v", err)
	}
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		data, err := os.ReadFile(candidate)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("read %s: %v", candidate, err)
		}
		if bytes.Contains(data, []byte("SENTINEL-LEGACY-RAW-RESPONSE-")) {
			t.Fatalf("legacy raw response remains recoverable in %s", candidate)
		}
	}
}
