// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	airNowKeyEnv  = "AIR_QUALITY_AIRNOW_API_KEY"
	airNowBaseEnv = "AIR_QUALITY_AIRNOW_BASE_URL"
)

type airNowClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newAirNowClient(timeout time.Duration) airNowClient {
	baseURL := strings.TrimRight(strings.TrimSpace(env(airNowBaseEnv)), "/")
	if baseURL == "" {
		baseURL = "https://www.airnowapi.org"
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return airNowClient{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(env(airNowKeyEnv)),
		http:    &http.Client{Timeout: timeout},
	}
}

func (c airNowClient) configured() bool {
	return c.apiKey != ""
}

func (c airNowClient) currentByZip(ctx context.Context, zipCode string) ([]AirNowObservation, error) {
	if !c.configured() {
		return nil, fmt.Errorf("missing %s", airNowKeyEnv)
	}
	values := url.Values{}
	values.Set("format", "application/json")
	values.Set("zipCode", zipCode)
	values.Set("distance", "25")
	values.Set("API_KEY", c.apiKey)
	endpoint := c.baseURL + "/aq/observation/zipCode/current/?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("airnow %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var rows []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	observations := make([]AirNowObservation, 0, len(rows))
	for _, row := range rows {
		category := firstString(row["Category"])
		if categoryMap, ok := row["Category"].(map[string]any); ok {
			category = firstString(categoryMap["Name"], categoryMap["name"])
		}
		observations = append(observations, AirNowObservation{
			ReportingArea: firstString(row["ReportingArea"]),
			StateCode:     firstString(row["StateCode"]),
			Latitude:      firstString(row["Latitude"]),
			Longitude:     firstString(row["Longitude"]),
			Parameter:     firstString(row["ParameterName"]),
			AQI:           firstNonNil(row["AQI"], row["Aqi"]),
			Category:      category,
			Observed:      strings.Join(strings.Fields(fmt.Sprintf("%s %s %s", firstString(row["DateObserved"]), firstString(row["HourObserved"]), firstString(row["LocalTimeZone"]))), " "),
			Raw:           row,
		})
	}
	return observations, nil
}

func airNowGuidance(command string, query map[string]any) GuidanceResult {
	return GuidanceResult{
		Source:     "AirNow API",
		Configured: false,
		Command:    command,
		Query:      query,
		Title:      "AirNow API key is required for AirNow AQI observations",
		Setup: []string{
			"Create an AirNow API account.",
			"Set AIR_QUALITY_AIRNOW_API_KEY before running AirNow commands.",
			"Use file products instead of web services for large date ranges or broad geographic extracts.",
		},
		Caveats: []string{
			"AirNow observations are preliminary and subject to change.",
			"AirNow data are for reporting and forecasting AQI, not regulatory decisions.",
			"The first print keeps AirNow support narrow because several legacy zip and lat/long web services are marked for retirement in fall 2026.",
		},
	}
}
