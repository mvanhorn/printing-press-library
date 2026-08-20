package cli

import "testing"

func TestNovelOperationsReports(t *testing.T) {
	seed := seedNovelReportDB(t)

	fulfillment := arr(runNovelReport(t, seed.DBPath, "fulfillment-speed", "--days", "30"))
	closed := findRow(t, fulfillment, "status", "CLOSED")
	if closed["count"] != float64(1) {
		t.Fatalf("fulfillment-speed CLOSED count = %v, want 1", closed["count"])
	}
	assertFloat(t, closed["avg_hours"], 24)

	abandoned := obj(runNovelReport(t, seed.DBPath, "abandoned-checkout-analysis", "--days", "30"))
	if abandoned["checkouts"] != float64(2) || abandoned["completed"] != float64(1) || abandoned["abandoned"] != float64(1) {
		t.Fatalf("abandoned counts = checkouts %v completed %v abandoned %v, want 2/1/1", abandoned["checkouts"], abandoned["completed"], abandoned["abandoned"])
	}
	assertFloat(t, abandoned["total_value"], 200)
	assertFloat(t, abandoned["completion_rate_pct"], 50)

	buckets := arr(runNovelReport(t, seed.DBPath, "cart-value-distribution", "--days", "30"))
	b50 := findRow(t, buckets, "bucket", "50_99")
	if b50["orders"] != float64(2) {
		t.Fatalf("50_99 bucket orders = %v, want 2", b50["orders"])
	}
	assertFloat(t, b50["revenue"], 130)
}
