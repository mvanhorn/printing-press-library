// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
)

const federalRegisterDocumentsURL = "https://www.federalregister.gov/api/v1/documents.json"

type federalRegisterOptions struct {
	Term   string
	Agency string
	Since  string
	Types  []string
	Limit  int
	Kind   string
	Source string
}

type federalRegisterResponse struct {
	Count      int `json:"count"`
	TotalPages int `json:"total_pages"`
	Results    []struct {
		Title           string `json:"title"`
		Type            string `json:"type"`
		Abstract        string `json:"abstract"`
		DocumentNumber  string `json:"document_number"`
		HTMLURL         string `json:"html_url"`
		PDFURL          string `json:"pdf_url"`
		PublicationDate string `json:"publication_date"`
		Excerpts        string `json:"excerpts"`
		Agencies        []struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"agencies"`
	} `json:"results"`
}

func fetchFederalRegister(ctx context.Context, opts federalRegisterOptions) (FederalRegisterSearchResult, error) {
	limit := normalizeLimit(opts.Limit, 1, 50)
	query := url.Values{
		"order":    []string{"newest"},
		"per_page": []string{strconv.Itoa(limit)},
	}
	if opts.Term != "" {
		query.Set("conditions[term]", opts.Term)
	}
	if opts.Agency != "" {
		query.Add("conditions[agencies][]", opts.Agency)
	}
	if opts.Since != "" {
		query.Set("conditions[publication_date][gte]", opts.Since)
	}
	for _, docType := range opts.Types {
		query.Add("conditions[type][]", docType)
	}
	body, err := getJSON(ctx, federalRegisterDocumentsURL, query, nil)
	if err != nil {
		return FederalRegisterSearchResult{}, err
	}
	result, err := parseFederalRegister(body)
	if err != nil {
		return FederalRegisterSearchResult{}, err
	}
	kind := opts.Kind
	if kind == "" {
		kind = "federal_register_search"
	}
	source := opts.Source
	if source == "" {
		source = "FederalRegister.gov API"
	}
	return FederalRegisterSearchResult{
		Kind:       kind,
		Source:     source,
		Query:      querySummary(opts.Term, opts.Agency, opts.Since, strings.Join(opts.Types, ","), limit),
		Count:      result.Count,
		TotalPages: result.TotalPages,
		Results:    result.Results,
		Caveats: []string{
			"FederalRegister.gov is an informational XML/JSON rendering; use linked official PDFs on govinfo.gov for legal reliance.",
			"Agency filters use FederalRegister.gov agency slugs such as federal-trade-commission.",
		},
		SourceLinks: []string{
			"https://www.federalregister.gov/developers/documentation/api/v1",
		},
	}, nil
}

func parseFederalRegister(body []byte) (FederalRegisterSearchResult, error) {
	var payload federalRegisterResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return FederalRegisterSearchResult{}, err
	}
	results := make([]FederalRegisterDocument, 0, len(payload.Results))
	for _, item := range payload.Results {
		agencies := make([]string, 0, len(item.Agencies))
		for _, agency := range item.Agencies {
			if agency.Name != "" {
				agencies = append(agencies, agency.Name)
			}
		}
		results = append(results, FederalRegisterDocument{
			Title:           item.Title,
			Type:            item.Type,
			DocumentNumber:  item.DocumentNumber,
			PublicationDate: item.PublicationDate,
			Agencies:        agencies,
			Abstract:        item.Abstract,
			HTMLURL:         item.HTMLURL,
			PDFURL:          item.PDFURL,
			Excerpt:         cleanExcerpt(item.Excerpts),
		})
	}
	return FederalRegisterSearchResult{
		Count:      payload.Count,
		TotalPages: payload.TotalPages,
		Results:    results,
	}, nil
}

func normalizeLimit(limit, minValue, maxValue int) int {
	if limit <= 0 {
		return minValue
	}
	if limit < minValue {
		return minValue
	}
	if limit > maxValue {
		return maxValue
	}
	return limit
}

func querySummary(term, agency, since, types string, limit int) map[string]string {
	return map[string]string{
		"term":   term,
		"agency": agency,
		"since":  since,
		"types":  types,
		"limit":  fmt.Sprintf("%d", limit),
	}
}

func cleanExcerpt(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "<span class=\"match\">", "")
	value = strings.ReplaceAll(value, "</span>", "")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}
