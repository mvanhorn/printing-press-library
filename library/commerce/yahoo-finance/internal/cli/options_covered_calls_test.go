package cli

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

// TestParseCoveredCallsBasic ensures the chain parser returns rows that
// pass the min-yield + max-dte filters when given a synthetic Yahoo
// options envelope.
func TestParseCoveredCallsBasic(t *testing.T) {
	now := time.Now()
	exp := now.Add(45 * 24 * time.Hour).Unix()
	env := map[string]any{
		"optionChain": map[string]any{
			"result": []map[string]any{{
				"underlyingSymbol": "AAPL",
				"quote":            map[string]any{"regularMarketPrice": 175.0},
				"options": []map[string]any{{
					"expirationDate": exp,
					"calls": []map[string]any{
						{"strike": 180.0, "bid": 4.50},
						{"strike": 200.0, "bid": 0.05},
					},
				}},
			}},
		},
	}
	raw, _ := json.Marshal(env)
	got := parseCoveredCalls(raw, "AAPL", 100, 60, 0.08)
	if len(got) != 1 {
		t.Fatalf("expected 1 row (only 180-strike passes), got %d: %+v", len(got), got)
	}
	if got[0].Strike != 180 {
		t.Errorf("Strike = %v, want 180", got[0].Strike)
	}
	// DTE integer-truncation can shift the yield by 1 day, so allow a wider tolerance.
	if math.Abs(got[0].AnnualizedYield-0.208) > 0.015 {
		t.Errorf("AnnualizedYield = %v, want ~0.208", got[0].AnnualizedYield)
	}
}

func TestParseCoveredCallsMaxDTE(t *testing.T) {
	now := time.Now()
	expFar := now.Add(180 * 24 * time.Hour).Unix()
	env := map[string]any{
		"optionChain": map[string]any{
			"result": []map[string]any{{
				"quote":   map[string]any{"regularMarketPrice": 100.0},
				"options": []map[string]any{{"expirationDate": expFar, "calls": []map[string]any{{"strike": 110.0, "bid": 5.0}}}},
			}},
		},
	}
	raw, _ := json.Marshal(env)
	got := parseCoveredCalls(raw, "X", 100, 60, 0.01)
	if len(got) != 0 {
		t.Errorf("expected nothing (DTE>60), got %+v", got)
	}
}

func TestParseCoveredCallsMinYield(t *testing.T) {
	now := time.Now()
	exp := now.Add(30 * 24 * time.Hour).Unix()
	env := map[string]any{
		"optionChain": map[string]any{
			"result": []map[string]any{{
				"quote":   map[string]any{"regularMarketPrice": 100.0},
				"options": []map[string]any{{"expirationDate": exp, "calls": []map[string]any{{"strike": 105.0, "bid": 0.10}}}},
			}},
		},
	}
	raw, _ := json.Marshal(env)
	got := parseCoveredCalls(raw, "X", 100, 60, 0.20)
	if len(got) != 0 {
		t.Errorf("expected nothing (yield below min), got %+v", got)
	}
}
