// Copyright 2026 quantumglitch. Licensed under Apache-2.0. See LICENSE.

// Package mobalytics is a thin HTML-scraping client for mobalytics.gg.
//
// Mobalytics pages return ~1 MB of HTML with a normalized Apollo GraphQL
// cache embedded inline. We do not run a headless browser; we issue a
// single net/http GET with a real-browser User-Agent and tease the JSON
// out of the page with focused regexes.
package mobalytics

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RateLimitError is returned when Mobalytics responds 429 Too Many Requests.
// Callers (or a higher-level retry wrapper) can detect it via errors.As to
// honor the server's Retry-After hint instead of failing hard.
type RateLimitError struct {
	URL        string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("mobalytics: GET %s returned 429 (retry after %s)", e.URL, e.RetryAfter)
	}
	return fmt.Sprintf("mobalytics: GET %s returned 429", e.URL)
}

const baseURL = "https://mobalytics.gg"

// userAgent is the Chrome 126 UA that mobalytics.gg accepts without
// Cloudflare interactive challenge as of 2026-05.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// Client wraps an *http.Client with the headers Mobalytics requires.
type Client struct {
	HTTP *http.Client
}

// NewClient builds a client with a sensible timeout.
func NewClient(timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{HTTP: &http.Client{Timeout: timeout}}
}

// Fetch GETs a Mobalytics path (relative or absolute) and returns the
// HTML body. It applies the required headers; missing them produces a
// Cloudflare 403 in practice.
func (c *Client) Fetch(rawURL string) (string, error) {
	if c == nil || c.HTTP == nil {
		c = NewClient(30 * time.Second)
	}
	full := rawURL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		if !strings.HasPrefix(rawURL, "/") {
			full = baseURL + "/" + rawURL
		} else {
			full = baseURL + rawURL
		}
	}
	req, err := http.NewRequest(http.MethodGet, full, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", baseURL+"/")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		ra := time.Duration(0)
		if h := resp.Header.Get("Retry-After"); h != "" {
			if secs, perr := strconv.Atoi(h); perr == nil && secs > 0 {
				ra = time.Duration(secs) * time.Second
			}
		}
		return "", &RateLimitError{URL: full, RetryAfter: ra}
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("mobalytics: GET %s returned %d", full, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	html := string(body)
	if len(html) < 1024 {
		return html, errors.New("mobalytics: response body suspiciously small; possible challenge page")
	}
	return html, nil
}

// withQuery composes a URL with an optional key/value pair appended as
// query string; empty value yields the input unchanged.
func withQuery(in, key, value string) string {
	if value == "" {
		return in
	}
	if strings.Contains(in, "?") {
		return in + "&" + key + "=" + url.QueryEscape(value)
	}
	return in + "?" + key + "=" + url.QueryEscape(value)
}

// TierListPath returns the canonical tier-list URL with optional filters.
//
// role: "TOP" | "JUNGLE" | "MID" | "ADC" | "SUPPORT" | "" (all)
// rank: "low-elo" | "high-elo" | ""
// patch: "16.10" | "" (current)
// region: "kr" | "euw" | "na" | "" (default)
func TierListPath(role, rank, patch, region string) string {
	u := "/lol/tier-list/"
	u = withQuery(u, "role", strings.ToLower(role))
	u = withQuery(u, "skillLevel", rank)
	u = withQuery(u, "patch", patch)
	u = withQuery(u, "region", region)
	return u
}

// ChampionPath returns a path for a champion sub-page.
//
//	page: "build" | "counters" | "matchups" | "aram-builds" | "arena-builds" | "combos" | "runes"
func ChampionPath(slug, page string) string {
	if slug == "" {
		return ""
	}
	if page == "" {
		page = "build"
	}
	return fmt.Sprintf("/lol/champions/%s/%s", strings.ToLower(slug), page)
}
