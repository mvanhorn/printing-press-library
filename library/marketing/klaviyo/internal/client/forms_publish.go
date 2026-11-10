// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

type formPublicationTransport struct{ http.RoundTripper }

func (t formPublicationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// net/http can replay POST bodies with an inherited idempotency header.
	req.GetBody = nil
	return t.RoundTripper.RoundTrip(req)
}

// PostFormPublicationJob never replays a job, even with a configured idempotency key.
func (c *Client) PostFormPublicationJob(ctx context.Context, path string, body any) (json.RawMessage, int, error) {
	if path != "/api/form-publish-jobs" && path != "/api/form-unpublish-jobs" {
		return nil, 0, fmt.Errorf("invalid form publication job path %q", path)
	}
	httpClient := *c.HTTPClient
	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpClient.Transport = formPublicationTransport{transport}
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return fmt.Errorf("form publication jobs cannot follow redirects")
	}
	previousHTTPClient := c.HTTPClient
	c.HTTPClient = &httpClient
	defer func() { c.HTTPClient = previousHTTPClient }()
	budget := platform.EndpointBudget{
		Class: "POST " + path, Steady: 60, Interval: time.Minute, Burst: 3,
		MaxAttempts: 1, RetryBudget: time.Minute,
	}
	c.platformLimiterMu.Lock()
	if c.platformBudgets == nil {
		c.platformBudgets = map[string]platform.EndpointBudget{}
	}
	previous, existed := c.platformBudgets[budget.Class]
	c.platformBudgets[budget.Class] = budget
	delete(c.platformLimiters, budget.Class)
	c.platformLimiterMu.Unlock()
	defer func() {
		c.platformLimiterMu.Lock()
		defer c.platformLimiterMu.Unlock()
		if existed {
			c.platformBudgets[budget.Class] = previous
		} else {
			delete(c.platformBudgets, budget.Class)
		}
		delete(c.platformLimiters, budget.Class)
	}()
	return c.PostWithHeaders(ctx, path, body, map[string]string{
		"Accept": "application/vnd.api+json", "Content-Type": "application/vnd.api+json",
		ReplaySafeHeader: "false",
	})
}
