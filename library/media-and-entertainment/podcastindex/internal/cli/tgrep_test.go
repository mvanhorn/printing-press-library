// Copyright 2026 adbonnet and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTgrepExtractText(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantContain string
		wantExclude string
	}{
		{
			name:        "srt strips timestamps and indices",
			contentType: "application/srt",
			body:        "1\n00:00:01,000 --> 00:00:04,000\nwe talked about interest rates\n\n2\n00:00:04,000 --> 00:00:06,000\nand the economy\n",
			wantContain: "interest rates",
			wantExclude: "00:00:01",
		},
		{
			name:        "vtt strips header and cues",
			contentType: "text/vtt",
			body:        "WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nhello world\n",
			wantContain: "hello world",
			wantExclude: "WEBVTT",
		},
		{
			name:        "json segments concatenated",
			contentType: "application/json",
			body:        `{"version":"1.0.0","segments":[{"startTime":0,"body":"machine learning"},{"startTime":2,"body":"and jepa"}]}`,
			wantContain: "machine learning and jepa",
		},
		{
			name:        "plain text passes through",
			contentType: "text/plain",
			body:        "just some spoken words here",
			wantContain: "spoken words",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tgrepExtractText(tt.contentType, []byte(tt.body))
			if tt.wantContain != "" && !strings.Contains(got, tt.wantContain) {
				t.Errorf("expected text to contain %q, got %q", tt.wantContain, got)
			}
			if tt.wantExclude != "" && strings.Contains(got, tt.wantExclude) {
				t.Errorf("expected text to exclude %q, got %q", tt.wantExclude, got)
			}
		})
	}
}

func TestTgrepIsTimestampLine(t *testing.T) {
	cases := map[string]bool{
		"00:00:01,000 --> 00:00:04,000": true,
		"00:00:00.000 --> 00:00:02.000": true,
		"hello world":                   false,
		"42":                            false,
	}
	for in, want := range cases {
		if got := tgrepIsTimestampLine(in); got != want {
			t.Errorf("tgrepIsTimestampLine(%q)=%v want %v", in, got, want)
		}
	}
}

func TestTgrepIsIndexLine(t *testing.T) {
	cases := map[string]bool{
		"1":     true,
		"42":    true,
		"3a":    false,
		"hello": false,
		"":      false,
	}
	for in, want := range cases {
		if got := tgrepIsIndexLine(in); got != want {
			t.Errorf("tgrepIsIndexLine(%q)=%v want %v", in, got, want)
		}
	}
}

func TestTgrepSnippet(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog and keeps running through the meadow"
	loc := []int{16, 19} // "fox"
	got := tgrepSnippet(text, loc)
	if !strings.Contains(got, "fox") {
		t.Errorf("snippet should contain match, got %q", got)
	}
}

func TestTgrepSnippetUTF8Safe(t *testing.T) {
	// Multi-byte runes surround the match; padding must not split a codepoint,
	// so the snippet must always be valid UTF-8.
	text := "café ☕ discussion about 日本語 podcasts and ☕ interest rates in 日本 markets ☕☕☕"
	idx := strings.Index(text, "interest")
	got := tgrepSnippet(text, []int{idx, idx + len("interest")})
	if !utf8.ValidString(got) {
		t.Errorf("snippet is not valid UTF-8: %q", got)
	}
	if !strings.Contains(got, "interest") {
		t.Errorf("snippet should contain match, got %q", got)
	}
}

type fakeGetter struct {
	responses map[string]json.RawMessage
}

func (f fakeGetter) Get(_ context.Context, path string, _ map[string]string) (json.RawMessage, error) {
	return f.responses[path], nil
}

func TestTgrepResolveFeedsFromCSV(t *testing.T) {
	ids, err := tgrepResolveFeeds(context.Background(), fakeGetter{}, "75075, 920666 ,", "", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != 75075 || ids[1] != 920666 {
		t.Errorf("expected [75075 920666], got %v", ids)
	}
}

func TestTgrepResolveFeedsFromCategory(t *testing.T) {
	fg := fakeGetter{responses: map[string]json.RawMessage{
		"/podcasts/trending": json.RawMessage(`{"feeds":[{"id":111},{"id":222}]}`),
	}}
	ids, err := tgrepResolveFeeds(context.Background(), fg, "", "Technology", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != 111 || ids[1] != 222 {
		t.Errorf("expected [111 222], got %v", ids)
	}
}

func TestTgrepResolveFeedsInvalidID(t *testing.T) {
	if _, err := tgrepResolveFeeds(context.Background(), fakeGetter{}, "abc", "", 5); err == nil {
		t.Error("expected error for non-numeric feed id")
	}
}
