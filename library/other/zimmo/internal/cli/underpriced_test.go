// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

// TestNovelUnderpricedHelpWires smoke-tests that the underpriced command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelUnderpricedHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"underpriced", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("underpriced --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "underpriced"} {
		if !strings.Contains(help, want) {
			t.Fatalf("underpriced --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestTypeSpecificCommunePPM2SkipsMixedAverage(t *testing.T) {
	mixed := zimmo.LocalityPrice{Price: 4000, Types: []zimmo.TypePrice{{Type: "HOUSE", Price: 5000, Count: 12}}}
	if price, n, ok := typeSpecificCommunePPM2(mixed, "APARTMENT"); ok || price != 0 || n != 0 {
		t.Fatalf("missing apartment price must not fall back to the all-types average, got %v %d %v", price, n, ok)
	}
	if price, n, ok := typeSpecificCommunePPM2(mixed, "HOUSE"); !ok || price != 5000 || n != 12 {
		t.Fatalf("type-specific house price must be used, got %v %d %v", price, n, ok)
	}
	if _, _, ok := typeSpecificCommunePPM2(zimmo.LocalityPrice{Price: 4000}, "HOUSE"); ok {
		t.Fatal("all-types price alone is not a house or apartment benchmark")
	}
}
