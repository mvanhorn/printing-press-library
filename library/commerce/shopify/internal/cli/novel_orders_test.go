package cli

import "testing"

func TestNovelOrderReports(t *testing.T) {
	seed := seedNovelReportDB(t)

	trends := arr(runNovelReport(t, seed.DBPath, "order-trends", "--days", "30"))
	if len(trends) < 3 {
		t.Fatalf("order-trends rows = %d, want at least 3", len(trends))
	}
	firstTrend := obj(trends[0])
	if firstTrend["orders"] != float64(1) {
		t.Fatalf("latest order-trends orders = %v, want 1", firstTrend["orders"])
	}
	assertFloat(t, firstTrend["revenue"], 50)

	aov := obj(runNovelReport(t, seed.DBPath, "aov-analysis", "--days", "30"))
	if aov["orders"] != float64(4) {
		t.Fatalf("aov orders = %v, want 4", aov["orders"])
	}
	assertFloat(t, aov["revenue"], 430)
	assertFloat(t, aov["aov"], 107.5)
	web := findRow(t, arr(aov["by_source"]), "source", "web")
	assertFloat(t, web["revenue"], 150)

	discounts := arr(runNovelReport(t, seed.DBPath, "discount-impact", "--days", "30"))
	discounted := findRow(t, discounts, "bucket", "discounted")
	if discounted["orders"] != float64(1) {
		t.Fatalf("discounted orders = %v, want 1", discounted["orders"])
	}
	assertFloat(t, discounted["revenue"], 100)

	refunds := obj(runNovelReport(t, seed.DBPath, "refund-analysis", "--days", "30", "--limit", "5"))
	products := arr(refunds["products"])
	if len(products) != 1 {
		t.Fatalf("refund product rows = %d, want 1", len(products))
	}
	refundProduct := obj(products[0])
	if refundProduct["product"] != "Widget A" {
		t.Fatalf("refund product = %v, want Widget A", refundProduct["product"])
	}
	assertFloat(t, refundProduct["units"], 1)

	peaks := arr(runNovelReport(t, seed.DBPath, "peak-hours", "--days", "30"))
	if len(peaks) == 0 {
		t.Fatal("peak-hours returned no rows")
	}
	if _, ok := obj(peaks[0])["hour_utc"]; !ok {
		t.Fatalf("peak-hours missing hour_utc: %#v", peaks[0])
	}

	firstPurchase := obj(runNovelReport(t, seed.DBPath, "first-purchase-analysis", "--days", "365", "--limit", "5"))
	fps := arr(firstPurchase["first_purchase_products"])
	if len(fps) == 0 {
		t.Fatal("first-purchase-analysis returned no products")
	}
	if obj(fps[0])["first_product"] != "Widget B" {
		t.Fatalf("first-purchase top product = %v, want Widget B", obj(fps[0])["first_product"])
	}

	klaviyo := obj(runNovelReport(t, seed.DBPath, "klaviyo-attribution", "--days", "90"))
	if klaviyo["orders"] != float64(2) {
		t.Fatalf("klaviyo orders = %v, want 2 (source email + klaviyo tag)", klaviyo["orders"])
	}
	assertFloat(t, klaviyo["revenue"], 130)
}
