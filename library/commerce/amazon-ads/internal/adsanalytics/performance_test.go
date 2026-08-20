package adsanalytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPerformanceReportCSV(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "performance.csv")
	if err := os.WriteFile(path, []byte(`Campaign Name,Ad Group Name,Advertised ASIN,Date,Hour,Spend,Sales,Orders,Clicks,Impressions,Daily Budget
Core,Exact,B0A,2026-01-05,9,25.00,100.00,4,20,1000,50
Core,Broad,B0A,2026-01-05,10,10.00,40.00,1,10,500,50
Scale,Auto,B0B,2026-01-06,11,60.00,120.00,3,40,2000,75
`), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	rows, err := LoadPerformanceReport(path)
	if err != nil {
		t.Fatalf("LoadPerformanceReport returned error: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	if rows[0].Campaign != "Core" || rows[0].ASIN != "B0A" || rows[0].Spend != 25 {
		t.Fatalf("first row = %+v", rows[0])
	}
	if !rows[0].HasHour || rows[0].Hour != 9 {
		t.Fatalf("hour parsing failed: %+v", rows[0])
	}
}

func TestPortfolioDashboard(t *testing.T) {
	t.Parallel()
	rows := []PerformanceRow{
		{Spend: 25, Sales: 100, Orders: 4, Clicks: 20, Impressions: 1000},
		{Spend: 10, Sales: 40, Orders: 1, Clicks: 10, Impressions: 500},
	}
	got := PortfolioDashboard(rows)
	if got.Spend != 35 || got.Sales != 140 || got.Orders != 5 || got.Clicks != 30 || got.Impressions != 1500 {
		t.Fatalf("summary = %+v", got)
	}
	if got.ACOS != 0.25 || got.CPC != 35.0/30.0 || got.CTR != 0.02 {
		t.Fatalf("derived summary = %+v", got)
	}
}

func TestCampaignComparison(t *testing.T) {
	t.Parallel()
	rows := []PerformanceRow{
		{Campaign: "Core", Spend: 25, Sales: 100, Orders: 4},
		{Campaign: "Core", Spend: 10, Sales: 40, Orders: 1},
		{Campaign: "Scale", Spend: 60, Sales: 120, Orders: 3},
	}
	got := CampaignComparison(rows)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Campaign != "Scale" || got[0].Spend != 60 {
		t.Fatalf("first campaign = %+v", got[0])
	}
	if got[1].Campaign != "Core" || got[1].Spend != 35 {
		t.Fatalf("second campaign = %+v", got[1])
	}
}

func TestProductAdProfitability(t *testing.T) {
	t.Parallel()
	rows := []PerformanceRow{
		{ASIN: "B0A", Spend: 25, Sales: 100, Orders: 4},
		{ASIN: "B0B", Spend: 60, Sales: 120, Orders: 3},
	}
	costs := map[string]ProductCost{
		"B0A": {Name: "A", COGS: 8},
		"B0B": {Name: "B", COGS: 20},
	}
	got := ProductAdProfitability(rows, costs, 30)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ASIN != "B0B" {
		t.Fatalf("least profitable should sort first: %+v", got)
	}
	if got[1].EstimatedProfit != 100-32-30-25 {
		t.Fatalf("B0A profit = %+v", got[1])
	}
}

func TestPlacementAnalysis(t *testing.T) {
	t.Parallel()
	rows := []PerformanceRow{
		{Placement: "Top of Search", Spend: 20, Sales: 100, Orders: 4, Clicks: 10, Impressions: 1000},
		{Placement: "Product Pages", Spend: 30, Sales: 60, Orders: 2, Clicks: 20, Impressions: 2000},
	}
	got := PlacementAnalysis(rows)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Placement != "Product Pages" || got[0].ACOS != 0.5 {
		t.Fatalf("first placement = %+v", got[0])
	}
}

func TestCompetitorASINMining(t *testing.T) {
	t.Parallel()
	rows := []PerformanceRow{
		{ASIN: "B0OWN", TargetASIN: "B0COMP", Spend: 10, Sales: 100, Orders: 3},
		{ASIN: "B0OWN", TargetASIN: "B0COMP", Spend: 5, Sales: 0, Orders: 0},
		{ASIN: "B0OWN", TargetASIN: "B0OWN", Spend: 99, Sales: 99, Orders: 1},
	}
	got := CompetitorASINMining(rows, "B0OWN")
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1: %+v", len(got), got)
	}
	if got[0].ASIN != "B0COMP" || got[0].Spend != 15 || got[0].ACOS != 0.15 {
		t.Fatalf("finding = %+v", got[0])
	}
}

func TestSeasonalPlanner(t *testing.T) {
	t.Parallel()
	rows := []PerformanceRow{
		{Date: "2026-01-15", Spend: 100, Sales: 500, Orders: 10},
		{Date: "2026-01-20", Spend: 50, Sales: 100, Orders: 2},
		{Date: "2026-02-01", Spend: 80, Sales: 0, Orders: 0},
	}
	got := SeasonalPlanner(rows, 1.5)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2: %+v", len(got), got)
	}
	if got[0].Period != "2026-01" || got[0].RecommendedBudget != 225 {
		t.Fatalf("January plan = %+v", got[0])
	}
	if got[1].Period != "2026-02" || got[1].RecommendedBudget != 40 {
		t.Fatalf("February plan = %+v", got[1])
	}
}

func TestDaypartingAnalysis(t *testing.T) {
	t.Parallel()
	rows := []PerformanceRow{
		{Date: "2026-01-05", Hour: 9, HasHour: true, Spend: 10, Sales: 100, Orders: 4, Clicks: 20, Impressions: 1000},
		{Date: "2026-01-05", Hour: 9, HasHour: true, Spend: 5, Sales: 0, Orders: 0, Clicks: 10, Impressions: 500},
		{Date: "2026-01-05", Hour: 20, HasHour: true, Spend: 25, Sales: 0, Orders: 0, Clicks: 20, Impressions: 1000},
	}
	got := DaypartingAnalysis(rows)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2: %+v", len(got), got)
	}
	if got[0].DayOfWeek != "Monday" || got[0].Hour != 9 || got[0].Spend != 15 {
		t.Fatalf("first cell = %+v", got[0])
	}
}

func TestBudgetPacing(t *testing.T) {
	t.Parallel()
	rows := []PerformanceRow{
		{Campaign: "Core", Date: "2026-01-05", Hour: 8, HasHour: true, Spend: 30, Budget: 100},
		{Campaign: "Core", Date: "2026-01-05", Hour: 10, HasHour: true, Spend: 60, Budget: 100},
		{Campaign: "Core", Date: "2026-01-05", Hour: 20, HasHour: true, Spend: 5, Budget: 100},
		{Campaign: "Slow", Date: "2026-01-05", Hour: 22, HasHour: true, Spend: 95, Budget: 100},
	}
	got := BudgetPacing(rows, 0.9, 18)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1: %+v", len(got), got)
	}
	if got[0].Campaign != "Core" || got[0].ExhaustedHour != 10 {
		t.Fatalf("finding = %+v", got[0])
	}
}
