// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

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
)

var newHTTPClient = func(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

type client struct {
	baseURL   string
	userAgent string
	http      *http.Client
}

func newClient(timeout time.Duration) *client {
	cfg := currentConfig()
	return &client{
		baseURL:   cfg.BaseURL,
		userAgent: cfg.UserAgent,
		http:      newHTTPClient(timeout),
	}
}

func (c *client) getJSON(ctx context.Context, path string, query url.Values, target any) error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("GET %s returned HTTP %d: %s", path, resp.StatusCode, summarizeErrorBody(body))
	}
	if len(body) == 0 {
		return fmt.Errorf("GET %s returned empty response", path)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func summarizeErrorBody(body []byte) string {
	bodyText := strings.Join(strings.Fields(strings.TrimSpace(string(body))), " ")
	if bodyText == "" {
		return "<empty body>"
	}

	const maxRunes = 500
	runes := []rune(bodyText)
	if len(runes) <= maxRunes {
		return bodyText
	}
	return string(runes[:maxRunes]) + "... [truncated]"
}

func (c *client) product(ctx context.Context, barcode string) (productResponse, error) {
	var response productResponse
	err := c.getJSON(ctx, "/api/v3/product/"+url.PathEscape(barcode)+".json", nil, &response)
	if err != nil {
		return response, err
	}
	if response.Status != "" && response.Status != "success" {
		return response, fmt.Errorf("product %s not found", barcode)
	}
	if response.Product.Code == "" {
		response.Product.Code = response.Code
	}
	return response, nil
}

func (c *client) search(ctx context.Context, query url.Values) (searchResponse, error) {
	var response searchResponse
	err := c.getJSON(ctx, "/api/v2/search", query, &response)
	return response, err
}
