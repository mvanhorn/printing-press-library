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
	return c.postCampaignOnce(ctx, "/api/template-preview-send-jobs", body, platform.EndpointBudget{
		Class: "POST /api/template-preview-send-jobs", Steady: 15, Interval: time.Minute, Burst: 1,
		MaxAttempts: 1, RetryBudget: time.Minute,
	})
}

// PostCampaignVariation must not replay a create, even with an inherited idempotency key.
func (c *Client) PostCampaignVariation(ctx context.Context, body any) (json.RawMessage, int, error) {
	return c.postCampaignOnce(ctx, "/api/campaign-variations", body, platform.EndpointBudget{
		Class: "POST /api/campaign-variations", Steady: 150, Interval: time.Minute, Burst: 10,
		MaxAttempts: 1, RetryBudget: time.Minute,
	})
}

func (c *Client) postCampaignOnce(ctx context.Context, path string, body any, budget platform.EndpointBudget) (json.RawMessage, int, error) {
	httpClient := *c.HTTPClient
	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpClient.Transport = campaignTestSendTransport{transport}
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return fmt.Errorf("campaign POST %s cannot follow redirects", path)
	}
	previousHTTPClient := c.HTTPClient
	c.HTTPClient = &httpClient
	defer func() { c.HTTPClient = previousHTTPClient }()
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
	raw, code, err := c.PostWithHeaders(ctx, path, body, map[string]string{
		"Accept": "application/vnd.api+json", "Content-Type": "application/vnd.api+json",
		ReplaySafeHeader: "false",
	})
	if err == nil && code >= 300 && code < 400 {
		return raw, code, fmt.Errorf("campaign POST %s cannot follow redirects (HTTP %d)", path, code)
	}
	return raw, code, err
}
