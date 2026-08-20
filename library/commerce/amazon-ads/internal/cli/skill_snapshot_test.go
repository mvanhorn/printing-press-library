package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSkillStartHereProfitabilitySnapshot(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../SKILL.md")
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "## Start here: profitability") {
		t.Fatalf("SKILL.md missing Start here profitability section")
	}
	commands := []string{
		"break-even-acos",
		"true-profit",
		"acos-vs-tacos",
		"portfolio-dashboard",
		"product-ad-profitability",
		"campaign-comparison",
	}
	last := -1
	for _, command := range commands {
		idx := strings.Index(text, "`amazon-ads-pp-cli "+command)
		if idx < 0 {
			t.Fatalf("SKILL.md profitability section missing %s", command)
		}
		if idx < last {
			t.Fatalf("SKILL.md profitability commands out of order at %s", command)
		}
		last = idx
	}
	for _, phrase := range []string{
		"organic + advertising",
		"does not yet cover organic sessions",
		"parent/child ASIN rollups",
		"amazon-seller-pp-cli sales-intel dashboard",
	} {
		if !strings.Contains(text, phrase) {
			t.Fatalf("SKILL.md missing profitability scope phrase %q", phrase)
		}
	}
}
