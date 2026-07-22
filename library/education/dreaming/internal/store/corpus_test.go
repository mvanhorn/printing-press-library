// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

// seedVideo upserts one video so transcript export has a catalog row to carry.
func seedVideo(t *testing.T, s *Store, id, title string) {
	t.Helper()
	v, _ := json.Marshal(map[string]any{"_id": id, "title": title, "level": "beginner"})
	if err := s.UpsertVideos(v); err != nil {
		t.Fatalf("UpsertVideos(%s): %v", id, err)
	}
}

func TestCorpusExportImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	src, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "src.db"))
	if err != nil {
		t.Fatalf("open src: %v", err)
	}
	defer src.Close()

	seedVideo(t, src, "vid1", "Sofía loves animals")
	cues := []TranscriptCue{
		{VideoID: "vid1", Index: 0, StartMS: 0, EndMS: 1000, Text: "hola gato"},
		{VideoID: "vid1", Index: 1, StartMS: 1000, EndMS: 2000, Text: "quiero un gato"},
	}
	if err := src.ReplaceTranscript("vid1", cues); err != nil {
		t.Fatalf("ReplaceTranscript: %v", err)
	}

	// Export pieces from src.
	videos, err := src.AllVideoData()
	if err != nil {
		t.Fatalf("AllVideoData: %v", err)
	}
	if len(videos) != 1 {
		t.Fatalf("expected 1 video, got %d", len(videos))
	}
	transcripts, err := src.AllTranscripts()
	if err != nil {
		t.Fatalf("AllTranscripts: %v", err)
	}
	if got := len(transcripts["vid1"]); got != 2 {
		t.Fatalf("expected 2 cues for vid1, got %d", got)
	}

	// Import into a fresh destination store.
	dst, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "dst.db"))
	if err != nil {
		t.Fatalf("open dst: %v", err)
	}
	defer dst.Close()

	for _, v := range videos {
		if err := dst.UpsertVideos(v); err != nil {
			t.Fatalf("dst UpsertVideos: %v", err)
		}
	}
	if got := dst.TranscriptCueCount("vid1"); got != 0 {
		t.Fatalf("fresh dst should have 0 cues, got %d", got)
	}
	if err := dst.ReplaceTranscript("vid1", transcripts["vid1"]); err != nil {
		t.Fatalf("dst ReplaceTranscript: %v", err)
	}

	// Verify the corpus is searchable in the destination with all cues.
	if got := dst.TranscriptCueCount("vid1"); got != 2 {
		t.Fatalf("dst should have 2 cues after import, got %d", got)
	}
	hits, err := dst.SearchTranscriptFTS("gato", TranscriptFilter{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 'gato' hits, got %d", len(hits))
	}
	if hits[0].VideoTitle != "Sofía loves animals" {
		t.Fatalf("expected video title carried via catalog join, got %q", hits[0].VideoTitle)
	}
}

func TestTranscriptCueCount(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if got := s.TranscriptCueCount("absent"); got != 0 {
		t.Fatalf("absent video should be 0, got %d", got)
	}
	if err := s.ReplaceTranscript("v", []TranscriptCue{{VideoID: "v", Index: 0, Text: "a"}}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := s.TranscriptCueCount("v"); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}
