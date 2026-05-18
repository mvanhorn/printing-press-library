// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMemoQueue_FilterByDuplicate asserts memo queue's filtering: a
// meeting with a transcript but already-existing full file is NOT
// emitted; a fresh meeting is.
func TestMemoQueue_FilterByDuplicate(t *testing.T) {
	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "cache.json")
	now := "2026-05-01T10:00:00Z"
	cache := buildSyntheticCache(map[string]docFixture{
		"old": {Title: "Old Meeting", CreatedAt: now, HasTranscript: true},
		"new": {Title: "New Meeting", CreatedAt: now, HasTranscript: true},
	})
	data, _ := json.Marshal(cache)
	_ = os.WriteFile(cachePath, data, 0o644)
	t.Setenv("GRANOLA_CACHE_PATH", cachePath)

	// Pre-create full_old.md to simulate already-extracted
	root := filepath.Join(tmp, "root")
	_ = os.MkdirAll(root, 0o755)
	_ = os.WriteFile(filepath.Join(root, "full_old.md"), []byte("x"), 0o644)

	// Build queue records by calling the inner helper paths directly.
	c, _ := openGranolaCache()
	emitted := []string{}
	for _, id := range c.SortedDocumentIDs() {
		segs := c.TranscriptByID(id)
		mic, sys := countSources(segs)
		if mic < 5 || sys < 5 {
			continue
		}
		d := c.DocumentByID(id)
		if dup := findDuplicateFile(root, id, d.Title); dup != "" {
			continue
		}
		emitted = append(emitted, id)
	}
	if len(emitted) != 1 || emitted[0] != "new" {
		t.Errorf("expected ['new'], got %v", emitted)
	}
}

type docFixture struct {
	Title         string
	CreatedAt     string
	HasTranscript bool
}

// buildSyntheticCache assembles a v6-shaped cache with the named docs.
// Transcripts are 5 mic + 5 system segments when HasTranscript is true.
func buildSyntheticCache(docs map[string]docFixture) map[string]any {
	state := map[string]any{
		"documents":   map[string]any{},
		"transcripts": map[string]any{},
	}
	for id, f := range docs {
		state["documents"].(map[string]any)[id] = map[string]any{
			"id":            id,
			"title":         f.Title,
			"created_at":    f.CreatedAt,
			"valid_meeting": true,
		}
		if f.HasTranscript {
			segs := []map[string]any{}
			for i := 0; i < 5; i++ {
				segs = append(segs, map[string]any{
					"source":          "microphone",
					"text":            "m",
					"start_timestamp": "2026-05-01T10:00:00Z",
					"end_timestamp":   "2026-05-01T10:00:05Z",
				})
			}
			for i := 0; i < 5; i++ {
				segs = append(segs, map[string]any{
					"source":          "system",
					"text":            "s",
					"start_timestamp": "2026-05-01T10:00:05Z",
					"end_timestamp":   "2026-05-01T10:00:10Z",
				})
			}
			state["transcripts"].(map[string]any)[id] = segs
		}
	}
	return map[string]any{
		"cache": map[string]any{
			"version": 6,
			"state":   state,
		},
	}
}

// TestMemoRun_OneMeetingProducesThreeFiles smoke-tests runOneMemo.
func TestMemoRun_OneMeetingProducesThreeFiles(t *testing.T) {
	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "cache.json")
	cache := buildSyntheticCache(map[string]docFixture{
		"m1": {Title: "M1", CreatedAt: "2026-05-01T10:00:00Z", HasTranscript: true},
	})
	data, _ := json.Marshal(cache)
	_ = os.WriteFile(cachePath, data, 0o644)
	t.Setenv("GRANOLA_CACHE_PATH", cachePath)
	outDir := filepath.Join(tmp, "out")
	root := filepath.Join(tmp, "root")
	_ = os.MkdirAll(root, 0o755)
	rec := runOneMemo("m1", outDir, root, false)
	if rec.Status != "new" {
		t.Fatalf("expected status=new, got %q (err=%q)", rec.Status, rec.Error)
	}
	if len(rec.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(rec.Files))
	}
	// Check at least one is full_m1.md
	gotFull := false
	for _, f := range rec.Files {
		if strings.HasSuffix(f, "full_m1.md") {
			gotFull = true
		}
	}
	if !gotFull {
		t.Errorf("missing full_m1.md among %v", rec.Files)
	}
}
