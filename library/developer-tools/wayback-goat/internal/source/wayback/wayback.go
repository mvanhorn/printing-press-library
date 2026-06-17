// Copyright 2026 Alex Bresler and contributors. Licensed under Apache-2.0. See LICENSE.

// Package wayback wraps the Internet Archive CDX Server API
// (https://web.archive.org/cdx/search/cdx) for capture-history and
// content-change analytics. No authentication is required.
//
// The CDX index returns one row per archived capture, including the content
// digest (a SHA1 of the archived payload). Consecutive identical digests mean
// the page did not change between captures; a digest flip marks a real content
// change. The Changes function turns that property into a change feed — the
// analysis the Wayback web UI and URL-listing tools do not provide.
package wayback

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

const cdxBaseURL = "https://web.archive.org"

// Client queries the Wayback CDX server. The zero value is not usable; call
// NewClient. baseURL is unexported so tests can point it at an httptest server.
type Client struct {
	HTTP    *http.Client
	baseURL string
}

// NewClient returns a Client with a sane timeout against the public CDX server.
func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}, baseURL: cdxBaseURL}
}

// Capture is one archived snapshot row from the CDX index.
type Capture struct {
	Timestamp string `json:"timestamp"` // YYYYMMDDhhmmss, UTC
	Original  string `json:"original"`
	Status    string `json:"status"`
	MimeType  string `json:"mimetype"`
	Digest    string `json:"digest"` // SHA1 (base32) of the archived payload
}

// Time parses the CDX timestamp into a UTC time.Time.
func (c Capture) Time() (time.Time, error) {
	return time.Parse("20060102150405", c.Timestamp)
}

// CapturesOptions tunes a Captures query.
type CapturesOptions struct {
	MatchType string // exact|prefix|host|domain (default: server default = exact)
	From      string // lower-bound timestamp prefix
	To        string // upper-bound timestamp prefix
	Limit     int    // 0 = no limit; negative = most-recent N
	Status200 bool   // restrict to HTTP 200 captures (filters revisits/redirects/404s)
}

// Captures returns the chronological capture list the Wayback Machine holds for
// target. An empty result (no captures) returns (nil, nil), distinct from an error.
func (c *Client) Captures(ctx context.Context, target string, opt CapturesOptions) ([]Capture, error) {
	q := url.Values{}
	q.Set("url", target)
	q.Set("output", "json")
	q.Set("fl", "timestamp,original,statuscode,mimetype,digest")
	if opt.MatchType != "" {
		q.Set("matchType", opt.MatchType)
	}
	if opt.From != "" {
		q.Set("from", opt.From)
	}
	if opt.To != "" {
		q.Set("to", opt.To)
	}
	if opt.Limit != 0 {
		q.Set("limit", strconv.Itoa(opt.Limit))
	}
	if opt.Status200 {
		q.Set("filter", "statuscode:200")
	}
	u := strings.TrimRight(c.baseURL, "/") + "/cdx/search/cdx?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	// Identify the client so the Archive (a nonprofit) can attribute and, if
	// needed, contact traffic — basic courtesy for an unauthenticated public API.
	req.Header.Set("User-Agent", "wayback-goat (+https://github.com/mvanhorn/printing-press-library)")
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
		return nil, fmt.Errorf("wayback cdx %d: %s", resp.StatusCode, brief(body))
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, nil // empty body = no captures
	}
	var rows [][]string
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode cdx response: %w", err)
	}
	if len(rows) <= 1 {
		return nil, nil // header row only (or empty) = no captures
	}
	out := make([]Capture, 0, len(rows)-1)
	for _, r := range rows[1:] { // row 0 is the field header
		if len(r) < 5 {
			continue
		}
		out = append(out, Capture{Timestamp: r[0], Original: r[1], Status: r[2], MimeType: r[3], Digest: r[4]})
	}
	return out, nil
}

// Change is a detected content change between two consecutive kept captures.
type Change struct {
	Timestamp  string `json:"timestamp"`
	PrevDigest string `json:"prev_digest"`
	NewDigest  string `json:"new_digest"`
	Status     string `json:"status"`
}

// Changes collapses a chronological capture list into content-change events.
//
// The first capture establishes the baseline (first-seen) and is never itself a
// change; every later capture whose digest differs from the last kept digest is
// a change event. Captures with an empty digest are skipped (they carry no
// content identity). Callers should pass a status-filtered list (HTTP 200) so a
// transient 404/redirect snapshot is not mistaken for a content change.
func Changes(caps []Capture) (firstSeen *Capture, changes []Change) {
	if len(caps) == 0 {
		return nil, nil
	}
	fs := caps[0]
	last := caps[0].Digest
	for _, c := range caps[1:] {
		if c.Digest == "" || c.Digest == last {
			continue
		}
		changes = append(changes, Change{Timestamp: c.Timestamp, PrevDigest: last, NewDigest: c.Digest, Status: c.Status})
		last = c.Digest
	}
	return &fs, changes
}

func brief(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
