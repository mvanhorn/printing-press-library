// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Withings-specific transport. The Withings Public Cloud API is an
// action-based RPC: every call is a form-urlencoded POST whose operation is
// selected by an `action` field, and responses are wrapped in a
// {"status": <int>, "body": {...}} envelope (HTTP is always 200; the logical
// result lives in `status`). The generated generic client JSON-encodes POST
// bodies and treats HTTP<400 as success, neither of which fits Withings — so
// every Withings command routes through WithingsForm instead.
//
// This is a hand-authored extension file (no "DO NOT EDIT" header) so it
// survives `generate --force` regen-merge as a whole unit.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/cliutil"
)

// withingsEnvelope is the standard Withings response wrapper. HTTP status is
// almost always 200; the real outcome is the integer `status` field.
type withingsEnvelope struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
	Error  string          `json:"error"`
}

// WithingsError carries a non-zero Withings status code so callers (and the
// CLI's exit-code classifier) can react to rate limits, auth failures, etc.
type WithingsError struct {
	Status  int
	Message string
	Path    string
}

func (e *WithingsError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("withings %s: status %d: %s", e.Path, e.Status, e.Message)
	}
	return fmt.Sprintf("withings %s: status %d", e.Path, e.Status)
}

// withingsStatusMessage maps the common Withings status codes to human text.
func withingsStatusMessage(status int, apiErr string) string {
	if apiErr != "" {
		return apiErr
	}
	switch status {
	case 100, 247, 286, 294:
		return "invalid or unauthorized request / unknown resource"
	case 214:
		return "the provided user does not match the token"
	case 250, 283, 401:
		return "unauthorized — token missing, invalid, or expired (try: withings-pp-cli auth refresh)"
	case 342, 343, 344:
		return "signature or nonce invalid/expired"
	case 503:
		return "invalid params — a required parameter is missing"
	case 522, 524, 2556:
		return "Withings service timeout — retry shortly"
	case 601:
		return "rate limited (120 req/min) — slow down or rely on local sync"
	case 2554:
		return "action not implemented for this endpoint"
	}
	return ""
}

// WithingsForm performs a form-encoded POST to the Withings API and unwraps the
// {status, body} envelope. On Withings status 0 it returns the inner `body`.
// On any non-zero status it returns a *WithingsError. When the response is not
// an envelope (e.g. printing-press verify mock servers return the spec-shaped
// body directly), the raw response is returned unchanged so verify/dogfood mock
// runs keep working.
//
// form values are stringified; empty strings are omitted. A 401-class result
// triggers one automatic token refresh + retry when a refresh token is on file.
func (c *Client) WithingsForm(ctx context.Context, path string, form map[string]any) (json.RawMessage, error) {
	return c.withingsFormAttempt(ctx, path, form, true)
}

func (c *Client) withingsFormAttempt(ctx context.Context, path string, form map[string]any, allowRefresh bool) (json.RawMessage, error) {
	values := url.Values{}
	keys := make([]string, 0, len(form))
	for k := range form {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := stringifyFormValue(form[k])
		if s != "" {
			values.Set(k, s)
		}
	}
	encoded := values.Encode()

	authHeader, err := c.authHeader(ctx)
	if err != nil {
		return nil, err
	}

	if c.DryRun {
		fmt.Fprintf(os.Stderr, "POST %s\n", c.displayURL(c.BaseURL+path, authHeader))
		for _, k := range keys {
			if s := stringifyFormValue(form[k]); s != "" {
				fmt.Fprintf(os.Stderr, "  %s=%s\n", k, c.maskCredentialText(s, authHeader))
			}
		}
		if authHeader != "" {
			fmt.Fprintf(os.Stderr, "  Authorization: %s\n", maskToken(authHeader))
		}
		fmt.Fprintln(os.Stderr, "\n(dry run - no request sent)")
		return json.RawMessage(`{"dry_run": true}`), nil
	}

	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, strings.NewReader(encoded))
		if reqErr != nil {
			return nil, fmt.Errorf("creating request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "withings-pp-cli/0.1.0")
		}
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		if c.Config != nil {
			for k, v := range c.Config.Headers {
				req.Header.Set(k, v)
			}
		}

		resp, doErr := c.HTTPClient.Do(req)
		if doErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			lastErr = fmt.Errorf("POST %s: %w", c.displayURL(path, authHeader), c.maskError(doErr, authHeader))
			continue
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading response: %w", readErr)
		}

		// Transport-level retry on 429/5xx (Withings rarely uses these, but
		// proxies and gateways do).
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			c.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			fmt.Fprintf(os.Stderr, "rate limited, waiting %s (attempt %d/%d)\n", wait, attempt+1, maxRetries)
			if err := sleepContext(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode >= 500 && attempt < maxRetries {
			wait := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			fmt.Fprintf(os.Stderr, "server error %d, retrying in %s (attempt %d/%d)\n", resp.StatusCode, wait, attempt+1, maxRetries)
			if err := sleepContext(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, &APIError{Method: "POST", Path: c.displayURL(path, authHeader), StatusCode: resp.StatusCode, Body: c.maskCredentialText(truncateBody(respBody), authHeader)}
		}

		c.limiter.OnSuccess()
		clean := sanitizeJSONResponse(respBody)

		// Envelope detection: a real Withings response has top-level "status".
		// Verify/dogfood mock servers return the spec-shaped body directly,
		// which has no "status" — return it unchanged so mock runs pass.
		var env withingsEnvelope
		if err := json.Unmarshal(clean, &env); err != nil || !hasTopLevelKey(clean, "status") {
			return json.RawMessage(clean), nil
		}

		if env.Status == 0 {
			if len(env.Body) == 0 {
				return json.RawMessage(`{}`), nil
			}
			return env.Body, nil
		}

		// Auth-class failure: refresh once and retry the whole call.
		if allowRefresh && isWithingsAuthStatus(env.Status) && c.Config != nil && c.Config.RefreshToken != "" {
			if rErr := c.RefreshAccessToken(ctx); rErr == nil {
				return c.withingsFormAttempt(ctx, path, form, false)
			}
		}
		return nil, &WithingsError{Status: env.Status, Message: withingsStatusMessage(env.Status, env.Error), Path: path}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("POST %s: request failed after %d attempts", path, maxRetries)
}

func isWithingsAuthStatus(status int) bool {
	switch status {
	case 401, 250, 283:
		return true
	}
	return false
}

func stringifyFormValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		if t == 0 {
			return ""
		}
		return fmt.Sprintf("%d", t)
	case int64:
		if t == 0 {
			return ""
		}
		return fmt.Sprintf("%d", t)
	case float64:
		if t == 0 {
			return ""
		}
		return fmt.Sprintf("%g", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// hasTopLevelKey reports whether raw is a JSON object with the given top-level
// key. Used to distinguish a real Withings envelope from a mock's raw body.
func hasTopLevelKey(raw []byte, key string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}
