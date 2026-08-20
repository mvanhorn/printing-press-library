// Copyright 2026 ghltshubh and contributors. Licensed under Apache-2.0. See LICENSE.

// Package cli shared STAC helpers used by the hand-authored novel commands
// (scenes best, clouds, coverage, timeline, gaps, watch, assets, compare,
// stack-snippet). These wrap the generated client/store with STAC-specific
// request shaping and response parsing. Kept in a standalone file (no
// generated header) so regen preserves it whole.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/maps/stac/internal/client"
)

// parseBBox parses a "west,south,east,north" (4) or 3D (6) comma-separated
// bounding box into floats, with an error that names the expected shape.
func parseBBox(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("--bbox is empty; expected west,south,east,north")
	}
	parts := strings.Split(s, ",")
	if len(parts) != 4 && len(parts) != 6 {
		return nil, fmt.Errorf("--bbox must be 4 (west,south,east,north) or 6 comma-separated numbers, got %d", len(parts))
	}
	out := make([]float64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("--bbox value %q is not a number (expected west,south,east,north)", strings.TrimSpace(p))
		}
		out[i] = v
	}
	return out, nil
}

// bboxToQuery renders a bbox slice as the comma-joined string the /aggregate
// query parameter expects.
func bboxToQuery(b []float64) string {
	ss := make([]string, len(b))
	for i, v := range b {
		ss[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strings.Join(ss, ",")
}

// buildSearchBody assembles a POST /search body. Cloud-cover bounds use the
// STAC `query` extension (the filter mechanism Earth Search supports; CQL2 is
// silently ignored there). Sort uses the object form `{field,direction}` — the
// `+`/`-` string shorthand returns HTTP 400 on stac-server.
func buildSearchBody(collections []string, bbox []float64, datetime string, maxCloud, minCloud *float64, limit int, sortCloudAsc bool) map[string]any {
	body := map[string]any{}
	if len(collections) > 0 {
		body["collections"] = collections
	}
	if len(bbox) > 0 {
		body["bbox"] = bbox
	}
	if datetime != "" {
		body["datetime"] = normalizeDatetime(datetime)
	}
	if limit > 0 {
		body["limit"] = limit
	}
	cc := map[string]any{}
	if maxCloud != nil {
		cc["lt"] = *maxCloud
	}
	if minCloud != nil {
		cc["gte"] = *minCloud
	}
	if len(cc) > 0 {
		body["query"] = map[string]any{"eo:cloud_cover": cc}
	}
	if sortCloudAsc {
		body["sortby"] = []map[string]any{{"field": "properties.eo:cloud_cover", "direction": "asc"}}
	}
	return body
}

// commonBandNames is the set of Sentinel-2/Landsat band common-names that
// appear as asset keys on Earth Search items.
var commonBandNames = map[string]bool{
	"coastal": true, "blue": true, "green": true, "red": true,
	"rededge1": true, "rededge2": true, "rededge3": true,
	"nir": true, "nir08": true, "nir09": true,
	"swir16": true, "swir22": true, "visual": true, "scl": true, "thumbnail": true,
}

// bandAssetKey maps a band common-name to its asset key. The COG asset uses the
// bare common name; its JPEG2000 twin appends "-jp2".
func bandAssetKey(common string, includeJP2 bool) string {
	if includeJP2 {
		return common + "-jp2"
	}
	return common
}

type resolvedAsset struct {
	Band  string   `json:"band"`
	Key   string   `json:"key"`
	Href  string   `json:"href"`
	Type  string   `json:"type"`
	Roles []string `json:"roles"`
}

func containsRole(roles []string, want string) bool {
	for _, r := range roles {
		if strings.EqualFold(r, want) {
			return true
		}
	}
	return false
}

// resolveAssets resolves requested bands (or, when bands is empty, every COG
// asset) against an item's assets map. COG variants are preferred over their
// `-jp2` twins unless includeJP2 is set. roleFilter, when non-empty, keeps only
// assets carrying that role.
func resolveAssets(assets map[string]json.RawMessage, bands []string, roleFilter string, includeJP2 bool) []resolvedAsset {
	type assetObj struct {
		Href  string   `json:"href"`
		Type  string   `json:"type"`
		Roles []string `json:"roles"`
	}
	out := []resolvedAsset{}
	pick := func(band, key string) bool {
		raw, ok := assets[key]
		if !ok {
			return false
		}
		var a assetObj
		if err := json.Unmarshal(raw, &a); err != nil {
			return false
		}
		if roleFilter != "" && !containsRole(a.Roles, roleFilter) {
			return false
		}
		out = append(out, resolvedAsset{Band: band, Key: key, Href: a.Href, Type: a.Type, Roles: a.Roles})
		return true
	}
	if len(bands) > 0 {
		for _, b := range bands {
			b = strings.TrimSpace(b)
			if b == "" {
				continue
			}
			if !pick(b, bandAssetKey(b, false)) && includeJP2 {
				pick(b, bandAssetKey(b, true))
			}
		}
		return out
	}
	keys := make([]string, 0, len(assets))
	for k := range assets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.HasSuffix(k, "-jp2") && !includeJP2 {
			continue
		}
		pick(strings.TrimSuffix(k, "-jp2"), k)
	}
	return out
}

// stacLink is a STAC link object, including the POST `body` carried by the
// `rel:next` pagination link.
type stacLink struct {
	Rel    string          `json:"rel"`
	Href   string          `json:"href"`
	Method string          `json:"method"`
	Body   json.RawMessage `json:"body"`
}

// stacFeature is the subset of a STAC item the novel commands read.
type stacFeature struct {
	ID         string                     `json:"id"`
	Collection string                     `json:"collection"`
	Properties map[string]json.RawMessage `json:"properties"`
	Assets     map[string]json.RawMessage `json:"assets"`
	Links      []stacLink                 `json:"links"`
}

func (f *stacFeature) stringProp(key string) string {
	if raw, ok := f.Properties[key]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	}
	return ""
}

