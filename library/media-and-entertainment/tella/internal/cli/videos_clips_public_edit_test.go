// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseCutRangeFlags(t *testing.T) {
	ranges, err := parseCutRangeFlags([]string{"100:200", "300:450"})
	if err != nil {
		t.Fatalf("parseCutRangeFlags returned error: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("got %d ranges, want 2", len(ranges))
	}
	if ranges[0]["fromMs"] != 100 || ranges[0]["toMs"] != 200 {
		t.Fatalf("first range = %#v", ranges[0])
	}
	if ranges[1]["fromMs"] != 300 || ranges[1]["toMs"] != 450 {
		t.Fatalf("second range = %#v", ranges[1])
	}
}

func TestParseCutRangeFlagsRejectsBadShape(t *testing.T) {
	if _, err := parseCutRangeFlags([]string{"100-200"}); err == nil {
		t.Fatal("expected invalid range shape to fail")
	}
}

func TestCutByTranscriptBody(t *testing.T) {
	body, err := cutByTranscriptBody(false, `[{"fromWordIndex":1,"toWordIndex":3}]`, []string{"8:9"})
	if err != nil {
		t.Fatalf("cutByTranscriptBody returned error: %v", err)
	}
	ranges, ok := body["wordRanges"].([]map[string]int)
	if !ok {
		t.Fatalf("wordRanges type = %T", body["wordRanges"])
	}
	if len(ranges) != 2 {
		t.Fatalf("got %d ranges, want 2", len(ranges))
	}
	if ranges[0]["fromWordIndex"] != 1 || ranges[0]["toWordIndex"] != 3 {
		t.Fatalf("json range = %#v", ranges[0])
	}
	if ranges[1]["fromWordIndex"] != 8 || ranges[1]["toWordIndex"] != 9 {
		t.Fatalf("flag range = %#v", ranges[1])
	}
}

func TestTranscriptTermRangesFromValueDeterministicAndWordOnly(t *testing.T) {
	transcript := map[string]any{
		"metadata": map[string]any{"text": "mistake in metadata should not shift words"},
		"words": []any{
			map[string]any{"word": "hello"},
			map[string]any{"word": "mistake"},
			map[string]any{"text": "done"},
		},
	}

	for i := 0; i < 20; i++ {
		ranges := transcriptTermRangesFromValue(transcript, "mistake")
		if len(ranges) != 1 {
			t.Fatalf("iteration %d got %d ranges: %#v", i, len(ranges), ranges)
		}
		if ranges[0]["fromWordIndex"] != 1 || ranges[0]["toWordIndex"] != 1 {
			t.Fatalf("iteration %d ranges = %#v", i, ranges)
		}
	}
}

func TestTranscriptTermRangesFromValueUsesStableWordIndexField(t *testing.T) {
	transcript := map[string]any{
		"words": []any{
			map[string]any{"index": float64(12), "text": "mistake"},
		},
	}
	ranges := transcriptTermRangesFromValue(transcript, "mistake")
	if len(ranges) != 1 || ranges[0]["fromWordIndex"] != 12 || ranges[0]["toWordIndex"] != 12 {
		t.Fatalf("ranges = %#v", ranges)
	}
}

func TestTranscriptLooksEmpty(t *testing.T) {
	if !transcriptLooksEmpty([]byte(`{"words":[]}`)) {
		t.Fatal("empty words should be empty")
	}
	if transcriptLooksEmpty([]byte(`{"words":[{"word":"hello"}]}`)) {
		t.Fatal("word transcript should not be empty")
	}
}

func TestReplaceWordRangesRequiresWordRanges(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newVideosClipsReplaceWordRangesCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"vid_abc", "cl_xyz", "--apply"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --word-ranges, got nil")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode = %d, want 2 (usage error)", ExitCode(err))
	}
	if !strings.Contains(err.Error(), "word-ranges") {
		t.Fatalf("error = %q, want word-ranges validation message", err.Error())
	}
}

func TestCutRequestBodyUsesCutsForRangeFlags(t *testing.T) {
	body, err := cutRequestBody(0, 0, []string{"100:200", "300:400"})
	if err != nil {
		t.Fatalf("cutRequestBody returned error: %v", err)
	}
	if _, ok := body["ranges"]; ok {
		t.Fatalf("body must not use unsupported ranges field: %#v", body)
	}
	cuts, ok := body["cuts"].([]map[string]int)
	if !ok {
		t.Fatalf("cuts type = %T, body=%#v", body["cuts"], body)
	}
	if len(cuts) != 2 || cuts[0]["fromMs"] != 100 || cuts[1]["toMs"] != 400 {
		t.Fatalf("cuts = %#v", cuts)
	}
}

func TestValidateReplacementSourceInputBeforeCut(t *testing.T) {
	tmp := t.TempDir() + "/replacement.mp4"
	if err := os.WriteFile(tmp, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateReplacementSourceInput(tmp, 0, 1080, 3); err == nil || !strings.Contains(err.Error(), "--width") {
		t.Fatalf("missing width error = %v", err)
	}
	if err := validateReplacementSourceInput(t.TempDir()+"/missing.mp4", 1920, 1080, 3); err == nil || !strings.Contains(err.Error(), "opening --insert-file before cutting") {
		t.Fatalf("missing file error = %v", err)
	}
	if err := validateReplacementSourceInput(tmp, 1920, 1080, 3); err != nil {
		t.Fatalf("valid replacement input returned error: %v", err)
	}
}

func TestVideoPatchBodyFiltersResponseEnvelope(t *testing.T) {
	body := videoPatchBody(map[string]any{
		"video": map[string]any{
			"id":         "vid_abc",
			"name":       "Edited",
			"dimensions": map[string]any{"width": 1920, "height": 1080},
			"createdAt":  "response-only",
		},
	})
	if body["name"] != "Edited" || body["dimensions"] == nil {
		t.Fatalf("body missing patchable fields: %#v", body)
	}
	if _, ok := body["id"]; ok {
		t.Fatalf("body includes response-only id: %#v", body)
	}
	if _, ok := body["createdAt"]; ok {
		t.Fatalf("body includes response-only createdAt: %#v", body)
	}
}
