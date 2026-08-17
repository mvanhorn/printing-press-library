// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Client wraps batchexecute RPC calls for NotebookLM.
type Client struct {
	Session *Session
}

// NewClient bootstraps a session and returns an RPC client.
func NewClient(ctx context.Context, httpClient *http.Client) (*Client, error) {
	sess, err := Bootstrap(ctx, httpClient)
	if err != nil {
		return nil, err
	}
	return &Client{Session: sess}, nil
}

// Call executes a single batchexecute RPC and returns the decoded inner payload.
func (c *Client) Call(ctx context.Context, rpcid, sourcePath string, params any) (json.RawMessage, error) {
	inner := []any{rpcid, mustJSONString(params), nil, "generic"}
	freq, err := json.Marshal([]any{[]any{inner}})
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("f.req", string(freq))
	if c.Session.AT != "" {
		form.Set("at", c.Session.AT)
	}
	u := c.Session.BuildBatchURL(rpcid, sourcePath)

	body, err := c.Session.postForm(ctx, u, form.Encode(), http.Header{
		"User-Agent": {chromeUserAgent},
		"Origin":     {BaseURL},
		"Referer":    {BaseURL + "/"},
	})
	if err != nil {
		return nil, err
	}
	frames, err := ParseFrames(string(body))
	if err != nil {
		return nil, err
	}
	return ExtractRPCResult(frames, rpcid)
}

func mustJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
