// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"context"
	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
	"os"
)

// TestNovelPriceHistoryHelpWires smoke-tests that the price history command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPriceHistoryHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"price", "history", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("price history --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "history"} {
		if !strings.Contains(help, want) {
			t.Fatalf("price history --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestRecordPricesHonoursLocalDataOptOut(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("REWE_NO_LOCAL_DATA", "1")
	recordPrices(context.Background(), []rewe.Product{{ProductID: "7381513", Name: "Hafer Drink", PriceCents: 189}}, "3200008")
	if _, err := os.Stat(defaultDBPath("rewe-pp-cli")); err == nil {
		t.Fatal("recordPrices wrote the local store although REWE_NO_LOCAL_DATA=1")
	}
	if err := recordOffers(context.Background(), []rewe.Offer{{NAN: "1", Week: "2026/47", Market: "3200008", Title: "x", PriceCents: 99}}); err != nil {
		t.Fatalf("recordOffers with opt-out: %v", err)
	}
	if _, err := os.Stat(defaultDBPath("rewe-pp-cli")); err == nil {
		t.Fatal("recordOffers wrote the local store although REWE_NO_LOCAL_DATA=1")
	}
}
