// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package gtrends

// This file implements Google's "Trending Now" surface: a two-step,
// completely undocumented flow that (1) scrapes a session id ("f.sid") and
// build label ("bl") out of inline JS on the /trending HTML page, then (2)
// replays them into an internal batchexecute RPC (rpcid "DqDTgb") that the
// live page's own JS calls to render the trending list.
//
// This is, by a wide margin, the flakiest code path in this CLI. There is
// no schema for batchexecute's wire format — it's a sequence of
// length-prefixed frames, most of which carry a doubly-JSON-encoded string
// payload with null-padded positional arrays instead of named fields.
// Google can and does reshape both the /trending page's bootstrap data and
// the RPC's frame shape without notice; when that happens, extraction here
// should degrade to the heuristic fallback or a typed error, never a panic
// or a silently-empty result mistaken for "nothing trending today".

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/client"
)

// TrendingTopic is one entry in the current (or historical, once persisted
// locally) trending-topics list. Rank is the entry's 1-based position in the
// list Google returned.
type TrendingTopic struct {
	Term string `json:"term"`
	Rank int    `json:"rank"`
}

// ErrTrendingScrapeFailed indicates the /trending HTML page no longer
// contains the session tokens this scraper looks for. Most likely cause:
// Google changed the page's bootstrap-data format.
var ErrTrendingScrapeFailed = errors.New("could not extract trending session tokens from page — Google may have changed the /trending page structure")

// ErrTrendingParseFailed indicates the batchexecute RPC response could not
// be parsed by either the strict frame decoder or the heuristic fallback.
var ErrTrendingParseFailed = errors.New("could not parse trending response — Google's batchexecute format is undocumented and may have changed")

var (
	// "FdrFJe" is the bootstrap key Google's /trending page uses for the
	// session id as of this writing; "f.sid" is kept as a fallback since
	// pytrends and earlier captures of this page used that key name — the
	// page's bootstrap-data format has changed key names before (see
	// ErrTrendingScrapeFailed) and may again.
	sidPattern = regexp.MustCompile(`"FdrFJe"\s*:\s*"(-?\d+)"|"f\.sid"\s*:\s*"(-?\d+)"|f\.sid=(-?\d+)`)
	blPattern  = regexp.MustCompile(`"cfb2h"\s*:\s*"([\w.-]+)"|\bbl=([\w.-]+)`)
)

// extractTrendingSessionTokens scrapes the f.sid session id and bl build
// label out of the /trending page's inline bootstrap JS. Pure function, unit
// tested against inline HTML fixtures.
func extractTrendingSessionTokens(html string) (sid, bl string, err error) {
	if m := sidPattern.FindStringSubmatch(html); m != nil {
		for _, g := range m[1:] {
			if g != "" {
				sid = g
				break
			}
		}
	}
	if m := blPattern.FindStringSubmatch(html); m != nil {
		if m[1] != "" {
			bl = m[1]
		} else {
			bl = m[2]
		}
	}
	if sid == "" || bl == "" {
		return "", "", ErrTrendingScrapeFailed
	}
	return sid, bl, nil
}

// trendingRPCID is the batchexecute RPC that returns the actual daily
// trending-searches list for the geo scoped by the session's f.sid/bl
// tokens (themselves scraped from a /trending?geo=<geo> page load — the RPC
// call itself carries no geo param). "DqDTgb", used by an earlier revision
// of this scraper, was a misidentification: it returns the page's
// geo-picker dropdown (a full country list), not trending terms — confirmed
// live by dumping its response, which yielded ISO country codes.
const trendingRPCID = "Tnt4U"

// buildTrendingRPCFields builds the url.Values form body for the batchexecute
// call: a single f.req field carrying the (URL-encoded, by url.Values.Encode)
// nested-array RPC envelope. Tnt4U takes an empty inner request — the real
// browser capture sent "[]", not [hl, geoFlag, 0] (that shape belongs to
// DqDTgb, the geo-picker RPC).
func buildTrendingRPCFields() url.Values {
	frame := []any{[]any{[]any{trendingRPCID, "[]", nil, "generic"}}}
	fReq, _ := json.Marshal(frame)
	return url.Values{"f.req": []string{string(fReq)}}
}

