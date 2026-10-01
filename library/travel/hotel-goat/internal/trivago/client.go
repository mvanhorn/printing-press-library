// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

// Package trivago is a thin client for Trivago's public MCP server
// (https://mcp.trivago.com/mcp). Used as a second cash-price source
// alongside Google Hotels. No API key required.
//
// Transport: JSON-RPC over MCP Streamable-HTTP. Each session does
// initialize -> notifications/initialized -> tools/call. The session ID
// returned in the initialize response header must be echoed on every
// subsequent call as Mcp-Session-Id. Tool-call responses come back as
// text/event-stream; parseMaybeSSE strips the SSE framing.
package trivago

import (
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

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/cliutil"
)

const DefaultEndpoint = "https://mcp.trivago.com/mcp"

// DefaultRatePerSec paces outbound calls to the Trivago MCP server.
// 2 rps is conservative for a free public endpoint and matches the CLI's
// global --rate-limit default. The AdaptiveLimiter ramps up after
// sustained success and halves on 429.
const DefaultRatePerSec = 2.0

type Client struct {
	HTTPClient *http.Client
	Endpoint   string

	// Limiter paces outbound HTTP calls. Nil disables rate limiting;
	// NewClient seeds a DefaultRatePerSec AdaptiveLimiter.
	Limiter *cliutil.AdaptiveLimiter

	initGateOnce sync.Once
	initGate     chan struct{}
	initialized  bool
	sessionID    string
	reqID        int64
	reqIDMu      sync.Mutex
}

func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Endpoint:   DefaultEndpoint,
		Limiter:    cliutil.NewAdaptiveLimiter(DefaultRatePerSec),
	}
}

