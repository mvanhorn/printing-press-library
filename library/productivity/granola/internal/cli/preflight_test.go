// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func TestCountSources(t *testing.T) {
	segs := []granola.TranscriptSegment{
		{Source: "microphone"}, {Source: "microphone"}, {Source: "system"}, {Source: "Mic"}, {Source: "SYSTEM"},
	}
	mic, sys := countSources(segs)
	if mic != 3 {
		t.Errorf("expected mic=3, got %d", mic)
	}
	if sys != 2 {
		t.Errorf("expected sys=2, got %d", sys)
	}
}

func TestFindDuplicateFile(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "full_abc.md"), []byte("x"), 0o644)
	got := findDuplicateFile(tmp, "abc", "Anything")
	if got == "" {
		t.Errorf("expected hit on abc, got empty")
	}
	// Title-fuzzy match
	os.WriteFile(filepath.Join(tmp, "the_big_meeting.md"), []byte("x"), 0o644)
	got = findDuplicateFile(tmp, "zzz", "the big meeting")
	if got == "" {
		t.Errorf("expected fuzzy hit, got empty")
	}
}

func TestPreflight_SyntheticCache_OK(t *testing.T) {
	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "cache.json")
	cache := buildSyntheticCache(map[string]docFixture{
		"good": {Title: "Good", CreatedAt: "2026-05-01T10:00:00Z", HasTranscript: true},
	})
	data, _ := json.Marshal(cache)
	_ = os.WriteFile(cachePath, data, 0o644)
	t.Setenv("GRANOLA_CACHE_PATH", cachePath)
	c, err := openGranolaCache()
	if err != nil {
		t.Fatal(err)
	}
	segs := c.TranscriptByID("good")
	mic, sys := countSources(segs)
	if mic < 5 || sys < 5 {
		t.Errorf("synthetic cache: mic=%d sys=%d", mic, sys)
	}
}
