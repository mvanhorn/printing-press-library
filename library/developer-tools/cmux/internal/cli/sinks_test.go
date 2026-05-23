// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseSink(t *testing.T) {
	cases := []struct {
		in     string
		kind   string
		target string
	}{
		{"", "stdout", ""},
		{"-", "stdout", ""},
		{"stdout", "stdout", ""},
		{"stdout:", "stdout", ""},
		{"file:/tmp/out.jsonl", "file", "/tmp/out.jsonl"},
		{"exec:/usr/local/bin/handle.sh", "exec", "/usr/local/bin/handle.sh"},
		{"slack:https://hooks.slack.com/services/X", "slack", "https://hooks.slack.com/services/X"},
		{"webhook:https://example.com/hook", "webhook", "https://example.com/hook"},
		{"macos:", "macos", ""},
		{"macos:cmux alert", "macos", "cmux alert"},
	}
	for _, tc := range cases {
		s, err := ParseSink(tc.in)
		if err != nil {
			t.Errorf("ParseSink(%q) returned error: %v", tc.in, err)
			continue
		}
		if s.Kind != tc.kind || s.Target != tc.target {
			t.Errorf("ParseSink(%q) = {%s %s}, want {%s %s}", tc.in, s.Kind, s.Target, tc.kind, tc.target)
		}
	}
	if _, err := ParseSink("garbage:thing"); err == nil {
		t.Errorf("expected error for unknown sink kind")
	}
}

func TestSummarizeEventTransition(t *testing.T) {
	event := map[string]any{"workspace_title": "Tuck", "prev_value": "Running", "new_value": "Needs input"}
	got := summarizeEvent(event)
	want := "Tuck: Running → Needs input"
	if got != want {
		t.Errorf("summarizeEvent = %q, want %q", got, want)
	}
}
