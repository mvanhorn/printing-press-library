package adsanalytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadShareOfVoiceReportCSV(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sov.csv")
	if err := os.WriteFile(path, []byte(`ASIN,Keyword,Impressions,Impression Share,Rank
B0A,self journal,1000,12%,2
B0A,daily planner,500,0.05,6
`), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	rows, err := LoadShareOfVoiceReport(path)
	if err != nil {
		t.Fatalf("LoadShareOfVoiceReport returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].ImpressionShare != 0.12 || rows[1].ImpressionShare != 0.05 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestShareOfVoice(t *testing.T) {
	t.Parallel()
	rows := []ShareOfVoiceRow{
		{ASIN: "B0A", Keyword: "self journal", Impressions: 1000, ImpressionShare: 0.12, Rank: 2},
		{ASIN: "B0A", Keyword: "self journal", Impressions: 500, ImpressionShare: 0.08, Rank: 3},
		{ASIN: "B0B", Keyword: "self journal", Impressions: 9999, ImpressionShare: 0.50, Rank: 1},
	}
	got := ShareOfVoice(rows, "B0A", []string{"self journal"}, 0.11)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1: %+v", len(got), got)
	}
	if got[0].AverageShare != 0.10 || got[0].BestRank != 2 || got[0].Impressions != 1500 {
		t.Fatalf("finding = %+v", got[0])
	}
	if got[0].LowVisibilityReason == "" {
		t.Fatalf("expected low visibility reason: %+v", got[0])
	}
}
