// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const censusProfileURL = "https://api.census.gov/data/2023/acs/acs5/profile"

type placeRef struct {
	Name  string
	State string
	Place string
}

var knownPlaces = map[string]placeRef{
	"austin, tx":        {Name: "Austin city, Texas", State: "48", Place: "05000"},
	"seattle, wa":       {Name: "Seattle city, Washington", State: "53", Place: "63000"},
	"san francisco, ca": {Name: "San Francisco city, California", State: "06", Place: "67000"},
	"new york, ny":      {Name: "New York city, New York", State: "36", Place: "51000"},
}

func fetchPopulation(ctx context.Context, place string) (PopulationResult, error) {
	key := env("US_DATA_CENSUS_API_KEY")
	if key == "" {
		return PopulationResult{}, missingKey("Census population lookup", "US_DATA_CENSUS_API_KEY", []string{
			"Census Data API data queries currently require an API key.",
			"Request a key from https://api.census.gov/data/key_signup.html and set US_DATA_CENSUS_API_KEY.",
		})
	}
	ref, ok := resolvePlace(place)
	if !ok {
		return PopulationResult{}, usageErr("unsupported place %q; supported examples include Austin, TX, Seattle, WA, San Francisco, CA, New York, NY", place)
	}
	query := url.Values{
		"get": []string{"NAME,DP05_0001E"},
		"for": []string{"place:" + ref.Place},
		"in":  []string{"state:" + ref.State},
		"key": []string{key},
	}
	body, err := getJSON(ctx, censusProfileURL, query, nil)
	if err != nil {
		return PopulationResult{}, err
	}
	return parsePopulation(ref, body)
}

func resolvePlace(place string) (placeRef, bool) {
	ref, ok := knownPlaces[strings.ToLower(strings.TrimSpace(place))]
	return ref, ok
}

func parsePopulation(ref placeRef, body []byte) (PopulationResult, error) {
	var rows [][]string
	if err := json.Unmarshal(body, &rows); err != nil {
		return PopulationResult{}, err
	}
	if len(rows) < 2 || len(rows[1]) < 2 {
		return PopulationResult{}, fmt.Errorf("Census response did not include population row")
	}
	return PopulationResult{
		Kind:       "census_population",
		Source:     "Census Data API ACS 5-year profile",
		Place:      rows[1][0],
		Dataset:    "2023/acs/acs5/profile DP05_0001E",
		Population: rows[1][1],
		Year:       "2023",
		Caveats: []string{
			"ACS profile estimates are tied to the dataset vintage and can differ from decennial Census counts.",
			"Place resolution is intentionally narrow in this first CLI version; use an explicit supported city/state label.",
		},
	}, nil
}

func missingKey(title, envVar string, messages []string) error {
	return guidanceError{result: GuidanceResult{
		Kind:     "setup_guidance",
		Status:   "needs_auth",
		Title:    title,
		Messages: messages,
		EnvVars:  []string{envVar},
	}}
}

type guidanceError struct {
	result GuidanceResult
}

func (e guidanceError) Error() string {
	return e.result.Title + ": missing " + strings.Join(e.result.EnvVars, ", ")
}