// TrendingNow fetches the current trending-topics list for geo/hl ("" geo
// means worldwide/no geo filter). It scrapes fresh session tokens from the
// /trending HTML page on every call — Google does not document a way to
// reuse them, and they appear to be short-lived.
func TrendingNow(ctx context.Context, c *client.Client, geo, hl string) ([]TrendingTopic, error) {
	if hl == "" {
		hl = DefaultHL
	}
	params := map[string]string{"hl": hl}
	if geo != "" {
		params["geo"] = geo
	}
	html, err := c.Get(ctx, "/trending", params)
	if err != nil {
		return nil, fmt.Errorf("gtrends: fetching /trending page: %w", err)
	}
	sid, bl, err := extractTrendingSessionTokens(string(html))
	if err != nil {
		return nil, err
	}

	rpcParams := map[string]string{
		"rpcids":       trendingRPCID,
		"source-path":  "/trending",
		"f.sid":        sid,
		"bl":           bl,
		"hl":           hl,
		"soc-app":      "1",
		"soc-platform": "1",
		"soc-device":   "1",
		"_reqid":       strconv.Itoa(1000 + rand.Intn(8999)), // #nosec G404 -- not security sensitive, just a client request-id nonce Google's RPC expects.
		"rt":           "c",
	}
	fields := buildTrendingRPCFields()

	// Referer and X-Same-Domain mirror what a real browser sends on every
	// batchexecute call from this page; harmless to include even though
	// they did not resolve the "e" error frame documented below.
	rpcHeaders := map[string]string{
		"Referer":       "https://trends.google.com/",
		"X-Same-Domain": "1",
	}
	data, _, err := c.PostFormWithParamsAndHeaders(ctx, "/_/TrendsUi/data/batchexecute", rpcParams, fields, rpcHeaders)
	if err != nil {
		return nil, fmt.Errorf("gtrends: batchexecute request failed: %w", err)
	}

	return parseTrendingBatchExecute(data)
}

// parseTrendingBatchExecute is the pure-parsing half of TrendingNow. See the
// file doc comment for why this is inherently best-effort.
func parseTrendingBatchExecute(body []byte) ([]TrendingTopic, error) {
	if frame, ok := decodeBatchExecuteFrame(body); ok {
		if topics, ok := extractTrendingTopicsFromFrame(frame); ok && len(topics) > 0 {
			return topics, nil
		}
	}
	// An explicit "e" (error) frame means the server rejected or errored on
	// the request itself — the low-confidence heuristic scraper below would
	// otherwise happily extract plausible-looking garbage (RPC ids,
	// structural tokens) out of the surrounding frame syntax and report it
	// as real trending terms. A clear error beats confidently-wrong output.
	if hasBatchExecuteErrorFrame(body) {
		sample := string(body)
		if len(sample) > 200 {
			sample = sample[:200] + "..."
		}
		return nil, fmt.Errorf("%w: server returned an error frame (%d bytes, starts with: %q)", ErrTrendingParseFailed, len(body), sample)
	}
	if topics := trendingTopicsFromHeuristic(body); len(topics) > 0 {
		return topics, nil
	}
	sample := string(body)
	if len(sample) > 200 {
		sample = sample[:200] + "..."
	}
	return nil, fmt.Errorf("%w (%d bytes, starts with: %q)", ErrTrendingParseFailed, len(body), sample)
}

// hasBatchExecuteErrorFrame reports whether any JSON-array line in the
// response carries a top-level ["e", <code>, ...] error entry — batchexecute's
// error-signaling shape, distinct from a "wrb.fr" data frame.
func hasBatchExecuteErrorFrame(body []byte) bool {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		var outer []any
		if err := json.Unmarshal([]byte(line), &outer); err != nil {
			continue
		}
		for _, el := range outer {
			entry, ok := el.([]any)
			if !ok || len(entry) == 0 {
				continue
			}
			if tag, ok := entry[0].(string); ok && tag == "e" {
				return true
			}
		}
	}
	return false
}

// decodeBatchExecuteFrame scans the (already XSSI-stripped, by the client
// layer) response line by line looking for the "wrb.fr" frame carrying the
// trending-terms RPC's payload. batchexecute interleaves decimal length-prefix
// lines with JSON-array lines; only the JSON-array lines matter here.
func decodeBatchExecuteFrame(body []byte) ([]any, bool) {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		var outer []any
		if err := json.Unmarshal([]byte(line), &outer); err != nil {
			continue
		}
		for _, el := range outer {
			entry, ok := el.([]any)
			if !ok || len(entry) < 3 {
				continue
			}
			if tag, ok := entry[0].(string); ok && tag == "wrb.fr" {
				return entry, true
			}
		}
	}
	return nil, false
}

