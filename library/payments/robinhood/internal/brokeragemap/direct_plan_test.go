// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package brokeragemap

import (
	"strings"
	"testing"
)

func TestBuildDirectPlanResolvesPathAndQuery(t *testing.T) {
	plan := BuildDirectPlan(
		"api.robinhood.com",
		"/portfolios/historicals/{account_id}/",
		"GET",
		"sensitive-read",
		map[string]string{"account_id": "1AB23456"},
		map[string]string{"span": "year", "interval": "week", "empty": ""},
	)
	if plan.Method != "GET" {
		t.Fatalf("method = %q, want GET", plan.Method)
	}
	if len(plan.MissingParams) != 0 {
		t.Fatalf("missing params = %v, want none", plan.MissingParams)
	}
	// Query keys are sorted; empty values dropped.
	want := "https://api.robinhood.com/portfolios/historicals/1AB23456/?interval=week&span=year"
	if plan.URL != want {
		t.Fatalf("url = %q, want %q", plan.URL, want)
	}
	if strings.Contains(plan.URL, "empty=") {
		t.Fatalf("empty query value should be dropped: %q", plan.URL)
	}
}

func TestBuildDirectPlanReportsMissingParams(t *testing.T) {
	plan := BuildDirectPlan(
		"api.robinhood.com",
		"/orders/{order_id}/cancel/",
		"POST",
		"write-mutate",
		nil,
		nil,
	)
	if len(plan.MissingParams) != 1 || plan.MissingParams[0] != "order_id" {
		t.Fatalf("missing params = %v, want [order_id]", plan.MissingParams)
	}
	if !plan.MutatesAccount {
		t.Fatalf("write-mutate plan should report MutatesAccount = true")
	}
	if !strings.Contains(plan.URL, "{order_id}") {
		t.Fatalf("unresolved placeholder should remain in URL: %q", plan.URL)
	}
}

func TestBuildDirectPlanNoQueryNoTrailingSep(t *testing.T) {
	plan := BuildDirectPlan("api.robinhood.com", "/accounts/", "GET", "sensitive-read", nil, nil)
	if plan.URL != "https://api.robinhood.com/accounts/" {
		t.Fatalf("url = %q, want clean accounts URL", plan.URL)
	}
}

// TestBuildDirectPlanEscapesReservedChars guards the path/query percent-encoding:
// a path param containing "/" or ".." must not collapse into extra path segments,
// and a query value with reserved characters must not inject extra parameters.
func TestBuildDirectPlanEscapesReservedChars(t *testing.T) {
	plan := BuildDirectPlan(
		"api.robinhood.com",
		"/orders/{order_id}/",
		"GET",
		"sensitive-read",
		map[string]string{"order_id": "../evil/segment"},
		map[string]string{"q": "a&b=c"},
	)
	if strings.Contains(plan.URL, "../") {
		t.Fatalf("path param should be escaped, got %q", plan.URL)
	}
	want := "https://api.robinhood.com/orders/..%2Fevil%2Fsegment/?q=a%26b%3Dc"
	if plan.URL != want {
		t.Fatalf("url = %q, want %q", plan.URL, want)
	}
}
