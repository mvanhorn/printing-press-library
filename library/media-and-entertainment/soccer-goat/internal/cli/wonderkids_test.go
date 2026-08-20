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

// TestNovelWonderkidsHelpWires smoke-tests that the wonderkids command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWonderkidsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"wonderkids", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("wonderkids --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "wonderkids"} {
		if !strings.Contains(help, want) {
			t.Fatalf("wonderkids --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestRankWonderkidsPrioritizesPotentialAndKeepsMissingPotential(t *testing.T) {
	team := &report.TeamReport{ClubName: "Benfica", Players: []report.PlayerReport{
		{Name: "No Potential", Age: 19, MarketValue: 30_000_000, MarketValueLabel: "€30.00m"},
		{Name: "Top Potential", Age: 20, Potential: 90, MarketValue: 5_000_000, MarketValueLabel: "€5.00m"},
		{Name: "Too Old", Age: 22, Potential: 95, MarketValue: 50_000_000, MarketValueLabel: "€50.00m"},
	}}
	result := rankWonderkids(team, 21, 15)
	if len(result.Wonderkids) != 2 {
		t.Fatalf("wonderkids = %#v, want two eligible players", result.Wonderkids)
	}
	if result.Wonderkids[0].Name != "Top Potential" || result.Wonderkids[1].Name != "No Potential" {
		t.Fatalf("wonderkid order = %#v", result.Wonderkids)
	}
}
