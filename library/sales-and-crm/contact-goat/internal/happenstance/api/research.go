// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PollResearchOptions controls PollResearch's loop behavior. A nil pointer
// means "defaults": 180s timeout, 1s interval. Mirrors PollSearchOptions so
// the two surfaces have a consistent shape, even though research has no
// pagination.
type PollResearchOptions struct {
	// Timeout bounds the total wall-clock spent polling before giving up.
	// Zero falls back to DefaultPollTimeout.
	Timeout time.Duration

	// Interval is the delay between successive GetResearch calls. Zero
	// falls back to DefaultPollInterval.
	Interval time.Duration
}

// createResearchRequest is the POST /v1/research body. The OpenAPI spec
// documents a single required field, `description`, which carries the
// natural-language prose the research agent operates on.
type createResearchRequest struct {
	Description string `json:"description"`
}

// Research calls POST /v1/research. Returns the asynchronous research id;
// callers must poll via PollResearch (or GetResearch in a custom loop) until
// Status is COMPLETED, FAILED, or FAILED_AMBIGUOUS. Costs 1 credit on
// completion (no charge for FAILED runs per the upstream docs).
func (c *Client) Research(ctx context.Context, description string) (ResearchEnvelope, error) {
	if strings.TrimSpace(description) == "" {
		return ResearchEnvelope{}, errors.New("happenstance api: research description is empty")
	}
	body := createResearchRequest{Description: description}
	raw, err := c.do(ctx, http.MethodPost, "/research", body)
	if err != nil {
		return ResearchEnvelope{}, err
	}
	var env ResearchEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ResearchEnvelope{}, fmt.Errorf("happenstance api: decoding /research response: %w", err)
	}
	return env, nil
}

// GetResearch calls GET /v1/research/{id}. Free probe — no credits spent.
// The Profile field is populated only once Status is COMPLETED.
func (c *Client) GetResearch(ctx context.Context, id string) (ResearchEnvelope, error) {
	if strings.TrimSpace(id) == "" {
		return ResearchEnvelope{}, errors.New("happenstance api: GetResearch requires a non-empty research id")
	}
	path := "/research/" + url.PathEscape(id)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return ResearchEnvelope{}, err
	}
	var env ResearchEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ResearchEnvelope{}, fmt.Errorf("happenstance api: decoding /research/%s response: %w", id, err)
	}
	return env, nil
}

// PollResearch repeatedly calls GetResearch until Status is terminal
// (COMPLETED, FAILED, or FAILED_AMBIGUOUS) or the configured timeout fires.
// On terminal status returns the final envelope; on timeout returns the
// last polled envelope (Status will still be RUNNING) without an error.
// Honors ctx.Done(): if the context is cancelled mid-loop the function
// returns ctx.Err() immediately.
func (c *Client) PollResearch(ctx context.Context, id string, opts *PollResearchOptions) (ResearchEnvelope, error) {
	if strings.TrimSpace(id) == "" {
		return ResearchEnvelope{}, errors.New("happenstance api: PollResearch requires a non-empty research id")
	}
	timeout := DefaultPollTimeout
	interval := DefaultPollInterval
	if opts != nil {
		if opts.Timeout > 0 {
			timeout = opts.Timeout
		}
		if opts.Interval > 0 {
			interval = opts.Interval
		}
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var last ResearchEnvelope
	for {
		env, err := c.GetResearch(ctx, id)
		if err != nil {
			return ResearchEnvelope{}, err
		}
		last = env
		if isTerminalStatus(env.Status) {
			return env, nil
		}
		if time.Now().After(deadline) {
			return last, nil
		}
		select {
		case <-ctx.Done():
			return ResearchEnvelope{}, ctx.Err()
		case <-ticker.C:
		}
	}
}
