// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
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
		"kind":   "bea_industry_regional",
		"source": "BEA API Regional dataset",
		"requested": map[string]string{
			"naics":    naics,
			"industry": industry,
			"state":    state,
		},
		"dataset": "Regional SAINC5N",
		"note":    "The first release focuses on source-backed retrieval plus setup; use BEA metadata methods for exact table/line discovery when extending industry mappings.",
		"raw":     decoded,
	}, nil
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

func unsupportedWagesGuidance(occupation string) GuidanceResult {
	message := "The first us-data print does not guess occupational wage tables without a source-backed mapping."
	if occupation != "" {
		message = fmt.Sprintf("%s Requested occupation: %s.", message, occupation)
	}
	return GuidanceResult{
		Kind:   "source_guidance",
		Status: "needs_dataset_mapping",
		Title:  "BLS occupational wage lookup",
		Messages: []string{
			message,
			"Use BLS OEWS or Modeled Wage Estimates source tables for occupation-level wage expansion.",
			"This command is intentionally honest rather than returning a national earnings proxy for an occupation.",
		},
		Sources: []string{"https://www.bls.gov/oes/", "https://www.bls.gov/developers/"},
	}
}
