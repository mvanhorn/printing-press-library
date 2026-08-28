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

// TestNovelStaleHelpWires smoke-tests that the stale command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelStaleHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"stale", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stale --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "stale"} {
		if !strings.Contains(help, want) {
			t.Fatalf("stale --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestStale_IncreaseOnlySurvivesNeverCut(t *testing.T) {
	raised := "5YJ3E1EA7LF000011"
	cut := "5YJ3E1EA7LF000012"
	dbPath := seedVehicleMirror(t,
		Vehicle{
			VIN:   raised,
			Year:  intPtr(2021),
			Model: "Model 3",
			Trim:  "LR",
			PriceHistory: []struct {
				Price     *int64 `json:"price"`
				ScrapedAt string `json:"scrapedAt"`
			}{
				{Price: int64Ptr(4_000_000), ScrapedAt: "2026-01-01T00:00:00Z"},
				{Price: int64Ptr(4_250_000), ScrapedAt: "2026-02-01T00:00:00Z"},
			},
		},
		Vehicle{
			VIN:   cut,
			Year:  intPtr(2021),
			Model: "Model 3",
			Trim:  "LR",
			PriceHistory: []struct {
				Price     *int64 `json:"price"`
				ScrapedAt string `json:"scrapedAt"`
			}{
				{Price: int64Ptr(4_200_000), ScrapedAt: "2026-01-01T00:00:00Z"},
				{Price: int64Ptr(3_900_000), ScrapedAt: "2026-02-01T00:00:00Z"},
			},
		},
	)

	stdout, stderr, err := runRootArgs(t, "stale", "--never-cut", "--db", dbPath, "--json", "--no-learn")
	if err != nil {
		t.Fatalf("stale --never-cut: %v (stderr=%q stdout=%q)", err, stderr, stdout)
	}
	var view struct {
		Cars []struct {
			VIN       string `json:"vin"`
			PriceCuts int    `json:"price_cuts"`
			NeverCut  bool   `json:"never_cut"`
		} `json:"cars"`
	}
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("decode stale JSON: %v (stdout=%q)", err, stdout)
	}

	byVIN := map[string]struct {
		PriceCuts int
		NeverCut  bool
	}{}
	for _, c := range view.Cars {
		byVIN[c.VIN] = struct {
			PriceCuts int
			NeverCut  bool
		}{c.PriceCuts, c.NeverCut}
	}
	got, ok := byVIN[raised]
	if !ok {
		t.Fatalf("increase-only VIN %s omitted from --never-cut; PriceCuts must ignore raises", raised)
	}
	if got.PriceCuts != 0 || !got.NeverCut {
		t.Fatalf("increase-only VIN %s: price_cuts=%d never_cut=%t, want 0 / true", raised, got.PriceCuts, got.NeverCut)
	}
	if _, found := byVIN[cut]; found {
		t.Fatalf("VIN %s had a real cut and should be excluded by --never-cut", cut)
	}
}
