package cli

import "testing"

func TestNovelCustomerReports(t *testing.T) {
	seed := seedNovelReportDB(t)

	cohorts := arr(runNovelReport(t, seed.DBPath, "customer-cohorts", "--days", "3650"))
	if len(cohorts) == 0 {
		t.Fatal("customer-cohorts returned no rows")
	}
	var sawRetained bool
	for _, row := range cohorts {
		m := obj(row)
		if m["retained_30d"].(float64) > 0 {
			sawRetained = true
			break
		}
	}
	if !sawRetained {
		t.Fatalf("customer-cohorts did not report any 30d retention: %#v", cohorts)
	}

	rfm := arr(runNovelReport(t, seed.DBPath, "customer-rfm", "--days", "3650", "--limit", "10"))
	c1 := findRow(t, rfm, "customer_id", "gid://shopify/Customer/1")
	if c1["frequency"] != float64(3) {
		t.Fatalf("customer-rfm c1 frequency = %v, want 3", c1["frequency"])
	}
	assertFloat(t, c1["monetary"], 220)

	ltv := arr(runNovelReport(t, seed.DBPath, "customer-ltv", "--days", "3650", "--limit", "10"))
	c1ltv := findRow(t, ltv, "customer_id", "gid://shopify/Customer/1")
	assertFloat(t, c1ltv["ltv"], 220)
	if c1ltv["orders"] != float64(3) {
		t.Fatalf("customer-ltv c1 orders = %v, want 3", c1ltv["orders"])
	}

	repeat := obj(runNovelReport(t, seed.DBPath, "repeat-rate", "--days", "3650"))
	if repeat["customers"] != float64(3) || repeat["repeaters"] != float64(2) {
		t.Fatalf("repeat-rate customers/repeaters = %v/%v, want 3/2", repeat["customers"], repeat["repeaters"])
	}
	assertFloat(t, repeat["repeat_rate_pct"], 66.67)
	monthly := arr(repeat["monthly"])
	lastMonth := obj(monthly[len(monthly)-1])
	assertFloat(t, lastMonth["repeat_rate_pct"], 66.67)

	churn := arr(runNovelReport(t, seed.DBPath, "customer-churn-risk", "--days", "3650", "--limit", "10"))
	c1churn := findRow(t, churn, "customer_id", "gid://shopify/Customer/1")
	if c1churn["risk"] != "normal" {
		t.Fatalf("customer-churn-risk c1 risk = %v, want normal", c1churn["risk"])
	}
	assertFloat(t, c1churn["orders"], 3)
}
