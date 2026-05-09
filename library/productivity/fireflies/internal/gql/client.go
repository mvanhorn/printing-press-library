// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

// Package gql provides a lightweight GraphQL client for the Fireflies.ai API.
// All operations POST to https://api.fireflies.ai/graphql with a Bearer token.
package gql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/config"
)

const endpoint = "/graphql"

// Client wraps HTTP calls to the Fireflies GraphQL API.
type Client struct {
	baseURL    string
	authHeader string
	http       *http.Client
	limiter    *cliutil.AdaptiveLimiter
}

// New creates a Client from the loaded config.
// Rate limit defaults to 1 req/s (conservative; Business plan allows 60 req/min = 1/s).
func New(cfg *config.Config) (*Client, error) {
	auth := cfg.AuthHeader()
	if auth == "" {
		return nil, fmt.Errorf("not authenticated — set FIREFLIES_API_KEY or run 'fireflies-pp-cli auth set-token'")
	}
	return &Client{
		baseURL:    cfg.BaseURL,
		authHeader: auth,
		http:       &http.Client{Timeout: 30 * time.Second},
		limiter:    cliutil.NewAdaptiveLimiter(1.0),
	}, nil
}

type request struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

type gqlError struct {
	Message string `json:"message"`
}

// Do executes a GraphQL query and returns the raw data field.
func (c *Client) Do(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	if c.limiter != nil {
		c.limiter.Wait()
	}

	body, err := json.Marshal(request{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == 429 {
		if c.limiter != nil {
			c.limiter.OnRateLimit()
		}
		return nil, &cliutil.RateLimitError{URL: c.baseURL + endpoint, RetryAfter: 60 * time.Second}
	}
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("forbidden (HTTP 403) — API access requires a Fireflies Business plan or higher")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	var gqlResp response
	if err := json.Unmarshal(raw, &gqlResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}
	return gqlResp.Data, nil
}

// Query calls Do and extracts a named top-level field from the data envelope.
func (c *Client) Query(ctx context.Context, query string, variables map[string]any, field string) (json.RawMessage, error) {
	data, err := c.Do(ctx, query, variables)
	if err != nil {
		return nil, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing response data: %w", err)
	}
	val, ok := top[field]
	if !ok {
		return nil, fmt.Errorf("field %q not found in response", field)
	}
	return val, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
