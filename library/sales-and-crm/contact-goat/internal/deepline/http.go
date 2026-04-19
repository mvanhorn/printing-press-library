// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package deepline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// executeHTTP is the direct HTTP fallback used when the `deepline` CLI is
// not on PATH or the subprocess path failed. It POSTs the payload to
// /integrations/{toolId}/execute with a Bearer token.
func (c *Client) executeHTTP(ctx context.Context, toolID string, payload map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("deepline http: marshaling payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/integrations/%s/execute", c.baseURL, url.PathEscape(toolID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("deepline http: building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "contact-goat-pp-cli/deepline")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deepline http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("deepline http: reading response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("deepline http: 401 unauthorized — DEEPLINE_API_KEY is missing or invalid. Keys start with %q; verify at https://code.deepline.com/dashboard/api-keys", KeyPrefix)
	case resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("deepline http: 403 forbidden — key valid but lacks access to tool %q", toolID)
	case resp.StatusCode == http.StatusPaymentRequired:
		return nil, fmt.Errorf("deepline http: 402 payment required — out of credits. Top up at https://code.deepline.com/dashboard/billing")
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("deepline http: 429 rate limited")
	case resp.StatusCode >= 400:
		tail := string(respBody)
		if len(tail) > 400 {
			tail = tail[:400] + "..."
		}
		return nil, fmt.Errorf("deepline http: HTTP %d from %s: %s", resp.StatusCode, endpoint, tail)
	}

	trimmed := bytes.TrimSpace(respBody)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("deepline http: empty response body")
	}
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("deepline http: non-JSON response body")
	}
	return json.RawMessage(trimmed), nil
}
