// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: PortaleServices client — preserve on regeneration.

package client

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/openipa/internal/cliutil"
	"os"
	"strings"
	"time"
)

const portaleBaseURL = "https://www.indicepa.gov.it/PortaleServices"

// NewPortale returns a Client pre-configured for the IPA PortaleServices API.
// These endpoints use JSON bodies (not form-encoded) and require no AUTH_ID.
func NewPortale(timeout time.Duration) *Client {
	return &Client{
		BaseURL:    portaleBaseURL,
		HTTPClient: newHTTPClient(timeout, nil),
		limiter:    cliutil.NewAdaptiveLimiter(2),
	}
}

// PostJSON sends a POST request with a JSON body, bypassing the form-encoded
// path used by the public-ws endpoints. No AUTH_ID is included.
func (c *Client) PostJSON(path string, body any) (json.RawMessage, int, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshaling body: %w", err)
	}
	targetURL := c.BaseURL + path

	if c.DryRun {
		fmt.Fprintf(os.Stderr, "POST %s\n  Body: %s\n\n(dry run - no request sent)\n", targetURL, string(bodyBytes))
		return json.RawMessage(`{"dry_run": true}`), 0, nil
	}

	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequest("POST", targetURL, strings.NewReader(string(bodyBytes)))
		if err != nil {
			return nil, 0, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "github.com/mvanhorn/printing-press-library/library/developer-tools/openipa/0.1.0")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("POST %s: %w", path, err)
			continue
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, 0, fmt.Errorf("reading response: %w", err)
		}

		if resp.StatusCode < 400 {
			c.limiter.OnSuccess()
			return json.RawMessage(sanitizeJSONResponse(respBody)), resp.StatusCode, nil
		}

		apiErr := &APIError{Method: "POST", Path: path, StatusCode: resp.StatusCode, Body: truncateBody(respBody)}
		if resp.StatusCode == 429 && attempt < maxRetries {
			c.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			time.Sleep(wait)
			lastErr = apiErr
			continue
		}
		if resp.StatusCode >= 500 && attempt < maxRetries {
			wait := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(wait)
			lastErr = apiErr
			continue
		}
		return nil, resp.StatusCode, apiErr
	}
	return nil, 0, lastErr
}
