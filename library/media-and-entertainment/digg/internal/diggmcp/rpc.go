// Package diggmcp calls tools on Digg's public MCP server.
//
// The server is stateless for public reads: one HTTP POST per tools/call,
// with no initialize handshake and no session header. It answers with a
// single server-sent event whose data line is the JSON-RPC response.
package diggmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/cliutil"
)

// DefaultEndpoint is Digg's MCP server.
const DefaultEndpoint = "https://digg.com/mcp"

// acceptHeader is what MCP Streamable HTTP servers expect from clients.
const acceptHeader = "application/json, text/event-stream"

// maxResponseBytes bounds one response body.
const maxResponseBytes = 8 << 20

// Client calls Digg MCP tools. The zero value calls DefaultEndpoint with
// no rate limiting and a 30 second timeout.
type Client struct {
	// Endpoint replaces DefaultEndpoint when set.
	Endpoint string
	// HTTP sends the requests; nil uses a plain http.Client.
	HTTP *http.Client
	// Limiter paces requests and learns from 429s. Nil disables pacing.
	Limiter *cliutil.AdaptiveLimiter
	// Timeout bounds each request; zero means 30 seconds.
	Timeout time.Duration
	// UserAgent identifies the caller to Digg.
	UserAgent string
}

// Error is a failure the server reported for a tool call: a JSON-RPC
// error, a result flagged isError, or a non-2xx HTTP status.
type Error struct {
	Tool    string
	Code    int // JSON-RPC error code, or 0
	Status  int // HTTP status when it was not 2xx, or 0
	Message string
}

func (e *Error) Error() string { return e.Tool + ": " + e.Message }

type request struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      int        `json:"id"`
	Method  string     `json:"method"`
	Params  callParams `json:"params"`
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Call invokes one tool and returns its structured result. A 429 is
// retried once after the server's Retry-After delay.
func (c *Client) Call(ctx context.Context, tool string, args map[string]any) (json.RawMessage, error) {
	if args == nil {
		args = map[string]any{}
	}
	body, err := json.Marshal(request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: callParams{Name: tool, Arguments: args}})
	if err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		status, wait, raw, err := c.post(ctx, tool, body)
		if err != nil {
			return nil, err
		}
		if status == http.StatusTooManyRequests {
			c.Limiter.OnRateLimit()
			if attempt > 0 {
				return nil, &cliutil.RateLimitError{URL: c.endpoint(), RetryAfter: wait, Body: clip(raw, 200)}
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if status < 200 || status > 299 {
			// A JSON-RPC error body explains more than the status does.
			if _, uerr := Unwrap(tool, raw); uerr != nil {
				var e *Error
				if errors.As(uerr, &e) && e.Code != 0 {
					e.Status = status
					return nil, e
				}
			}
			return nil, &Error{Tool: tool, Status: status, Message: fmt.Sprintf("HTTP %d: %s", status, clip(raw, 200))}
		}
		c.Limiter.OnSuccess()
		return Unwrap(tool, raw)
	}
}

func (c *Client) endpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return DefaultEndpoint
}

// post sends one request. wait is the Retry-After delay on a 429.
func (c *Client) post(ctx context.Context, tool string, body []byte) (status int, wait time.Duration, raw []byte, err error) {
	if err := c.Limiter.WaitContext(ctx); err != nil {
		return 0, 0, nil, err
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, c.endpoint(), bytes.NewReader(body))
	if err != nil {
		return 0, 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", acceptHeader)
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	resp, err := httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		raw, err = io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	}
	if err != nil {
		// The parent context ending is the caller's cancellation, not a
		// slow server.
		if ctx.Err() == nil && errors.Is(cctx.Err(), context.DeadlineExceeded) {
			return 0, 0, nil, fmt.Errorf("%s: call timed out after %s", tool, timeout)
		}
		return 0, 0, nil, fmt.Errorf("%s: POST %s: %w", tool, c.endpoint(), err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		wait = cliutil.RetryAfter(resp)
	}
	return resp.StatusCode, wait, raw, nil
}

// Unwrap decodes a tools/call response body, SSE-framed or plain JSON. It
// returns structuredContent when present, else the first text content
// parsed as JSON. A JSON-RPC error or a result flagged isError becomes an
// *Error.
func Unwrap(tool string, raw []byte) (json.RawMessage, error) {
	payload := StripSSE(raw)
	var env struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result *struct {
			IsError bool `json:"isError"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("%s: unreadable MCP response: %s", tool, clip(payload, 200))
	}
	if env.Error != nil {
		return nil, &Error{Tool: tool, Code: env.Error.Code, Message: env.Error.Message}
	}
	if env.Result == nil {
		return nil, fmt.Errorf("%s: MCP response has no result or error", tool)
	}
	text := ""
	for _, part := range env.Result.Content {
		if part.Type == "text" && part.Text != "" {
			text = strings.TrimSpace(part.Text)
			break
		}
	}
	if env.Result.IsError {
		if text == "" {
			text = "tool reported an error with no message"
		}
		return nil, &Error{Tool: tool, Message: text}
	}
	if sc := env.Result.StructuredContent; len(sc) > 0 && string(sc) != "null" {
		return sc, nil
	}
	if json.Valid([]byte(text)) {
		return json.RawMessage(text), nil
	}
	return nil, fmt.Errorf("%s: MCP result has no structured content", tool)
}

// StripSSE pulls the JSON out of a single-event text/event-stream body. A
// plain JSON body is returned unchanged.
func StripSSE(raw []byte) []byte {
	s := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(s, "data:") && !strings.HasPrefix(s, "event:") && !strings.Contains(s, "\ndata:") {
		return raw
	}
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			b.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if b.Len() == 0 {
		return raw
	}
	return []byte(b.String())
}

func clip(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
