package cli

import "testing"

func TestNovelPhase2StoreCommands(t *testing.T) {
	seed := seedNovelReportDB(t)

	daily := obj(runNovelCommand(t, seed.DBPath, "store", "daily-brief", "--days", "30"))
	if daily["summary"] == nil {
		t.Fatalf("daily-brief missing summary: %#v", daily)
	}
	if len(arr(daily["recommended_actions"])) == 0 {
		t.Fatalf("daily-brief should include recommended actions: %#v", daily)
	}

	audit := obj(runNovelCommand(t, seed.DBPath, "store", "audit", "--days", "30"))
	if audit["score"] == nil || len(arr(audit["checks"])) == 0 {
		t.Fatalf("audit missing score/checks: %#v", audit)
	}
	checks := arr(audit["checks"])
	shippingCheck := findRow(t, checks, "check", "shipping_anomalies")
	if shippingCheck["status"] != "review" {
		t.Fatalf("shipping anomalies should be flagged for seeded high/free shipping: %#v", shippingCheck)
	}
}

func TestNovelPhase2GrowthCommands(t *testing.T) {
	seed := seedNovelReportDB(t)

	winback := arr(runNovelCommand(t, seed.DBPath, "growth", "winback-candidates", "--idle-days", "5", "--limit", "10"))
	row := findRow(t, winback, "email", "c2@example.com")
	assertFloat(t, row["lifetime_value"], 280)

	vip := arr(runNovelCommand(t, seed.DBPath, "growth", "vip-segments", "--days", "365", "--limit", "5"))
	vipRow := findRow(t, vip, "email", "c2@example.com")
	if vipRow["segment"] == "" {
		t.Fatalf("vip row missing segment: %#v", vipRow)
	}

	brief := obj(runNovelCommand(t, seed.DBPath, "growth", "campaign-brief", "--days", "90"))
	if len(arr(brief["plays"])) < 2 {
		t.Fatalf("campaign-brief should include multiple plays: %#v", brief)
	}
}

func TestNovelPhase2OpsCommands(t *testing.T) {
	seed := seedNovelReportDB(t)

	risks := arr(runNovelCommand(t, seed.DBPath, "ops", "fulfillment-risk", "--hours", "12", "--limit", "10"))
	findRow(t, risks, "fulfillment_order_id", "fo-risk")

	shipping := arr(runNovelCommand(t, seed.DBPath, "ops", "shipping-anomalies", "--days", "30", "--limit", "10"))
	row := findRow(t, shipping, "order_name", "#1004")
	assertFloat(t, row["shipping_amount"], 60)
	if row["country"] != "US" {
		t.Fatalf("shipping-anomalies country = %v, want US from synced shippingAddress.countryCode", row["country"])
	}
}

func TestNovelPhase2MerchandisingCommands(t *testing.T) {
	seed := seedNovelReportDB(t)

	bundles := arr(runNovelCommand(t, seed.DBPath, "merchandising", "bundle-opportunities", "--days", "90", "--limit", "10"))
	if len(bundles) == 0 {
		t.Fatalf("bundle-opportunities should find Widget A/B pair")
	}

	dead := arr(runNovelCommand(t, seed.DBPath, "merchandising", "dead-stock-actions", "--days", "90", "--limit", "10"))
	findRow(t, dead, "sku", "SKU-Z")

	launch := obj(runNovelCommand(t, seed.DBPath, "merchandising", "launch-brief", "--product", "Widget A", "--days", "90"))
	if launch["product"] != "Widget A" || len(arr(launch["suggested_actions"])) == 0 {
		t.Fatalf("launch-brief missing product/actions: %#v", launch)
	}
}
