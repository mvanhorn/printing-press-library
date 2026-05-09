// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGoogleFlightsPriceCommandsExposeCurrencyFlag(t *testing.T) {
	var flags rootFlags
	commands := map[string]func(*rootFlags) *cobra.Command{
		"flights":           newGfFlightsCmd,
		"dates":             newGfDatesCmd,
		"compare":           newCompareCmd,
		"gf-search":         newGfSearchCmd,
		"cheapest-longhaul": newCheapestLonghaulCmd,
	}

	for name, build := range commands {
		cmd := build(&flags)
		if cmd.Flags().Lookup("currency") == nil {
			t.Fatalf("%s command missing --currency flag", name)
		}
	}
}

func TestFormatPriceDefaultsToUSD(t *testing.T) {
	if got := formatPrice("", 123); got != "USD 123" {
		t.Fatalf("formatPrice(empty, 123) = %q, want USD 123", got)
	}
	if got := formatPrice("gbp", 45); got != "GBP 45" {
		t.Fatalf("formatPrice(gbp, 45) = %q, want GBP 45", got)
	}
}

func TestCheapestLonghaulRejectsInvalidCurrencyBeforeAPISetup(t *testing.T) {
	var flags rootFlags
	cmd := newCheapestLonghaulCmd(&flags)
	cmd.SetArgs([]string{
		"SEA",
		"--from", "2026-05-01",
		"--to", "2026-05-31",
		"--currency", "NOTACODE",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("cheapest-longhaul accepted invalid currency, want error")
	}
	if !strings.Contains(err.Error(), "ISO 4217") {
		t.Fatalf("error %q did not mention ISO 4217", err)
	}
}
