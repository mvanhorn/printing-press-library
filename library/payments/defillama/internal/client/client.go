// Package client is a minimal HTTP client for the DefiLlama free API
// (api.llama.fi, stablecoins.llama.fi, yields.llama.fi, coins.llama.fi).
// MVP scope: free tier only. Pro path remapping is deferred.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Host string

const (
	HostAPI         Host = "https://api.llama.fi"
	HostCoins       Host = "https://coins.llama.fi"
	HostStablecoins Host = "https://stablecoins.llama.fi"
	HostYields      Host = "https://yields.llama.fi"
)

type Client struct {
	HTTP      *http.Client
	UserAgent string

	mu       sync.Mutex
	nextSlot map[Host]time.Time
	// minimum spacing between requests to the same host
	minInterval time.Duration
}

func New() *Client {
	return &Client{
		HTTP:        &http.Client{Timeout: 60 * time.Second},
		UserAgent:   "github.com/mvanhorn/printing-press-library/library/payments/defillama/0.1 (+https://defillama.com)",
		nextSlot:    map[Host]time.Time{},
		minInterval: 150 * time.Millisecond,
	}
}

// throttle reserves the next available send-slot for host h under the
// mutex, then sleeps until that slot arrives. Each caller reserves a
// *distinct* slot before releasing the lock, so N concurrent goroutines
// get N slots spaced minInterval apart — there's no shared `wait` value
// that could let several of them wake and fire together.
func (c *Client) throttle(h Host) {
	c.mu.Lock()
	now := time.Now()
	slot := c.nextSlot[h]
	if slot.Before(now) {
		slot = now
	}
	c.nextSlot[h] = slot.Add(c.minInterval)
	c.mu.Unlock()
	if d := time.Until(slot); d > 0 {
		time.Sleep(d)
	}
}

// Get performs a GET request with retry/backoff and returns the response body.
// Callers must Close the returned io.ReadCloser.
func (c *Client) Get(ctx context.Context, host Host, path string, query url.Values) (io.ReadCloser, error) {
	u := string(host) + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		c.throttle(host)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			continue
		}
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			return nil, fmt.Errorf("GET %s: status %d: %s", u, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return resp.Body, nil
	}
	if lastErr == nil {
		lastErr = errors.New("unknown error")
	}
	return nil, fmt.Errorf("GET %s: %w", u, lastErr)
}

// GetJSON decodes the response body as JSON into v (streaming).
func (c *Client) GetJSON(ctx context.Context, host Host, path string, query url.Values, v any) error {
	body, err := c.Get(ctx, host, path, query)
	if err != nil {
		return err
	}
	defer body.Close()
	dec := json.NewDecoder(body)
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
