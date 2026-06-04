package adsanalytics

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeReportReadsGzipSearchTerms(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "search.csv.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gzip report: %v", err)
	}
	zw := gzip.NewWriter(f)
	if _, err := zw.Write([]byte(`Campaign Name,Ad Group Name,Customer Search Term,Spend,Sales,Orders,Clicks
Core,Auto,self journal,12.50,100.00,4,20
`)); err != nil {
		t.Fatalf("write gzip report: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close gzip file: %v", err)
	}

	report, err := NormalizeReport(path, "search-terms")
	if err != nil {
		t.Fatalf("NormalizeReport returned error: %v", err)
	}
	if report.Kind != "search-terms" || report.RowCount != 1 || report.ID == "" {
		t.Fatalf("report = %+v", report)
	}
}

func TestNormalizeGenericCSVReport(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "generic.csv")
	if err := os.WriteFile(path, []byte("Campaign Name,Spend\nCore,12.50\n"), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	report, err := NormalizeReport(path, "generic")
	if err != nil {
		t.Fatalf("NormalizeReport returned error: %v", err)
	}
	if report.RowCount != 1 {
		t.Fatalf("RowCount = %d, want 1", report.RowCount)
	}
}
