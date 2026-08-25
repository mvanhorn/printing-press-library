// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

// Package gtrends implements the undocumented Google Trends explore ->
// widget-token flow (see package-level notes in trending.go for the
// separate, far less stable "Trending Now" surface). There is no official
// Google Trends API; every shape here was reconstructed from browser-sniffed
// traffic and is subject to change without notice.
package gtrends

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/client"
)

// DefaultHL and DefaultTZ are the language/timezone parameters sent with
// every explore/widget request when the caller doesn't need to override
// them. tz=0 (UTC) keeps date bucketing predictable for the local-store
// commands that key on calendar dates.
const (
	DefaultHL = "en-US"
	DefaultTZ = "0"
)

// breakoutThreshold is Google's sentinel for the "rising" related-search
// list: a value >= this means the live UI would show "Breakout" (>5000%
// growth) instead of a literal percentage.
const breakoutThreshold = 5000

// Widget is one entry from an explore() response's "widgets" array. Request
// is kept as raw JSON and echoed back verbatim to the matching widgetdata
// endpoint, exactly as the live UI does.
type Widget struct {
	ID      string          `json:"id"`
	Token   string          `json:"token"`
	Request json.RawMessage `json:"request"`
}

// ExploreResult is the parsed response of POST /trends/api/explore.
type ExploreResult struct {
	Widgets []Widget `json:"widgets"`
}

// FindWidget returns the first widget with the given id (e.g. "TIMESERIES",
// "GEO_MAP", "RELATED_QUERIES", "RELATED_TOPICS").
func FindWidget(widgets []Widget, id string) (Widget, bool) {
	for _, w := range widgets {
		if w.ID == id {
			return w, true
		}
	}
	return Widget{}, false
}

// InterestPoint is one row of interest-over-time data. Values is per-keyword
// when the explore() call compared multiple keywords (parallel to the
// comparisonItem order), so Values[i] corresponds to the i-th keyword passed
// to Explore.
type InterestPoint struct {
	Time          time.Time `json:"time"`
	FormattedTime string    `json:"formatted_time"`
	Values        []int     `json:"values"`
}

// RegionInterest is one row of interest-by-region data. Values is per-keyword,
// parallel to the comparisonItem order (same convention as InterestPoint).
type RegionInterest struct {
	GeoCode string `json:"geo_code"`
	GeoName string `json:"geo_name"`
	Values  []int  `json:"values"`
}

// RelatedTerm is one row from the related-queries/related-topics widget.
// IsBreakout is only meaningful for rising-list terms; Google represents an
// unbounded ("Breakout") rise as a large sentinel value (typically 5000)
// rather than a literal percentage.
type RelatedTerm struct {
	Query      string `json:"query"`
	Value      int    `json:"value"`
	IsBreakout bool   `json:"is_breakout"`
}

// stripXSSIPrefix trims Google's ")]}'" anti-hijacking prefix (and any
// leading whitespace/newline after it) when present. internal/client already
// strips this on the live request path (sanitizeJSONResponse), so in
// production this is a no-op; it exists here so the pure parse functions
// below can be unit-tested directly against XSSI-prefixed fixtures and stay
// correct if ever fed a response that bypassed the client layer.
// Widget and picker responses use a 5-byte prefix that includes the comma
// (")]}',"), not just the 4-byte ")]}'" explore() uses — the longer/comma
// variants must be checked first, since ")]}'" is itself a byte-prefix of
// ")]}'," and would otherwise match first and leave a stray leading comma
// that breaks json.Unmarshal ("invalid character ',' looking for beginning
// of value").
func stripXSSIPrefix(body []byte) []byte {
	for _, p := range [][]byte{
		[]byte(")]}',\n"),
		[]byte(")]}',"),
		[]byte(")]}'\n"),
		[]byte(")]}'"),
	} {
		if bytes.HasPrefix(body, p) {
			return bytes.TrimLeft(bytes.TrimPrefix(body, p), " \t\r\n")
		}
	}
	return body
}

type comparisonItem struct {
	Keyword string `json:"keyword"`
	Geo     string `json:"geo"`
	Time    string `json:"time"`
}

