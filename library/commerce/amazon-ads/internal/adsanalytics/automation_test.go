package adsanalytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutoNegate(t *testing.T) {
	t.Parallel()
	rows := []SearchTermPerformance{
		{SearchTerm: "bad", Spend: 20, Clicks: 25, Conversions: 0},
		{SearchTerm: "cheap", Spend: 5, Clicks: 25, Conversions: 0},
		{SearchTerm: "converted", Spend: 50, Clicks: 25, Conversions: 1},
	}
	plans := AutoNegate(rows, 15, 20)
	if len(plans) != 1 {
		t.Fatalf("len(plans) = %d, want 1", len(plans))
	}
	if plans[0].SearchTerm != "bad" || plans[0].Action != "create_negative_keyword" {
		t.Fatalf("plan = %+v", plans[0])
	}
}

func TestAutoPromote(t *testing.T) {
	t.Parallel()
	rows := []SearchTermPerformance{
		{SearchTerm: "good", Spend: 10, Sales: 100, Conversions: 4},
		{SearchTerm: "few", Spend: 10, Sales: 100, Conversions: 1},
		{SearchTerm: "expensive", Spend: 80, Sales: 100, Conversions: 4},
	}
	plans := AutoPromote(rows, 3, 25)
	if len(plans) != 1 {
		t.Fatalf("len(plans) = %d, want 1", len(plans))
	}
	if plans[0].SearchTerm != "good" || plans[0].Action != "create_exact_keyword" {
		t.Fatalf("plan = %+v", plans[0])
	}
}

func TestBudgetRebalance(t *testing.T) {
	t.Parallel()
	campaigns := []CampaignSummary{
		{Campaign: "Winner", PortfolioSummary: PortfolioSummary{Spend: 20, Sales: 200, ACOS: 0.10}, Budget: 50},
		{Campaign: "Loser", PortfolioSummary: PortfolioSummary{Spend: 50, Sales: 100, ACOS: 0.50}, Budget: 50},
	}
	plans := BudgetRebalance(campaigns, 100)
	if len(plans) != 2 {
		t.Fatalf("len(plans) = %d, want 2", len(plans))
	}
	if plans[0].Campaign != "Winner" || plans[0].Action != "increase" {
		t.Fatalf("first plan = %+v", plans[0])
	}
	if plans[1].Campaign != "Loser" || plans[1].Action != "decrease" {
		t.Fatalf("second plan = %+v", plans[1])
	}
}

func TestLoadBidRulesAndApplyBidRules(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(path, []byte(`{
  "rules": [
    {"name":"trim high acos","min_acos":35,"min_spend":50,"action":"decrease","change_percent":15},
    {"name":"scale winner","max_acos":20,"min_orders":2,"action":"increase","change_percent":10}
  ]
}`), 0o600); err != nil {
		t.Fatalf("write rules: %v", err)
	}
	rules, err := LoadBidRules(path)
	if err != nil {
		t.Fatalf("LoadBidRules returned error: %v", err)
	}
	rows := []KeywordPerformance{
		{Keyword: "bad", Bid: 1.00, Spend: 60, Sales: 100, Orders: 1},
		{Keyword: "good", Bid: 1.50, Spend: 20, Sales: 200, Orders: 4},
	}
	plans := ApplyBidRules(rows, rules)
	if len(plans) != 2 {
		t.Fatalf("len(plans) = %d, want 2: %+v", len(plans), plans)
	}
	if plans[0].Keyword != "good" || plans[0].RecommendedBid < 1.649 || plans[0].RecommendedBid > 1.651 {
		t.Fatalf("first plan = %+v", plans[0])
	}
	if plans[1].Keyword != "bad" || plans[1].RecommendedBid < 0.849 || plans[1].RecommendedBid > 0.851 {
		t.Fatalf("second plan = %+v", plans[1])
	}
}
