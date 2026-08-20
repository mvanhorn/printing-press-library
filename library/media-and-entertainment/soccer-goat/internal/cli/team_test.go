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

// TestNovelTeamHelpWires smoke-tests that the team command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTeamHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"team", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("team --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "team"} {
		if !strings.Contains(help, want) {
			t.Fatalf("team --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestPrintTeamReportSortsByMarketValue(t *testing.T) {
	team := &report.TeamReport{
		ClubName: "Benfica", SquadValueLabel: "€15.00m",
		Players: []report.PlayerReport{
			{Name: "Lower", MarketValue: 5_000_000, MarketValueLabel: "€5.00m", Position: "CB"},
			{Name: "Higher", MarketValue: 10_000_000, MarketValueLabel: "€10.00m", Position: "FW", EAOverall: 80, Potential: 88},
		},
	}
	var out bytes.Buffer
	if err := printTeamReport(&out, team); err != nil {
		t.Fatalf("printTeamReport error = %v", err)
	}
	text := out.String()
	if strings.Index(text, "Higher") > strings.Index(text, "Lower") {
		t.Fatalf("players not sorted by descending value:\n%s", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "Lower") && strings.Join(strings.Fields(line), " ") != "Lower €5.00m CB - -" {
			t.Fatalf("missing-rating placeholders not rendered in row %q", line)
		}
	}
}
