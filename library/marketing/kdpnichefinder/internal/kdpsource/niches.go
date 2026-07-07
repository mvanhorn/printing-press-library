// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// Package kdpsource holds pure, dependency-free helpers for parsing the KDP
// Niche Finder Inertia.js bucket pages. The bucket route returns a full HTML
// page with the Inertia payload embedded in a `data-page` attribute; the
// helpers here extract and decode that payload without importing internal/cli
// (avoiding an import cycle) or making any network calls.
package kdpsource

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
)

// Buckets are the four curated niche buckets exposed by the site as
// /app/category/{bucket} routes.
var Buckets = []string{"evergreen", "fresh_money", "hidden_gems", "high_ticket"}

// Book is a single niche row from props.books.data. JSON tags match the
// real Inertia payload exactly.
type Book struct {
	ID                      int     `json:"id"`
	Title                   string  `json:"title"`
	AmazonURL               string  `json:"amazon_url"`
	ImageURL                string  `json:"image_url"`
	Price                   string  `json:"price"`
	Publisher               string  `json:"publisher"`
	EstimatedMonthlySales   int     `json:"estimated_monthly_sales"`
	EstimatedMonthlyRevenue float64 `json:"estimated_monthly_revenue"`
}

// dataPageRE captures the HTML-entity-escaped JSON in the `data-page`
// attribute of the root <div id="app">.
var dataPageRE = regexp.MustCompile(`data-page="([^"]*)"`)

// asinRE extracts the 10-character alphanumeric ASIN from a /dp/ Amazon URL.
var asinRE = regexp.MustCompile(`/dp/([A-Za-z0-9]{10})`)

// ASIN extracts the /dp/XXXXXXXXXX ASIN from an Amazon URL, or "" if none.
func ASIN(amazonURL string) string {
	m := asinRE.FindStringSubmatch(amazonURL)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// inertiaEnvelope mirrors the decoded data-page JSON. Only the fields we
// consume are declared.
type inertiaEnvelope struct {
	Props struct {
		Books struct {
			CurrentPage int    `json:"current_page"`
			LastPage    int    `json:"last_page"`
			PerPage     int    `json:"per_page"`
			Total       int    `json:"total"`
			Data        []Book `json:"data"`
		} `json:"books"`
	} `json:"props"`
}

// ParseDataPage extracts the data-page attribute from an Inertia bucket page,
// HTML-unescapes it, and decodes the books array plus pagination cursors.
func ParseDataPage(htmlBytes []byte) (books []Book, currentPage, lastPage int, err error) {
	m := dataPageRE.FindSubmatch(htmlBytes)
	if len(m) < 2 {
		return nil, 0, 0, fmt.Errorf("no data-page attribute found in HTML (got %d bytes)", len(htmlBytes))
	}
	decoded := html.UnescapeString(string(m[1]))
	var env inertiaEnvelope
	if err := json.Unmarshal([]byte(decoded), &env); err != nil {
		return nil, 0, 0, fmt.Errorf("decoding data-page JSON: %w", err)
	}
	return env.Props.Books.Data, env.Props.Books.CurrentPage, env.Props.Books.LastPage, nil
}
