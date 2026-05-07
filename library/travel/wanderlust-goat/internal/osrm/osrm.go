// Package osrm is a thin client for the public OSRM walking-router service.
//
// Endpoint shape:
//
//	GET <base>/route/v1/foot/<from_lng>,<from_lat>;<to_lng>,<to_lat>?...
package osrm

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
	defaultBaseURL   = "https://router.project-osrm.org"
	defaultUserAgent = "wanderlust-goat-pp-cli/0.1 (+https://github.com/mvanhorn/printing-press-library)"
	defaultTimeout   = 10 * time.Second
)

// RouteResult is the basic distance/duration pair for a walking route.
type RouteResult struct {
	DistanceMeters  float64
	DurationSeconds float64
	WalkingMinutes  float64
}

// PolylineResult adds the GeoJSON LineString coordinates ([[lng, lat], ...]).
type PolylineResult struct {
	RouteResult
	Coords [][2]float64
}

// Client talks to an OSRM-compatible server.
type Client struct {
	BaseURL            string
	HTTPClient         *http.Client
	UserAgent          string
	RateLimitPerSecond float64

	mu       sync.Mutex
	lastSent time.Time
}

// New constructs a Client. Empty baseURL falls back to the public OSRM demo
// server; nil httpClient uses a sensible default; empty ua uses the project
// default.
func New(baseURL string, httpClient *http.Client, ua string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if ua == "" {
		ua = defaultUserAgent
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: httpClient,
		UserAgent:  ua,
	}
}

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

// osrmGeometry is the GeoJSON LineString shape OSRM returns when geometries=geojson.
type osrmGeometry struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

type osrmRoute struct {
	Distance float64      `json:"distance"`
	Duration float64      `json:"duration"`
	Geometry osrmGeometry `json:"geometry"`
}

type osrmResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message,omitempty"`
	Routes  []osrmRoute `json:"routes"`
}

// fetch issues the GET and decodes the OSRM JSON envelope, returning a useful
// error containing the URL and status code on non-200 responses.
func (c *Client) fetch(ctx context.Context, fullURL string) (*osrmResponse, error) {
	if err := c.throttle(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("osrm request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("osrm read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d: %s", fullURL, resp.StatusCode, httperr.Snippet(body))
	}
	var out osrmResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("osrm parse json: %w", err)
	}
	if out.Code != "Ok" {
		return nil, fmt.Errorf("%s returned code=%q message=%q", fullURL, out.Code, out.Message)
	}
	if len(out.Routes) == 0 {
		return nil, fmt.Errorf("%s returned no routes", fullURL)
	}
	return &out, nil
}

// buildURL assembles an OSRM walking-route URL with the given query overrides.
func (c *Client) buildURL(fromLat, fromLng, toLat, toLng float64, q url.Values) string {
	path := fmt.Sprintf("%s/route/v1/foot/%f,%f;%f,%f", c.BaseURL, fromLng, fromLat, toLng, toLat)
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

// WalkingRoute returns distance + duration only (overview=false) for caching efficiency.
func (c *Client) WalkingRoute(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (*RouteResult, error) {
	q := url.Values{}
	q.Set("overview", "false")
	q.Set("geometries", "geojson")
	full := c.buildURL(fromLat, fromLng, toLat, toLng, q)

	resp, err := c.fetch(ctx, full)
	if err != nil {
		return nil, err
	}
	r := resp.Routes[0]
	return &RouteResult{
		DistanceMeters:  r.Distance,
		DurationSeconds: r.Duration,
		WalkingMinutes:  r.Duration / 60.0,
	}, nil
}

// WalkingPolyline returns the full GeoJSON polyline for the walking route.
func (c *Client) WalkingPolyline(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (*PolylineResult, error) {
	q := url.Values{}
	q.Set("overview", "full")
	q.Set("geometries", "geojson")
	full := c.buildURL(fromLat, fromLng, toLat, toLng, q)

	resp, err := c.fetch(ctx, full)
	if err != nil {
		return nil, err
	}
	r := resp.Routes[0]
	return &PolylineResult{
		RouteResult: RouteResult{
			DistanceMeters:  r.Distance,
			DurationSeconds: r.Duration,
			WalkingMinutes:  r.Duration / 60.0,
		},
		Coords: r.Geometry.Coordinates,
	}, nil
}
