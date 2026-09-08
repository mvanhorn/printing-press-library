// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

var plannerMonthNames = [...]string{
	"", "JANUARY", "FEBRUARY", "MARCH", "APRIL", "MAY", "JUNE",
	"JULY", "AUGUST", "SEPTEMBER", "OCTOBER", "NOVEMBER", "DECEMBER",
}

type plannerRESTYearMonth struct {
	Year  string `json:"year"`
	Month string `json:"month"`
}

type plannerRESTYearMonthRange struct {
	Start plannerRESTYearMonth `json:"start"`
	End   plannerRESTYearMonth `json:"end"`
}

type plannerRESTHistoricalMetricsOptions struct {
	IncludeAverageCPC bool                      `json:"includeAverageCpc"`
	YearMonthRange    plannerRESTYearMonthRange `json:"yearMonthRange"`
}

type plannerRESTIdeasRequest struct {
	IncludeAdultKeywords     bool                                `json:"includeAdultKeywords"`
	GeoTargetConstants       []string                            `json:"geoTargetConstants"`
	PageToken                string                              `json:"pageToken,omitempty"`
	KeywordSeed              plannerRESTKeywordSeed              `json:"keywordSeed"`
	KeywordPlanNetwork       string                              `json:"keywordPlanNetwork"`
	HistoricalMetricsOptions plannerRESTHistoricalMetricsOptions `json:"historicalMetricsOptions"`
	Language                 string                              `json:"language"`
	PageSize                 int                                 `json:"pageSize"`
}

type plannerRESTKeywordSeed struct {
	Keywords []string `json:"keywords"`
}

type plannerRESTHistoricalRequest struct {
	Language                 string                              `json:"language"`
	IncludeAdultKeywords     bool                                `json:"includeAdultKeywords"`
	GeoTargetConstants       []string                            `json:"geoTargetConstants"`
	KeywordPlanNetwork       string                              `json:"keywordPlanNetwork"`
	Keywords                 []string                            `json:"keywords"`
	HistoricalMetricsOptions plannerRESTHistoricalMetricsOptions `json:"historicalMetricsOptions"`
}

func plannerRESTMonth(value string) (plannerRESTYearMonth, error) {
	month, err := parsePlannerMonth(value)
	if err != nil {
		return plannerRESTYearMonth{}, err
	}
	return plannerRESTYearMonth{Year: fmt.Sprintf("%04d", month.year), Month: plannerMonthNames[month.month]}, nil
}

func plannerRESTRange(window plannerWindow) (plannerRESTYearMonthRange, error) {
	start, err := plannerRESTMonth(window.Start)
	if err != nil {
		return plannerRESTYearMonthRange{}, err
	}
	end, err := plannerRESTMonth(window.End)
	if err != nil {
		return plannerRESTYearMonthRange{}, err
	}
	return plannerRESTYearMonthRange{Start: start, End: end}, nil
}

func buildPlannerIdeasRequest(input plannerIdeasInput, pageToken string) ([]byte, error) {
	rangeValue, err := plannerRESTRange(input.Window)
	if err != nil {
		return nil, err
	}
	geos := append([]string{}, input.GeoTargets...)
	if geos == nil {
		geos = []string{}
	}
	seeds := append([]string{}, input.Seeds...)
	request := plannerRESTIdeasRequest{
		IncludeAdultKeywords: input.IncludeAdult,
		GeoTargetConstants:   geos,
		PageToken:            pageToken,
		KeywordSeed:          plannerRESTKeywordSeed{Keywords: seeds},
		KeywordPlanNetwork:   input.Network,
		HistoricalMetricsOptions: plannerRESTHistoricalMetricsOptions{
			IncludeAverageCPC: true,
			YearMonthRange:    rangeValue,
		},
		Language: input.Language,
		PageSize: input.PageSize,
	}
	return json.Marshal(request)
}

func buildPlannerHistoricalRequest(input plannerHistoricalInput, keywords []string) ([]byte, error) {
	rangeValue, err := plannerRESTRange(input.Window)
	if err != nil {
		return nil, err
	}
	if len(keywords) == 0 {
		return nil, fmt.Errorf("historical request requires at least one keyword")
	}
	if err := validatePlannerKeywords(keywords); err != nil {
		return nil, err
	}
	geos := append([]string{}, input.GeoTargets...)
	if geos == nil {
		geos = []string{}
	}
	request := plannerRESTHistoricalRequest{
		Language:             input.Language,
		IncludeAdultKeywords: input.IncludeAdult,
		GeoTargetConstants:   geos,
		KeywordPlanNetwork:   input.Network,
		Keywords:             append([]string{}, keywords...),
		HistoricalMetricsOptions: plannerRESTHistoricalMetricsOptions{
			IncludeAverageCPC: true,
			YearMonthRange:    rangeValue,
		},
	}
	return json.Marshal(request)
}

// plannerKeywordBatches splits without sorting or deduplicating. Google may
// near-exact-deduplicate returned terms, so the submitted batch sequence is
// retained separately for provenance and no cross-batch identity is invented.
func plannerKeywordBatches(values []string, batchSize int) ([][]string, error) {
	if err := validatePlannerBatchSize(batchSize); err != nil {
		return nil, err
	}
	if err := validatePlannerKeywords(values); err != nil {
		return nil, err
	}
	result := make([][]string, 0, (len(values)+batchSize-1)/batchSize)
	for start := 0; start < len(values); start += batchSize {
		end := start + batchSize
		if end > len(values) {
			end = len(values)
		}
		result = append(result, append([]string{}, values[start:end]...))
	}
	return result, nil
}

func plannerCustomerPath(customerID, method string) (string, error) {
	customerID, err := normalizePlannerCustomerID(customerID)
	if err != nil {
		return "", err
	}
	if customerID == "" {
		return "", fmt.Errorf("customer target is unresolved")
	}
	suffix := strings.TrimSpace(method)
	switch suffix {
	case "ideas":
		return "/v25/customers/" + customerID + ":generateKeywordIdeas", nil
	case "historical":
		return "/v25/customers/" + customerID + ":generateKeywordHistoricalMetrics", nil
	default:
		return "", fmt.Errorf("unsupported Planner method %q", method)
	}
}
