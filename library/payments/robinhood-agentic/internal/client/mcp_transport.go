// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-written MCP transport bridge. NOT generated — do not let `generate
// --force` overwrite it. Robinhood's Agentic surface is a single JSON-RPC 2.0
// endpoint (https://agent.robinhood.com/mcp/trading) that speaks the Model
// Context Protocol, not a REST API. The generated client builds ordinary
// REST requests (`GET /tools/<tool>?arg=val`, `POST /tools/<tool>` with a JSON
// body); this RoundTripper intercepts them, performs the one-time MCP
// handshake, rewrites each request into a `tools/call`, parses the JSON-or-SSE
// response, and returns the tool's `{data, guide}` result envelope so the
// generated command layer's response_path extraction works unchanged.
//
// The seam is deliberate: OAuth (login/refresh), dry-run, retry/backoff, rate
// limiting, and response_path extraction all stay in the generated code. Only
// the wire format is translated here.

package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/cliutil"
)

// mcpProtocolVersion is the MCP revision the CLI negotiates at initialize.
const mcpProtocolVersion = "2025-06-18"

// mcpRequestTimeout bounds a single MCP round trip. The request bypasses the
// generated http.Client's own Timeout (it rides the base RoundTripper), so this
// is the only deadline protecting against a stalled connection.
const mcpRequestTimeout = 120 * time.Second

// MutationGuard, when set, runs before any mutating tool call. A non-nil error
// refuses the call — the transport returns that error to the command layer
// without touching the network. The cli package installs a guard at init that
// enforces the write gate (ROBINHOOD_AGENTIC_PP_ALLOW_WRITES), the client-side
// trade policy, and journaling. Kept as a package var (not a Client field) so
// the generated newClient() needs no edit and no client->store import exists.
var MutationGuard func(ctx context.Context, tool string, args map[string]any) error

// MutationJournal, when set, records the outcome of a mutating tool call after
// it returns (success or failure). Best-effort — never blocks the call.
var MutationJournal func(ctx context.Context, tool string, args map[string]any, status int, callErr error)

// mutatingTools is the set of MCP tools that change remote state. review_*
// tools are server-side simulations (read-only); run_scan returns matches
// without persisting; every get_* tool is a read. Only true mutations are
// gated, journaled, and policy-checked.
var mutatingTools = map[string]struct{}{
	"place_equity_order":           {},
	"place_option_order":           {},
	"cancel_equity_order":          {},
	"cancel_option_order":          {},
	"create_watchlist":             {},
	"update_watchlist":             {},
	"follow_watchlist":             {},
	"unfollow_watchlist":           {},
	"add_to_watchlist":             {},
	"remove_from_watchlist":        {},
	"add_option_to_watchlist":      {},
	"remove_option_from_watchlist": {},
	"create_scan":                  {},
	"update_scan_filters":          {},
	"update_scan_config":           {},
}

func isMutatingTool(tool string) bool {
	_, ok := mutatingTools[tool]
	return ok
}

// argKind classifies how a tool argument must be coerced onto the JSON-RPC
// wire. The generated command layer serializes GET params as query strings and
// POST bodies as JSON; the MCP server wants correctly typed arguments, and some
// fields must be arrays or the call is rejected. Anything not listed for a tool
// defaults to a passthrough string.
type argKind int

const (
	argString  argKind = iota // leave as-is (default)
	argInt                    // coerce string -> JSON number (integer)
	argFloat                  // coerce string -> JSON number (float)
	argBool                   // coerce string -> JSON bool
	argStrList                // split "a,b,c" -> ["a","b","c"]
)