type exploreRequestBody struct {
	ComparisonItem []comparisonItem `json:"comparisonItem"`
	Category       int              `json:"category"`
	Property       string           `json:"property"`
}

// buildExploreRequestJSON builds the JSON string sent as the explore()
// request's "req" form field. geos may be empty (worldwide for every
// keyword), length 1 (applied to every keyword), or the same length as
// keywords (one geo per keyword).
func buildExploreRequestJSON(keywords, geos []string, timeframe string, category int, property string) ([]byte, error) {
	if len(keywords) == 0 {
		return nil, fmt.Errorf("gtrends: at least one keyword is required")
	}
	if len(keywords) > 5 {
		return nil, fmt.Errorf("gtrends: at most 5 keywords are supported (comparisonItem limit), got %d", len(keywords))
	}
	if len(geos) != 0 && len(geos) != 1 && len(geos) != len(keywords) {
		return nil, fmt.Errorf("gtrends: geos must have length 0, 1, or match keywords (%d), got %d", len(keywords), len(geos))
	}
	items := make([]comparisonItem, len(keywords))
	for i, kw := range keywords {
		geo := ""
		switch {
		case len(geos) == 1:
			geo = geos[0]
		case len(geos) == len(keywords):
			geo = geos[i]
		}
		items[i] = comparisonItem{Keyword: kw, Geo: geo, Time: timeframe}
	}
	return json.Marshal(exploreRequestBody{ComparisonItem: items, Category: category, Property: property})
}

// parseExplore is the pure-parsing half of Explore, split out so it can be
// unit-tested against fixture JSON without a network round-trip.
func parseExplore(body []byte) (*ExploreResult, error) {
	body = stripXSSIPrefix(body)
	var result ExploreResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("gtrends: parsing explore response: %w", err)
	}
	if result.Widgets == nil {
		result.Widgets = make([]Widget, 0)
	}
	return &result, nil
}

// Explore requests per-widget tokens for a comparison of up to 5 keywords.
// keywords and geos are parallel slices (geos may be shorter — see
// buildExploreRequestJSON). category and property mirror the explore()
// request body's own fields (property: "" for web, or "images", "news",
// "youtube", "froogle").
func Explore(ctx context.Context, c *client.Client, keywords []string, geos []string, timeframe string, category int, property string) (*ExploreResult, error) {
	reqJSON, err := buildExploreRequestJSON(keywords, geos, timeframe, category, property)
	if err != nil {
		return nil, err
	}
	params := map[string]string{"hl": DefaultHL, "tz": DefaultTZ, "req": string(reqJSON)}
	data, _, err := c.PostWithParams(ctx, "/trends/api/explore", params, nil)
	if err != nil {
		return nil, fmt.Errorf("gtrends: explore request failed: %w", err)
	}
	return parseExplore(data)
}

// widgetParams builds the shared hl/tz/token/req query params every
// widgetdata/* endpoint takes, from a Widget returned by Explore.
func widgetParams(widget Widget) (map[string]string, error) {
	if widget.Token == "" {
		return nil, fmt.Errorf("gtrends: widget %q has no token", widget.ID)
	}
	if len(widget.Request) == 0 {
		return nil, fmt.Errorf("gtrends: widget %q has no request payload", widget.ID)
	}
	return map[string]string{
		"hl":    DefaultHL,
		"tz":    DefaultTZ,
		"token": widget.Token,
		"req":   string(widget.Request),
	}, nil
}

type multilineResponse struct {
	Default struct {
		TimelineData []struct {
			Time          string `json:"time"`
			FormattedTime string `json:"formattedTime"`
			Value         []int  `json:"value"`
		} `json:"timelineData"`
	} `json:"default"`
}

