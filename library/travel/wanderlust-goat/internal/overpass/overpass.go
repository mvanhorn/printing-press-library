// Package overpass is a thin client for the OSM Overpass API.
//
// Endpoint: POST https://overpass-api.de/api/interpreter with body data=<QL>
// Returns parsed Response with typed Element entries (nodes/ways with tags).
package overpass

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/httperr"
)

const (
	defaultEndpoint  = "https://overpass-api.de/api/interpreter"
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
)

// TagFilter selects elements by an OSM tag. Empty Value means "key exists",
// emitted as [<key>].
type TagFilter struct {
	Key   string
	Value string
}

// Centroid is the lat/lon center reported by Overpass for non-node elements
// (ways, relations) when out center is requested.
type Centroid struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Element is a single OSM feature returned by the Overpass interpreter.
type Element struct {
	ID     int64             `json:"id"`
	Type   string            `json:"type"`
	Lat    float64           `json:"lat,omitempty"`
	Lon    float64           `json:"lon,omitempty"`
	Center *Centroid         `json:"center,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// LatLng resolves the element's effective coordinate; ways/relations fall back
// to Center when no top-level lat/lon was emitted.
func (e *Element) LatLng() (float64, float64) {
	if e.Type == "node" {
		return e.Lat, e.Lon
	}
	if e.Center != nil {
		return e.Center.Lat, e.Center.Lon
	}
	return e.Lat, e.Lon
}

// Response is the top-level Overpass JSON envelope.
type Response struct {
	Elements []Element `json:"elements"`
}

// Client talks to the Overpass interpreter. Endpoint is overridable for tests.
type Client struct {
	HTTPClient         *http.Client
	UserAgent          string
	Endpoint           string
	RateLimitPerSecond float64

	mu       sync.Mutex
	lastSent time.Time
}

// New constructs a Client with the given http client and user agent. Either
// argument may be zero-valued and a sensible default is substituted.
func New(httpClient *http.Client, userAgent string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	return &Client{
		HTTPClient: httpClient,
		UserAgent:  userAgent,
		Endpoint:   defaultEndpoint,
	}
}

// throttle blocks until the next request slot is available given the configured
// rate. It is a no-op when RateLimitPerSecond <= 0.
func (c *Client) throttle(ctx context.Context) error {
	if c.RateLimitPerSecond <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	gap := time.Duration(float64(time.Second) / c.RateLimitPerSecond)
	wait := time.Until(c.lastSent.Add(gap))
	if wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.lastSent = time.Now()
	return nil
}

// Query sends an Overpass QL string and returns the parsed Response.
func (c *Client) Query(ctx context.Context, ql string) (*Response, error) {
	if err := c.throttle(ctx); err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("data", ql)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("overpass request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("overpass read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d: %s", c.Endpoint, resp.StatusCode, httperr.Snippet(body))
	}

	var out Response
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("overpass parse json: %w", err)
	}
	return &out, nil
}

// NearbyByTags is a convenience wrapper that builds an Overpass QL query for
// nodes and ways within radiusMeters of (lat, lng) matching any of the given
// TagFilters, then executes it.
func (c *Client) NearbyByTags(ctx context.Context, lat, lng float64, radiusMeters int, tagFilters []TagFilter) (*Response, error) {
	ql := buildNearbyQL(lat, lng, radiusMeters, tagFilters)
	return c.Query(ctx, ql)
}

// buildNearbyQL assembles the multi-statement Overpass QL string used by
// NearbyByTags. Exposed as an unexported function so tests can inspect it.
func buildNearbyQL(lat, lng float64, radiusMeters int, tagFilters []TagFilter) string {
	var b strings.Builder
	b.WriteString("[out:json][timeout:25];\n(\n")
	for _, tf := range tagFilters {
		var sel string
		if tf.Value == "" {
			sel = fmt.Sprintf("[%s]", tf.Key)
		} else {
			sel = fmt.Sprintf("[%s=%s]", tf.Key, tf.Value)
		}
		fmt.Fprintf(&b, "  node%s(around:%d,%f,%f);\n", sel, radiusMeters, lat, lng)
		fmt.Fprintf(&b, "  way%s(around:%d,%f,%f);\n", sel, radiusMeters, lat, lng)
	}
	b.WriteString(");\nout center tags 50;\n")
	return b.String()
}