// toolArgSpecs records the non-string argument kinds per tool. Only arguments
// that need coercion are listed; every other argument passes through as a
// string. Ground truth: the per-tool schemas captured during Phase 1.5
// research (authenticated tools/list exports + community SDK source).
//
// The `symbols` argument is array-typed on most market tools but a
// comma-separated STRING on get_indexes — which is exactly why this table is
// keyed per tool rather than per argument name.
var toolArgSpecs = map[string]map[string]argKind{
	"get_equity_quotes":       {"symbols": argStrList},
	"get_equity_price_book":   {"symbols": argStrList},
	"get_equity_fundamentals": {"symbols": argStrList},
	"get_financials":          {"symbols": argStrList, "limit": argInt},
	"get_equity_historicals":  {"symbols": argStrList},
	"get_equity_tradability":  {"symbols": argStrList},
	"get_index_quotes":        {"instrument_ids": argStrList},
	// get_indexes.symbols stays a comma-separated string — do NOT split it.
	"get_realized_pnl":                {"asset_classes": argStrList},
	"search":                          {"limit": argInt},
	"get_earnings_calendar":           {"days": argInt},
	"get_equity_technical_indicators": {"period": argInt, "num_std": argFloat, "fast_period": argInt, "slow_period": argInt, "signal_period": argInt},
	"get_option_chains":               {"ids": argStrList},
	"get_option_instruments":          {"ids": argStrList, "expiration_dates": argStrList},
	"get_option_quotes":               {"instrument_ids": argStrList},
	"get_option_positions":            {"nonzero": argBool},
	"get_option_orders":               {"chain_ids": argStrList},
	"add_option_to_watchlist":         {"option_ids": argStrList},
	"remove_option_from_watchlist":    {"option_ids": argStrList},
	"add_to_watchlist":                {"symbols": argStrList, "currency_pair_ids": argStrList, "index_ids": argStrList},
	"remove_from_watchlist":           {"symbols": argStrList, "currency_pair_ids": argStrList, "index_ids": argStrList},
}

// mcpTransport wraps the real RoundTripper and translates REST-shaped requests
// into MCP tool calls. One instance per Client; the MCP session is established
// lazily on the first tool call and reused for the process lifetime.
type mcpTransport struct {
	base     http.RoundTripper
	endpoint string

	mu          sync.Mutex
	sessionID   string
	initialized bool
	nextID      int64
}

func newMCPTransport(base http.RoundTripper, endpoint string) *mcpTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &mcpTransport{base: base, endpoint: strings.TrimRight(endpoint, "/")}
}

func (t *mcpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Verify/dogfood mock mode always points the CLI at a spec-shaped mock HTTP
	// server, never at a real MCP endpoint — even when LIVE_HTTP=1 is set (that
	// flag only lets the httptest mock observe mutating requests; it does not
	// switch the target to the real API). So whenever the verifier is driving,
	// pass through untouched: the mock's plain `{data: ...}` responses must
	// reach the command layer's response_path extraction without the MCP
	// JSON-RPC rewrite (which would try an `initialize` handshake the mock can't
	// answer). Real execution (dogfood --live, ordinary use) has no verify env
	// set and takes the MCP-bridge path below.
	if cliutil.IsVerifyEnv() {
		return t.base.RoundTrip(req)
	}

	tool, ok := toolNameFromPath(req.URL.Path)
	if !ok {
		// Not a /tools/<name> request (health probes, redirects) — pass through.
		return t.base.RoundTrip(req)
	}

	args, err := collectArgs(req)
	if err != nil {
		return nil, err
	}
	args = coerceArgs(tool, args)
	args = applyToolTransforms(tool, args)

	auth := req.Header.Get("Authorization")

	// Mutation safety chokepoint: every mutating tool passes through the guard
	// (write gate + trade policy) before it can dial out. A refusal never
	// reaches the network. This is the single enforcement point that makes
	// "reads are free, writes are gated" true regardless of which command or
	// agent drove the call.
	if isMutatingTool(tool) && MutationGuard != nil {
		if gerr := MutationGuard(req.Context(), tool, args); gerr != nil {
			return synthResponse(req, http.StatusForbidden, errorEnvelope(gerr)), nil
		}
	}

	result, callErr := t.callTool(req.Context(), tool, args, auth)
	if callErr != nil && isSessionError(callErr) {
		// A session that expired mid-process returns a JSON-RPC session error;
		// reset and retry the handshake once before surfacing the failure.
		t.reset()
		result, callErr = t.callTool(req.Context(), tool, args, auth)
	}
	// Journal only after the session retry settles: an order that fails on an
	// expired session but succeeds on retry is a placement, and journaling the
	// transient error instead would drop it from the daily-cap sum.
	if isMutatingTool(tool) && MutationJournal != nil {
		status := http.StatusOK
		if callErr != nil {
			status = http.StatusBadGateway
		} else if result.isToolError {
			status = http.StatusBadRequest
		}
		MutationJournal(req.Context(), tool, args, status, callErr)
	}
	if callErr != nil {
		// Propagate the real HTTP status when the failure carries one (401 auth,
		// 429 rate-limit, 5xx server) so the client's do() loop handles it
		// correctly — an auth 401 must not be retried as if it were a transient
		// 5xx, and a 429 must feed the rate limiter. Only genuinely
		// status-less failures fall back to 502.
		return synthResponse(req, statusForCallErr(callErr), errorEnvelope(callErr)), nil
	}

	if result.isToolError {
		return synthResponse(req, http.StatusBadRequest, result.body), nil
	}
	return synthResponse(req, http.StatusOK, result.body), nil
}

