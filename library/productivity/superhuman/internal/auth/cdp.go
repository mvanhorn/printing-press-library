// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Package auth implements the Chrome DevTools Protocol (CDP) discovery, tab
// listing, and Runtime.evaluate plumbing that the `auth login --chrome` flow
// uses to extract a Firebase ID token + OAuth refresh token from a running
// logged-in Superhuman tab.
//
// WebSocket library: we use nhooyr.io/websocket (v1.8.x). It's smaller and
// more modern than gorilla/websocket, with a single transitive dep, and is
// the recommended choice in the Go ecosystem today. The codebase did not
// previously have a websocket dep; this package introduces it as the only
// allowed go.mod addition for U1 (per the implementation plan).
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

// CDPClient discovers a running Chrome with --remote-debugging-port enabled,
// lists tabs, and runs Runtime.evaluate against a chosen tab via WebSocket.
//
// Zero value is usable: Port==0 triggers auto-discovery starting at 9222.
type CDPClient struct {
	// Port pins a specific CDP port. 0 means auto-discover via DiscoverPort
	// (9222 first, then 9223-9229).
	Port int

	// HTTPClient overrides the default HTTP client used for /json/version and
	// /json endpoints. Tests inject httptest server clients here. When nil,
	// a localhost-friendly http.Client with a 2s timeout is used.
	HTTPClient *http.Client
}

// Tab is one entry from CDP's /json endpoint, narrowed to the fields the
// caller cares about. Chrome returns more fields than this; we ignore them.
type Tab struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	Title                string `json:"title"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// Typed errors returned by this package. Callers use errors.Is to branch.
var (
	// ErrChromeNotRunning is returned by DiscoverPort when no port in
	// [9222, 9229] responds to /json/version. The wrapped error message
	// contains the actionable relaunch instruction (see relaunchHint).
	ErrChromeNotRunning = errors.New("chrome not running with remote-debugging-port enabled")

	// ErrPageNotReady is returned by Evaluate when the JS expression raised
	// an exception (Chrome populates result.exceptionDetails). Used by U2 to
	// distinguish "the IIFE threw" from "the WebSocket failed".
	ErrPageNotReady = errors.New("superhuman page not loaded")
)

// relaunchHint is the user-facing remediation string. R9 requires this exact
// wording so users have a copy-pasteable command.
const relaunchHint = `Chrome doesn't have remote debugging enabled. Quit Chrome and relaunch with:
  open -a "Google Chrome" --args --remote-debugging-port=9222
Or run: superhuman-pp-cli auth login --auto-launch-chrome`

// portScanLo and portScanHi bracket the auto-discovery range. 9222 is the
// Chrome default; we scan up to 9229 to dodge conflicts with VS Code or
// other tooling that grabs 9222 first.
const (
	portScanLo = 9222
	portScanHi = 9229
)

// httpClient returns c.HTTPClient or a sensible localhost default.
func (c *CDPClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 2 * time.Second}
}

// DiscoverPort tries c.Port first if set, otherwise scans [9222, 9229] and
// returns the first port that responds to /json/version with HTTP 200.
//
// On total failure, it returns 0 and an error that wraps ErrChromeNotRunning
// with the actionable relaunch hint embedded in the message, so callers can
// surface it directly to the user without re-templating.
func (c *CDPClient) DiscoverPort(ctx context.Context) (int, error) {
	candidates := []int{}
	if c.Port != 0 {
		candidates = append(candidates, c.Port)
	} else {
		for p := portScanLo; p <= portScanHi; p++ {
			candidates = append(candidates, p)
		}
	}

	for _, port := range candidates {
		if err := c.ping(ctx, port); err == nil {
			return port, nil
		} else if !isConnRefused(err) && !isTimeout(err) {
			// Non-connection errors (e.g. malformed response) are surfaced
			// rather than silently falling through to the next port. This
			// catches the case where /json/version is reachable but
			// returning garbage (proxy in front, captive portal, etc.).
			return 0, fmt.Errorf("cdp discover port %d: %w", port, err)
		}
	}

	return 0, fmt.Errorf("%s\n\n%w", relaunchHint, ErrChromeNotRunning)
}

// ping does a GET /json/version against the given port and returns nil iff
// the response is HTTP 200 with a JSON body. We don't parse the body — its
// presence + 200 is enough to confirm Chrome speaks CDP here.
func (c *CDPClient) ping(ctx context.Context, port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// ListTabs returns every tab Chrome exposes via /json on the given port.
// The caller is responsible for filtering (see FilterSuperhumanTabs).
//
// Errors from the underlying decode are wrapped as "cdp tab list: <err>" so
// the layer above can surface a coherent message without re-wrapping.
func (c *CDPClient) ListTabs(ctx context.Context, port int) ([]Tab, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cdp tab list: %w", err)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("cdp tab list: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cdp tab list: status %d", resp.StatusCode)
	}
	var tabs []Tab
	if err := json.NewDecoder(resp.Body).Decode(&tabs); err != nil {
		return nil, fmt.Errorf("cdp tab list: %w", err)
	}
	return tabs, nil
}

// FilterSuperhumanTabs narrows a tab list to tabs that are pages (not
// service workers, extension backgrounds, etc.) and whose URL contains the
// Superhuman web app host. Returns a new slice; never returns nil for an
// empty result (returns an empty slice).
func FilterSuperhumanTabs(tabs []Tab) []Tab {
	out := make([]Tab, 0, len(tabs))
	for _, t := range tabs {
		if t.Type != "page" {
			continue
		}
		if !strings.Contains(t.URL, "mail.superhuman.com") {
			continue
		}
		out = append(out, t)
	}
	return out
}

// cdpRequest is the JSON-RPC request frame CDP expects over the tab
// WebSocket. We only ever send Runtime.evaluate frames from this package;
// id=1 is hard-coded because we send exactly one request per Evaluate call.
type cdpRequest struct {
	ID     int                    `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// cdpResponse mirrors the response frame. We don't decode result eagerly —
