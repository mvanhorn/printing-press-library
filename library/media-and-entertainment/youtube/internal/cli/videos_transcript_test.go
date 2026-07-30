// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestPickSoleLanguageTrack(t *testing.T) {
	t.Run("single asr track", func(t *testing.T) {
		tracks := []captionTrack{{LanguageCode: "it", Kind: "asr"}}
		got, err := pickSoleLanguageTrack(tracks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.LanguageCode != "it" {
			t.Fatalf("got %q, want it", got.LanguageCode)
		}
	})

	t.Run("prefers manual over asr for the same language", func(t *testing.T) {
		tracks := []captionTrack{
			{LanguageCode: "it", Kind: "asr"},
			{LanguageCode: "it", Kind: ""},
		}
		got, err := pickSoleLanguageTrack(tracks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Kind == "asr" {
			t.Fatalf("picked asr track; want manual")
		}
	})

	t.Run("errors on multiple languages", func(t *testing.T) {
		tracks := []captionTrack{
			{LanguageCode: "it", Kind: "asr"},
			{LanguageCode: "fr", Kind: "asr"},
		}
		if _, err := pickSoleLanguageTrack(tracks); err == nil {
			t.Fatal("want error for multi-language track set")
		}
	})

	t.Run("errors on empty set", func(t *testing.T) {
		if _, err := pickSoleLanguageTrack(nil); err == nil {
			t.Fatal("want error for empty track set")
		}
	})
}

func TestRenderTranscript(t *testing.T) {
	r := &transcriptResult{
		VideoID:  "abc123def45",
		Language: "it",
		Kind:     "asr",
		Segments: []transcriptSegment{
			{StartMs: 1160, DurationMs: 6840, Text: "prima riga"},
			{StartMs: 65000, DurationMs: 5000, Text: "seconda riga"},
		},
		Text: "prima riga seconda riga",
	}

	t.Run("markdown", func(t *testing.T) {
		var b strings.Builder
		if err := renderTranscript(&b, r, "markdown"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := b.String()
		for _, want := range []string{"# Transcript — abc123def45", "_language: it (asr)_", "**[00:01]** prima riga", "**[01:05]** seconda riga"} {
			if !strings.Contains(out, want) {
				t.Fatalf("markdown output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("text", func(t *testing.T) {
		var b strings.Builder
		if err := renderTranscript(&b, r, "text"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(b.String()) != r.Text {
			t.Fatalf("text output = %q, want %q", b.String(), r.Text)
		}
	})

	t.Run("json default keeps segment envelope", func(t *testing.T) {
		var b strings.Builder
		if err := renderTranscript(&b, r, "json"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, want := range []string{`"videoId": "abc123def45"`, `"start_ms": 1160`} {
			if !strings.Contains(b.String(), want) {
				t.Fatalf("json output missing %q:\n%s", want, b.String())
			}
		}
	})
}
