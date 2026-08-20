package adsanalytics

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestReportSchemaDetectsAndNormalizesCSVTSVJSONGzip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "campaign.csv")
	csvData := "date,campaignId,campaignName,impressions,clicks,cost,sales,orders,acos,dailyBudget\n2026-06-01,111,Launch,1000,20,12.50,50.00,2,25%,40\n"
	if err := os.WriteFile(csvPath, []byte(csvData), 0o600); err != nil {
		t.Fatal(err)
	}
	candidates, err := DetectReportKind(csvPath, []string{"sp-campaign-daily", "sp-search-term"})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 || candidates[0].Kind != "sp-campaign-daily" || candidates[0].Confidence != 1 {
		t.Fatalf("campaign candidates = %+v", candidates)
	}
	report, err := NormalizeSchemaReport(csvPath, "sp-campaign-daily", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	rows := PerformanceRowsFromCanonical(report.Rows)
	if len(rows) != 1 || rows[0].CampaignID != "111" || rows[0].Spend != 12.50 {
		t.Fatalf("normalized campaign rows = %+v", rows)
	}

	tsvPath := filepath.Join(dir, "search.tsv")
	tsvData := "campaignId\tcampaignName\tadGroupId\tadGroupName\tsearchTerm\timpressions\tclicks\tcost\tsales\torders\n111\tLaunch\t222\tCore\tbad query\t100\t25\t15\t0\t0\n"
	if err := os.WriteFile(tsvPath, []byte(tsvData), 0o600); err != nil {
		t.Fatal(err)
	}
	search, err := NormalizeSchemaReport(tsvPath, "sp-search-term", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := SearchTermRowsFromCanonical(search.Rows)[0].SearchTerm; got != "bad query" {
		t.Fatalf("search term = %q", got)
	}

	gzPath := filepath.Join(dir, "keywords.json.gz")
	f, err := os.Create(gzPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	if _, err := zw.Write([]byte(`[{"campaignId":"111","campaignName":"Launch","adGroupId":"222","adGroupName":"Core","keywordId":"333","keyword":"blue widget","matchType":"exact","bid":1.2,"clicks":10,"cost":5,"sales":20,"orders":1}]`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	keyword, err := NormalizeSchemaReport(gzPath, "sp-keyword", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := KeywordRowsFromCanonical(keyword.Rows)[0].KeywordID; got != "333" {
		t.Fatalf("keyword id = %q", got)
	}
}

func TestReportSchemaStrictAndAllowPartial(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "partial.csv")
	if err := os.WriteFile(path, []byte("campaignId,campaignName,clicks,cost\n111,Launch,2,1.00\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeSchemaReport(path, "sp-campaign-summary", nil, false); err == nil {
		t.Fatalf("strict validation unexpectedly passed")
	}
	report, err := NormalizeSchemaReport(path, "sp-campaign-summary", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Validation.Partial || len(report.Validation.Missing) == 0 {
		t.Fatalf("partial validation = %+v", report.Validation)
	}
}
