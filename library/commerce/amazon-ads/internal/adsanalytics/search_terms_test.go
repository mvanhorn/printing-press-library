package adsanalytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSearchTermReportCSV(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "search_terms.csv")
	if err := os.WriteFile(path, []byte(`Campaign Name,Ad Group Name,Customer Search Term,Spend,Sales,Orders,Clicks,Impressions
Core,Auto,self journal,12.50,100.00,4,20,1000
Core,Auto,bad match,15.00,0,0,30,1200
`), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	rows, err := LoadSearchTermReport(path)
	if err != nil {
		t.Fatalf("LoadSearchTermReport returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].SearchTerm != "self journal" || rows[0].Spend != 12.50 || rows[0].Conversions != 4 {
		t.Fatalf("first row = %+v", rows[0])
	}
}

func TestSearchTermMining(t *testing.T) {
	t.Parallel()
	rows := []SearchTermPerformance{
		{Campaign: "Core", SearchTerm: "self journal", Spend: 12.50, Sales: 100, Conversions: 4},
		{Campaign: "Core", SearchTerm: "bad match", Spend: 15, Sales: 0, Conversions: 0},
		{Campaign: "Core", SearchTerm: "too expensive", Spend: 40, Sales: 50, Conversions: 4},
	}
	recs := SearchTermMining(rows, 3, 10, 25)
	if len(recs) != 2 {
		t.Fatalf("len(recs) = %d, want 2: %+v", len(recs), recs)
	}
	if recs[0].SearchTerm != "bad match" || recs[0].Action != "negative_exact" {
		t.Fatalf("first rec = %+v", recs[0])
	}
	if recs[1].SearchTerm != "self journal" || recs[1].Action != "promote_exact" {
		t.Fatalf("second rec = %+v", recs[1])
	}
}

func TestWastedSpend(t *testing.T) {
	t.Parallel()
	rows := []SearchTermPerformance{
		{SearchTerm: "small waste", Spend: 5, Conversions: 0},
		{SearchTerm: "big waste", Spend: 20, Conversions: 0},
		{SearchTerm: "converted", Spend: 30, Conversions: 1},
	}
	recs := WastedSpend(rows, 10)
	if len(recs) != 1 {
		t.Fatalf("len(recs) = %d, want 1", len(recs))
	}
	if recs[0].SearchTerm != "big waste" {
		t.Fatalf("rec = %+v", recs[0])
	}
}

func TestKeywordCannibalization(t *testing.T) {
	t.Parallel()
	rows := []SearchTermPerformance{
		{Campaign: "Exact", SearchTerm: "self journal", Spend: 10, Sales: 100, Clicks: 10},
		{Campaign: "Broad", SearchTerm: "self journal", Spend: 25, Sales: 50, Clicks: 20},
		{Campaign: "Auto", SearchTerm: "self journal", Spend: 15, Sales: 0, Clicks: 10},
		{Campaign: "Exact", SearchTerm: "unique", Spend: 5, Sales: 20, Clicks: 5},
	}
	got := KeywordCannibalization(rows)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1: %+v", len(got), got)
	}
	if got[0].SearchTerm != "self journal" || got[0].WinnerCampaign != "Exact" {
		t.Fatalf("finding = %+v", got[0])
	}
	if got[0].LoserSpend != 40 {
		t.Fatalf("LoserSpend = %v, want 40", got[0].LoserSpend)
	}
}

func TestNewKeywordOpportunities(t *testing.T) {
	t.Parallel()
	rows := []SearchTermPerformance{
		{Campaign: "Auto", AdGroup: "Discovery", SearchTerm: "self journal", Keyword: "auto", Spend: 15, Sales: 100, Conversions: 4},
		{Campaign: "Exact", AdGroup: "Exact", SearchTerm: "covered term", Keyword: "covered term", Spend: 5, Sales: 50, Conversions: 3},
		{Campaign: "Auto", AdGroup: "Discovery", SearchTerm: "covered term", Keyword: "auto", Spend: 5, Sales: 50, Conversions: 3},
		{Campaign: "Auto", AdGroup: "Discovery", SearchTerm: "too costly", Keyword: "auto", Spend: 50, Sales: 100, Conversions: 4},
	}
	got := NewKeywordOpportunities(rows, 3, 25)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1: %+v", len(got), got)
	}
	if got[0].SearchTerm != "self journal" || got[0].SuggestedMatch != "exact" {
		t.Fatalf("opportunity = %+v", got[0])
	}
}
