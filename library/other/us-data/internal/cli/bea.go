// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"net/url"
)

const beaAPIURL = "https://apps.bea.gov/api/data"

func fetchBEAIndustry(ctx context.Context, naics, industry, state string) (map[string]any, error) {
	key := env("US_DATA_BEA_API_KEY")
	if key == "" {
		return nil, missingKey("BEA industry lookup", "US_DATA_BEA_API_KEY", []string{
			"BEA API requests require a registered UserID.",
			"Register at https://apps.bea.gov/API/signup/ and set US_DATA_BEA_API_KEY.",
			"Useful BEA regional datasets include SAINC5N, SAINC7N, CAGDP2, SAGDP2, and GDPbyIndustry tables.",
		})
	}
	query := url.Values{
		"UserID":       []string{key},
		"method":       []string{"GetData"},
		"DataSetName":  []string{"Regional"},
		"TableName":    []string{"SAINC5N"},
		"LineCode":     []string{"10"},
		"GeoFips":      []string{"STATE"},
		"Year":         []string{"LAST5"},
		"ResultFormat": []string{"JSON"},
	}
	body, err := getJSON(ctx, beaAPIURL, query, nil)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	return map[string]any{
		"kind":            "bea_industry_regional",
		"source":          "BEA API Regional dataset",
		"request_context": beaIndustryRequestContext(naics, industry, state),
		"applied_query": map[string]string{
			"dataset":   "Regional",
			"table":     "SAINC5N",
			"line_code": "10",
			"geo_fips":  "STATE",
			"year":      "LAST5",
		},
		"dataset": "Regional SAINC5N",
		"caveats": []string{
			"The --naics, --industry, and --state flags are returned as caller context only; they are not applied to the first-print BEA query.",
			"The live BEA request uses Regional SAINC5N, LineCode 10, GeoFips STATE, Year LAST5 until table and line mappings are expanded.",
		},
		"note": "Use BEA metadata methods for exact dataset, table, line, and geography discovery when extending industry mappings.",
		"raw":  decoded,
	}, nil
}

func beaIndustryRequestContext(naics, industry, state string) map[string]any {
	return map[string]any{
		"naics":                naics,
		"industry":             industry,
		"state":                state,
		"applied_to_bea_query": false,
	}
}

func beaSetupGuidance() GuidanceResult {
	return GuidanceResult{
		Kind:   "setup_guidance",
		Status: "needs_auth",
		Title:  "BEA industry lookup",
		Messages: []string{
			"BEA API requests require a registered UserID.",
			"Register at https://apps.bea.gov/API/signup/ and set US_DATA_BEA_API_KEY.",
			"Use this command for regional or industry facts once a key is configured.",
		},
		EnvVars: []string{"US_DATA_BEA_API_KEY"},
		Sources: []string{"https://apps.bea.gov/API/signup/"},
	}
}
