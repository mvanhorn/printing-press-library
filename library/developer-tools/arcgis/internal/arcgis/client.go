// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.

// Package arcgis is a client for the ArcGIS REST query protocol. Unlike the
// generated internal/client, every call targets an arbitrary FeatureServer /
// MapServer / layer URL supplied at runtime, because ArcGIS REST is a uniform
// protocol over per-host endpoints rather than a single fixed base API.
package arcgis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Client issues ArcGIS REST requests. Token is optional; when set it is added
// to every request as ?token= for secured layers. Limiter paces outbound
// requests and adapts to 429s; nil disables pacing (its methods no-op).
type Client struct {
	HTTP    *http.Client
	Token   string
	Limiter *cliutil.AdaptiveLimiter
}

// New returns a Client with the given timeout and optional token. ratePerSec
// caps outbound requests per second (0 disables pacing); the limiter halves on
// a 429 and ramps back up on sustained success.
func New(timeout time.Duration, token string, ratePerSec float64) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		HTTP:    &http.Client{Timeout: timeout},
		Token:   token,
		Limiter: cliutil.NewAdaptiveLimiter(ratePerSec),
	}
}

// Field is one attribute column on a layer.
type Field struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Alias  string `json:"alias"`
	Length int    `json:"length"`
}

// LayerRef is a layer entry in a service's metadata.
type LayerRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ServiceInfo is a FeatureServer/MapServer's top-level metadata.
type ServiceInfo struct {
	CurrentVersion float64    `json:"currentVersion"`
	ServiceDesc    string     `json:"serviceDescription"`
	MaxRecordCount int        `json:"maxRecordCount"`
	Layers         []LayerRef `json:"layers"`
	Tables         []LayerRef `json:"tables"`
}

// LayerInfo is a single layer's metadata.
type LayerInfo struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	GeometryType   string  `json:"geometryType"`
	MaxRecordCount int     `json:"maxRecordCount"`
	ObjectIDField  string  `json:"objectIdField"`
	Fields         []Field `json:"fields"`
	Extent         *Extent `json:"extent"`
	Advanced       struct {
		SupportsPagination bool `json:"supportsPagination"`
	} `json:"advancedQueryCapabilities"`
}

// Extent is a bounding box with a spatial reference.
type Extent struct {
	XMin    float64          `json:"xmin"`
	YMin    float64          `json:"ymin"`
	XMax    float64          `json:"xmax"`
	YMax    float64          `json:"ymax"`
	Spatial *json.RawMessage `json:"spatialReference"`
}

// Feature is one queried record: raw attributes plus raw esri geometry.
type Feature struct {
	Attributes map[string]any  `json:"attributes"`
	Geometry   json.RawMessage `json:"geometry,omitempty"`
}

// OID returns the OBJECTID value for the feature under the given field, or
// (0,false) when absent or non-numeric.
func (f Feature) OID(field string) (int64, bool) {
	if field == "" {
		field = "OBJECTID"
	}
	v, ok := f.Attributes[field]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		return i, err == nil
	}
	return 0, false
}

// QueryResult is one page of a /query response.
type QueryResult struct {
	Fields        []Field   `json:"fields"`
	GeometryType  string    `json:"geometryType"`
	Features      []Feature `json:"features"`
	ExceededLimit bool      `json:"exceededTransferLimit"`
	ObjectIDField string    `json:"objectIdFieldName"`
	Error         *apiError `json:"error"`
}

type apiError struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details"`
}

func (e *apiError) String() string {
	if e == nil {
		return ""
	}
	if len(e.Details) > 0 {
		return fmt.Sprintf("ArcGIS error %d: %s (%s)", e.Code, e.Message, strings.Join(e.Details, "; "))
	}
	return fmt.Sprintf("ArcGIS error %d: %s", e.Code, e.Message)
}

var trailingQuery = regexp.MustCompile(`(?i)/query/?$`)

// NormalizeLayerURL trims a trailing /query and any trailing slash so callers
// can paste either a layer URL or a full query URL.
func NormalizeLayerURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = trailingQuery.ReplaceAllString(raw, "")
	return strings.TrimRight(raw, "/")
}

func (c *Client) getJSON(ctx context.Context, endpoint string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	if params.Get("f") == "" {
		params.Set("f", "json")
	}
	if c.Token != "" && params.Get("token") == "" {
		params.Set("token", c.Token)
	}
	full := endpoint
	if strings.Contains(full, "?") {
		full += "&" + params.Encode()
	} else {
		full += "?" + params.Encode()
	}
	// Bounded retry loop with typed 429 handling. Empty-on-throttle is
	// indistinguishable from "no rows match", so a throttled request that
	// exhausts retries returns a typed *cliutil.RateLimitError instead of an
	// empty result.
	const maxRateLimitRetries = 3
	var body []byte
	for attempt := 0; ; attempt++ {
		c.Limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			wait := cliutil.RetryAfter(resp)
			limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
			_ = resp.Body.Close()
			c.Limiter.OnRateLimit()
			if attempt >= maxRateLimitRetries {
				return &cliutil.RateLimitError{URL: endpoint, RetryAfter: wait, Body: snippet(limited)}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, endpoint, snippet(b))
		}
		c.Limiter.OnSuccess()
		body = b
		break
	}
	// ArcGIS returns HTTP 200 with an {"error":{...}} body for many failures
	// (invalid layer, token required, bad params). Surface those explicitly
	// rather than decoding into an empty struct.
	var probe struct {
		Error *apiError `json:"error"`
	}
	if json.Unmarshal(body, &probe) == nil && probe.Error != nil {
		return fmt.Errorf("%s (from %s)", probe.Error.String(), endpoint)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding response from %s: %w (body: %s)", endpoint, err, snippet(body))
	}
	return nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

