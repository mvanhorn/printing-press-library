// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: shared HTTP fetch + URL builders for the rich goquery commands.

// Package motohunt holds the hand-written goquery scrapers that back the
// rich `search`, `get`, `makes`, `models`, `deal`, and `watch` commands.
// It deliberately does NOT route through internal/client, whose BaseURL is
// pinned to the config file (motohunt.com). The --site flag needs to swap the
// whole host + base search path at runtime, so this package owns its own tiny
// net/http client and builds absolute URLs from a SiteConfig.
package motohunt

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// userAgent is the desktop UA verified to get HTTP 200 with no bot wall on
// every content page of motohunt.com / atvhunt.com (see brief).
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// SiteConfig resolves a --site value into the host + base search path shared
// by every networked command. motohunt and atvhunt run the byte-identical HTML
// engine; only Base and SearchPath differ.
type SiteConfig struct {
	Site       string // "moto" | "atv"
	Base       string // e.g. https://motohunt.com (no trailing slash)
	SearchPath string // e.g. /motorcycles-for-sale
	Vehicle    string // human label: "motorcycle" | "ATV/UTV"
}

// ResolveSite maps a --site string to its SiteConfig. Default is moto. An
// unknown value is a usage error so a typo doesn't silently scrape the wrong
// marketplace.
func ResolveSite(site string) (SiteConfig, error) {
	switch strings.ToLower(strings.TrimSpace(site)) {
	case "", "moto", "motohunt", "motorcycle", "motorcycles":
		return SiteConfig{Site: "moto", Base: "https://motohunt.com", SearchPath: "/motorcycles-for-sale", Vehicle: "motorcycle"}, nil
	case "atv", "atvhunt", "utv", "sxs":
		return SiteConfig{Site: "atv", Base: "https://atvhunt.com", SearchPath: "/atv-utv-for-sale", Vehicle: "ATV/UTV"}, nil
	default:
		return SiteConfig{}, fmt.Errorf("unknown --site %q: must be moto or atv", site)
	}
}

// DetailURL returns the absolute detail URL for a listing id (slug optional;
// the site redirects to the canonical slug).
func (s SiteConfig) DetailURL(id string) string {
	return s.Base + "/l/" + id
}

// Client is a thin net/http wrapper that sends the desktop UA on every GET and
// returns a parsed goquery document.
type Client struct {
	HTTP *http.Client
}

// NewClient builds a Client over the provided *http.Client (so the caller's
// --timeout-bound transport is honored). Pass nil to use http.DefaultClient.
func NewClient(h *http.Client) *Client {
	if h == nil {
		h = http.DefaultClient
	}
	return &Client{HTTP: h}
}

// Fetch GETs rawURL with the desktop UA and returns the parsed document.
func (c *Client) Fetch(ctx context.Context, rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML from %s: %w", rawURL, err)
	}
	return doc, nil
}

// SearchParams collects the search facets and query knobs. The path-segment
// facet (Make/Model/Style/State) is resolved by priority in BuildSearchURL.
type SearchParams struct {
	Q        string
	Location string
	Make     string
	Model    string // combined as Make-Model when Make is also set
	Style    string
	State    string
	Sort     string // t|p|a|c
	Start    int
}

// BuildSearchURL assembles {base}{searchPath}[/{Facet}]?q=&location=&sort=&start=.
//
// Only ONE path-segment facet is honored by the site, so we apply a priority:
// make (+model as "Make-Model") > model alone > style > state. AppliedFacet
// names which one landed in the path; IgnoredFacets lists the rest so the
// result can tell the caller what was dropped. The remaining facets are NOT
// re-encoded as query params — the site ignores them there.
func (s SiteConfig) BuildSearchURL(p SearchParams) (rawURL, appliedFacet string, ignored []string) {
	facetSeg := ""
	add := func(name, seg string) {
		if facetSeg == "" {
			facetSeg = seg
			appliedFacet = name
		} else {
			ignored = append(ignored, name)
		}
	}
	switch {
	case p.Make != "" && p.Model != "":
		add("make-model", facetSlug(p.Make)+"-"+facetSlug(p.Model))
	case p.Make != "":
		add("make", facetSlug(p.Make))
	case p.Model != "":
		add("model", facetSlug(p.Model))
	}
	if p.Style != "" {
		add("style", facetSlug(p.Style))
	}
	if p.State != "" {
		add("state", facetSlug(p.State))
	}

	path := s.SearchPath
	if facetSeg != "" {
		path += "/" + facetSeg
	}
	u, _ := url.Parse(s.Base + path)
	q := u.Query()
	if p.Q != "" {
		q.Set("q", p.Q)
	}
	if p.Location != "" {
		q.Set("location", p.Location)
	}
	if p.Sort != "" {
		q.Set("sort", p.Sort)
	}
	if p.Start > 0 {
		q.Set("start", fmt.Sprintf("%d", p.Start))
	}
	u.RawQuery = q.Encode()
	return u.String(), appliedFacet, ignored
}

// ModelSelectorURL returns the make->model cascade fragment URL for a make.
func (s SiteConfig) ModelSelectorURL(make string) string {
	u, _ := url.Parse(s.Base + "/model-selector")
	q := u.Query()
	q.Set("make", make)
	u.RawQuery = q.Encode()
	return u.String()
}

// facetSlug normalizes a user-supplied facet value into the site's path-segment
// form: spaces become hyphens (the site uses "Harley-Davidson", "BMW-S-1000-RR").
// Already-hyphenated input passes through unchanged.
func facetSlug(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, " ", "-")
	return v
}
