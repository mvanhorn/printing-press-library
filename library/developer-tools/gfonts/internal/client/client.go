// Package client provides the bounded HTTP transport used by the gfonts CLI.
package client

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxResponseBytes = 32 << 20

// Client fetches Google Fonts resources with an explicit timeout and bounded responses.
type Client struct {
	http *http.Client
}

// New constructs a client with the supplied request timeout.
func New(timeout time.Duration) *Client {
	return &Client{http: &http.Client{Timeout: timeout}}
}

// Get fetches a resource and returns its status and body.
func (c *Client) Get(url string, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return resp.StatusCode, nil, fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}
	return resp.StatusCode, body, nil
}
