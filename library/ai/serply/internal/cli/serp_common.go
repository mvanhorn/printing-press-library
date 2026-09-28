// Copyright 2026 googio and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/client"
)

// serpOptions carries the request knobs shared by the hand-written SERP
// commands (rank, serp diff, research). Location and device travel as the
// X-Proxy-Location and X-User-Agent headers; gl and hl are query params.
type serpOptions struct {
	Num      int
	Location string
	Device   string
	Gl       string
	Hl       string
}

func (o serpOptions) params(q string) map[string]string {
	p := map[string]string{"q": q}
	if o.Num > 0 {
		p["num"] = strconv.Itoa(o.Num)
	}
	if o.Gl != "" {
		p["gl"] = o.Gl
	}
	if o.Hl != "" {
		p["hl"] = o.Hl
	}
	return p
}

func (o serpOptions) headers() map[string]string {
	h := map[string]string{}
	if o.Location != "" {
		h["X-Proxy-Location"] = strings.ToUpper(o.Location)
	}
	if o.Device != "" {
		h["X-User-Agent"] = strings.ToLower(o.Device)
	}
	return h
}

func validateDevice(device string) error {
	switch strings.ToLower(device) {
	case "", "desktop", "mobile":
		return nil
	}
	return usageErr(fmt.Errorf("invalid value %q for --x-user-agent: must be desktop or mobile", device))
}

// serpResult is one organic result, normalized across verticals.
type serpResult struct {
	Position  int    `json:"position"`
	Title     string `json:"title"`
	Link      string `json:"link"`
	Domain    string `json:"domain"`
	Snippet   string `json:"snippet,omitempty"`
	Published string `json:"published,omitempty"`
	Author    string `json:"author,omitempty"`
}

// verticalSpec maps a vertical name to its path and the response array key.
type verticalSpec struct {
	Name string
	Path string
	Key  string
}

var serpVerticals = map[string]verticalSpec{
	"web":     {Name: "web", Path: "/v1/search", Key: "results"},
	"news":    {Name: "news", Path: "/v1/news", Key: "entries"},
	"scholar": {Name: "scholar", Path: "/v1/scholar", Key: "articles"},
}

// fetchVertical runs one query-string search (?q=...) against a vertical and
// returns its results with 1-based positions in response order. Some verticals
// (news) ignore num upstream, so the result list is capped at opts.Num here.
func fetchVertical(ctx context.Context, c *client.Client, v verticalSpec, q string, opts serpOptions, noCache bool) ([]serpResult, error) {
	var (
		data json.RawMessage
		err  error
	)
	if noCache {
		data, err = c.GetWithHeadersNoCache(ctx, v.Path, opts.params(q), opts.headers())
	} else {
		data, err = c.GetWithHeaders(ctx, v.Path, opts.params(q), opts.headers())
	}
	if err != nil {
		return nil, err
	}
	results, err := parseSerpResults(data, v.Key)
	if err != nil {
		return nil, err
	}
	if opts.Num > 0 && len(results) > opts.Num {
		results = results[:opts.Num]
	}
	return results, nil
}

func parseSerpResults(data json.RawMessage, key string) ([]serpResult, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}
	raw, ok := envelope[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return []serpResult{}, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", key, err)
	}
	out := make([]serpResult, 0, len(items))
	for _, item := range items {
		link := stringField(item, "link")
		if link == "" {
			continue
		}
		r := serpResult{
			Position:  len(out) + 1,
			Title:     strings.TrimSpace(stringField(item, "title")),
			Link:      link,
			Domain:    hostOf(link),
			Snippet:   strings.TrimSpace(strings.ReplaceAll(stringField(item, "description"), "\u00a0", " ")),
			Published: stringField(item, "published"),
			Author:    stringField(item, "author"),
		}
		// Scholar nests authors as {"names": "...", "authors": [...]}.
		if a, ok := item["author"].(map[string]any); ok {
			r.Author = strings.TrimSpace(strings.ReplaceAll(stringField(a, "names"), "\u00a0", " "))
		}
		// News links are news.google.com redirects; source.href names the
		// publisher, which is the domain a reader cares about.
		if src, ok := item["source"].(map[string]any); ok {
			if r.Author == "" {
				r.Author = stringField(src, "title")
			}
			if host := hostOf(stringField(src, "href")); host != "" {
				r.Domain = host
			}
		}
		out = append(out, r)
	}
	return out, nil
}

func stringField(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// hostOf returns the lowercase host of a URL without a leading "www.".
func hostOf(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// normalizeDomain accepts "example.com", "www.example.com" or a full URL.
func normalizeDomain(d string) string {
	d = strings.TrimSpace(strings.ToLower(d))
	if strings.Contains(d, "://") {
		return hostOf(d)
	}
	d = strings.SplitN(d, "/", 2)[0]
	return strings.TrimPrefix(d, "www.")
}

// domainMatches reports whether host is domain or one of its subdomains.
func domainMatches(host, domain string) bool {
	if host == "" || domain == "" {
		return false
	}
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// normalizeLink gives a stable identity for a result URL: lowercase host,
// no "www.", no fragment, no trailing slash.
func normalizeLink(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Host == "" {
		return strings.TrimRight(strings.TrimSpace(link), "/")
	}
	u.Fragment = ""
	u.Host = strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	u.Scheme = strings.ToLower(u.Scheme)
	return strings.TrimRight(u.String(), "/")
}
