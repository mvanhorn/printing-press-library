// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelCompareHelpWires smoke-tests that the compare command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCompareHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"compare", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "compare"} {
		if !strings.Contains(help, want) {
			t.Fatalf("compare --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestSplitTrim(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ", []string{"a", "b"}},
		{"single", []string{"single"}},
		{"", nil},
		{",,", nil},
	}
	for _, c := range cases {
		got := splitTrim(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("splitTrim(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("splitTrim(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}

func TestRound2(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{1.23456, 1.23},
		{99.999, 100.0},
		{0.005, 0.01},
		{42.0, 42.0},
	}
	for _, c := range cases {
		if got := round2(c.in); got != c.want {
			t.Fatalf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