// waitForSlot blocks until the AdaptiveLimiter releases the next slot,
// honoring ctx cancellation. No-op when Limiter is nil.
//
// The channel is buffered so the limiter goroutine never blocks on send
// after the caller has already returned on ctx.Done(); the goroutine
// finishes its Limiter.Wait() and exits without leaking under repeated
// cancellation (AdaptiveLimiter backoff after 429 can otherwise outlive
// the request).
func (c *Client) waitForSlot(ctx context.Context) error {
	if c.Limiter == nil {
		return nil
	}
	done := make(chan struct{}, 1)
	go func() {
		c.Limiter.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

func readHandshakeBody(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	const maxBytes = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBytes {
		return nil, fmt.Errorf("handshake response exceeds 1 MiB")
	}
	return raw, nil
}

func (c *Client) nextID() int64 {
	c.reqIDMu.Lock()
	defer c.reqIDMu.Unlock()
	c.reqID++
	return c.reqID
}

func (c *Client) ensureInit(ctx context.Context) error {
	// A channel serializes first use while allowing waiting callers to cancel.
	// Only the gate itself is initialized once; a failed handshake is retryable.
	if err := ctx.Err(); err != nil {
		return err
	}
	c.initGateOnce.Do(func() { c.initGate = make(chan struct{}, 1) })
	select {
	case c.initGate <- struct{}{}:
		defer func() { <-c.initGate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if c.initialized {
		return nil
	}
	c.sessionID = ""

	initID := c.nextID()
	body, _ := json.Marshal(rpcRequest{
		JSONRPC: "2.0", ID: initID, Method: "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "hotel-goat-pp-cli", "version": "1"},
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if err := c.waitForSlot(ctx); err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	raw, readErr := readHandshakeBody(resp.Body)
	if readErr != nil {
		return fmt.Errorf("trivago: read initialize response: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("trivago: initialize HTTP %d", resp.StatusCode)
	}
	var initialized rpcResponse
	if err := json.Unmarshal(parseMaybeSSE(raw), &initialized); err != nil {
		return fmt.Errorf("trivago: decode initialize response: %w", err)
	}
	if initialized.JSONRPC != "2.0" || initialized.ID != initID {
		return fmt.Errorf("trivago: invalid initialize response envelope")
	}
	if initialized.Error != nil {
		return fmt.Errorf("trivago initialize: %s", truncate([]byte(initialized.Error.Message)))
	}
	if result := bytes.TrimSpace(initialized.Result); len(result) == 0 || bytes.Equal(result, []byte("null")) {
		return fmt.Errorf("trivago: initialize response has no result")
	}
	var initResult initializeResult
	if err := json.Unmarshal(initialized.Result, &initResult); err != nil {
		return fmt.Errorf("trivago: invalid initialize result: %w", err)
	}
	if strings.TrimSpace(initResult.ProtocolVersion) == "" ||
		strings.TrimSpace(initResult.ServerInfo.Name) == "" ||
		strings.TrimSpace(initResult.ServerInfo.Version) == "" {
		return fmt.Errorf("trivago: invalid initialize result: missing protocolVersion or serverInfo")
	}
	var capabilities map[string]json.RawMessage
	if len(initResult.Capabilities) == 0 || bytes.Equal(bytes.TrimSpace(initResult.Capabilities), []byte("null")) ||
		json.Unmarshal(initResult.Capabilities, &capabilities) != nil || capabilities == nil {
		return fmt.Errorf("trivago: invalid initialize result: capabilities must be an object")
	}
	sessionID := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id"))
	if sessionID == "" {
		return fmt.Errorf("trivago: no mcp-session-id in initialize response")
	}
	sessionReady := false
	defer func() {
		if !sessionReady {
			// Attempt cleanup before a short-lived CLI exits. Bound the wait so
			// a slow DELETE cannot hold a canceled lookup for long.
			c.closeFailedSession(sessionID)
		}
	}()

	// Per spec, send the initialized notification before any tools/call.
	nb, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
	nReq, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(nb))
	if err != nil {
		return err
	}
	nReq.Header.Set("Content-Type", "application/json")
	nReq.Header.Set("Accept", "application/json, text/event-stream")
	nReq.Header.Set("Mcp-Session-Id", sessionID)
	if err := c.waitForSlot(ctx); err != nil {
		return err
	}
	nResp, err := c.HTTPClient.Do(nReq)
	if err != nil {
		return err
	}
	notificationBody, readErr := readHandshakeBody(nResp.Body)
	if readErr != nil {
		return fmt.Errorf("trivago: read initialized notification response: %w", readErr)
	}
	if nResp.StatusCode < 200 || nResp.StatusCode >= 300 {
		return fmt.Errorf("trivago: initialized notification HTTP %d", nResp.StatusCode)
	}
	if len(bytes.TrimSpace(notificationBody)) > 0 {
		var notification rpcResponse
		if err := json.Unmarshal(parseMaybeSSE(notificationBody), &notification); err != nil {
			return fmt.Errorf("trivago: invalid initialized notification response: %w", err)
		}
		if notification.Error != nil {
			return fmt.Errorf("trivago initialized notification: %s", truncate([]byte(notification.Error.Message)))
		}
		return fmt.Errorf("trivago: unexpected initialized notification response body")
	}

	c.sessionID = sessionID
	c.initialized = true
	sessionReady = true
	return nil
}

// closeFailedSession releases a session created by initialize but rejected by
// the notification step. Cleanup is best effort and has its own short timeout
// so a canceled caller can return promptly after the DELETE attempt.
func (c *Client) closeFailedSession(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.Endpoint, nil)
	if err != nil {
		return
	}
	req.Header.Set("Mcp-Session-Id", sessionID)
	// This is an exceptional cleanup request; waiting for the normal pacing
	// slot could consume the whole deadline before DELETE reaches the server.
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	_ = resp.Body.Close()
}

func (c *Client) callTool(ctx context.Context, name string, args any) (json.RawMessage, error) {
	if err := c.ensureInit(ctx); err != nil {
		return nil, err
	}
	body, _ := json.Marshal(rpcRequest{
		JSONRPC: "2.0", ID: c.nextID(), Method: "tools/call",
		Params: map[string]any{"name": name, "arguments": args},
	})

	// Single 429 retry honoring Retry-After. Trivago's MCP server is a
	// shared public endpoint; rate spikes are possible. We retry once
	// then surface the failure to the caller.
	var raw []byte
	var statusCode int
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Session-Id", c.sessionID)
		if err := c.waitForSlot(ctx); err != nil {
			return nil, err
		}
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		raw, err = io.ReadAll(resp.Body)
		statusCode = resp.StatusCode
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		if resp.StatusCode == 429 && attempt == 0 {
			if c.Limiter != nil {
				c.Limiter.OnRateLimit()
			}
			wait := cliutil.RetryAfter(resp)
			resp.Body.Close()
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		resp.Body.Close()
		break
	}
	if statusCode == 429 {
		return nil, &cliutil.RateLimitError{URL: c.Endpoint, Body: truncate(raw)}
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("trivago: HTTP %d: %s", statusCode, truncate(raw))
	}
	payload := parseMaybeSSE(raw)
	var rr rpcResponse
	if err := json.Unmarshal(payload, &rr); err != nil {
		return nil, fmt.Errorf("trivago: decode %s: %w (body=%s)", name, err, truncate(raw))
	}
	if rr.Error != nil {
		return nil, fmt.Errorf("trivago %s: %s", name, rr.Error.Message)
	}
	// Signal success only after the response is structurally valid; a 2xx
	// with malformed JSON shouldn't ramp the limiter as if the call
	// produced usable data. Nil-guarded to match waitForSlot, since the
	// Limiter field is documented as nil-safe and TestWaitForSlot_DisabledWhenNil
	// exercises that path.
	if c.Limiter != nil {
		c.Limiter.OnSuccess()
	}
	return rr.Result, nil
}

// parseMaybeSSE returns the JSON-RPC payload from either a plain
// application/json body or a text/event-stream body. Trivago returns SSE
// for tools/call. We concatenate consecutive "data:" lines (per SSE rules
// they belong to the same event) and use the last event's payload, which
// is the terminal "message" event carrying the JSON-RPC response.
func parseMaybeSSE(b []byte) []byte {
	if !bytes.Contains(b, []byte("\ndata:")) && !bytes.HasPrefix(b, []byte("data:")) {
		return b
	}
	var current, last []byte
	for _, line := range bytes.Split(b, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 {
			if len(current) > 0 {
				last = append(last[:0], current...)
				current = current[:0]
			}
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			chunk := bytes.TrimSpace(line[5:])
			current = append(current, chunk...)
		}
	}
	if len(current) > 0 {
		last = current
	}
	if last == nil {
		return b
	}
	return last
}

func truncate(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}

// Accommodation mirrors the schema declared by Trivago's
// trivago-accommodation-search and -radius-search output. Prices are
// returned as preformatted strings (e.g. "$199") because Trivago localizes
// the currency symbol; callers parse the numeric value with ParsePrice.
// Field notes for the current API (serverInfo "trivago Accommodation
// Search" v0.4.0):
//   - review_count is a localized string ("25,429"); callers use
//     ParseReviewCount to get the int. ReviewCount's custom
//     UnmarshalJSON accepts both a bare JSON number and a string so a
//     schema drift in either direction stays tolerant.
//   - booking_url is not returned today — only accommodation_url — so
//     BookingURL falls back to URL (accommodation_url) during decode.
type Accommodation struct {
	ID            string      `json:"accommodation_id"`
	Name          string      `json:"accommodation_name"`
	URL           string      `json:"accommodation_url"`
	Address       string      `json:"address"`
	PostalCode    string      `json:"postal_code"`
	CountryCity   string      `json:"country_city"`
	Currency      string      `json:"currency"`
	PricePerNight string      `json:"price_per_night"`
	PricePerStay  string      `json:"price_per_stay"`
	Advertiser    string      `json:"advertisers"`
	BookingURL    string      `json:"booking_url"`
	HotelRating   int         `json:"hotel_rating"`
	ReviewRating  string      `json:"review_rating"`
	ReviewCount   ReviewCount `json:"review_count"`
	Latitude      float64     `json:"latitude"`
	Longitude     float64     `json:"longitude"`
	Distance      string      `json:"distance"`
	Image         string      `json:"main_image"`
	Amenities     string      `json:"top_amenities"`
	Description   string      `json:"description"`
}

// ReviewCount is a tolerance wrapper around Trivago's review_count field,
// which the current API emits as a localized string ("25,429"). Accepting
// a bare JSON number too means a server change in either direction won't
// fail the whole decode. Use ParseReviewCount to get an int.
type ReviewCount string

// UnmarshalJSON accepts a JSON string or a JSON number.
func (r *ReviewCount) UnmarshalJSON(b []byte) error {
	// JSON null is a missing count, not an unsupported type.
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		*r = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*r = ReviewCount(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*r = ReviewCount(n.String())
		return nil
	}
	return fmt.Errorf("trivago: review_count must be a string or number, got %s", b)
}

// ParseReviewCount returns the integer from a localized review-count
// string like "25,429", "1.563", or "42". Returns 0 when unparseable.
func ParseReviewCount(s string) int {
	if s == "" {
		return 0
	}
	clean := priceRE.FindString(s)
	if clean == "" {
		return 0
	}
	// Strip thousands separators: keep the rightmost as decimal only when
	// it has 1-2 trailing digits (a decimal), else drop it. Reviews are
	// whole numbers, so collapse every , and . except a trailing decimal.
	if i := strings.LastIndexAny(clean, ",."); i >= 0 && len(clean)-i-1 <= 2 {
		clean = clean[:i] + strings.ReplaceAll(clean[i+1:], ",", "") + strings.ReplaceAll(clean[:i], ",", "")
	}
	cleaned := strings.NewReplacer(",", "", ".", "").Replace(clean)
	n, _ := strconv.Atoi(cleaned)
	return n
}

type Suggestion struct {
	ID             int    `json:"id"`
	NS             int    `json:"ns"`
	PlaceID        string `json:"place_id"`
	Location       string `json:"location"`
	LocationLabel  string `json:"location_label"`
	LocationType   string `json:"location_type"`
	SuggestionType string `json:"suggestion_type"`
}

type RadiusOpts struct {
	Lat, Lng           float64
	RadiusMeters       int
	Arrival, Departure string
	Adults, Rooms      int
}

// SearchOpts are the arguments for the query-based
// trivago-accommodation-search tool (the current API's primary entry
// point). The server resolves the query to a destination itself, so no
// separate suggestions/geocoding step is needed.
type SearchOpts struct {
	Query              string
	Arrival, Departure string
	Adults, Rooms      int
}

func (c *Client) RadiusSearch(ctx context.Context, o RadiusOpts) ([]Accommodation, error) {
	args := map[string]any{
		"latitude":  o.Lat,
		"longitude": o.Lng,
		"radius":    o.RadiusMeters,
		"arrival":   o.Arrival,
		"departure": o.Departure,
	}
	if o.Adults > 0 {
		args["adults"] = o.Adults
	}
	if o.Rooms > 0 {
		args["rooms"] = o.Rooms
	}
	raw, err := c.callTool(ctx, "trivago-accommodation-radius-search", args)
	if err != nil {
		return nil, err
	}
	return decodeAccommodations(raw)
}

// Search runs trivago-accommodation-search with a free-form destination
// query. This is the replacement for the old suggestions->area flow: the
// server dropped trivago-search-suggestions, so resolving `<location>` to
// an id/ns pair is no longer possible (or necessary) — passing the query
// string directly returns the same ~50-result candidate set.
func (c *Client) Search(ctx context.Context, o SearchOpts) ([]Accommodation, error) {
	args := map[string]any{
		"query":     o.Query,
		"arrival":   o.Arrival,
		"departure": o.Departure,
	}
	if o.Adults > 0 {
		args["adults"] = o.Adults
	}
	if o.Rooms > 0 {
		args["rooms"] = o.Rooms
	}
	raw, err := c.callTool(ctx, "trivago-accommodation-search", args)
	if err != nil {
		return nil, err
	}
	return decodeAccommodations(raw)
}

func decodeAccommodations(raw json.RawMessage) ([]Accommodation, error) {
	var wrapper struct {
		StructuredContent struct {
			Accommodations []Accommodation `json:"accommodations"`
		} `json:"structuredContent"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.StructuredContent.Accommodations) > 0 {
		acc := wrapper.StructuredContent.Accommodations
		for i := range acc {
			acc[i].BookingURL = bookingLink(acc[i])
		}
		return acc, nil
	}
	for _, c := range wrapper.Content {
		if c.Type != "text" {
			continue
		}
		var s struct {
			Accommodations []Accommodation `json:"accommodations"`
		}
		if json.Unmarshal([]byte(c.Text), &s) == nil && len(s.Accommodations) > 0 {
			for i := range s.Accommodations {
				s.Accommodations[i].BookingURL = bookingLink(s.Accommodations[i])
			}
			return s.Accommodations, nil
		}
	}
	return nil, nil
}
