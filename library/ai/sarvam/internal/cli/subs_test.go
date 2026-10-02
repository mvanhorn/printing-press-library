// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// TestNovelSubsHelpWires smoke-tests that the subs command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSubsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"subs", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("subs --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "subs"} {
		if !strings.Contains(help, want) {
			t.Fatalf("subs --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestFormatSRTTime(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "00:00:00,000"},
		{1.5, "00:00:01,500"},
		{65.25, "00:01:05,250"},
		{3661.999, "01:01:01,999"},
	}
	for _, tc := range cases {
		if got := formatSRTTime(tc.in); got != tc.want {
			t.Errorf("formatSRTTime(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatVTTTime(t *testing.T) {
	if got, want := formatVTTTime(0), "00:00:00.000"; got != want {
		t.Errorf("formatVTTTime(0) = %q, want %q", got, want)
	}
	if got, want := formatVTTTime(125.5), "00:02:05.500"; got != want {
		t.Errorf("formatVTTTime(125.5) = %q, want %q", got, want)
	}
}

func TestRenderSRT(t *testing.T) {
	cues := []sttTimestampCue{
		{Start: 0, End: 2.5, Text: "नमस्ते"},
		{Start: 2.5, End: 5.0, Text: "आप कैसे हैं?"},
	}
	got := renderSRT(cues)
	for _, want := range []string{"1", "00:00:00,000 --> 00:00:02,500", "नमस्ते", "2", "00:00:02,500 --> 00:00:05,000", "आप कैसे हैं?"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderSRT output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderVTT(t *testing.T) {
	cues := []sttTimestampCue{
		{Start: 0, End: 1.0, Text: "hello"},
	}
	got := renderVTT(cues)
	for _, want := range []string{"WEBVTT", "00:00:00.000 --> 00:00:01.000", "hello"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVTT output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderEmptyCues(t *testing.T) {
	if got := renderSRT(nil); got != "\n" {
		t.Errorf("renderSRT(nil) = %q, want newline-only", got)
	}
	if got := renderVTT(nil); got != "WEBVTT\n" {
		t.Errorf("renderVTT(nil) = %q, want WEBVTT header", got)
	}
}

func TestSubtitleCuesValidatesProviderTimestamps(t *testing.T) {
	cues, err := subtitleCues([]string{"hello", "world"}, []float64{0, 1}, []float64{1, 2.5})
	if err != nil {
		t.Fatalf("subtitleCues() error = %v", err)
	}
	if len(cues) != 2 || cues[1].Start != 1 || cues[1].End != 2.5 || cues[1].Text != "world" {
		t.Fatalf("subtitleCues() = %#v", cues)
	}

	tests := []struct {
		name   string
		words  []string
		starts []float64
		ends   []float64
	}{
		{name: "missing start", words: []string{"hello"}, ends: []float64{1}},
		{name: "missing end", words: []string{"hello"}, starts: []float64{0}},
		{name: "negative start", words: []string{"hello"}, starts: []float64{-0.1}, ends: []float64{1}},
		{name: "reversed", words: []string{"hello"}, starts: []float64{2}, ends: []float64{1}},
		{name: "zero duration", words: []string{"hello"}, starts: []float64{1}, ends: []float64{1}},
		{name: "sub-millisecond duration", words: []string{"hello"}, starts: []float64{1.0001}, ends: []float64{1.0009}},
		{name: "nan", words: []string{"hello"}, starts: []float64{math.NaN()}, ends: []float64{1}},
		{name: "infinite", words: []string{"hello"}, starts: []float64{0}, ends: []float64{math.Inf(1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := subtitleCues(tt.words, tt.starts, tt.ends); err == nil {
				t.Fatal("subtitleCues() unexpectedly accepted malformed timestamps")
			}
		})
	}
}
