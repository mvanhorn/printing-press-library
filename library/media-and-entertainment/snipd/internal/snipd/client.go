// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored Snipd source layer — NOT generator output.
// pp:data-source live
//
// client.go is the keyed HTTP layer for the sanctioned Snipd "Obsidian export"
// API. It sits beside the generated internal/client (which is JSON-only and can't
// handle the ZIP export response). Two calls only:
//   - FetchMetadata  GET  /obsidian/fetch-export-metadata   (JSON: which episodes exist)
//   - ExportEpisodes POST /obsidian/export-episode-snips     (ZIP of server-rendered markdown)
//
// Read-only, own account. The Bearer token is never logged (see MaskToken).
package snipd

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/cliutil"
)

const (
	metadataPath   = "/obsidian/fetch-export-metadata"
	exportPath     = "/obsidian/export-episode-snips"
	maxResponse    = 512 << 20 // 512 MiB — a full-account export batch is well under this
	defaultRateSec = 1.0       // polite: the export POST is expensive server-side
	errSnippetLen  = 300
)

// Client is the keyed Snipd export client.
type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	base    string
	token   string
}

// NewClient builds a Snipd export client. baseURL and token come from the
// generated config (config.BaseURL, config.SnipdToken) — there is no second
// credential channel.
func NewClient(baseURL, token string) *Client {
	return &Client{
		// No client-level deadline: the export POST is slow (the server renders
		// markdown for every episode in the batch). Cancellation flows through
		// the request context instead.
		http:    &http.Client{Timeout: 0},
		limiter: cliutil.NewAdaptiveLimiter(defaultRateSec),
		base:    strings.TrimRight(baseURL, "/"),
		token:   token,
	}
}

// HasToken reports whether a credential is present (value never revealed).
func (c *Client) HasToken() bool { return strings.TrimSpace(c.token) != "" }

// MaskToken renders a token for diagnostics: presence-revealing, value-hiding.
func MaskToken(t string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return "(not set)"
	}
	return fmt.Sprintf("present (%d chars)", len(t))
}

// do is the single choke point: rate-limit, auth header, size cap, and typed
// 429/auth/error classification. Returns the raw response body.
func (c *Client) do(ctx context.Context, method, reqPath string, body []byte) ([]byte, error) {
	c.limiter.Wait()
	full := c.base + reqPath
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, full, rdr)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", reqPath, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", reqPath, err)
	}
	if int64(len(data)) > maxResponse {
		return nil, fmt.Errorf("%s response exceeded %d bytes", reqPath, maxResponse)
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{
			URL:        full,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
			Body:       snippet(data),
		}
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("snipd auth failed (HTTP %d): set a valid SNIPD_TOKEN — get a fresh one from Snipd's browser sign-in (see README Authentication). %s",
			resp.StatusCode, snippet(data))
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, fmt.Errorf("snipd API error (HTTP %d) for %s: %s", resp.StatusCode, reqPath, snippet(data))
	}
	c.limiter.OnSuccess()
	return data, nil
}

// Metadata is the fetch-export-metadata response: which episodes exist upstream.
type Metadata struct {
	EpisodeBatchCount int     `json:"episode_batch_count"`
	EpisodeBatches    []Batch `json:"episode_batches"`
}

// Batch is one export batch (the export POST takes a batch's episode ids).
type Batch struct {
	Index    int           `json:"index"`
	Episodes []MetaEpisode `json:"episodes"`
}

// MetaEpisode is the per-episode entry in the metadata: id + snip count (+ an
// update cursor when the API provides one, used for incremental sync).
type MetaEpisode struct {
	EpisodeID          string `json:"episode_id"`
	TotalSnipCount     int    `json:"total_snip_count"`
	LatestSnipUpdateTS string `json:"latest_snip_update_ts,omitempty"`
}

// TotalEpisodes counts episodes across all batches.
func (m *Metadata) TotalEpisodes() int {
	n := 0
	for _, b := range m.EpisodeBatches {
		n += len(b.Episodes)
	}
	return n
}

// TotalSnips sums the expected snip count across all episodes.
func (m *Metadata) TotalSnips() int {
	n := 0
	for _, b := range m.EpisodeBatches {
		for _, e := range b.Episodes {
			n += e.TotalSnipCount
		}
	}
	return n
}

// FetchMetadata lists episodes available upstream. updatedAfter (optional,
// ISO-8601) narrows to episodes changed after that time for incremental sync.
func (c *Client) FetchMetadata(ctx context.Context, updatedAfter string) (*Metadata, error) {
	p := metadataPath
	if strings.TrimSpace(updatedAfter) != "" {
		p += "?updated_after=" + url.QueryEscape(updatedAfter)
	}
	data, err := c.do(ctx, http.MethodGet, p, nil)
	if err != nil {
		return nil, err
	}
	var m Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing export metadata: %w", err)
	}
	return &m, nil
}

// ExportEpisodes POSTs the labelled-delimiter templates for a batch of episode
// ids and returns the raw ZIP of server-rendered markdown.
func (c *Client) ExportEpisodes(ctx context.Context, episodeIDs []string) ([]byte, error) {
	reqBody, err := json.Marshal(map[string]any{
		"episode_ids":      episodeIDs,
		"episode_template": EpisodeTemplate(),
		"snip_template":    SnipTemplate(),
	})
	if err != nil {
		return nil, fmt.Errorf("building export request: %w", err)
	}
	return c.do(ctx, http.MethodPost, exportPath, reqBody)
}

// ParseZip unpacks an export ZIP into normalized Episodes + Snips. Each
// <episode_id>_full_content.md is parsed by ParseEpisode; the episode id comes
// from the filename. Non-markdown members (batch metadata.json) are ignored.
func ParseZip(zipBytes []byte) ([]Episode, []Snip, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, nil, fmt.Errorf("opening export zip: %w", err)
	}
	var eps []Episode
	var snips []Snip
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, "_full_content.md") {
			continue
		}
		episodeID := strings.TrimSuffix(path.Base(f.Name), "_full_content.md")
		rc, err := f.Open()
		if err != nil {
			return nil, nil, fmt.Errorf("reading %s from zip: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("reading %s: %w", f.Name, err)
		}
		ep, sn := ParseEpisode(episodeID, string(data))
		eps = append(eps, ep)
		snips = append(snips, sn...)
	}
	return eps, snips, nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > errSnippetLen {
		return s[:errSnippetLen] + "…"
	}
	return s
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}
