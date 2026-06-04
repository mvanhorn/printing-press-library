package cli

import "testing"

func TestNovelProductReports(t *testing.T) {
	seed := seedNovelReportDB(t)

	dashboard := obj(runNovelReport(t, seed.DBPath, "product-dashboard", "--days", "30", "--limit", "10"))
	products := arr(dashboard["products"])
	widgetB := findRow(t, products, "product", "Widget B")
	assertFloat(t, widgetB["units"], 5)
	assertFloat(t, widgetB["revenue"], 340)

	velocity := obj(runNovelReport(t, seed.DBPath, "product-velocity", "--days", "30", "--limit", "10"))
	velocityRows := arr(velocity["products"])
	widgetA := findRow(t, velocityRows, "product", "Widget A")
	assertFloat(t, widgetA["units"], 3)

	affinity := obj(runNovelReport(t, seed.DBPath, "product-affinity", "--days", "30", "--limit", "10"))
	pairs := arr(affinity["pairs"])
	if len(pairs) == 0 {
		t.Fatal("product-affinity returned no pairs")
	}
	pair := obj(pairs[0])
	if pair["product_a"] != "Widget A" || pair["product_b"] != "Widget B" {
		t.Fatalf("product-affinity pair = %v/%v, want Widget A/Widget B", pair["product_a"], pair["product_b"])
	}
	assertFloat(t, pair["pair_orders"], 1)

	cannibalization := arr(runNovelReport(t, seed.DBPath, "product-cannibalization", "--days", "3650", "--limit", "10"))
	if len(cannibalization) != 0 {
		if _, ok := obj(cannibalization[0])["correlation"]; !ok {
			t.Fatalf("product-cannibalization row missing correlation: %#v", cannibalization[0])
		}
	}

	seasonality := obj(runNovelReport(t, seed.DBPath, "product-seasonality", "--days", "3650"))
	seasonRows := arr(seasonality["seasonality"])
	if len(seasonRows) == 0 {
		t.Fatal("product-seasonality returned no rows")
	}
	if _, ok := obj(seasonRows[0])["seasonality_index"]; !ok {
		t.Fatalf("product-seasonality missing index: %#v", seasonRows[0])
	}
}
