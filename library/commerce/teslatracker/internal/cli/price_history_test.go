// Copyright 2026 michegz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelPriceHistoryHelpWires smoke-tests that the price-history command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPriceHistoryHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"price-history", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("price-history --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "price-history"} {
		if !strings.Contains(help, want) {
			t.Fatalf("price-history --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestLandedPriceCuts_IncreaseIsNotACut(t *testing.T) {
	pts := []pricePoint{
		{LandedUSD: 40000, At: "2026-01-01T00:00:00Z"},
		{LandedUSD: 42000, At: "2026-02-01T00:00:00Z"},
		{LandedUSD: 42000, At: "2026-03-01T00:00:00Z"},
	}
	if got := landedPriceCuts(pts); len(got) != 0 {
		t.Fatalf("increase-only path: cuts = %+v, want none", got)
	}
}

func TestLandedPriceCuts_DecreaseIsACut(t *testing.T) {
	pts := []pricePoint{
		{LandedUSD: 42000, At: "2026-01-01T00:00:00Z"},
		{LandedUSD: 40000, At: "2026-02-01T00:00:00Z"},
		{LandedUSD: 41000, At: "2026-03-01T00:00:00Z"},
		{LandedUSD: 39000, At: "2026-04-01T00:00:00Z"},
	}
	got := landedPriceCuts(pts)
	if len(got) != 2 {
		t.Fatalf("mixed path: got %d cuts %+v, want 2 decreases", len(got), got)
	}
	if got[0].FromUSD != 42000 || got[0].ToUSD != 40000 || got[0].DeltaUSD != -2000 {
		t.Fatalf("first cut = %+v, want 42000 -> 40000 (-2000)", got[0])
	}
	if got[1].FromUSD != 41000 || got[1].ToUSD != 39000 || got[1].DeltaUSD != -2000 {
		t.Fatalf("second cut = %+v, want 41000 -> 39000 (-2000)", got[1])
	}
}

func TestPriceHistory_IncreaseOnlyIsNotRecordedAsCut(t *testing.T) {
	vin := "5YJ3E1EA7LF000001"
	dbPath := seedVehicleMirror(t, Vehicle{
		VIN:   vin,
		Year:  intPtr(2021),
		Model: "Model 3",
		Trim:  "LR",
		PriceHistory: []struct {
			Price     *int64 `json:"price"`
			ScrapedAt string `json:"scrapedAt"`
		}{
			{Price: int64Ptr(4_000_000), ScrapedAt: "2026-01-01T00:00:00Z"},
			{Price: int64Ptr(4_200_000), ScrapedAt: "2026-02-01T00:00:00Z"},
		},
	})

	stdout, stderr, err := runRootArgs(t, "price-history", vin, "--db", dbPath, "--json", "--no-learn")
	if err != nil {
		t.Fatalf("price-history: %v (stderr=%q stdout=%q)", err, stderr, stdout)
	}
	var view struct {
		Changes []landedCut `json:"changes"`
		Note    string      `json:"note"`
	}
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("decode price-history JSON: %v (stdout=%q)", err, stdout)
	}
	if len(view.Changes) != 0 {
		t.Fatalf("increase-only vehicle recorded cuts %+v; a raise is not a cut", view.Changes)
	}
	if !strings.Contains(view.Note, "no price cut") {
		t.Fatalf("note = %q, want a no-cut explanation", view.Note)
	}
}
