// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package deepline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// ErrStub is returned by stubbed methods to make the gap explicit.
var ErrStub = errors.New("deepline: not yet wired (stub)")

// ErrInvalidKeyPrefix is returned when the provided key has the wrong prefix.
// Deepline keys must start with "dlp_". Keys starting with "dpl_" are Vercel
// tokens and are rejected.
var ErrInvalidKeyPrefix = errors.New(`deepline: API key must start with "dlp_" (note: "dpl_" is a Vercel prefix, not Deepline)`)

// ErrMissingKey is returned when no API key is provided.
var ErrMissingKey = errors.New("deepline: API key is required (set DEEPLINE_API_KEY or pass --deepline-key)")

// CLIBinary is the name of the official Deepline CLI we look for on PATH.
const CLIBinary = "deepline"

// Client is the hybrid Deepline client. It prefers to shell out to the
// official `deepline` CLI when present (which keeps auth handling and future
// tool additions consistent with upstream) and falls back to direct HTTP
// otherwise.
type Client struct {
	apiKey              string
	httpClient          *http.Client
	baseURL             string
	subprocessAvailable bool
	cliPath             string
}

// NewClient constructs a client. It validates the dlp_ prefix on apiKey and
// probes PATH for the official `deepline` CLI. An empty apiKey is allowed
// here so callers can surface a friendlier error at call time, but any
// non-empty key with the wrong prefix is rejected eagerly.
func NewClient(apiKey string) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    BaseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	if path, err := exec.LookPath(CLIBinary); err == nil {
		c.subprocessAvailable = true
		c.cliPath = path
	}
	return c
}

// ValidateKey returns an error if the client's API key is missing or has the
// wrong prefix. Intended for call-time checks before spending credits.
func (c *Client) ValidateKey() error {
	if c.apiKey == "" {
		return ErrMissingKey
	}
	if !strings.HasPrefix(c.apiKey, KeyPrefix) {
		return ErrInvalidKeyPrefix
	}
	return nil
}

// SubprocessAvailable reports whether the official `deepline` CLI was found
// on PATH at client construction. Used by `doctor` for informational status.
func (c *Client) SubprocessAvailable() bool { return c.subprocessAvailable }

// CLIPath returns the resolved path to the `deepline` binary, or "" if it is
// not on PATH.
func (c *Client) CLIPath() string { return c.cliPath }

// Execute runs a Deepline tool. It prefers the subprocess path when available
// and falls back to HTTP on any subprocess failure. The returned RawMessage
// is the tool's raw JSON response.
func (c *Client) Execute(ctx context.Context, toolID string, payload map[string]any) (json.RawMessage, error) {
	if err := c.ValidateKey(); err != nil {
		return nil, err
	}
	if toolID == "" {
		return nil, fmt.Errorf("deepline: toolID is required")
	}
	if payload == nil {
		payload = map[string]any{}
	}

	if c.subprocessAvailable {
		out, err := c.executeSubprocess(ctx, toolID, payload)
		if err == nil {
			return out, nil
		}
		// Fall through to HTTP on subprocess failure. This covers the case
		// where the binary is on PATH but the user is not logged in to it
		// locally while a DEEPLINE_API_KEY is still available.
	}
	return c.executeHTTP(ctx, toolID, payload)
}

// EstimateCost returns the estimated credit cost of an Execute call for the
// given tool. Unknown tools default to 1 credit. The `apollo-people-search`
// tool scales with the requested `limit` (1 credit per 25 results, rounded
// up, with a floor at the catalog default).
func (c *Client) EstimateCost(toolID string, payload map[string]any) (int, error) {
	info, ok := LookupTool(toolID)
	if !ok {
		return 1, nil
	}
	cost := info.DefaultCredits

	if toolID == ToolApolloPeopleSearch {
		if raw, ok := payload["limit"]; ok {
			if n := coerceInt(raw); n > 0 {
				scaled := (n + 24) / 25
				if scaled > cost {
					cost = scaled
				}
			}
		}
	}
	return cost, nil
}

// GetCredits is a stub. Deepline has no confirmed credit-balance endpoint at
// time of writing, so this intentionally returns an error rather than
// guessing a path. Callers should surface ErrStub to the user.
func (c *Client) GetCredits(ctx context.Context) (int, error) {
	_ = ctx
	return 0, fmt.Errorf("%w: GetCredits has no confirmed Deepline endpoint; see upstream docs at https://code.deepline.com/docs/quickstart", ErrStub)
}

func coerceInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	case string:
		var n int
		fmt.Sscanf(t, "%d", &n)
		return n
	}
	return 0
}
