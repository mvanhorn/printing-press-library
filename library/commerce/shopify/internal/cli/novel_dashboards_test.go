package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopify/internal/store"
)

func TestNovelDashboardReports(t *testing.T) {
	seed := seedNovelReportDB(t)

	dashboard := obj(runNovelReport(t, seed.DBPath, "dashboard", "--days", "30"))
	if dashboard["orders"] != float64(4) {
		t.Fatalf("dashboard orders = %v, want 4", dashboard["orders"])
	}
	assertFloat(t, dashboard["revenue"], 430)
	assertFloat(t, dashboard["refunds"], 10)
	if len(arr(dashboard["top_products"])) == 0 {
		t.Fatal("dashboard top_products empty")
	}

	weekly := obj(runNovelReport(t, seed.DBPath, "weekly-digest", "--days", "7"))
	current := obj(weekly["current"])
	previous := obj(weekly["previous"])
	if current["orders"] != float64(2) || previous["orders"] != float64(1) {
		t.Fatalf("weekly orders current/previous = %v/%v, want 2/1", current["orders"], previous["orders"])
	}
	assertFloat(t, current["revenue"], 150)
	assertFloat(t, previous["revenue"], 80)
	assertFloat(t, obj(weekly["change_pct"])["revenue"], 87.5)

	s, err := store.Open(seed.DBPath)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	mustExec(t, s.DB(), `INSERT INTO orders (id,data,name,created_at,processed_at,display_financial_status,currency_code,source_name,note) VALUES (?,?,?,?,?,?,?,?,?)`, "o-prev-health", `{"totalPriceSet":{"shopMoney":{"amount":"430.00","currencyCode":"USD"}},"totalRefundedSet":{"shopMoney":{"amount":"0.00","currencyCode":"USD"}}}`, "#0999", ts(seed.Now.AddDate(0, 0, -40)), ts(seed.Now.AddDate(0, 0, -40)), "PAID", "USD", "web", "")
	if err := s.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	health := obj(runNovelReport(t, seed.DBPath, "health-score", "--days", "30"))
	if health["score"].(float64) <= 0 || health["score"].(float64) > 100 {
		t.Fatalf("health score = %v, want in (0,100]", health["score"])
	}
	components := obj(health["components"])
	assertFloat(t, components["revenue"], 430)
	assertFloat(t, components["refund_rate_pct"], 2.33)
	assertFloat(t, components["fulfillment_risk"], 1)
	assertFloat(t, health["score"], 54.88)
}
