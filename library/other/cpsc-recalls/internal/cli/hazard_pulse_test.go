// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelHazardPulseHelpWires smoke-tests that the hazard-pulse command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelHazardPulseHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"hazard-pulse", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hazard-pulse --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "hazard-pulse"} {
		if !strings.Contains(help, want) {
			t.Fatalf("hazard-pulse --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestAddDistinctRecallLabelsNormalizesAndCountsOncePerRecall(t *testing.T) {
	counts := map[string]int{}
	addDistinctRecallLabels(counts, []string{"Fire", "FIRE", " fire "})
	if len(counts) != 1 || counts["fire"] != 1 {
		t.Fatalf("counts=%v", counts)
	}
}
