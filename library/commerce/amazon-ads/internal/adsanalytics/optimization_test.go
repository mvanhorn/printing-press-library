package adsanalytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadKeywordPerformanceReportCSV(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "keywords.csv")
	if err := os.WriteFile(path, []byte(`Campaign Name,Ad Group Name,Keyword,Match Type,Bid,Spend,Sales,Orders,Clicks
Core,Exact,self journal,exact,1.00,25.00,100.00,5,50
`), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	rows, err := LoadKeywordPerformanceReport(path)
	if err != nil {
		t.Fatalf("LoadKeywordPerformanceReport returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Keyword != "self journal" || rows[0].ConversionRate != 0.1 || rows[0].AverageOrderVal != 20 {
		t.Fatalf("row = %+v", rows[0])
	}
}

func TestBidOptimizer(t *testing.T) {
	t.Parallel()
	rows := []KeywordPerformance{
		{Keyword: "raise me", Bid: 0.25, Sales: 100, Orders: 5, Clicks: 50},
		{Keyword: "lower me", Bid: 2.00, Sales: 100, Orders: 5, Clicks: 50},
		{Keyword: "skip me", Bid: 1.00, Sales: 0, Orders: 0, Clicks: 50},
	}
	recs := BidOptimizer(rows, 25)
	if len(recs) != 2 {
		t.Fatalf("len(recs) = %d, want 2: %+v", len(recs), recs)
	}
	if recs[0].Keyword != "lower me" || recs[0].Action != "lower" {
		t.Fatalf("first rec = %+v", recs[0])
	}
	if recs[1].Keyword != "raise me" || recs[1].Action != "raise" {
		t.Fatalf("second rec = %+v", recs[1])
	}
}

func TestKeywordDecay(t *testing.T) {
	t.Parallel()
	baseline := []KeywordPerformance{
		{Keyword: "decayed", Spend: 10, Sales: 100, Orders: 10, Clicks: 100},
		{Keyword: "steady", Spend: 10, Sales: 100, Orders: 10, Clicks: 100},
	}
	current := []KeywordPerformance{
		{Keyword: "decayed", Spend: 40, Sales: 100, Orders: 3, Clicks: 100},
		{Keyword: "steady", Spend: 12, Sales: 100, Orders: 10, Clicks: 100},
	}
	got := KeywordDecay(baseline, current, 30, 20)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1: %+v", len(got), got)
	}
	if got[0].Keyword != "decayed" || got[0].ACOSIncreasePercent < 299 {
		t.Fatalf("finding = %+v", got[0])
	}
}

func TestKeywordLifecycle(t *testing.T) {
	t.Parallel()
	rows := []KeywordPerformance{
		{Keyword: "neglected", Spend: 20, Sales: 0, Orders: 0, Clicks: 40},
		{Keyword: "graduate", Spend: 10, Sales: 100, Orders: 3, Clicks: 30},
	}
	got := KeywordLifecycle(rows, 25)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Stage != "neglected" || got[1].Stage != "graduation" {
		t.Fatalf("stages = %+v", got)
	}
}

func TestBidHistory(t *testing.T) {
	t.Parallel()
	rows := []KeywordPerformance{
		{Keyword: "self journal", Bid: 1.25, CPC: 0.80, Spend: 8, Sales: 40, Orders: 2, Clicks: 10},
		{Keyword: "other", Bid: 2.00, Spend: 5, Sales: 0, Orders: 0, Clicks: 5},
	}
	got := BidHistory(rows, "self journal")
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Bid != 1.25 || got[0].ConversionRate != 0.2 || got[0].ACOS != 0.2 {
		t.Fatalf("history point = %+v", got[0])
	}
}
