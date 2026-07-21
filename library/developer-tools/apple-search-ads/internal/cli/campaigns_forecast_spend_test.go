// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"math"
	"testing"
)

func campaignsFixture() json.RawMessage {
	return json.RawMessage(`{
		"data": [
			{
				"id": "1001",
				"name": "Brand Campaign",
				"status": "ENABLED",
				"dailyBudget": {"amount": "50.00", "currency": "USD"}
			},
			{
				"id": "1002",
				"name": "Retargeting",
				"status": "PAUSED",
				"totalBudget": {"amount": "500.00", "currency": "USD"}
			}
		]
	}`)
}

func spendReportFixture() json.RawMessage {
	return json.RawMessage(`{
		"data": {
			"reportingDataResponse": {
				"row": [
					{
						"metadata": {"campaignId": "1001"},
						"granularity": [
							{"localSpend": {"amount": "20.00"}},
							{"localSpend": {"amount": "30.00"}}
						]
					},
					{
						"metadata": {"campaignId": "1002"},
						"granularity": [
							{"localSpend": {"amount": "10.00"}}
						]
					}
				]
			}
		}
	}`)
}

func TestExtractCampaignsForForecast(t *testing.T) {
	camps := extractCampaignsForForecast(campaignsFixture())
	if len(camps) != 2 {
		t.Fatalf("want 2 campaigns, got %d", len(camps))
	}

	brand := camps[0]
	if brand.id != "1001" {
		t.Errorf("want id=1001, got %q", brand.id)
	}
	if brand.name != "Brand Campaign" {
		t.Errorf("want name=Brand Campaign, got %q", brand.name)
	}
	if math.Abs(brand.dailyBudget-50.0) > 0.01 {
		t.Errorf("want dailyBudget=50.00, got %f", brand.dailyBudget)
	}
	if brand.lifetimeBudget != 0 {
		t.Errorf("want no lifetimeBudget, got %f", brand.lifetimeBudget)
	}

	retarget := camps[1]
	if retarget.id != "1002" {
		t.Errorf("want id=1002, got %q", retarget.id)
	}
	if math.Abs(retarget.lifetimeBudget-500.0) > 0.01 {
		t.Errorf("want lifetimeBudget=500.00, got %f", retarget.lifetimeBudget)
	}
	if retarget.dailyBudget != 0 {
		t.Errorf("want no dailyBudget, got %f", retarget.dailyBudget)
	}
}

func TestExtractCampaignsForForecast_Empty(t *testing.T) {
	camps := extractCampaignsForForecast(json.RawMessage(`{"data":[]}`))
	if len(camps) != 0 {
		t.Errorf("want 0, got %d", len(camps))
	}
}

func TestExtractCampaignsForForecast_MissingID(t *testing.T) {
	// Campaign without id must be skipped.
	camps := extractCampaignsForForecast(json.RawMessage(`{"data":[{"name":"no-id"}]}`))
	if len(camps) != 0 {
		t.Errorf("want 0 (no id), got %d", len(camps))
	}
}

func TestExtractCampaignSpendMap(t *testing.T) {
	// lookback=7 days; campaign 1001 spent 50 total → avg=50/7≈7.14
	m := extractCampaignSpendMap(spendReportFixture(), 7)
	avg1001 := m["1001"]
	if math.Abs(avg1001-(50.0/7)) > 0.01 {
		t.Errorf("want avg spend≈7.14 for 1001, got %f", avg1001)
	}
	avg1002 := m["1002"]
	if math.Abs(avg1002-(10.0/7)) > 0.001 {
		t.Errorf("want avg spend≈1.43 for 1002, got %f", avg1002)
	}
}

func TestExtractCampaignSpendMap_ZeroLookback(t *testing.T) {
	// Defensive: zero lookback must not divide by zero.
	m := extractCampaignSpendMap(spendReportFixture(), 0)
	if m["1001"] != 0 {
		t.Errorf("want 0 on zero lookback, got %f", m["1001"])
	}
}

func TestExtractCampaignSpendMap_Missing(t *testing.T) {
	// Campaign not in report → not in map.
	m := extractCampaignSpendMap(spendReportFixture(), 7)
	if _, ok := m["9999"]; ok {
		t.Errorf("want no entry for 9999, got %f", m["9999"])
	}
}