// parseMultiline is the pure-parsing half of InterestOverTime.
func parseMultiline(body []byte) ([]InterestPoint, error) {
	body = stripXSSIPrefix(body)
	var parsed multilineResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("gtrends: parsing multiline response: %w", err)
	}
	points := make([]InterestPoint, 0, len(parsed.Default.TimelineData))
	for _, tp := range parsed.Default.TimelineData {
		sec, err := strconv.ParseInt(tp.Time, 10, 64)
		if err != nil {
			// Malformed row (non-numeric time). Skip rather than fail the
			// whole batch — one bad row shouldn't lose an entire year of
			// interest-over-time data.
			continue
		}
		points = append(points, InterestPoint{
			Time:          time.Unix(sec, 0).UTC(),
			FormattedTime: tp.FormattedTime,
			Values:        tp.Value,
		})
	}
	return points, nil
}

// InterestOverTime fetches and parses the TIMESERIES widget's data.
func InterestOverTime(ctx context.Context, c *client.Client, widget Widget) ([]InterestPoint, error) {
	params, err := widgetParams(widget)
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/trends/api/widgetdata/multiline", params)
	if err != nil {
		return nil, fmt.Errorf("gtrends: multiline request failed: %w", err)
	}
	return parseMultiline(data)
}

type comparedgeoResponse struct {
	Default struct {
		GeoMapData []struct {
			GeoCode string `json:"geoCode"`
			GeoName string `json:"geoName"`
			Value   []int  `json:"value"`
		} `json:"geoMapData"`
	} `json:"default"`
}

// parseComparedGeo is the pure-parsing half of InterestByRegion.
func parseComparedGeo(body []byte) ([]RegionInterest, error) {
	body = stripXSSIPrefix(body)
	var parsed comparedgeoResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("gtrends: parsing comparedgeo response: %w", err)
	}
	regions := make([]RegionInterest, 0, len(parsed.Default.GeoMapData))
	for _, g := range parsed.Default.GeoMapData {
		regions = append(regions, RegionInterest{GeoCode: g.GeoCode, GeoName: g.GeoName, Values: g.Value})
	}
	return regions, nil
}

// InterestByRegion fetches and parses the GEO_MAP widget's data.
func InterestByRegion(ctx context.Context, c *client.Client, widget Widget) ([]RegionInterest, error) {
	params, err := widgetParams(widget)
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/trends/api/widgetdata/comparedgeo", params)
	if err != nil {
		return nil, fmt.Errorf("gtrends: comparedgeo request failed: %w", err)
	}
	return parseComparedGeo(data)
}

type relatedSearchesResponse struct {
	Default struct {
		RankedList []struct {
			RankedKeyword []struct {
				Query string      `json:"query"`
				Value json.Number `json:"value"`
			} `json:"rankedKeyword"`
		} `json:"rankedList"`
	} `json:"default"`
}

// parseRelatedSearches is the pure-parsing half of RelatedSearches.
// rankedList[0] is TOP, rankedList[1] is RISING; any further entries are
// ignored (the live API has never been observed to return more than two).
func parseRelatedSearches(body []byte) (top []RelatedTerm, rising []RelatedTerm, err error) {
	body = stripXSSIPrefix(body)
	var parsed relatedSearchesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, nil, fmt.Errorf("gtrends: parsing relatedsearches response: %w", err)
	}
	top = make([]RelatedTerm, 0)
	rising = make([]RelatedTerm, 0)
	for i, list := range parsed.Default.RankedList {
		if i > 1 {
			break
		}
		for _, kw := range list.RankedKeyword {
			v, _ := kw.Value.Int64()
			term := RelatedTerm{Query: kw.Query, Value: int(v)}
			if i == 0 {
				top = append(top, term)
			} else {
				term.IsBreakout = term.Value >= breakoutThreshold
				rising = append(rising, term)
			}
		}
	}
	return top, rising, nil
}

// RelatedSearches fetches and parses a RELATED_QUERIES or RELATED_TOPICS
// widget's data into top and rising term lists.
func RelatedSearches(ctx context.Context, c *client.Client, widget Widget) (top []RelatedTerm, rising []RelatedTerm, err error) {
	params, perr := widgetParams(widget)
	if perr != nil {
		return nil, nil, perr
	}
	data, gerr := c.Get(ctx, "/trends/api/widgetdata/relatedsearches", params)
	if gerr != nil {
		return nil, nil, fmt.Errorf("gtrends: relatedsearches request failed: %w", gerr)
	}
	return parseRelatedSearches(data)
}
