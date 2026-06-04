package research

import (
	"encoding/json"
	"testing"
)

func TestNormalizeRecordsPreservesEvidenceFields(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"listing_id":55043301,"title":"Teacher Gift Mug","tags":["teacher","gift"],"price":19.95}`),
	}

	evidence, coverage := NormalizeRecords("product_analytics", raw)

	if len(evidence) != 1 {
		t.Fatalf("evidence = %d, want 1", len(evidence))
	}
	if evidence[0].ID != "55043301" {
		t.Fatalf("id = %q, want 55043301", evidence[0].ID)
	}
	if evidence[0].ListingID != "55043301" {
		t.Fatalf("listing id = %q, want 55043301", evidence[0].ListingID)
	}
	if evidence[0].Title != "Teacher Gift Mug" {
		t.Fatalf("title = %q, want Teacher Gift Mug", evidence[0].Title)
	}
	if len(evidence[0].Tags) != 2 || evidence[0].Tags[0] != "teacher" || evidence[0].Tags[1] != "gift" {
		t.Fatalf("tags = %#v, want teacher and gift", evidence[0].Tags)
	}
	if evidence[0].Price == nil || *evidence[0].Price != 19.95 {
		t.Fatalf("price = %#v, want 19.95", evidence[0].Price)
	}
	if coverage.RawRecordCount != 1 || coverage.EvidenceRecordCount != 1 {
		t.Fatalf("coverage = %#v, want one raw and one evidence", coverage)
	}
	if coverage.ResourceCounts["product_analytics"] != 1 {
		t.Fatalf("resource count = %d, want 1", coverage.ResourceCounts["product_analytics"])
	}
}

func TestNormalizeRecordsExtractsKeywordsAndMetrics(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"kw-1","keyword":"teacher mug","shopName":"GiftShop","estimated_sales":42,"estimatedRevenue":"123.45","rank":7}`),
	}

	evidence, coverage := NormalizeRecords("keyword_research", raw)

	if len(evidence) != 1 {
		t.Fatalf("evidence = %d, want 1", len(evidence))
	}
	if evidence[0].ID != "kw-1" {
		t.Fatalf("id = %q, want kw-1", evidence[0].ID)
	}
	if evidence[0].ShopName != "GiftShop" {
		t.Fatalf("shop name = %q, want GiftShop", evidence[0].ShopName)
	}
	if len(evidence[0].Keywords) != 1 || evidence[0].Keywords[0] != "teacher mug" {
		t.Fatalf("keywords = %#v, want teacher mug", evidence[0].Keywords)
	}
	if evidence[0].EstimatedSales == nil || *evidence[0].EstimatedSales != 42 {
		t.Fatalf("estimated sales = %#v, want 42", evidence[0].EstimatedSales)
	}
	if evidence[0].EstimatedRevenue == nil || *evidence[0].EstimatedRevenue != 123.45 {
		t.Fatalf("estimated revenue = %#v, want 123.45", evidence[0].EstimatedRevenue)
	}
	if evidence[0].Rank != 7 {
		t.Fatalf("rank = %d, want 7", evidence[0].Rank)
	}
	if ConfidenceForCoverage(coverage) != 1 {
		t.Fatalf("confidence = %f, want 1", ConfidenceForCoverage(coverage))
	}
}

func TestNormalizeRecordsCountsInvalidRawRecords(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"ok","name":"Valid"}`),
		json.RawMessage(`{`),
	}

	evidence, coverage := NormalizeRecords("shops", raw)

	if len(evidence) != 1 {
		t.Fatalf("evidence = %d, want 1", len(evidence))
	}
	if coverage.RawRecordCount != 2 || coverage.EvidenceRecordCount != 1 {
		t.Fatalf("coverage = %#v, want two raw and one evidence", coverage)
	}
	if got := ConfidenceForCoverage(coverage); got != 0.5 {
		t.Fatalf("confidence = %f, want 0.5", got)
	}
}

func TestNormalizeRecordsDropsObjectsWithoutEvidenceFields(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`{"total":10,"page":1,"has_more":false}`),
		json.RawMessage(`{"data":[],"total":0}`),
	}

	evidence, coverage := NormalizeRecords("product_analytics", raw)

	if len(evidence) != 0 {
		t.Fatalf("evidence = %#v, want none", evidence)
	}
	if coverage.RawRecordCount != 3 || coverage.EvidenceRecordCount != 0 {
		t.Fatalf("coverage = %#v, want three raw and zero evidence", coverage)
	}
	if got := ConfidenceForCoverage(coverage); got != 0 {
		t.Fatalf("confidence = %f, want 0", got)
	}
}