// extractTrendingTopicsFromFrame decodes the wrb.fr frame's payload — a
// JSON-encoded STRING (double-encoded) holding the real nested-array data —
// and walks it for trending terms.
func extractTrendingTopicsFromFrame(entry []any) ([]TrendingTopic, bool) {
	if len(entry) < 3 {
		return nil, false
	}
	payload, ok := entry[2].(string)
	if !ok || payload == "" {
		return nil, false
	}
	var inner []any
	if err := json.Unmarshal([]byte(payload), &inner); err != nil {
		return nil, false
	}
	topics := trendingTopicsFromNestedArray(inner)
	return topics, len(topics) > 0
}

// trendingTopicsFromNestedArray walks the decoded nested-array payload and
// extracts one term per top-level "story" entry. The exact positional
// schema of a story entry is not documented; this takes the first
// plausible-looking string found within each entry (bounded-depth
// recursive search) as the display term.
func trendingTopicsFromNestedArray(inner []any) []TrendingTopic {
	topics := make([]TrendingTopic, 0)
	seen := map[string]bool{}

	list := inner
	if len(inner) > 0 {
		if first, ok := inner[0].([]any); ok {
			list = first
		}
	}

	rank := 0
	for _, item := range list {
		term, ok := findFirstPlausibleString(item, 3)
		if !ok || seen[term] {
			continue
		}
		seen[term] = true
		rank++
		topics = append(topics, TrendingTopic{Term: term, Rank: rank})
	}
	return topics
}

// findFirstPlausibleString does a bounded-depth depth-first search for the
// first string leaf that looks like a trending term (see
// isPlausibleTrendingTerm).
func findFirstPlausibleString(v any, depth int) (string, bool) {
	if depth <= 0 {
		return "", false
	}
	switch t := v.(type) {
	case string:
		if isPlausibleTrendingTerm(t) {
			return t, true
		}
		return "", false
	case []any:
		for _, elem := range t {
			if s, ok := findFirstPlausibleString(elem, depth-1); ok {
				return s, true
			}
		}
	}
	return "", false
}

// isPlausibleTrendingTerm filters candidate strings pulled out of the
// undocumented payload down to ones that look like a human-readable search
// term rather than an id, URL, or structural token.
func isPlausibleTrendingTerm(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || len(s) > 100 {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return false
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "/") {
		return false
	}
	return true
}

// quotedTermPattern is the last-resort heuristic scanner: any quoted string
// that starts with a letter and stays within a plausible term's character
// set. Deliberately permissive — trendingTopicsFromHeuristic and
// isPlausibleTrendingTerm do the real filtering.
var quotedTermPattern = regexp.MustCompile(`"([A-Za-z][A-Za-z0-9 .,'&-]{1,79})"`)

// heuristicNoiseTerms are known structural/RPC tokens that would otherwise
// slip through the heuristic scanner's character-class filter.
var heuristicNoiseTerms = map[string]bool{
	"wrb.fr": true, "generic": true, "dqdtgb": true, "trendingsearchescontroller": true,
}

// trendingTopicsFromHeuristic is the fallback path used when strict frame
// decoding fails (Google reshaped the frame/payload structure). It scans the
// raw response body for quoted strings that look like trending terms. Purely
// best-effort — see the file doc comment.
func trendingTopicsFromHeuristic(body []byte) []TrendingTopic {
	matches := quotedTermPattern.FindAllStringSubmatch(string(body), -1)
	topics := make([]TrendingTopic, 0)
	seen := map[string]bool{}
	rank := 0
	for _, m := range matches {
		term := strings.TrimSpace(m[1])
		if term == "" || seen[term] || !isPlausibleTrendingTerm(term) {
			continue
		}
		if heuristicNoiseTerms[strings.ToLower(term)] {
			continue
		}
		seen[term] = true
		rank++
		topics = append(topics, TrendingTopic{Term: term, Rank: rank})
	}
	return topics
}
