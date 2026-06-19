// Copyright 2026 Jonas Ekerhovd and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored polish helpers (not generated).
//
// Two TV Time-specific quirks the generic generated client can't handle:
//
//  1. /signin needs the credentials as a form-encoded POST body
//     (application/x-www-form-urlencoded), not as a Basic auth header
//     and not as a JSON body. tvtimeSigninForm does that.
//
//  2. Read endpoints under /user/{user_id}/... (stats, calendar, to_watch)
//     are publicly readable. Worse, sending Basic auth to /user/{user_id}/to_watch
//     returns HTTP 500 — so we must explicitly NOT send Authorization on these
//     paths. tvtimeUnauthGet bypasses the generated client's auth wiring and
//     issues a bare HTTP GET against the same base URL.
//
// These helpers exist so the four novel commands (stats, agenda, next,
// backlog) can hit the live API without rewriting the generated client.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tv-time-pp-cli/internal/client"
)

// tvtimeUnauthGet issues an unauthenticated GET against c.BaseURL + path
// using c.HTTPClient. Returned provenance always reports source=live.
//
// Use this from novel commands whose endpoint is publicly readable and
// rejects (or breaks on) Basic auth.
func tvtimeUnauthGet(ctx context.Context, c *client.Client, path string) (json.RawMessage, DataProvenance, error) {
	prov := DataProvenance{Source: "live"}
	if c == nil {
		return nil, prov, fmt.Errorf("nil client")
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	target := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, prov, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tv-time-pp-cli")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, prov, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, prov, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, prov, fmt.Errorf("GET %s returned HTTP %d: %s", path, resp.StatusCode, truncateBytes(body, 200))
	}
	return json.RawMessage(body), prov, nil
}

// tvtimeSigninForm POSTs username/password as form-encoded body to /signin
// and returns the raw JSON response (which includes the resolved user id).
//
// TV Time's signin endpoint rejects Basic auth and rejects JSON bodies;
// only application/x-www-form-urlencoded works.
func tvtimeSigninForm(ctx context.Context, c *client.Client, username, password string) (json.RawMessage, int, error) {
	if c == nil {
		return nil, 0, fmt.Errorf("nil client")
	}
	if username == "" || password == "" {
		return nil, 0, fmt.Errorf("missing credentials: set TVTIME_USERNAME and TVTIME_PASSWORD or populate ~/.config/tv-time-pp-cli/config.toml")
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	form := url.Values{}
	form.Set("username", username)
	form.Set("password", password)
	target := c.BaseURL + "/signin"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tv-time-pp-cli")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("POST /signin: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("POST /signin returned HTTP %d: %s", resp.StatusCode, truncateBytes(body, 200))
	}
	return json.RawMessage(body), resp.StatusCode, nil
}

func truncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