// statusForCallErr extracts a meaningful HTTP status from a tool-call failure.
// Transport/handshake errors that carry an HTTP status (via *rpcError) use it;
// everything else maps to 502 Bad Gateway.
func statusForCallErr(err error) int {
	if re, ok := err.(*rpcError); ok && re.Code >= 400 && re.Code < 600 {
		return re.Code
	}
	return http.StatusBadGateway
}

// toolCallResult carries the normalized `{data, guide}` envelope bytes and
// whether the MCP server flagged the tool call as an error.
type toolCallResult struct {
	body        []byte
	isToolError bool
}

func (t *mcpTransport) callTool(ctx context.Context, tool string, args map[string]any, auth string) (toolCallResult, error) {
	if err := t.ensureSession(ctx, auth); err != nil {
		return toolCallResult{}, err
	}
	params := map[string]any{"name": tool}
	if len(args) > 0 {
		params["arguments"] = args
	} else {
		params["arguments"] = map[string]any{}
	}
	msg, hdr, err := t.rpc(ctx, "tools/call", params, auth)
	if err != nil {
		return toolCallResult{}, err
	}
	_ = hdr
	if msg.Error != nil {
		return toolCallResult{}, msg.Error
	}
	body, isErr, err := normalizeToolResult(msg.Result)
	if err != nil {
		return toolCallResult{}, err
	}
	return toolCallResult{body: body, isToolError: isErr}, nil
}

// ensureSession performs the initialize + notifications/initialized handshake
// exactly once, capturing the Mcp-Session-Id the server assigns.
func (t *mcpTransport) ensureSession(ctx context.Context, auth string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.initialized {
		return nil
	}
	initParams := map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "robinhood-agentic-pp-cli", "version": "0.1.0"},
	}
	_, hdr, err := t.rpcLocked(ctx, "initialize", initParams, auth)
	if err != nil {
		return err
	}
	if sid := hdr.Get("Mcp-Session-Id"); sid != "" {
		t.sessionID = sid
	}
	// The notification carries no id and expects no response body.
	if err := t.notifyLocked(ctx, "notifications/initialized", auth); err != nil {
		return err
	}
	t.initialized = true
	return nil
}

func (t *mcpTransport) reset() {
	t.mu.Lock()
	t.initialized = false
	t.sessionID = ""
	t.mu.Unlock()
}

// rpc issues a JSON-RPC request/response pair, taking the session mutex to read
// the current session id. rpcLocked is the variant used inside ensureSession
// where the mutex is already held.
func (t *mcpTransport) rpc(ctx context.Context, method string, params any, auth string) (*rpcMessage, http.Header, error) {
	t.mu.Lock()
	sid := t.sessionID
	t.mu.Unlock()
	return t.send(ctx, method, params, auth, sid, true)
}

func (t *mcpTransport) rpcLocked(ctx context.Context, method string, params any, auth string) (*rpcMessage, http.Header, error) {
	return t.send(ctx, method, params, auth, t.sessionID, true)
}

func (t *mcpTransport) notifyLocked(ctx context.Context, method string, auth string) error {
	_, _, err := t.send(ctx, method, map[string]any{}, auth, t.sessionID, false)
	return err
}

func (t *mcpTransport) nextRequestID() int64 {
	t.nextID++
	return t.nextID
}

func (t *mcpTransport) send(ctx context.Context, method string, params any, auth, sessionID string, expectResponse bool) (*rpcMessage, http.Header, error) {
	payload := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		payload["params"] = params
	}
	if expectResponse {
		payload["id"] = t.nextRequestID()
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling %s: %w", method, err)
	}

	// The MCP request goes straight through the base RoundTripper, bypassing the
	// generated *http.Client's Timeout, so bound it explicitly here — otherwise a
	// stalled connection would hang the command (and, under concurrency, every
	// worker) with no deadline. WithTimeout respects an earlier deadline the
	// caller's context may already carry (e.g. a --timeout flag).
	ctx, cancel := context.WithTimeout(ctx, mcpRequestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if auth != "" {
		httpReq.Header.Set("Authorization", auth)
	}
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := t.base.RoundTrip(httpReq)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, resp.Header, &rpcError{Code: resp.StatusCode, Message: httpErrorMessage(resp.StatusCode, body)}
	}
	if !expectResponse {
		return nil, resp.Header, nil
	}

	msg, err := parseRPCResponse(resp.Header.Get("Content-Type"), body)
	if err != nil {
		return nil, resp.Header, err
	}
	return msg, resp.Header, nil
}

