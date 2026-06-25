package cli

import (
	"math"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
)

func TestFlattenRowsAndEnrich(t *testing.T) {
	raw := ga4.ReportResponse{DimensionHeaders: []ga4.Header{{Name: "channel"}}, MetricHeaders: []ga4.Header{{Name: "sessions"}, {Name: "conversions"}, {Name: "totalRevenue"}, {Name: "transactions"}}, Rows: []ga4.Row{{DimensionValues: []ga4.Value{{Value: "Organic Search"}}, MetricValues: []ga4.Value{{Value: "100"}, {Value: "5"}, {Value: "250.50"}, {Value: "2"}}}}}
	rows := enrich(flattenRows(raw))
	if rows[0]["channel"] != "Organic Search" {
		t.Fatalf("bad dimension: %#v", rows[0])
	}
	if math.Abs(rows[0]["conversion_rate"].(float64)-0.05) > 0.0001 {
		t.Fatalf("bad conversion rate: %#v", rows[0])
	}
	if math.Abs(rows[0]["aov"].(float64)-125.25) > 0.0001 {
		t.Fatalf("bad aov: %#v", rows[0])
	}
}
func TestCompareRowsCalculatesDeltasAndPct(t *testing.T) {
	a := []map[string]any{{"channel": "Organic", "sessions": 150, "totalRevenue": 75.0}}
	b := []map[string]any{{"channel": "Organic", "sessions": 100, "totalRevenue": 100.0}}
	out := compareRows(a, b, []string{"channel"}, []string{"sessions", "totalRevenue"})
	rows := out["rows"].([]map[string]any)
	sessions := rows[0]["sessions"].(map[string]float64)
	if sessions["delta"] != 50 || sessions["pct_change"] != 0.5 {
		t.Fatalf("bad sessions delta: %#v", sessions)
	}
	revenue := rows[0]["totalRevenue"].(map[string]float64)
	if revenue["delta"] != -25 || revenue["pct_change"] != -0.25 {
		t.Fatalf("bad revenue delta: %#v", revenue)
	}
}
func TestTrendAndInferPrevious(t *testing.T) {
	tr := trend([]map[string]any{{"eventCount": 10}, {"eventCount": 15}}, "eventCount")
	if tr["delta"].(float64) != 5 || tr["pct_change"].(float64) != 0.5 {
		t.Fatalf("bad trend: %#v", tr)
	}
	ps, pe := inferPrevious("7daysAgo", "yesterday", "wow")
	if ps != "14daysAgo" || pe != "8daysAgo" {
		t.Fatalf("bad previous: %s %s", ps, pe)
	}
	ps, pe = inferPrevious("2026-05-01", "2026-05-31", "trailing")
	if ps != "2026-03-31" || pe != "2026-04-30" {
		t.Fatalf("bad absolute previous window: %s %s", ps, pe)
	}
}
func TestWhatsChangedRankingMagnitude(t *testing.T) {
	movers := []map[string]any{{"largest_pct_change": -2.0}, {"largest_pct_change": 0.5}}
	if !(abs(toFloat(movers[0]["largest_pct_change"])) > abs(toFloat(movers[1]["largest_pct_change"]))) {
		t.Fatal("expected absolute pct ranking")
	}
}

func TestPreviousWindowRejectsPartialExplicitPreviousPeriod(t *testing.T) {
	_, _, err := previousWindow("2026-05-01", "2026-05-31", "2026-04-01", "", "trailing")
	if err == nil {
		t.Fatal("expected partial previous-period input to fail")
	}
	ps, pe, err := previousWindow("2026-05-01", "2026-05-31", "2026-04-01", "2026-04-30", "trailing")
	if err != nil || ps != "2026-04-01" || pe != "2026-04-30" {
		t.Fatalf("bad explicit previous window: %s %s %v", ps, pe, err)
	}
}
