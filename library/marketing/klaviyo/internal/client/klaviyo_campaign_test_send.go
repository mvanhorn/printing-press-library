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

type campaignTestSendTransport struct{ http.RoundTripper }

func (t campaignTestSendTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// An inherited idempotency header must not enable net/http POST replays.
	req.GetBody = nil
	return t.RoundTripper.RoundTrip(req)
}

// PostCampaignTestSend makes one attempt, including for rate limits and ambiguous failures.
func (c *Client) PostCampaignTestSend(ctx context.Context, body any) (json.RawMessage, int, error) {
	const path = "/api/template-preview-send-jobs"
	httpClient := *c.HTTPClient
	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpClient.Transport = campaignTestSendTransport{transport}
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return fmt.Errorf("campaign test sends cannot follow redirects")
	}
	previousHTTPClient := c.HTTPClient
	c.HTTPClient = &httpClient
	defer func() { c.HTTPClient = previousHTTPClient }()
	budget := platform.EndpointBudget{
		Class: "POST " + path, Steady: 15, Interval: time.Minute, Burst: 1,
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
