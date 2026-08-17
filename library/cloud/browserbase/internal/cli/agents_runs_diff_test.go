// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// TestNovelAgentsRunsDiffHelpWires smoke-tests that the agents runs diff command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAgentsRunsDiffHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"agents", "runs", "diff", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agents runs diff --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "diff"} {
		if !strings.Contains(help, want) {
			t.Fatalf("agents runs diff --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestLcsDiff exercises the LCS-based message diff on table-driven cases:
// identical sequences, all-added, all-removed, and a middle insertion.
func TestLcsDiff(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{
			name: "identical",
			a:    []string{"user: hi", "assistant: hello"},
			b:    []string{"user: hi", "assistant: hello"},
			want: []string{"  user: hi", "  assistant: hello"},
		},
		{
			name: "all added",
			a:    nil,
			b:    []string{"user: hi"},
			want: []string{"+ user: hi"},
		},
		{
			name: "all removed",
			a:    []string{"user: hi"},
			b:    nil,
			want: []string{"- user: hi"},
		},
		{
			name: "middle insertion",
			a:    []string{"user: a", "assistant: b"},
			b:    []string{"user: a", "user: extra", "assistant: b"},
			want: []string{"  user: a", "+ user: extra", "  assistant: b"},
		},
		{
			name: "both empty",
			a:    nil,
			b:    nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lcsDiff(tt.a, tt.b)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("lcsDiff(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
