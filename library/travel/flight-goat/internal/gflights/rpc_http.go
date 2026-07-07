// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gflights

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func postFlightsFrontendRPC(ctx context.Context, endpoint, label, body, currencyCode string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", chromeUserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("x-goog-ext-259736195-jspb", googleFlightsCurrencyHeader(currencyCode))

	resp, err := utlsClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s endpoint: %w", label, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return nil, fmt.Errorf("%s endpoint returned HTTP %d: %s", label, resp.StatusCode, snippet)
	}
	return respBody, nil
}
