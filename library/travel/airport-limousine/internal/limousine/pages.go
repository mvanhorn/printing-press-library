// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package limousine

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// PageSummary exposes canonical links without the page's embedded application data.
type PageSummary struct {
	Title     string              `json:"title"`
	SourceURL string              `json:"source_url"`
	Links     []map[string]string `json:"links"`
}

var pageTitle = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)

func (p *Provider) PageLinks(ctx context.Context, path string, prefixes []string) (Envelope, error) {
	body, err := p.Fetch(ctx, http.MethodGet, path, "")
	if err != nil {
		return Envelope{}, err
	}
	page, total, err := ParsePageLinks(string(body), Origin+path, prefixes)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Meta: p.Meta([]string{page.SourceURL}, total, total, len(page.Links), "Canonical page links only. Embedded application, reservation and inventory data are omitted. Timetables do not prove seats."), Results: []PageSummary{page}}, nil
}

func ParsePageLinks(body, source string, prefixes []string) (PageSummary, int, error) {
	page := PageSummary{SourceURL: source, Links: []map[string]string{}}
	if title := pageTitle.FindStringSubmatch(body); len(title) > 1 {
		page.Title = CleanHTML(title[1])
	}
	links, err := SourceLinks(body)
	if err != nil {
		return page, 0, err
	}
	total := 0
	for _, link := range links {
		u, err := url.Parse(link["url"])
		if err != nil || u.Host != "www.limousinebus.co.jp" || u.Scheme != "https" {
			continue
		}
		matches := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(u.Path, prefix) {
				matches = true
				break
			}
		}
		if !matches {
			continue
		}
		total++
		if len(page.Links) < 200 {
			page.Links = append(page.Links, link)
		}
	}
	return page, total, nil
}
