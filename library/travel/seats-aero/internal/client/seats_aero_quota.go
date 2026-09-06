package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const probePath = "/destinations?origin_airport=JFK"

type Quota struct {
	Limit        int    `json:"limit"`
	Remaining    int    `json:"remaining"`
	ResetSeconds int    `json:"reset_seconds"`
	Observed     bool   `json:"observed"`
	ProbePath    string `json:"probe_path,omitempty"`
}

func ParseQuotaHeaders(h http.Header) Quota {
	remaining := h.Get("X-RateLimit-Remaining")
	return Quota{
		Limit:        parseQuotaValue(h.Get("X-RateLimit-Limit")),
		Remaining:    parseQuotaValue(remaining),
		ResetSeconds: parseQuotaValue(h.Get("X-RateLimit-Reset")),
		Observed:     remaining != "",
	}
}

func parseQuotaValue(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func (c *Client) ProbeQuota(ctx context.Context) (Quota, error) {
	if c == nil {
		return Quota{}, fmt.Errorf("probe quota: nil client")
	}
	if c.DryRun {
		return Quota{ProbePath: probePath}, nil
	}
	authHeader, err := c.authHeader(ctx)
	if err != nil {
		return Quota{}, fmt.Errorf("probe quota auth: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+probePath, nil)
	if err != nil {
		return Quota{}, fmt.Errorf("create quota probe request: %w", err)
	}
	if authHeader != "" {
		req.Header.Set("Partner-Authorization", authHeader)
	}
	if c.Config != nil {
		for k, v := range c.Config.Headers {
			req.Header.Set(k, v)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		if ua := os.Getenv("SEATS_AERO_USER_AGENT"); ua != "" {
			req.Header.Set("User-Agent", ua)
		} else {
			req.Header.Set("User-Agent", "seats-aero-pp-cli/1.0")
		}
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Quota{}, fmt.Errorf("perform quota probe: %w", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return Quota{}, fmt.Errorf("drain quota probe response: %w", err)
	}
	quota := ParseQuotaHeaders(resp.Header)
	quota.ProbePath = probePath
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return quota, nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return Quota{}, fmt.Errorf("quota probe unauthorized: HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode < http.StatusInternalServerError && quota.Observed {
		return quota, nil
	}
	return Quota{}, fmt.Errorf("quota probe returned HTTP %d", resp.StatusCode)
}
