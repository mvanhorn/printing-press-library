// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelPronCheckHelpWires smoke-tests that the pron-check command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPronCheckHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"pron-check", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pron-check --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "pron-check"} {
		if !strings.Contains(help, want) {
			t.Fatalf("pron-check --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestPronunciationMatchesNormalizedText(t *testing.T) {
	for _, tc := range []struct {
		expected string
		spoken   string
		want     bool
	}{
		{expected: "SarvamPay", spoken: "sarvampay.", want: true},
		{expected: "B2B", spoken: "The term is B2B!", want: true},
		{expected: "pay", spoken: "repayment", want: false},
		{expected: "pay", spoken: "do not pay", want: true},
		{expected: "SarvamPay", spoken: "something else", want: false},
		{expected: "माला", spoken: "माला", want: true},
		{expected: "माला", spoken: "माली", want: false},
		{expected: "", spoken: "anything", want: false},
	} {
		if got := pronunciationMatches(tc.expected, tc.spoken); got != tc.want {
			t.Errorf("pronunciationMatches(%q, %q) = %v, want %v", tc.expected, tc.spoken, got, tc.want)
		}
	}
}