// ServiceMeta fetches a FeatureServer/MapServer's metadata.
func (c *Client) ServiceMeta(ctx context.Context, serviceURL string) (*ServiceInfo, error) {
	var info ServiceInfo
	if err := c.getJSON(ctx, NormalizeLayerURL(serviceURL), nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// LayerMeta fetches a single layer's metadata.
func (c *Client) LayerMeta(ctx context.Context, layerURL string) (*LayerInfo, error) {
	var info LayerInfo
	if err := c.getJSON(ctx, NormalizeLayerURL(layerURL), nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Count returns the number of features matching where (default "1=1").
func (c *Client) Count(ctx context.Context, layerURL, where string) (int, error) {
	if where == "" {
		where = "1=1"
	}
	p := url.Values{}
	p.Set("where", where)
	p.Set("returnCountOnly", "true")
	var res struct {
		Count int       `json:"count"`
		Error *apiError `json:"error"`
	}
	if err := c.getJSON(ctx, NormalizeLayerURL(layerURL)+"/query", p, &res); err != nil {
		return 0, err
	}
	if res.Error != nil {
		return 0, fmt.Errorf("%s", res.Error.String())
	}
	return res.Count, nil
}

// QueryOptions controls a query.
type QueryOptions struct {
	Where        string
	OutFields    string // default "*"
	OutSR        string // default "4326"
	ReturnGeom   bool
	OrderBy      string
	Geometry     string // esri geometry JSON or "xmin,ymin,xmax,ymax" for envelope
	GeometryType string // e.g. esriGeometryEnvelope, esriGeometryPoint
	SpatialRel   string // default esriSpatialRelIntersects when Geometry set
	ResultOffset int
	ResultCount  int
}

func (o QueryOptions) values() url.Values {
	p := url.Values{}
	where := o.Where
	if where == "" {
		where = "1=1"
	}
	p.Set("where", where)
	of := o.OutFields
	if of == "" {
		of = "*"
	}
	p.Set("outFields", of)
	sr := o.OutSR
	if sr == "" {
		sr = "4326"
	}
	p.Set("outSR", sr)
	if o.ReturnGeom {
		p.Set("returnGeometry", "true")
	} else {
		p.Set("returnGeometry", "false")
	}
	if o.OrderBy != "" {
		p.Set("orderByFields", o.OrderBy)
	}
	if o.Geometry != "" {
		p.Set("geometry", o.Geometry)
		gt := o.GeometryType
		if gt == "" {
			gt = "esriGeometryEnvelope"
		}
		p.Set("geometryType", gt)
		sr := o.SpatialRel
		if sr == "" {
			sr = "esriSpatialRelIntersects"
		}
		p.Set("spatialRel", sr)
		p.Set("inSR", "4326")
	}
	if o.ResultCount > 0 {
		p.Set("resultRecordCount", strconv.Itoa(o.ResultCount))
	}
	if o.ResultOffset > 0 {
		p.Set("resultOffset", strconv.Itoa(o.ResultOffset))
	}
	return p
}

// QueryPage runs one /query request and returns the parsed result.
func (c *Client) QueryPage(ctx context.Context, layerURL string, o QueryOptions) (*QueryResult, error) {
	var res QueryResult
	if err := c.getJSON(ctx, NormalizeLayerURL(layerURL)+"/query", o.values(), &res); err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, fmt.Errorf("%s", res.Error.String())
	}
	return &res, nil
}

// QueryRaw runs a /query with caller-supplied params (used by stats for
// outStatistics). f=json and the token are added automatically.
func (c *Client) QueryRaw(ctx context.Context, layerURL string, params url.Values) (*QueryResult, error) {
	var res QueryResult
	if err := c.getJSON(ctx, NormalizeLayerURL(layerURL)+"/query", params, &res); err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, fmt.Errorf("%s", res.Error.String())
	}
	return &res, nil
}

// IDs returns all OBJECTIDs matching where via returnIdsOnly (used for the OID
// chunking fallback on servers without resultOffset paging).
func (c *Client) IDs(ctx context.Context, layerURL, where string) ([]int64, string, error) {
	if where == "" {
		where = "1=1"
	}
	p := url.Values{}
	p.Set("where", where)
	p.Set("returnIdsOnly", "true")
	var res struct {
		ObjectIDField string    `json:"objectIdFieldName"`
		ObjectIDs     []int64   `json:"objectIds"`
		Error         *apiError `json:"error"`
	}
	if err := c.getJSON(ctx, NormalizeLayerURL(layerURL)+"/query", p, &res); err != nil {
		return nil, "", err
	}
	if res.Error != nil {
		return nil, "", fmt.Errorf("%s", res.Error.String())
	}
	return res.ObjectIDs, res.ObjectIDField, nil
}
