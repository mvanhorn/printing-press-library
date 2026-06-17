// Package hn wraps the Hacker News Algolia search API at
// https://hn.algolia.com/api/v1/. Used for Show HN posts and mention
// timeline.
package hn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	HTTP    *http.Client
	baseURL string
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, baseURL: hnBaseURL}
}

// Hit is one HN story or comment from the Algolia index.
type Hit struct {
	ObjectID    string   `json:"objectID"`
	StoryID     int      `json:"story_id"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Author      string   `json:"author"`
	Points      int      `json:"points"`
	NumComments int      `json:"num_comments"`
	CreatedAt   string   `json:"created_at"`
	CreatedAtI  int64    `json:"created_at_i"`
	Tags        []string `json:"_tags"`
}

// SearchResponse is the Algolia search envelope.
type SearchResponse struct {
	Hits         []Hit  `json:"hits"`
	NbHits       int    `json:"nbHits"`
	Page         int    `json:"page"`
	NbPages      int    `json:"nbPages"`
	HitsPerPage  int    `json:"hitsPerPage"`
	ProcessingMS int    `json:"processingTimeMS"`
	Query        string `json:"query"`
}

// SearchShowHN finds Show HN posts mentioning query. Sorted by relevance.
func (c *Client) SearchShowHN(ctx context.Context, query string, hitsPerPage int) (*SearchResponse, error) {
	if hitsPerPage <= 0 {
		hitsPerPage = 20
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("tags", "show_hn")
	q.Set("hitsPerPage", strconv.Itoa(hitsPerPage))
	return c.search(ctx, "search", q)
}

// SearchAll runs a relevance-sorted full-text search over stories matching
// query. Used for mention surfaces.
func (c *Client) SearchAll(ctx context.Context, query string, hitsPerPage int) (*SearchResponse, error) {
	if hitsPerPage <= 0 {
		hitsPerPage = 20
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("tags", "story")
	q.Set("hitsPerPage", strconv.Itoa(hitsPerPage))
	return c.search(ctx, "search", q)
}

// SearchByDate runs a chronological search; useful for building a mention
// timeline. hitsPerPage default 100 (max 1000 by Algolia).
func (c *Client) SearchByDate(ctx context.Context, query string, hitsPerPage int) (*SearchResponse, error) {
	if hitsPerPage <= 0 {
		hitsPerPage = 100
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("tags", "story")
	q.Set("hitsPerPage", strconv.Itoa(hitsPerPage))
	return c.search(ctx, "search_by_date", q)
}

// hnBaseURL is the public Hacker News Algolia search proxy.
const hnBaseURL = "https://hn.algolia.com/api/v1/"

// buildURL assembles an Algolia request URL with typo tolerance disabled.
//
// Algolia's default typoTolerance=true allows up to two character edits on
// words of eight or more characters. A long company token such as
// "pwcommunications" (16 chars) is therefore a two-deletion fuzzy match for the
// ordinary word "communications", which surfaces unrelated stories (e.g. an
// AI-box thread that merely contains the word "communications"). Company
// lookups want exact-token matching, so typo tolerance is turned off for every
// search. The public hn.algolia.com proxy honors this parameter; genuine
// mentions are unaffected (an exact "stripe" search still returns thousands).
func buildURL(base, endpoint string, q url.Values) string {
	if base == "" {
		base = hnBaseURL
	}
	// Copy so the caller's url.Values is never mutated, and normalize the
	// base/endpoint join so a missing or extra trailing slash cannot produce a
	// malformed URL (e.g. ".../v1search?...").
	vals := url.Values{}
	for k, v := range q {
		vals[k] = v
	}
	vals.Set("typoTolerance", "false")
	return strings.TrimRight(base, "/") + "/" + endpoint + "?" + vals.Encode()
}

func (c *Client) search(ctx context.Context, endpoint string, q url.Values) (*SearchResponse, error) {
	u := buildURL(c.baseURL, endpoint, q)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hn algolia %d: %s", resp.StatusCode, briefBody(body))
	}
	var sr SearchResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("decode hn response: %w", err)
	}
	return &sr, nil
}

func briefBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
