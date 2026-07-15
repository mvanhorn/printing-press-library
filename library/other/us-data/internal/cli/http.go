// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var httpClient = &http.Client{}

func getJSON(ctx context.Context, endpoint string, query url.Values, headers map[string]string) ([]byte, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("GET %s returned %s: %s", sanitizedURLForError(u), resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func sanitizedURLForError(u *url.URL) string {
	copyURL := *u
	query := copyURL.Query()
	for key := range query {
		if isSensitiveQueryParam(key) {
			query[key] = []string{"REDACTED"}
		}
	}
	copyURL.RawQuery = query.Encode()
	return copyURL.String()
}

func isSensitiveQueryParam(key string) bool {
	switch strings.ToLower(key) {
	case "key", "userid", "user_id", "api_key", "apikey", "token", "access_token":
		return true
	default:
		return false
	}
}
