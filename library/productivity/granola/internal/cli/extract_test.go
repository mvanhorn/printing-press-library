// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

// TestExtract_ThreeStreams asserts the three streams stay separate in
// the full markdown — H2 sections named exactly "Notes (Human)",
// "AI Summary", and "Transcript".
func TestExtract_ThreeStreams(t *testing.T) {
	tmp := t.TempDir()
	// Build a synthetic cache containing one doc with TipTap notes and
	// a transcript.
	notes := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hello world"}]}]}`
	cachePath := filepath.Join(tmp, "cache.json")
	cache := map[string]any{
		"cache": map[string]any{
			"version": 6,
			"state": map[string]any{
				"documents": map[string]any{
					"abc": map[string]any{
						"id":            "abc",
						"title":         "Test Meeting",
						"created_at":    "2026-05-01T10:00:00Z",
						"updated_at":    "2026-05-01T11:00:00Z",
						"notes":         json.RawMessage(notes),
						"notes_plain":   "hello world",
						"valid_meeting": true,
					},
				},
				"transcripts": map[string]any{
					"abc": []map[string]any{
						{"source": "microphone", "text": "mic-1", "start_timestamp": "2026-05-01T10:00:00Z", "end_timestamp": "2026-05-01T10:00:05Z"},
						{"source": "microphone", "text": "mic-2", "start_timestamp": "2026-05-01T10:00:05Z", "end_timestamp": "2026-05-01T10:00:10Z"},
						{"source": "microphone", "text": "mic-3", "start_timestamp": "2026-05-01T10:00:10Z", "end_timestamp": "2026-05-01T10:00:15Z"},
						{"source": "microphone", "text": "mic-4", "start_timestamp": "2026-05-01T10:00:15Z", "end_timestamp": "2026-05-01T10:00:20Z"},
						{"source": "microphone", "text": "mic-5", "start_timestamp": "2026-05-01T10:00:20Z", "end_timestamp": "2026-05-01T10:00:25Z"},
						{"source": "system", "text": "sys-1", "start_timestamp": "2026-05-01T10:00:25Z", "end_timestamp": "2026-05-01T10:00:30Z"},
						{"source": "system", "text": "sys-2", "start_timestamp": "2026-05-01T10:00:30Z", "end_timestamp": "2026-05-01T10:00:35Z"},
						{"source": "system", "text": "sys-3", "start_timestamp": "2026-05-01T10:00:35Z", "end_timestamp": "2026-05-01T10:00:40Z"},
						{"source": "system", "text": "sys-4", "start_timestamp": "2026-05-01T10:00:40Z", "end_timestamp": "2026-05-01T10:00:45Z"},
						{"source": "system", "text": "sys-5", "start_timestamp": "2026-05-01T10:00:45Z", "end_timestamp": "2026-05-01T10:00:50Z"},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRANOLA_CACHE_PATH", cachePath)

	outDir := filepath.Join(tmp, "out")
	// Force "local" data source via flag so the live API isn't dialed.
	res, err := runExtract("abc", outDir, "", false)
	if err != nil {
		t.Fatalf("runExtract: %v", err)
	}
	if len(res.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(res.Files))
	}
	fullBytes, err := os.ReadFile(filepath.Join(outDir, "full_abc.md"))
	if err != nil {
		t.Fatalf("read full: %v", err)
	}
	body := string(fullBytes)
	for _, section := range []string{"## Notes (Human)", "## AI Summary", "## Transcript"} {
		if !strings.Contains(body, section) {
			t.Errorf("missing section %q in full file", section)
		}
	}
	// Three streams stay separate: human notes "hello world" only in
	// notes section, mic-1 only in transcript section.
	notesIdx := strings.Index(body, "## Notes (Human)")
	summaryIdx := strings.Index(body, "## AI Summary")
	transcriptIdx := strings.Index(body, "## Transcript")
	if !(notesIdx < summaryIdx && summaryIdx < transcriptIdx) {
		t.Errorf("section order wrong: notes=%d summary=%d transcript=%d", notesIdx, summaryIdx, transcriptIdx)
	}
	if !strings.Contains(body[notesIdx:summaryIdx], "hello world") {
		t.Errorf("expected 'hello world' in Notes section")
	}
	if !strings.Contains(body[transcriptIdx:], "mic-1") {
		t.Errorf("expected 'mic-1' in Transcript section")
	}

	// metadata file has yaml frontmatter
	metaBytes, _ := os.ReadFile(filepath.Join(outDir, "metadata_abc.md"))
	if !strings.HasPrefix(string(metaBytes), "---\n") || !strings.Contains(string(metaBytes), "id: abc") {
		t.Errorf("metadata file missing frontmatter")
	}
}

func TestExtract_MissingTranscriptExits2(t *testing.T) {
	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "cache.json")
	cache := map[string]any{
		"cache": map[string]any{
			"version": 6,
			"state": map[string]any{
				"documents": map[string]any{
					"xyz": map[string]any{
						"id":            "xyz",
						"title":         "No Transcript",
						"created_at":    "2026-05-01T10:00:00Z",
						"valid_meeting": true,
					},
				},
				"transcripts": map[string]any{},
			},
		},
	}
	data, _ := json.Marshal(cache)
	_ = os.WriteFile(cachePath, data, 0o644)
	t.Setenv("GRANOLA_CACHE_PATH", cachePath)
	_, err := runExtract("xyz", filepath.Join(tmp, "o"), "", false)
	if err == nil {
		t.Fatalf("expected error")
	}
	var ce *cliError
	if !As(err, &ce) {
		t.Fatalf("expected cliError, got %v", err)
	}
	if ce.code != 2 {
		t.Errorf("expected exit code 2, got %d", ce.code)
	}
}

// Reference granola import.
var _ = granola.DefaultCachePath
