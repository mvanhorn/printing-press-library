package cli

import "testing"

func TestAddOrderUsesDimensionPayloadWhenOrderingByRequestedDimension(t *testing.T) {
	req := reportRequest("sessions", "date", "7daysAgo", "yesterday", 10)
	addOrder(req, "date")

	orderBys, ok := req["orderBys"].([]map[string]any)
	if !ok || len(orderBys) != 1 {
		t.Fatalf("orderBys = %#v, want one order", req["orderBys"])
	}
	if _, ok := orderBys[0]["metric"]; ok {
		t.Fatalf("dimension order unexpectedly used metric payload: %#v", orderBys[0])
	}
	dim, ok := orderBys[0]["dimension"].(map[string]string)
	if !ok {
		t.Fatalf("dimension payload = %#v", orderBys[0]["dimension"])
	}
	if got := dim["dimensionName"]; got != "date" {
		t.Fatalf("dimensionName = %q, want date", got)
	}
}

func TestAddOrderKeepsMetricPayloadForRequestedMetric(t *testing.T) {
	req := reportRequest("sessions", "date", "7daysAgo", "yesterday", 10)
	addOrder(req, "-sessions")

	orderBys, ok := req["orderBys"].([]map[string]any)
	if !ok || len(orderBys) != 1 {
		t.Fatalf("orderBys = %#v, want one order", req["orderBys"])
	}
	metric, ok := orderBys[0]["metric"].(map[string]string)
	if !ok {
		t.Fatalf("metric payload = %#v", orderBys[0]["metric"])
	}
	if got := metric["metricName"]; got != "sessions" {
		t.Fatalf("metricName = %q, want sessions", got)
	}
	if got := orderBys[0]["desc"]; got != true {
		t.Fatalf("desc = %#v, want true", got)
	}
}

func TestAddDimensionOrderUsesDimensionName(t *testing.T) {
	req := map[string]any{}
	addDimensionOrder(req, "firstSessionDate")

	orderBys, ok := req["orderBys"].([]map[string]any)
	if !ok || len(orderBys) != 1 {
		t.Fatalf("orderBys = %#v, want one order", req["orderBys"])
	}
	if _, ok := orderBys[0]["metric"]; ok {
		t.Fatalf("dimension order unexpectedly used metric payload: %#v", orderBys[0])
	}
	dim, ok := orderBys[0]["dimension"].(map[string]string)
	if !ok {
		t.Fatalf("dimension payload = %#v", orderBys[0]["dimension"])
	}
	if got := dim["dimensionName"]; got != "firstSessionDate" {
		t.Fatalf("dimensionName = %q, want firstSessionDate", got)
	}
}

func TestInferPreviousUsesCurrentRelativeWindow(t *testing.T) {
	start, end := inferPrevious("28daysAgo", "yesterday", "wow")
	if start != "56daysAgo" || end != "29daysAgo" {
		t.Fatalf("inferPrevious = %s/%s, want 56daysAgo/29daysAgo", start, end)
	}
}

func TestInferPreviousFallsBackForUnsupportedDates(t *testing.T) {
	start, end := inferPrevious("2026-05-01", "2026-05-31", "mom")
	if start != "60daysAgo" || end != "31daysAgo" {
		t.Fatalf("inferPrevious fallback = %s/%s, want 60daysAgo/31daysAgo", start, end)
	}
}