func (f *stacFeature) datetime() string { return f.stringProp("datetime") }

// dateOnly returns the YYYY-MM-DD portion of the item datetime.
func (f *stacFeature) dateOnly() string {
	dt := f.datetime()
	if len(dt) >= 10 {
		return dt[:10]
	}
	return dt
}

func (f *stacFeature) cloud() (float64, bool) {
	if raw, ok := f.Properties["eo:cloud_cover"]; ok {
		var v float64
		if json.Unmarshal(raw, &v) == nil {
			return v, true
		}
	}
	return 0, false
}

func (f *stacFeature) relOrbit() (int, bool) {
	if raw, ok := f.Properties["sat:relative_orbit"]; ok {
		var v int
		if json.Unmarshal(raw, &v) == nil {
			return v, true
		}
	}
	return 0, false
}

func (f *stacFeature) gridCode() string { return f.stringProp("grid:code") }

// selfHref returns the item's canonical self/canonical link href, falling back
// to a constructed items path.
func (f *stacFeature) selfHref(baseURL string) string {
	for _, l := range f.Links {
		if l.Rel == "self" || l.Rel == "canonical" {
			if l.Href != "" {
				return l.Href
			}
		}
	}
	if f.Collection != "" && f.ID != "" {
		return strings.TrimRight(baseURL, "/") + "/collections/" + f.Collection + "/items/" + f.ID
	}
	return ""
}

func parseFeatures(raw []json.RawMessage) []stacFeature {
	out := make([]stacFeature, 0, len(raw))
	for _, r := range raw {
		var f stacFeature
		if json.Unmarshal(r, &f) == nil {
			out = append(out, f)
		}
	}
	return out
}

type searchResponse struct {
	Features      []json.RawMessage `json:"features"`
	NumberMatched int               `json:"numberMatched"`
	Links         []stacLink        `json:"links"`
}

// stacSearch performs one POST /search call and returns the raw features, the
// numberMatched count, and the next-page POST body (nil when there is no next
// link). PostQueryWithParams routes through the client's read path so the
// verify mutation gate does not intercept this read-shaped POST.
func stacSearch(ctx context.Context, c *client.Client, body map[string]any) ([]json.RawMessage, int, map[string]any, error) {
	data, status, err := c.PostQueryWithParams(ctx, "/search", nil, body)
	if err != nil {
		return nil, 0, nil, err
	}
	if status >= 400 {
		return nil, 0, nil, fmt.Errorf("search returned HTTP %d", status)
	}
	var resp searchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, 0, nil, fmt.Errorf("parsing search response: %w", err)
	}
	var next map[string]any
	for _, l := range resp.Links {
		if l.Rel == "next" && len(l.Body) > 0 {
			_ = json.Unmarshal(l.Body, &next)
			break
		}
	}
	return resp.Features, resp.NumberMatched, next, nil
}

