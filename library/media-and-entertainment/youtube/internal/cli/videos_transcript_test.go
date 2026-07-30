// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
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

// TestTranscriptCacheFallbackAlias covers the fallback cache-key contract:
// a first default-lang run that fell back to the sole non-English track
// writes the row under both the resolved language and the requested default
// key, so a second default-lang run is a pure cache hit (no network) —
// while an explicit --lang that doesn't match still refuses the alias row.
func TestTranscriptCacheFallbackAlias(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate the on-disk cache DB
	ctx := context.Background()

	res := &transcriptResult{
		VideoID:  "vid1234abcd",
		Language: "it",
		Kind:     "asr",
		Segments: []transcriptSegment{{StartMs: 0, DurationMs: 1000, Text: "ciao"}},
		Text:     "ciao",
	}

	// First default-lang (--lang en) run: fallback resolved "it"; the
	// write-through stores the resolved key plus the requested default key.
	writeTranscriptCache(ctx, res, "it", "en")

	// Second default-lang run: the lookup under the default key must hit.
	cached, ok := readTranscriptCache(ctx, "vid1234abcd", "en")
	if !ok {
		t.Fatal("want cache hit under the default key after fallback aliasing")
	}
	if cached.Language != "it" {
		t.Fatalf("alias row must keep the true language; got %q", cached.Language)
	}
	if !cachedTranscriptUsable(cached, "en", false) {
		t.Fatal("default-lang request must accept the fallback alias row")
	}

	// An explicit --lang en must not be satisfied by the Italian alias row.
	if cachedTranscriptUsable(cached, "en", true) {
		t.Fatal("explicit --lang en must reject an alias row whose language is it")
	}

	// An explicit --lang it hits the primary row and is usable.
	cachedIt, ok := readTranscriptCache(ctx, "vid1234abcd", "it")
	if !ok {
		t.Fatal("want cache hit under the resolved language key")
	}
	if !cachedTranscriptUsable(cachedIt, "it", true) {
		t.Fatal("explicit --lang it must accept the primary row")
	}

	// Historical single-key behavior is preserved when no keys are passed.
	res2 := &transcriptResult{VideoID: "vid5678efgh", Language: "en", Kind: "manual", Segments: res.Segments, Text: "hi"}
	writeTranscriptCache(ctx, res2)
	if _, ok := readTranscriptCache(ctx, "vid5678efgh", "en"); !ok {
		t.Fatal("want cache hit under the result's own language when no keys passed")
	}
}

// TestAliasUpgradeTrack covers the staleness guard on fallback alias rows:
// when the video later gains captions in the requested language, an alias
// cache hit must upgrade to the genuine track; when nothing changed, or the
// track list is unreachable (offline), it must keep serving the cache.
func TestAliasUpgradeTrack(t *testing.T) {
	t.Run("requested language appeared after the fallback", func(t *testing.T) {
		tracks := []captionTrack{
			{LanguageCode: "it", Kind: "asr"},
			{LanguageCode: "en", Kind: "asr"},
		}
		got, found := aliasUpgradeTrack(tracks, nil, "en")
		if !found {
			t.Fatal("want upgrade when the requested language is now available")
		}
		if got.LanguageCode != "en" {
			t.Fatalf("upgrade picked %q, want en", got.LanguageCode)
		}
	})

	t.Run("prefers manual over asr when upgrading", func(t *testing.T) {
		tracks := []captionTrack{
			{LanguageCode: "it", Kind: "asr"},
			{LanguageCode: "en", Kind: "asr"},
			{LanguageCode: "en", Kind: ""},
		}
		got, found := aliasUpgradeTrack(tracks, nil, "en")
		if !found || got.Kind == "asr" {
			t.Fatalf("want manual en track; got found=%v kind=%q", found, got.Kind)
		}
	})

	t.Run("still no requested-language track keeps serving cache", func(t *testing.T) {
		tracks := []captionTrack{{LanguageCode: "it", Kind: "asr"}}
		if _, found := aliasUpgradeTrack(tracks, nil, "en"); found {
			t.Fatal("want cache-serve when the requested language is still absent")
		}
	})

	t.Run("track-list fetch failure (offline) keeps serving cache", func(t *testing.T) {
		if _, found := aliasUpgradeTrack(nil, errors.New("innertube request failed: no route to host"), "en"); found {
			t.Fatal("want cache-serve when the track list is unreachable")
		}
	})

	t.Run("probe deadline exceeded (slow upstream) keeps serving cache", func(t *testing.T) {
		// The revalidation probe runs under its own short deadline
		// (aliasRevalidateTimeout); a slow YouTube surfaces here as
		// context.DeadlineExceeded and must fall back to the cache
		// instead of blocking the invocation.
		if _, found := aliasUpgradeTrack(nil, context.DeadlineExceeded, "en"); found {
			t.Fatal("want cache-serve when the probe deadline is exceeded")
		}
	})

	t.Run("empty track list keeps serving cache", func(t *testing.T) {
		if _, found := aliasUpgradeTrack(nil, nil, "en"); found {
			t.Fatal("want cache-serve on an empty track list")
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
