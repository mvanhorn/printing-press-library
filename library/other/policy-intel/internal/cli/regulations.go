// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

const regulationsBaseURL = "https://api.regulations.gov/v4"

type regulationsListOptions struct {
	Kind         string
	Term         string
	Agency       string
	DocketID     string
	CommentEndGE string
	Sort         string
	Limit        int
}

type regulationsListResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			AgencyID            string `json:"agencyId"`
			Title               string `json:"title"`
			DocumentType        string `json:"documentType"`
			DocketID            string `json:"docketId"`
			FRDocumentNumber    string `json:"frDocNum"`
			PostedDate          string `json:"postedDate"`
			CommentStartDate    string `json:"commentStartDate"`
			CommentEndDate      string `json:"commentEndDate"`
			OpenForComment      bool   `json:"openForComment"`
			WithinCommentPeriod bool   `json:"withinCommentPeriod"`
		} `json:"attributes"`
	} `json:"data"`
	Meta struct {
		TotalElements int  `json:"totalElements"`
		PageSize      int  `json:"pageSize"`
		HasNextPage   bool `json:"hasNextPage"`
	} `json:"meta"`
}

type regulationsDocketResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			AgencyID   string `json:"agencyId"`
			Title      string `json:"title"`
			DocketType string `json:"docketType"`
			ModifyDate string `json:"modifyDate"`
			Abstract   string `json:"dkAbstract"`
			ShortTitle string `json:"shortTitle"`
			RIN        string `json:"rin"`
		} `json:"attributes"`
	} `json:"data"`
}

func fetchRegulationsDocuments(ctx context.Context, opts regulationsListOptions) (RegulationsListResult, error) {
	limit := normalizeLimit(opts.Limit, 5, 50)
	query := regulationsQuery(opts, limit)
	body, err := getJSON(ctx, regulationsBaseURL+"/documents", query, nil)
	if err != nil {
		return RegulationsListResult{}, err
	}
	result, err := parseRegulationsList(body)
	if err != nil {
		return RegulationsListResult{}, err
	}
	result.Kind = opts.Kind
	if result.Kind == "" {
		result.Kind = "regulations_documents"
	}
	result.Source = "Regulations.gov API v4"
	result.Query = map[string]string{
		"term":           opts.Term,
		"agency":         opts.Agency,
		"docket_id":      opts.DocketID,
		"comment_end_ge": opts.CommentEndGE,
		"sort":           opts.Sort,
	}
	result.Caveats = []string{
		"Regulations.gov data fields vary by agency and document type.",
		"DEMO_KEY is suitable for smoke testing; set POLICY_INTEL_REGULATIONS_API_KEY for regular use.",
	}
	result.SourceLinks = []string{"https://open.gsa.gov/api/regulationsgov/"}
	return result, nil
}

func fetchRegulationsComments(ctx context.Context, docketID string, limit int) (RegulationsListResult, error) {
	opts := regulationsListOptions{
		Kind:     "regulations_comments",
		DocketID: docketID,
		Limit:    limit,
		Sort:     "-postedDate",
	}
	query := regulationsQuery(opts, normalizeLimit(limit, 5, 50))
	body, err := getJSON(ctx, regulationsBaseURL+"/comments", query, nil)
	if err != nil {
		return RegulationsListResult{}, err
	}
	result, err := parseRegulationsList(body)
	if err != nil {
		return RegulationsListResult{}, err
	}
	result.Kind = "regulations_comments"
	result.Source = "Regulations.gov API v4"
	result.Query = map[string]string{"docket_id": docketID, "sort": opts.Sort}
	result.Caveats = []string{
		"Comment fields are limited to fields Regulations.gov exposes publicly for each agency.",
	}
	result.SourceLinks = []string{"https://open.gsa.gov/api/regulationsgov/"}
	return result, nil
}

func fetchRegulationsDocket(ctx context.Context, docketID string) (DocketResult, error) {
	query := url.Values{"api_key": []string{regulationsAPIKey()}}
	body, err := getJSON(ctx, regulationsBaseURL+"/dockets/"+url.PathEscape(docketID), query, nil)
	if err != nil {
		return DocketResult{}, err
	}
	var payload regulationsDocketResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return DocketResult{}, err
	}
	attrs := payload.Data.Attributes
	return DocketResult{
		Kind:       "regulations_docket",
		Source:     "Regulations.gov API v4",
		ID:         payload.Data.ID,
		AgencyID:   attrs.AgencyID,
		Title:      attrs.Title,
		DocketType: attrs.DocketType,
		ModifyDate: attrs.ModifyDate,
		Abstract:   attrs.Abstract,
		RawFields: map[string]string{
			"short_title": attrs.ShortTitle,
			"rin":         attrs.RIN,
		},
		SourceLinks: []string{"https://open.gsa.gov/api/regulationsgov/"},
	}, nil
}

func regulationsQuery(opts regulationsListOptions, limit int) url.Values {
	query := url.Values{
		"api_key":      []string{regulationsAPIKey()},
		"page[size]":   []string{strconv.Itoa(limit)},
		"page[number]": []string{"1"},
	}
	if opts.Term != "" {
		query.Set("filter[searchTerm]", opts.Term)
	}
	if opts.Agency != "" {
		query.Set("filter[agencyId]", opts.Agency)
	}
	if opts.DocketID != "" {
		query.Set("filter[docketId]", opts.DocketID)
	}
	if opts.CommentEndGE != "" {
		query.Set("filter[commentEndDate][ge]", opts.CommentEndGE)
	}
	if opts.Sort != "" {
		query.Set("sort", opts.Sort)
	}
	return query
}

func parseRegulationsList(body []byte) (RegulationsListResult, error) {
	var payload regulationsListResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return RegulationsListResult{}, err
	}
	results := make([]RegulationsDocument, 0, len(payload.Data))
	for _, item := range payload.Data {
		attrs := item.Attributes
		results = append(results, RegulationsDocument{
			ID:                  item.ID,
			AgencyID:            attrs.AgencyID,
			Title:               attrs.Title,
			DocumentType:        attrs.DocumentType,
			DocketID:            attrs.DocketID,
			FRDocumentNumber:    attrs.FRDocumentNumber,
			PostedDate:          attrs.PostedDate,
			CommentStartDate:    attrs.CommentStartDate,
			CommentEndDate:      attrs.CommentEndDate,
			OpenForComment:      attrs.OpenForComment,
			WithinCommentPeriod: attrs.WithinCommentPeriod,
		})
	}
	return RegulationsListResult{
		PageSize:    payload.Meta.PageSize,
		Total:       payload.Meta.TotalElements,
		HasNextPage: payload.Meta.HasNextPage,
		Results:     results,
	}, nil
}

func regulationsAPIKey() string {
	if key := env("POLICY_INTEL_REGULATIONS_API_KEY"); key != "" {
		return key
	}
	return "DEMO_KEY"
}

func todayDate() string {
	return time.Now().UTC().Format("2006-01-02")
}
