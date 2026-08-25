// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var httpClient = &http.Client{}

func env(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func getJSON(ctx context.Context, baseURL string, query url.Values, headers map[string]string) ([]byte, error) {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		values := endpoint.Query()
		for key, items := range query {
			for _, item := range items {
				values.Add(key, item)
			}
		}
		endpoint.RawQuery = values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", sanitizedURLForError(endpoint), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GET %s: read response: %w", sanitizedURLForError(endpoint), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: status %d: %s", sanitizedURLForError(endpoint), resp.StatusCode, truncateForError(string(body)))
	}
	return body, nil
}

func sanitizedURLForError(endpoint *url.URL) string {
	clone := *endpoint
	query := clone.Query()
	for _, key := range []string{"api_key", "apikey", "key", "token", "access_token", "x-api-key"} {
		if _, ok := query[key]; ok {
			query.Set(key, "REDACTED")
		}
	}
	clone.RawQuery = query.Encode()
	return clone.String()
}

func truncateForError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 240 {
		return value
	}
	return value[:240] + "..."
}
