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

	mixed := parseUnitPrice("Family Milk (1 1/2 Gallon)", &price)
	if mixed.Value == nil {
		t.Fatalf("expected mixed-number unit price, got warning %q", mixed.Warning)
	}
	if got, want := *mixed.Value, 2.66; got < want-0.001 || got > want+0.001 {
		t.Fatalf("mixed-number value = %.3f, want %.3f", got, want)
	}
	for _, quantity := range []string{"1 1/2", "1 1/ 2", "1 1 /2", "1 1 / 2", "1   1  /  2"} {
		info := parseUnitPrice("Family Milk ("+quantity+" Gallon)", &price)
		if info.Value == nil || *info.Value < 2.659 || *info.Value > 2.661 {
			t.Fatalf("mixed quantity %q gave %+v, want 1.5 gallons at 2.66 per gallon", quantity, info)
		}
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
