// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelTrendsGeoGapHelpWires smoke-tests that the trends geo-gap command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTrendsGeoGapHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"trends", "geo-gap", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trends geo-gap --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "geo-gap"} {
		if !strings.Contains(help, want) {
			t.Fatalf("trends geo-gap --help missing %q in output:\n%s", want, help)
		}
	}
}
