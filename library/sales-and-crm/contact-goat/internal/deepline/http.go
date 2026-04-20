// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package deepline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrDeeplineAuth is returned when the upstream rejects the API key: either
// it is missing from the request, malformed, revoked, or expired. Distinct
// from ErrProviderNotEntitled below so callers can give targeted hints.
var ErrDeeplineAuth = errors.New("deepline: API key rejected by upstream (missing, invalid, or revoked)")

// ErrProviderNotEntitled is returned when a Deepline integration refuses
// the call because the key-holder's account lacks access to that specific
// provider (e.g. the account has Apollo + Hunter + Dropleads entitlements
// but no Datagma / ContactOut). The key itself is fine; telling the user
// to re-check DEEPLINE_API_KEY would be misleading.
type ErrProviderNotEntitled struct {
	Provider string
	ToolID   string
	Message  string
}

func (e *ErrProviderNotEntitled) Error() string {
	prov := e.Provider
	if prov == "" {
		prov = "upstream provider"
	}
	msg := "not enabled on this Deepline account"
	if e.Message != "" {
		msg = e.Message
	}
	return fmt.Sprintf("deepline: %s is %s for tool %q (key is valid; try a different provider or request access)", prov, msg, e.ToolID)
}

// executeHTTP is the direct HTTP fallback used when the `deepline` CLI is
// not on PATH or the subprocess path failed. It POSTs the payload wrapped
// in a {"payload": {...}} envelope to /integrations/{toolId}/execute.
//
// Wire protocol note: the endpoint rejects a flat payload with the same
// error as a missing payload ("At least one of id, email, ... is
// required") even when the required key IS present at the top level.
// The correct envelope, verified 2026-04-20 against
// apollo_people_match, dropleads_email_finder, and ai_ark_find_emails, is:
//
//	{"payload": {<tool-specific fields>}}
//
// Previously this function sent the flat payload, which meant the HTTP
// fallback never worked for any tool; any call that didn't resolve via
// the `deepline` subprocess failed with a misleading 422.
func (c *Client) executeHTTP(ctx context.Context, toolID string, payload map[string]any) (json.RawMessage, error) {
	envelope := map[string]any{"payload": payload}
	body, err := json.Marshal(envelope)
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
		return nil, fmt.Errorf("%w (HTTP 401): verify the key at https://code.deepline.com/dashboard/api-keys (keys start with %q)", ErrDeeplineAuth, KeyPrefix)
	case resp.StatusCode == http.StatusForbidden:
		return nil, classify403(toolID, respBody)
	case resp.StatusCode == http.StatusPaymentRequired:
		return nil, fmt.Errorf("deepline http: 402 payment required: out of credits. Top up at https://code.deepline.com/dashboard/billing")
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

// classify403 distinguishes between auth failures (key missing/invalid/
// revoked) and provider-entitlement failures (key valid but this specific
// integration not enabled on the account). The Deepline API uses the same
// status code for both cases, but the response body carries different
// signals:
//
//	{"error_category": "auth", "code": "AUTH_*", ...} -> auth failure
//	{"provider": "contactout", "error_category": "authorization",
//	 "message": "... not enabled"}                  -> entitlement failure
//
// When the body doesn't match either pattern, fall back to a generic 403
// wrapper that includes the upstream message so the user can still debug.
func classify403(toolID string, body []byte) error {
	var parsed struct {
		ErrorCategory string `json:"error_category"`
		Code          string `json:"code"`
		Provider      string `json:"provider"`
		Operation     string `json:"operation"`
		Message       string `json:"message"`
		Error         string `json:"error"`
	}
	_ = json.Unmarshal(body, &parsed)

	msg := parsed.Message
	if msg == "" {
		msg = parsed.Error
	}

	// Auth failure patterns.
	lowerCat := strings.ToLower(parsed.ErrorCategory)
	lowerCode := strings.ToUpper(parsed.Code)
	lowerMsg := strings.ToLower(msg)
	if lowerCat == "auth" || strings.HasPrefix(lowerCode, "AUTH_") ||
		strings.Contains(lowerMsg, "api key") || strings.Contains(lowerMsg, "invalid token") {
		return fmt.Errorf("%w (HTTP 403): %s", ErrDeeplineAuth, msg)
	}

	// Entitlement failure patterns.
	provider := parsed.Provider
	if provider == "" {
		// Some endpoints put the provider slug on the toolID itself
		// (e.g. "contactout_enrich_person" -> "contactout").
		if i := strings.IndexByte(toolID, '_'); i > 0 {
			provider = toolID[:i]
		}
	}
	if strings.Contains(lowerMsg, "not enabled") || strings.Contains(lowerMsg, "not authorized for this integration") ||
		strings.Contains(lowerMsg, "integration access") || strings.Contains(lowerMsg, "not connected") ||
		lowerCat == "authorization" {
		return &ErrProviderNotEntitled{
			Provider: provider,
			ToolID:   toolID,
			Message:  msg,
		}
	}

	// Default: unclassified 403 with upstream body for debugging.
	tail := string(body)
	if len(tail) > 300 {
		tail = tail[:300] + "..."
	}
	return fmt.Errorf("deepline http: 403 forbidden on tool %q: %s", toolID, tail)
}