// MCPListTools performs an MCP tools/list against the live endpoint and returns
// the tools array as JSON. Used by the `surface` command to snapshot the tool
// surface for change detection — Robinhood grows and reshapes the beta surface
// without notice. Returns an error if the client is not backed by the MCP
// transport (e.g. in a unit test with a stub transport).
func (c *Client) MCPListTools(ctx context.Context) (json.RawMessage, error) {
	mt, ok := c.HTTPClient.Transport.(*mcpTransport)
	if !ok {
		return nil, fmt.Errorf("client is not backed by the MCP transport")
	}
	auth, err := c.authHeader(ctx)
	if err != nil {
		return nil, err
	}
	return mt.listTools(ctx, auth)
}

// listTools handshakes if needed, then pages through tools/list, returning the
// concatenated tools array.
func (t *mcpTransport) listTools(ctx context.Context, auth string) (json.RawMessage, error) {
	if err := t.ensureSession(ctx, auth); err != nil {
		return nil, err
	}
	var all []json.RawMessage
	cursor := ""
	for i := 0; i < 50; i++ { // hard page cap
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		msg, _, err := t.rpc(ctx, "tools/list", params, auth)
		if err != nil {
			return nil, err
		}
		if msg.Error != nil {
			return nil, msg.Error
		}
		var page struct {
			Tools      []json.RawMessage `json:"tools"`
			NextCursor string            `json:"nextCursor"`
		}
		if err := json.Unmarshal(msg.Result, &page); err != nil {
			return nil, fmt.Errorf("mcp: decoding tools/list: %w", err)
		}
		all = append(all, page.Tools...)
		if page.NextCursor == "" || len(page.Tools) == 0 {
			break
		}
		cursor = page.NextCursor
	}
	return json.Marshal(all)
}

// --- request parsing -------------------------------------------------------

// toolNameFromPath extracts the tool name from a `.../tools/<name>` path.
func toolNameFromPath(path string) (string, bool) {
	idx := strings.LastIndex(path, "/tools/")
	if idx < 0 {
		return "", false
	}
	name := path[idx+len("/tools/"):]
	name = strings.Trim(name, "/")
	if name == "" || strings.Contains(name, "/") {
		return "", false
	}
	return name, true
}

// collectArgs reads arguments from the query string (GET) or JSON body (POST).
func collectArgs(req *http.Request) (map[string]any, error) {
	args := map[string]any{}
	for k, vs := range req.URL.Query() {
		if len(vs) > 0 && vs[0] != "" {
			args[k] = vs[0]
		}
	}
	if req.Body != nil && (req.Method == http.MethodPost || req.Method == http.MethodPut) {
		raw, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		if len(bytes.TrimSpace(raw)) > 0 {
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				return nil, fmt.Errorf("parsing request body: %w", err)
			}
			for k, v := range body {
				if v == nil {
					continue
				}
				if s, ok := v.(string); ok && s == "" {
					continue
				}
				args[k] = v
			}
		}
	}
	return args, nil
}

// coerceArgs applies the per-tool argument kinds. String inputs (GET query, or
// string-typed body fields) are converted to the target JSON type; values that
// already arrive correctly typed from the JSON body are left untouched.
func coerceArgs(tool string, args map[string]any) map[string]any {
	specs := toolArgSpecs[tool]
	if specs == nil {
		return args
	}
	for name, kind := range specs {
		v, ok := args[name]
		if !ok {
			continue
		}
		s, isStr := v.(string)
		switch kind {
		case argStrList:
			if isStr {
				args[name] = splitCSV(s)
			}
		case argInt:
			if isStr {
				if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
					args[name] = n
				}
			}
		case argFloat:
			if isStr {
				if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
					args[name] = f
				}
			}
		case argBool:
			if isStr {
				if b, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
					args[name] = b
				}
			}
		case argString:
			// no-op
		}
	}
	return args
}

