package cli

import "testing"

func TestNovelInventoryReports(t *testing.T) {
	seed := seedNovelReportDB(t)

	health := arr(runNovelReport(t, seed.DBPath, "inventory-health", "--days", "30", "--limit", "10"))
	skuA := findRow(t, health, "sku", "SKU-A")
	assertFloat(t, skuA["available"], 10)
	assertFloat(t, skuA["units_sold"], 3)
	assertFloat(t, skuA["units_per_day"], 0.1)
	assertFloat(t, skuA["days_of_supply"], 100)

	dead := arr(runNovelReport(t, seed.DBPath, "dead-inventory", "--days", "30", "--limit", "10"))
	skuZ := findRow(t, dead, "sku", "SKU-Z")
	assertFloat(t, skuZ["available"], 5)
	if skuZ["tracked"] != true {
		t.Fatalf("dead-inventory SKU-Z tracked = %v, want true", skuZ["tracked"])
	}
}
