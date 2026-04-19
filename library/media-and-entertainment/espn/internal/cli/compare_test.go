package cli

import (
	"strings"
	"testing"
)

func TestCompareCmdRequiresTwoNames(t *testing.T) {
	cmd := newCompareCmd(&rootFlags{})
	cmd.SetArgs([]string{"Mahomes"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing second athlete name")
	}
}

func TestCompareCmdRequiresSportLeague(t *testing.T) {
	cmd := newCompareCmd(&rootFlags{})
	cmd.SetArgs([]string{"Mahomes", "Allen"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --sport/--league")
	}
	if !strings.Contains(err.Error(), "sport") {
		t.Errorf("expected sport-related error, got %v", err)
	}
}

func TestRenderCompareNoStats(t *testing.T) {
	a := &athleteResult{ID: "1", DisplayName: "A", Position: "QB", TeamAbbr: "KC", Stats: map[string]string{}}
	b := &athleteResult{ID: "2", DisplayName: "B", Position: "QB", TeamAbbr: "BUF", Stats: map[string]string{}}
	cmd := newCompareCmd(&rootFlags{})
	var sb strings.Builder
	cmd.SetOut(&sb)
	if err := renderCompare(cmd, a, b); err != nil {
		t.Fatalf("renderCompare: %v", err)
	}
	if !strings.Contains(sb.String(), "No season stats") {
		t.Errorf("expected hint about missing stats, got: %s", sb.String())
	}
}

func TestRenderCompareWithStats(t *testing.T) {
	a := &athleteResult{ID: "1", DisplayName: "A", Position: "QB", TeamAbbr: "KC",
		Stats: map[string]string{"passing.YDS": "4000"}}
	b := &athleteResult{ID: "2", DisplayName: "B", Position: "QB", TeamAbbr: "BUF",
		Stats: map[string]string{"passing.YDS": "4200"}}
	cmd := newCompareCmd(&rootFlags{})
	var sb strings.Builder
	cmd.SetOut(&sb)
	if err := renderCompare(cmd, a, b); err != nil {
		t.Fatalf("renderCompare: %v", err)
	}
	out := sb.String()
	for _, want := range []string{"4000", "4200", "passing.YDS"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %s", want, out)
		}
	}
}