// applyToolTransforms handles per-tool shape differences that a flat argument
// map can't express — notably folding the flat single-leg option-order fields
// into the `legs[]` array the MCP order tools require.
func applyToolTransforms(tool string, args map[string]any) map[string]any {
	switch tool {
	case "review_option_order", "place_option_order":
		leg := map[string]any{"ratio_quantity": "1"}
		for _, f := range []string{"option_id", "side", "position_effect"} {
			if v, ok := args[f]; ok {
				leg[f] = v
				delete(args, f)
			}
		}
		if len(leg) > 1 {
			args["legs"] = []any{leg}
		}
	}
	return args
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// --- response parsing ------------------------------------------------------

type rpcMessage struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return "mcp: unknown error"
	}
	return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message)
}

func isSessionError(err error) bool {
	var re *rpcError
	if e, ok := err.(*rpcError); ok {
		re = e
	}
	if re == nil {
		return false
	}
	if re.Code == http.StatusNotFound || re.Code == http.StatusUnauthorized {
		return false // 401 is an auth problem, not a stale session
	}
	msg := strings.ToLower(re.Message)
	return strings.Contains(msg, "session")
}

// parseRPCResponse decodes a JSON-RPC message from either a plain JSON body or
// an SSE (text/event-stream) body whose `data:` lines carry the JSON.
func parseRPCResponse(contentType string, body []byte) (*rpcMessage, error) {
	trimmed := bytes.TrimSpace(body)
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") || bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte("data:")) {
		data, err := extractSSEData(body)
		if err != nil {
			return nil, err
		}
		trimmed = data
	}
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("mcp: empty response body")
	}
	var msg rpcMessage
	if err := json.Unmarshal(trimmed, &msg); err != nil {
		return nil, fmt.Errorf("mcp: decoding response: %w", err)
	}
	return &msg, nil
}

// extractSSEData returns the JSON payload of the last `data:` event in an SSE
// stream (the JSON-RPC response frame).
func extractSSEData(body []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var last []byte
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			last = []byte(cur.String())
			cur.Reset()
		}
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "data:") {
			cur.WriteString(strings.TrimSpace(line[len("data:"):]))
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("mcp: reading event stream: %w", err)
	}
	if len(last) == 0 {
		return nil, fmt.Errorf("mcp: no data frame in event stream")
	}
	return last, nil
}

// mcpToolResult mirrors the MCP tools/call result object.
type mcpToolResult struct {
	Content           []mcpContent    `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// normalizeToolResult reduces a tools/call result to the `{data, guide}`
// envelope the generated command layer's response_path expects. Handles all
// three observed shapes: structuredContent, structuredContent.data, and a
// JSON string inside content[].text. When the payload lacks a top-level
// "data" key it is wrapped as {"data": <payload>} so response_path resolves.
func normalizeToolResult(raw json.RawMessage) ([]byte, bool, error) {
	if len(raw) == 0 {
		body, err := json.Marshal(map[string]any{"data": map[string]any{}})
		return body, false, err
	}
	var res mcpToolResult
	if err := json.Unmarshal(raw, &res); err != nil {
		// Not the standard result shape — return it verbatim, wrapped.
		body, werr := wrapEnvelope(raw)
		return body, false, werr
	}

	if len(res.StructuredContent) > 0 && !isJSONNull(res.StructuredContent) {
		body, err := wrapEnvelope(res.StructuredContent)
		return body, res.IsError, err
	}
	for _, c := range res.Content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			if json.Valid([]byte(c.Text)) {
				body, err := wrapEnvelope(json.RawMessage(c.Text))
				return body, res.IsError, err
			}
			// Plain-text error/message payload.
			body, err := json.Marshal(map[string]any{"data": map[string]any{"message": c.Text}})
			return body, res.IsError, err
		}
	}
	// No structured or text content — surface the raw result wrapped.
	body, err := wrapEnvelope(raw)
	return body, res.IsError, err
}

// wrapEnvelope ensures the returned bytes are an object with a top-level "data"
// key. If the payload already has one, it is returned unchanged.
func wrapEnvelope(payload json.RawMessage) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err == nil {
		if _, ok := obj["data"]; ok {
			return payload, nil
		}
	}
	// Object without "data", or a non-object payload (array/scalar) — wrap so
	// response_path "data" resolves.
	return json.Marshal(map[string]json.RawMessage{"data": payload})
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

func errorEnvelope(err error) []byte {
	b, _ := json.Marshal(map[string]any{"data": map[string]any{}, "error": err.Error()})
	return b
}

func httpErrorMessage(status int, body []byte) string {
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return http.StatusText(status)
	}
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return msg
}

// synthResponse builds an *http.Response the generated do() loop can consume as
// if it came straight off the wire.
func synthResponse(req *http.Request, status int, body []byte) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}
