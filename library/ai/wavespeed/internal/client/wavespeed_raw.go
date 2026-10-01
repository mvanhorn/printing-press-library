// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"net/http"
)

// AuthHeader exposes the resolved Authorization value so hand-authored
// download code can attach it only when the target is the API host. CDN
// output URLs must never receive the API key.
func (c *Client) AuthHeader(ctx context.Context) (string, error) {
	return c.authHeader(ctx)
}

// DoRaw executes a caller-built GET (output downloads) through the shared rate
// limiter. It uses the streaming client so a large video is bounded by the
// header deadline rather than the JSON-sized whole-call --timeout.
func (c *Client) DoRaw(req *http.Request) (*http.Response, error) {
	if c.limiter != nil {
		if err := c.limiter.Wait(req.Context()); err != nil {
			return nil, err
		}
	}
	resp, err := StreamingHTTPClient(c.HTTPClient, c.ConfiguredTimeout()).Do(req)
	if err != nil {
		return nil, err
	}
	if c.limiter != nil {
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
		} else if resp.StatusCode < http.StatusBadRequest {
			c.limiter.OnSuccess()
		}
	}
	return resp, nil
}
