package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const userAgent = "jinko-pp-cli"

// Client is the HTTP wrapper around the Jinko BFF.
//
// One Client per CLI invocation. It carries:
//   - the resolved credential (API key or OAuth bearer)
//   - a stable session id (X-Session-ID — same across calls in this process)
//   - one trace id (traceparent — every call gets a fresh span under it)
type Client struct {
	baseURL   string
	httpc     *http.Client
	auth      *ResolvedAuth
	sessionID string
	traceID   string
	userID    string // populated for OAuth via JWT sub claim (best-effort)
	version   string // CLI version (X-Client-Kind)
}

// New constructs a Client and resolves auth in one step.
// flagAPIKey may be empty (the resolver will fall through to env / config).
func New(flagAPIKey, version string) (*Client, error) {
	auth, err := ResolveAuth(flagAPIKey)
	if err != nil {
		return nil, err
	}
	sessionID, _ := LoadOrCreateCLISession() // best-effort
	c := &Client{
		baseURL: ResolveBaseURL(),
		httpc: &http.Client{
			Timeout: 60 * time.Second,
		},
		auth:      auth,
		sessionID: sessionID,
		traceID:   NewTraceID(),
		version:   version,
	}
	if auth.Method == MethodOAuth {
		c.userID = extractUserIDFromJWT(auth.Token)
	}
	return c, nil
}

// Auth exposes the resolved credential for `auth status`.
func (c *Client) Auth() *ResolvedAuth { return c.auth }

// Post sends a JSON body to path and decodes the JSON response into `out`.
func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

// Get sends a GET to path and decodes the JSON response into `out`.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	// Headers mirror @gojinko/cli/api-client client.ts middleware exactly.
	// Keep these in sync — Datadog dashboards filter on X-Source / X-Client-Kind
	// across TS and Go entry points.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tenant-ID", "jinko")
	req.Header.Set("X-Source", "cli")
	req.Header.Set("X-Client-Kind", fmt.Sprintf("%s/%s", userAgent, c.version))
	req.Header.Set("X-Session-ID", c.sessionID)
	req.Header.Set("X-Request-ID", newUUID())
	// W3C trace-context — one trace per CLI run, fresh span per request.
	req.Header.Set("traceparent", fmt.Sprintf("00-%s-%s-01", c.traceID, NewSpanID()))

	if c.userID != "" {
		req.Header.Set("X-User-ID", c.userID)
	}
	switch c.auth.Method {
	case MethodAPIKey:
		req.Header.Set("X-API-Key", c.auth.Token)
	case MethodOAuth:
		req.Header.Set("Authorization", "Bearer "+c.auth.Token)
	}

	resp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Code:       "API_ERROR",
			Message:    resp.Status,
		}
		// Try to extract structured error from body — BFF returns either
		// { "error": { code, message, doc_url } } or { "message": "..." }.
		var envelope struct {
			Error   *APIError `json:"error,omitempty"`
			Message string    `json:"message,omitempty"`
		}
		if err := json.Unmarshal(respBody, &envelope); err == nil {
			if envelope.Error != nil {
				if envelope.Error.Code != "" {
					apiErr.Code = envelope.Error.Code
				}
				if envelope.Error.Message != "" {
					apiErr.Message = envelope.Error.Message
				}
				apiErr.DocURL = envelope.Error.DocURL
			} else if envelope.Message != "" {
				apiErr.Message = envelope.Message
			}
		}
		return apiErr
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
