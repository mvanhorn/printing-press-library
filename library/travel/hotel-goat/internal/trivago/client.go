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

	initOnce  sync.Once
	initErr   error
	sessionID string
	reqID     int64
	reqIDMu   sync.Mutex
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

func (c *Client) nextID() int64 {
	c.reqIDMu.Lock()
	defer c.reqIDMu.Unlock()
	c.reqID++
	return c.reqID
}

func (c *Client) ensureInit(ctx context.Context) error {
	c.initOnce.Do(func() {
		body, _ := json.Marshal(rpcRequest{
			JSONRPC: "2.0", ID: c.nextID(), Method: "initialize",
			Params: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{},
				"clientInfo":      map[string]any{"name": "hotel-goat-pp-cli", "version": "1"},
			},
		})
		req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(body))
		if err != nil {
			c.initErr = err
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if err := c.waitForSlot(ctx); err != nil {
			c.initErr = err
			return
		}
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			c.initErr = err
			return
		}
		defer resp.Body.Close()
		c.sessionID = resp.Header.Get("Mcp-Session-Id")
		if c.sessionID == "" {
			c.initErr = fmt.Errorf("trivago: no mcp-session-id in initialize response")
			return
		}
		io.Copy(io.Discard, resp.Body)

		// Per spec, send the initialized notification before any tools/call.
		nb, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
		nReq, _ := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(nb))
		nReq.Header.Set("Content-Type", "application/json")
		nReq.Header.Set("Accept", "application/json, text/event-stream")
		nReq.Header.Set("Mcp-Session-Id", c.sessionID)
		if err := c.waitForSlot(ctx); err != nil {
			c.initErr = err
			return
		}
		nResp, err := c.HTTPClient.Do(nReq)
		if err != nil {
			c.initErr = err
			return
		}
		io.Copy(io.Discard, nResp.Body)
		nResp.Body.Close()
	})
	return c.initErr
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
type Accommodation struct {
	ID            string  `json:"accommodation_id"`
	Name          string  `json:"accommodation_name"`
	URL           string  `json:"accommodation_url"`
	Address       string  `json:"address"`
	PostalCode    string  `json:"postal_code"`
	CountryCity   string  `json:"country_city"`
	Currency      string  `json:"currency"`
	PricePerNight string  `json:"price_per_night"`
	PricePerStay  string  `json:"price_per_stay"`
	Advertiser    string  `json:"advertisers"`
	BookingURL    string  `json:"booking_url"`
	HotelRating   int     `json:"hotel_rating"`
	ReviewRating  string  `json:"review_rating"`
	ReviewCount   int     `json:"review_count"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Distance      string  `json:"distance"`
	Image         string  `json:"main_image"`
	Amenities     string  `json:"top_amenities"`
	Description   string  `json:"description"`
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

type AreaOpts struct {
	ID, NS             int
	Arrival, Departure string
	Adults, Rooms      int
}

func (c *Client) AreaSearch(ctx context.Context, o AreaOpts) ([]Accommodation, error) {
	args := map[string]any{
		"id":        o.ID,
		"ns":        o.NS,
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

func (c *Client) Suggestions(ctx context.Context, query string) ([]Suggestion, error) {
	raw, err := c.callTool(ctx, "trivago-search-suggestions", map[string]any{"query": query})
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		StructuredContent struct {
			Suggestions []Suggestion `json:"suggestions"`
		} `json:"structuredContent"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.StructuredContent.Suggestions) > 0 {
		return wrapper.StructuredContent.Suggestions, nil
	}
	for _, c := range wrapper.Content {
		if c.Type != "text" {
			continue
		}
		var s struct {
			Suggestions []Suggestion `json:"suggestions"`
		}
		if json.Unmarshal([]byte(c.Text), &s) == nil && len(s.Suggestions) > 0 {
			return s.Suggestions, nil
		}
	}
	return nil, nil
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
		return wrapper.StructuredContent.Accommodations, nil
	}
	for _, c := range wrapper.Content {
		if c.Type != "text" {
			continue
		}
		var s struct {
			Accommodations []Accommodation `json:"accommodations"`
		}
		if json.Unmarshal([]byte(c.Text), &s) == nil && len(s.Accommodations) > 0 {
			return s.Accommodations, nil
		}
	}
	return nil, nil
}