// the caller's IIFE shape determines what's inside. The shape here is the
// Runtime.evaluate envelope: result.result.value carries the IIFE return
// value when returnByValue=true.
type cdpResponse struct {
	ID     int             `json:"id"`
	Result *cdpResult      `json:"result,omitempty"`
	Error  *cdpResponseErr `json:"error,omitempty"`
}

type cdpResult struct {
	Result           *cdpRemoteObject  `json:"result,omitempty"`
	ExceptionDetails *cdpExceptionInfo `json:"exceptionDetails,omitempty"`
}

type cdpRemoteObject struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value,omitempty"`
}

type cdpExceptionInfo struct {
	Text       string `json:"text"`
	LineNumber int    `json:"lineNumber"`
}

type cdpResponseErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Evaluate connects to the tab's webSocketDebuggerUrl, sends a
// Runtime.evaluate JSON-RPC frame with the given expression, and returns
// the raw JSON value from result.result.value. The caller unmarshals into
// the struct that matches their IIFE's return shape.
//
// Returns ErrPageNotReady (wrapped) when the IIFE threw an exception.
// All other failures (handshake, transport, decode) wrap as "cdp attach: ...".
func (c *CDPClient) Evaluate(ctx context.Context, wsURL string, expression string) (json.RawMessage, error) {
	ws, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cdp attach: %w", err)
	}
	// Use NormalClosure on success; nhooyr's Close is safe to call even on
	// the error path (it just becomes a no-op once the underlying conn is
	// torn down).
	defer ws.Close(websocket.StatusNormalClosure, "")

	// Chrome's Runtime.evaluate accepts these params; returnByValue is
	// essential — without it we get a remote object handle, not a value.
	// awaitPromise lets the IIFE return a Promise (the extract IIFE in U2
	// calls getIDTokenAsync which is async).
	req := cdpRequest{
		ID:     1,
		Method: "Runtime.evaluate",
		Params: map[string]interface{}{
			"expression":    expression,
			"awaitPromise":  true,
			"returnByValue": true,
		},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("cdp attach: marshal request: %w", err)
	}
	if err := ws.Write(ctx, websocket.MessageText, payload); err != nil {
		return nil, fmt.Errorf("cdp attach: write: %w", err)
	}

	// CDP may interleave protocol events on the same socket (e.g.
	// Runtime.executionContextCreated). Loop until we see the response
	// frame with id==1, or the context expires.
	for {
		_, data, err := ws.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("cdp attach: read: %w", err)
		}
		var resp cdpResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("cdp attach: decode response: %w", err)
		}
		if resp.ID != req.ID {
			// Event frame, not our response. Keep reading.
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("cdp attach: chrome error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if resp.Result == nil {
			return nil, fmt.Errorf("cdp attach: empty result frame")
		}
		if resp.Result.ExceptionDetails != nil {
			return nil, fmt.Errorf("cdp attach: %s: %w", resp.Result.ExceptionDetails.Text, ErrPageNotReady)
		}
		if resp.Result.Result == nil {
			return nil, fmt.Errorf("cdp attach: missing result.result")
		}
		return resp.Result.Result.Value, nil
	}
}

// isConnRefused returns true if err looks like the kernel's "nothing
// listening here" response. We deliberately do a string match instead of
// pulling in syscall constants — net.OpError wraps syscall errors and the
// platform-specific check is brittle across darwin/linux.
func isConnRefused(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "connection refused") || strings.Contains(s, "connect: connection refused")
}

// isTimeout returns true for net.Error timeouts and context-deadline cases.
// During port scanning, we treat a timeout the same as connection-refused:
// the port is not a live CDP endpoint, move to the next.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}
