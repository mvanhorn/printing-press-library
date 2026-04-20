// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// do is the single seam through which every Client method makes an HTTP
// request. It centralizes auth, dry-run, error-class translation, and body
// decoding so the per-resource methods (Me, Search, Research, Groups, Usage)
// stay one-liners.
//
// path is joined to the configured baseURL; pass "/users/me" not
// "/v1/users/me" — the v1 prefix is part of the default base URL.
//
// body, if non-nil, is JSON-encoded and sent as the request payload with
// Content-Type: application/json.
//
// On 401/402/429 the error message is shaped to be actionable (env-var name,
// rotation URL, /v1/usage hint, or typed *RateLimitError so callers can
// implement custom backoff). Other 4xx/5xx errors include the first 200
// bytes of the response body to aid debugging when the body is HTML (e.g.
// a CDN error page).
func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	endpoint := c.baseURL + path

	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("happenstance api: marshaling body: %w", err)
		}
		bodyBytes = b
	}

	if c.dryRun {
		c.printDryRun(method, endpoint, bodyBytes)
		// Return a synthetic-success body. Decoders in the per-resource
		// methods that try to unmarshal this will likely surface zero
		// values, which is the right behavior for a dry-run preview.
		return []byte(`{"dry_run":true}`), nil
	}

	var reqBody io.Reader
	if bodyBytes != nil {
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("happenstance api: building request: %w", err)
	}
	// IMPORTANT: the bearer key is set per-request and never logged. Any
	// future change that adds request logging must redact this header
	// using RedactedBearerLine. Tests in client_test.go grep for the
	// literal key value across all error paths to lock this down.
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("happenstance api: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("happenstance api: reading response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf(
			"happenstance api: 401 unauthorized — %s is missing or invalid. Rotate at %s",
			KeyEnvVar, RotationURL,
		)
	case resp.StatusCode == http.StatusPaymentRequired:
		return nil, fmt.Errorf(
			"happenstance api: 402 payment required — out of credits. Check live balance at %s",
			UsagePath,
		)
	case resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf(
			"happenstance api: 403 forbidden — key is valid but lacks access to %s %s",
			method, path,
		)
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf(
			"happenstance api: 404 not found — %s %s: %s",
			method, path, truncateBody(respBody, 200),
		)
	case resp.StatusCode == http.StatusUnprocessableEntity:
		return nil, fmt.Errorf(
			"happenstance api: 422 unprocessable — %s %s: %s",
			method, path, truncateBody(respBody, 200),
		)
	case resp.StatusCode == http.StatusTooManyRequests:
		retryAfter := 0
		if h := resp.Header.Get("Retry-After"); h != "" {
			if n, perr := strconv.Atoi(h); perr == nil {
				retryAfter = n
			}
		}
		return nil, &RateLimitError{
			RetryAfterSeconds: retryAfter,
			Body:              truncateBody(respBody, 200),
		}
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf(
			"happenstance api: HTTP %d from %s %s: %s",
			resp.StatusCode, method, path, truncateBody(respBody, 200),
		)
	}

	trimmed := bytes.TrimSpace(respBody)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("happenstance api: empty response body from %s %s", method, path)
	}
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf(
			"happenstance api: non-JSON response body from %s %s: %s",
			method, path, truncateBody(respBody, 200),
		)
	}
	return trimmed, nil
}

// printDryRun writes a redacted preview of the request to stderr. Stderr
// matches internal/client/client.go's existing dry-run convention and
// keeps stdout reserved for --json output so `--dry-run --json` stays
// pipeable. The bearer key is replaced verbatim with RedactedBearerLine
// so callers piping --dry-run into a log file can never leak the key.
func (c *Client) printDryRun(method, endpoint string, body []byte) {
	fmt.Fprintf(os.Stderr, "%s %s\n", method, endpoint)
	fmt.Fprintf(os.Stderr, "  Authorization: %s\n", RedactedBearerLine)
	fmt.Fprintf(os.Stderr, "  Accept: application/json\n")
	if len(body) > 0 {
		fmt.Fprintf(os.Stderr, "  Content-Type: application/json\n")
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, body, "  ", "  "); err == nil {
			fmt.Fprintf(os.Stderr, "  Body:\n  %s\n", pretty.String())
		} else {
			fmt.Fprintf(os.Stderr, "  Body: %s\n", string(body))
		}
	}
	fmt.Fprintf(os.Stderr, "\n(dry run - no request sent)\n")
}

// truncateBody clips b to at most n bytes, appending an ellipsis if it had
// to clip. Used to keep error messages bounded when the upstream returns a
// large HTML error page.
func truncateBody(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
