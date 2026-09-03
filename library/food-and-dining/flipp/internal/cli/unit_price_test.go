// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelUnitPriceHelpWires smoke-tests that the unit-price command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelUnitPriceHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"unit-price", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unit-price --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "unit-price"} {
		if !strings.Contains(help, want) {
			t.Fatalf("unit-price --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestParseUnitPriceHandlesLiquidGrocerySizes(t *testing.T) {
	price := 3.99
	info := parseUnitPrice("Producers Dairy Whole Milk (1 Gallon)", &price)
	if info.Value == nil {
		t.Fatalf("expected unit price, got warning %q", info.Warning)
	}
	if info.Unit != "gal" {
		t.Fatalf("unit = %q, want gal", info.Unit)
	}
	if got, want := *info.Value, 3.99; got != want {
		t.Fatalf("value = %.2f, want %.2f", got, want)
	}

	half := parseUnitPrice("Horizon Organic Whole Milk High Vitamin D (1/2 Gallon)", &price)
	if half.Value == nil {
		t.Fatalf("expected fractional gallon unit price, got warning %q", half.Warning)
	}
	if got, want := *half.Value, 7.98; got != want {
		t.Fatalf("fractional value = %.2f, want %.2f", got, want)
	}
}

func TestMatchesSearchIntentRejectsMilkCandyForMilkQuery(t *testing.T) {
	if matchesSearchIntent(flippItem{Name: "LINDT MILK CHG"}, "milk") {
		t.Fatal("milk chocolate abbreviation should not satisfy a plain milk staple query")
	}
	if !matchesSearchIntent(flippItem{Name: "Producers Dairy Whole Milk (1 Gallon)"}, "milk") {
		t.Fatal("whole milk should satisfy a plain milk staple query")
	}
}