// stacSearchPaged auto-paginates POST /search by replaying the next-page body,
// bounded by maxPages. It returns accumulated features, the first page's
// numberMatched, and the number of pages actually scanned. A mid-walk error
// returns the partial result rather than failing the whole command.
func stacSearchPaged(ctx context.Context, c *client.Client, body map[string]any, maxPages int) ([]json.RawMessage, int, int, error) {
	if maxPages < 1 {
		maxPages = 1
	}
	var all []json.RawMessage
	matched, pages := 0, 0
	cur := body
	for pages < maxPages {
		feats, m, next, err := stacSearch(ctx, c, cur)
		if err != nil {
			if pages == 0 {
				return nil, 0, pages, err
			}
			break
		}
		pages++
		if pages == 1 {
			matched = m
		}
		all = append(all, feats...)
		if next == nil || len(feats) == 0 {
			break
		}
		cur = next
	}
	return all, matched, pages, nil
}

type aggBucket struct {
	Key       string   `json:"key"`
	Frequency int      `json:"frequency"`
	From      *float64 `json:"from,omitempty"`
	To        *float64 `json:"to,omitempty"`
}

type aggResult struct {
	Name     string          `json:"name"`
	DataType string          `json:"data_type"`
	Buckets  []aggBucket     `json:"buckets"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type aggregateResponse struct {
	Aggregations []aggResult `json:"aggregations"`
}

// stacAggregate calls GET /aggregate with the given filters and aggregation
// names, returning the parsed aggregation results.
func stacAggregate(ctx context.Context, c *client.Client, collection string, bbox []float64, datetime string, aggs []string) ([]aggResult, error) {
	params := map[string]string{}
	if collection != "" {
		params["collections"] = collection
	}
	if len(bbox) > 0 {
		params["bbox"] = bboxToQuery(bbox)
	}
	if datetime != "" {
		params["datetime"] = normalizeDatetime(datetime)
	}
	if len(aggs) > 0 {
		params["aggregations"] = strings.Join(aggs, ",")
	}
	data, err := c.Get(ctx, "/aggregate", params)
	if err != nil {
		return nil, err
	}
	var resp aggregateResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing aggregate response: %w", err)
	}
	return resp.Aggregations, nil
}

// findAgg returns the aggregation result with the given name.
func findAgg(aggs []aggResult, name string) (aggResult, bool) {
	for _, a := range aggs {
		if a.Name == name {
			return a, true
		}
	}
	return aggResult{}, false
}

// scalarAgg extracts a scalar aggregation value (e.g. datetime_min) as a string.
func scalarAgg(aggs []aggResult, name string) string {
	a, ok := findAgg(aggs, name)
	if !ok || len(a.Value) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(a.Value, &s) == nil {
		return s
	}
	return strings.Trim(string(a.Value), `"`)
}

// emitNovel writes a novel command's view value: JSON (filtered by --select/
// --compact) for machine consumers, indented JSON for an interactive terminal.
func emitNovel(out io.Writer, flags *rootFlags, terminal bool, view any) error {
	if flags.asJSON || !terminal {
		return printJSONFiltered(out, view, flags)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(view)
}

// splitCSV splits a comma-separated flag value into trimmed, non-empty tokens.
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// normalizeDatetime expands date-only inputs to the RFC3339 datetimes
// stac-server requires. A single "YYYY-MM-DD" becomes start-of-day; an interval
// "A/B" expands A to start-of-day and B to end-of-day. Open ends (".."), empty
// sides, and values already carrying a time component pass through unchanged.
func normalizeDatetime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	expand := func(part string, end bool) string {
		part = strings.TrimSpace(part)
		if part == "" || part == ".." {
			return part
		}
		if len(part) == 10 && !strings.Contains(part, "T") {
			if end {
				return part + "T23:59:59Z"
			}
			return part + "T00:00:00Z"
		}
		return part
	}
	if i := strings.Index(s, "/"); i >= 0 {
		return expand(s[:i], false) + "/" + expand(s[i+1:], true)
	}
	return expand(s, false)
}

// jsonRawOrString returns the input as a json.RawMessage when it parses as
// valid JSON (e.g. a GeoJSON geometry), otherwise the raw string.
func jsonRawOrString(s string) any {
	trimmed := strings.TrimSpace(s)
	if json.Valid([]byte(trimmed)) && (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
		return json.RawMessage(trimmed)
	}
	return s
}
