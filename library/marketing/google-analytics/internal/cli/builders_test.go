package cli

import (
	"encoding/json"
	"testing"
)

func TestReportRequestBuildsTypedGA4Body(t *testing.T) {
	req := reportRequest("sessions,totalRevenue", "date,sessionDefaultChannelGroup", "28daysAgo", "yesterday", 7)
	if len(req.Metrics) != 2 || req.Metrics[1].Name != "totalRevenue" {
		t.Fatalf("metrics not parsed: %#v", req.Metrics)
	}
	if len(req.Dimensions) != 2 || req.Dimensions[0].Name != "date" {
		t.Fatalf("dimensions not parsed: %#v", req.Dimensions)
	}
	if req.Limit != "7" || req.DateRanges[0].StartDate != "28daysAgo" {
		t.Fatalf("bad date/limit: %#v", req)
	}
}
func TestAddOrderAndFilter(t *testing.T) {
	req := reportRequest("sessions", "date", "", "", 0)
	addOrder(&req, "-sessions")
	if len(req.OrderBys) != 1 || !req.OrderBys[0].Desc || req.OrderBys[0].Metric.MetricName != "sessions" {
		t.Fatalf("bad order: %#v", req.OrderBys)
	}
	if err := addRawDimensionFilter(&req, `{"filter":{"fieldName":"country"}}`); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(req)
	if !json.Valid(b) || string(b) == "" {
		t.Fatalf("bad json: %s", b)
	}
}
func TestFunnelRequestBuildsEventSteps(t *testing.T) {
	req := funnelRequest("view_item,add_to_cart", "30daysAgo", "yesterday")
	if len(req.Funnel.Steps) != 2 {
		t.Fatalf("steps=%d", len(req.Funnel.Steps))
	}
	if req.Funnel.Steps[1].FilterExpression.FunnelEventFilter.EventName != "add_to_cart" {
		t.Fatalf("bad step: %#v", req.Funnel.Steps[1])
	}
}
func TestPropertyResolutionPrefersFlag(t *testing.T) {
	f := &rootFlags{propertyID: "properties/123"}
	if got := configuredProperty(f); got != "123" {
		t.Fatalf("got %q", got)
	}
}
