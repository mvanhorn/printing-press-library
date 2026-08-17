// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

const blsBaseURL = "https://api.bls.gov/publicAPI/v1/timeseries/data"

type blsResponse struct {
	Status  string   `json:"status"`
	Message []string `json:"message"`
	Results struct {
		Series []struct {
			SeriesID string `json:"seriesID"`
			Data     []struct {
				Year       string `json:"year"`
				Period     string `json:"period"`
				PeriodName string `json:"periodName"`
				Latest     string `json:"latest"`
				Value      string `json:"value"`
			} `json:"data"`
		} `json:"series"`
	} `json:"Results"`
}

func fetchBLSSeries(ctx context.Context, seriesID, title string, years int) (SeriesResult, error) {
	now := time.Now().UTC()
	if years <= 0 {
		years = 3
	}
	start := now.Year() - years + 1
	if start < 1900 {
		start = 1900
	}
	query := url.Values{
		"startyear": []string{strconv.Itoa(start)},
		"endyear":   []string{strconv.Itoa(now.Year())},
	}
	body, err := getJSON(ctx, blsBaseURL+"/"+url.PathEscape(seriesID), query, nil)
	if err != nil {
		return SeriesResult{}, err
	}
	result, err := parseBLS(seriesID, title, body)
	if err != nil {
		return SeriesResult{}, err
	}
	return result, nil
}

func parseBLS(seriesID, title string, body []byte) (SeriesResult, error) {
	var payload blsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return SeriesResult{}, err
	}
	if payload.Status != "REQUEST_SUCCEEDED" {
		return SeriesResult{}, fmt.Errorf("BLS request status %q: %v", payload.Status, payload.Message)
	}
	if len(payload.Results.Series) == 0 {
		return SeriesResult{}, fmt.Errorf("BLS response did not contain series %s", seriesID)
	}
	series := payload.Results.Series[0]
	observations := make([]Observation, 0, len(series.Data))
	for _, item := range series.Data {
		if item.Period == "M13" {
			continue
		}
		observations = append(observations, Observation{
			Year:       item.Year,
			Period:     item.Period,
			PeriodName: item.PeriodName,
			Value:      item.Value,
			Latest:     item.Latest == "true",
		})
	}
	if len(observations) == 0 {
		return SeriesResult{}, fmt.Errorf("BLS series %s did not contain monthly observations", seriesID)
	}
	result := SeriesResult{
		Kind:          "bls_series_snapshot",
		Source:        "Bureau of Labor Statistics Public Data API v1",
		SeriesID:      series.SeriesID,
		Title:         title,
		Latest:        observations[0],
		Observations:  observations,
		FreshnessNote: "BLS data is published on scheduled release calendars; latest observations can be preliminary and may be revised.",
		Caveats: []string{
			"BLS v1 is keyless but lower-volume than registered v2 access.",
			"Series IDs are official BLS identifiers; use --series to override the default.",
		},
	}
	if len(observations) > 1 {
		prior := observations[1]
		result.Prior = &prior
		if pct, ok := percentChange(observations[1].Value, observations[0].Value); ok {
			result.PercentChange = &pct
		}
	}
	return result, nil
}

func percentChange(oldValue, newValue string) (float64, bool) {
	oldFloat, err := strconv.ParseFloat(oldValue, 64)
	if err != nil || oldFloat == 0 {
		return 0, false
	}
	newFloat, err := strconv.ParseFloat(newValue, 64)
	if err != nil {
		return 0, false
	}
	return ((newFloat - oldFloat) / oldFloat) * 100, true
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
