// Package polymarket provides read-only helpers that walk Polymarket's
// Gamma API by event-parent relationships rather than relying on the
// upstream /public-search endpoint, which is stale for hub topics like
// celebrity markets. The flow: from any known market slug, follow
// markets[].events[0].slug to the parent event, then enumerate the
// event's child markets. This was the only path that resolved every
// dogfood blocker; tag enumeration is deferred (/tags has a 100/page
// cap with no documented cursor).
package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://gamma-api.polymarket.com"

// Client is the minimal Polymarket Gamma HTTP client used by walk.
// BaseURL defaults to gamma-api.polymarket.com but tests override it.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New returns a Client with the default base URL and a 30-second timeout.
func New() *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Event is the parent shape returned by EventForMarket. Only fields the
// CLI surfaces are decoded; the upstream payload carries dozens of
// auxiliary fields we don't need.
type Event struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	EndDate     string `json:"endDate"`
	Description string `json:"description,omitempty"`
	MarketCount int    `json:"marketCount"`
}

// Sibling is one market under a parent event. YesProbability comes from
// the canonical Polymarket `outcomePrices[0]` JSON string; YesPercent is
// the rounded display companion.
type Sibling struct {
	Slug           string  `json:"slug"`
	Question       string  `json:"question"`
	YesProbability float64 `json:"yesProbability"`
	YesPercent     float64 `json:"yesPercent"`
	Volume         float64 `json:"volume"`
	Closed         bool    `json:"closed"`
	EndDate        string  `json:"endDate"`
	URL            string  `json:"url"`
}

// EventForMarket walks from a market slug to its parent event. Returns
// (Event{}, false, nil) when the market has no events array; returns
// non-nil error only on network or decoding failure.
func (c *Client) EventForMarket(ctx context.Context, slug string) (Event, bool, error) {
	if strings.TrimSpace(slug) == "" {
		return Event{}, false, fmt.Errorf("polymarket EventForMarket: empty slug")
	}
	body, err := c.get(ctx, "/markets?slug="+url.QueryEscape(slug))
	if err != nil {
		return Event{}, false, err
	}
	var markets []map[string]any
	if err := json.Unmarshal(body, &markets); err != nil {
		return Event{}, false, fmt.Errorf("decode markets: %w", err)
	}
	if len(markets) == 0 {
		return Event{}, false, nil
	}
	eventsRaw, _ := markets[0]["events"].([]any)
	if len(eventsRaw) == 0 {
		return Event{}, false, nil
	}
	first, _ := eventsRaw[0].(map[string]any)
	if first == nil {
		return Event{}, false, nil
	}
	ev := Event{
		Slug:        asString(first["slug"]),
		Title:       asString(first["title"]),
		EndDate:     asString(first["endDate"]),
		MarketCount: asInt(first["marketCount"]),
	}
	if ev.Slug == "" {
		return Event{}, false, nil
	}
	return ev, true, nil
}

// SiblingsForMarket walks market-slug -> parent-event-slug -> all child
// markets. Sorted by volume descending. The includeClosed flag controls
// whether closed sibling markets are returned (default false on most
// callsites, true when the user explicitly opts in).
func (c *Client) SiblingsForMarket(ctx context.Context, slug string, includeClosed bool) (Event, []Sibling, error) {
	ev, ok, err := c.EventForMarket(ctx, slug)
	if err != nil {
		return Event{}, nil, err
	}
	if !ok {
		return Event{}, nil, fmt.Errorf("polymarket SiblingsForMarket: market %q has no parent event", slug)
	}
	body, err := c.get(ctx, "/events?slug="+url.QueryEscape(ev.Slug))
	if err != nil {
		return ev, nil, err
	}
	var events []map[string]any
	if err := json.Unmarshal(body, &events); err != nil {
		return ev, nil, fmt.Errorf("decode event: %w", err)
	}
	if len(events) == 0 {
		return ev, nil, nil
	}
	marketsRaw, _ := events[0]["markets"].([]any)
	siblings := make([]Sibling, 0, len(marketsRaw))
	for _, m := range marketsRaw {
		mObj, ok := m.(map[string]any)
		if !ok {
			continue
		}
		closed := asBool(mObj["closed"])
		if closed && !includeClosed {
			continue
		}
		sib := Sibling{
			Slug:     asString(mObj["slug"]),
			Question: asString(mObj["question"]),
			Volume:   asFloat(mObj["volumeNum"]),
			Closed:   closed,
			EndDate:  asString(mObj["endDate"]),
		}
		if sib.Slug != "" {
			sib.URL = "https://polymarket.com/market/" + sib.Slug
		}
		if op, ok := mObj["outcomePrices"].(string); ok {
			sib.YesProbability = parseOutcomePriceYes(op)
		}
		sib.YesPercent = roundPercent(sib.YesProbability)
		siblings = append(siblings, sib)
	}
	// Stable sort by volume desc using a simple selection-style pass; the
	// markets count per event is bounded (<= ~60 for the largest events)
	// so the cost is irrelevant.
	for i := range siblings {
		bestIdx := i
		for j := i + 1; j < len(siblings); j++ {
			if siblings[j].Volume > siblings[bestIdx].Volume {
				bestIdx = j
			}
		}
		if bestIdx != i {
			siblings[i], siblings[bestIdx] = siblings[bestIdx], siblings[i]
		}
	}
	return ev, siblings, nil
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "prediction-goat-pp-cli/1.0")
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("polymarket GET %s: HTTP %d: %s", path, resp.StatusCode, snippet(body, 200))
	}
	return body, nil
}

func snippet(body []byte, limit int) string {
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit])
}

// parseOutcomePriceYes extracts the YES price from a JSON-string-encoded
// outcomePrices array (the canonical Polymarket Gamma shape is
// `"outcomePrices": "[\"0.062\", \"0.938\"]"`). Returns 0 on parse error.
func parseOutcomePriceYes(s string) float64 {
	var prices []string
	if err := json.Unmarshal([]byte(s), &prices); err != nil {
		return 0
	}
	if len(prices) == 0 {
		return 0
	}
	var f float64
	_, _ = fmt.Sscanf(prices[0], "%f", &f)
	return f
}

// roundPercent mirrors the cli package's yesPercent helper so the source
// layer can populate the percent companion without importing back into cli.
func roundPercent(v float64) float64 {
	if v == 0 {
		return 0
	}
	return float64(int(v*1000+0.5)) / 10
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		var f float64
		_, _ = fmt.Sscanf(n, "%f", &f)
		return f
	}
	return 0
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		return s == "true" || s == "True" || s == "TRUE"
	}
	return false
}
