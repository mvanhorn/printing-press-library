// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSafePathSegment(t *testing.T) {
	got := safePathSegment("vid/abc:123")
	if got != "vid_abc_123" {
		t.Fatalf("safePathSegment = %q", got)
	}
}

func TestReadCutSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	wantCuts := []map[string]int{{"fromMs": 100, "toMs": 200}}
	data, err := json.Marshal(cutSnapshot{VideoID: "vid", ClipID: "clip", CreatedAt: time.Unix(0, 0).UTC(), Cuts: wantCuts})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	snap, err := readCutSnapshot(path)
	if err != nil {
		t.Fatalf("readCutSnapshot returned error: %v", err)
	}
	if snap.VideoID != "vid" || snap.ClipID != "clip" {
		t.Fatalf("snapshot identity = %s/%s", snap.VideoID, snap.ClipID)
	}
	cuts, ok := snap.Cuts.([]any)
	if !ok || len(cuts) != 1 {
		t.Fatalf("cuts = %#v", snap.Cuts)
	}
}
