// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Open Library is passage's metadata spine: search + work/author records.
// Keyless, but policy asks for a real User-Agent (anonymous ~1 req/s).
package openlibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	baseURL   = "https://openlibrary.org"
	userAgent = "passage/1.0 (+https://github.com/justinwfu/passage)"
)

type Doc struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	AuthorName  []string `json:"author_name"`
	Year        int      `json:"first_publish_year"`
	HasFulltext bool     `json:"has_fulltext"`
	EbookAccess string   `json:"ebook_access"`
	Editions    int      `json:"edition_count"`
}

func (d Doc) Author() string {
	if len(d.AuthorName) == 0 {
		return "Unknown"
	}
	return strings.Join(d.AuthorName, ", ")
}

type Client struct{ hc *http.Client }

func New() *Client { return &Client{hc: &http.Client{Timeout: 25 * time.Second}} }

// Search returns up to limit works matching query.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Doc, error) {
	if limit <= 0 {
		limit = 20
	}
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("fields", "key,title,author_name,first_publish_year,has_fulltext,ebook_access,edition_count")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/search.json?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library search -> HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var out struct {
		Docs []Doc `json:"docs"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parsing open library search: %w", err)
	}
	return out.Docs, nil
}
