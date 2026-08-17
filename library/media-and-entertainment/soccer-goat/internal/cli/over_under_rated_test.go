// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/report"
)

// TestNovelOverUnderRatedHelpWires smoke-tests that the over-under-rated command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelOverUnderRatedHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"over-under-rated", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("over-under-rated --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "over-under-rated"} {
		if !strings.Contains(help, want) {
			t.Fatalf("over-under-rated --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestRankRatingDivergenceSplitsDirectionsAndSkipsMissingData(t *testing.T) {
	team := &report.TeamReport{ClubName: "Benfica", Players: []report.PlayerReport{
		{Name: "Low Value Star", MarketValue: 1_000_000, MarketValueLabel: "€1.00m", EAOverall: 90},
		{Name: "Middle", MarketValue: 5_000_000, MarketValueLabel: "€5.00m", EAOverall: 50},
		{Name: "High Value Prospect", MarketValue: 20_000_000, MarketValueLabel: "€20.00m", EAOverall: 60},
		{Name: "Missing Rating", MarketValue: 2_000_000, MarketValueLabel: "€2.00m"},
	}}
	result := rankRatingDivergence(team, 10)
	if result.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", result.Skipped)
	}
	if len(result.MarketHyped) != 1 || result.MarketHyped[0].Name != "High Value Prospect" {
		t.Fatalf("market-hyped = %#v", result.MarketHyped)
	}
	if len(result.MarketBargains) != 2 || result.MarketBargains[0].Name != "Low Value Star" {
		t.Fatalf("market-bargains = %#v", result.MarketBargains)
	}
}

func TestRankRatingDivergenceDoesNotMislabelNeutralScore(t *testing.T) {
	team := &report.TeamReport{ClubName: "Benfica", Players: []report.PlayerReport{
		{Name: "Neutral", MarketValue: 1_000_000, MarketValueLabel: "€1.00m", EAOverall: 99},
	}}
	result := rankRatingDivergence(team, 10)
	if len(result.MarketHyped) != 0 || len(result.MarketBargains) != 0 {
		t.Fatalf("neutral score was categorized: hyped=%#v bargains=%#v", result.MarketHyped, result.MarketBargains)
	}
}
