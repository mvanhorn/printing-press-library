package jalan

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestCompareRetainsPartialAndAllFailedCells(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Query().Get("stayDay") == "11" {
			return httpResult(403, "source access denied"), nil
		}
		return httpResult(200, offersHTML(r, 2)), nil
	})
	q := datedQuery()
	q.CheckIn = ""
	response, err := c.Compare(context.Background(), "385995", q, []string{"2026-11-10", "2026-11-11", "2026-11-12"}, nil)
	var partial *PartialError
	if !errors.As(err, &partial) || len(response.Results) != 2 || len(response.FetchFailures) != 1 || response.Meta["upstream_requests"] != 3 || calls != 3 {
		t.Fatalf("failed alternatives lost: %+v %v calls=%d", response, err, calls)
	}
	if response.FetchFailures[0]["check_in"] != "2026-11-11" || response.FetchFailures[0]["alternative_index"] != 1 {
		t.Fatalf("failure scope lost: %v", response.FetchFailures)
	}
	last := response.Results[1].(map[string]any)
	if last["query"].(Query).CheckIn != "2026-11-12" {
		t.Fatalf("per-cell date lost: %v", last)
	}
	c = testClient(t, func(*http.Request) (*http.Response, error) { return httpResult(403, "source access denied"), nil })
	response, err = c.Compare(context.Background(), "385995", q, []string{"2026-11-10", "2026-11-11"}, nil)
	var total *Error
	if errors.As(err, &partial) || !errors.As(err, &total) || len(response.Results) != 0 || len(total.FetchFailures) != 2 || total.Code != "access_failure" || response.Meta["status"] != "failed" {
		t.Fatalf("total failure became partial success: %+v %v", response, err)
	}
}
func TestCompareRejectsUnboundedCrossProductAndDuplicates(t *testing.T) {
	calls := 0
	c := testClient(t, func(*http.Request) (*http.Response, error) { calls++; return httpResult(200, "unused"), nil })
	q := datedQuery()
	tests := []struct {
		dates []string
		plans []PlanRef
	}{
		{dates: []string{"2026-11-10"}, plans: []PlanRef{{"03912759", "0576806"}}},
		{dates: []string{"2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13", "2026-11-14", "2026-11-15"}},
		{dates: []string{"2026-11-10", "2026-11-10"}},
		{plans: []PlanRef{{"03912759", "0576806"}, {"03912759", "0576806"}}},
	}
	for _, tc := range tests {
		if _, err := c.Compare(context.Background(), "385995", q, tc.dates, tc.plans); err == nil {
			t.Fatal("unsupported comparison accepted")
		}
	}
	if calls != 0 {
		t.Fatal("invalid comparisons sent requests")
	}
}
func TestComparableGroupsExcludeUnknownUnavailableAndDifferentBasis(t *testing.T) {
	amount := func(n int64) *int64 { return &n }
	items := []any{
		Offer{Availability: "available", Price: Price{Amount: amount(40000), Currency: "JPY", Basis: "whole_stay"}},
		Offer{Availability: "available", Price: Price{Amount: nil, Currency: "JPY", Basis: "whole_stay"}},
		Offer{Availability: "available", Price: Price{Amount: amount(15000), Currency: "JPY", Basis: "per_person_per_night"}},
		Plan{Offer: Offer{Availability: "unavailable", Price: Price{Amount: amount(1), Currency: "JPY", Basis: "whole_stay"}}},
		Offer{Availability: "available", Price: Price{Amount: amount(30000), Currency: "JPY", Basis: "whole_stay"}},
		Offer{Availability: "available", Price: Price{Amount: amount(2), Currency: "JPY", Basis: "unknown"}},
	}
	groups := comparablePriceGroups(items)
	if len(groups) != 2 {
		t.Fatalf("incomparable quote basis merged: %v", groups)
	}
	for _, group := range groups {
		g := group.(map[string]any)
		if g["currency_basis"] == "JPY/whole_stay" {
			observed := g["ordered_observations"].([]map[string]any)
			if len(observed) != 2 || observed[0]["result_index"] != 4 || observed[1]["result_index"] != 0 {
				t.Fatalf("unknown/reference quote ranked: %v", observed)
			}
		}
	}
}
