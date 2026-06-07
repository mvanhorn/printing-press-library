// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared helpers for the Tripadvisor transcendence commands
// (best, compare, nearby-best, drift, digest, fit). Not generated; preserved
// across regen as a whole hand-authored file.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/tripadvisor/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/tripadvisor/internal/cliutil"
)

// emitTANovel writes a novel command's result. Machine consumers (--json,
// --agent, or piped stdout) get the full structured view through
// printJSONFiltered (which honors --select/--compact); interactive terminals
// get a compact rating/review/ranking table over rows.
func emitTANovel(cmd *cobra.Command, flags *rootFlags, view any, rows []taDetail) error {
	if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No results.")
		return nil
	}
	items := make([]map[string]any, 0, len(rows))
	for _, d := range rows {
		items = append(items, map[string]any{
			"location_id": d.LocationID,
			"name":        d.Name,
			"rating":      d.Rating,
			"num_reviews": d.NumReviews,
			"ranking":     d.RankingString,
			"price":       d.PriceLevel,
		})
	}
	return printAutoTable(cmd.OutOrStdout(), items)
}

// taScanDefault bounds how many detail calls a fan-out command makes by
// default. The Content API is metered (5k calls/month free), so commands that
// scan-and-enrich cap detail fan-out separately from how many rows they return.
const taScanDefault = 10

// taTravelerProfiles maps a --traveler value to the Tripadvisor trip_type key.
var taTravelerProfiles = map[string]string{
	"families": "families",
	"family":   "families",
	"couples":  "couples",
	"couple":   "couples",
	"solo":     "solo",
	"business": "business",
	"friends":  "friends",
}

// taStub is one search/nearby result row.
type taStub struct {
	LocationID string `json:"location_id"`
	Name       string `json:"name"`
	Address    string `json:"address,omitempty"`
}

// taDetail is the high-gravity slice of a location-details response, normalized
// so downstream commands can rank and compare without re-parsing string fields.
type taDetail struct {
	LocationID    string             `json:"location_id"`
	Name          string             `json:"name"`
	Rating        float64            `json:"rating"`
	NumReviews    int                `json:"num_reviews"`
	RankingString string             `json:"ranking,omitempty"`
	Ranking       int                `json:"ranking_position,omitempty"`
	RankingOutOf  int                `json:"ranking_out_of,omitempty"`
	PriceLevel    string             `json:"price_level,omitempty"`
	WebURL        string             `json:"web_url,omitempty"`
	Address       string             `json:"address,omitempty"`
	TripTypes     map[string]int     `json:"trip_types,omitempty"`
	Subratings    map[string]float64 `json:"subratings,omitempty"`

	fetchErr error
}

// taFetchFailure is the JSON-envelope record for a detail call that errored
// during a fan-out. Failed fetches never dilute averages or counts.
type taFetchFailure struct {
	LocationID string `json:"location_id"`
	Error      string `json:"error"`
}

// taParseFloat parses a string number leniently; "" or unparseable -> 0.
func taParseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// taParseInt parses a string integer leniently; "" or unparseable -> 0.
func taParseInt(s string) int {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		// tolerate "1058.0" style values
		if f, ferr := strconv.ParseFloat(s, 64); ferr == nil {
			return int(f)
		}
		return 0
	}
	return n
}

// taParseIntRaw parses a JSON value that may be a number or a quoted string.
func taParseIntRaw(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return taParseInt(s)
	}
	return 0
}

// taParseStubs extracts location stubs from a search/nearby response. The
// Content API wraps results in {"data":[...]}, but tolerate a bare array too.
func taParseStubs(raw json.RawMessage) []taStub {
	var wrap struct {
		Data []json.RawMessage `json:"data"`
	}
	items := []json.RawMessage(nil)
	if json.Unmarshal(raw, &wrap) == nil && wrap.Data != nil {
		items = wrap.Data
	} else {
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil {
			items = arr
		}
	}
	out := make([]taStub, 0, len(items))
	for _, item := range items {
		var s struct {
			LocationID string `json:"location_id"`
			Name       string `json:"name"`
			AddressObj struct {
				AddressString string `json:"address_string"`
			} `json:"address_obj"`
		}
		if json.Unmarshal(item, &s) != nil || s.LocationID == "" {
			continue
		}
		out = append(out, taStub{LocationID: s.LocationID, Name: s.Name, Address: s.AddressObj.AddressString})
	}
	return out
}

// taExtractDataArray returns the elements of a Content API {"data":[...]}
// envelope as raw JSON, or a bare array, or an empty slice. Never returns nil
// so the field marshals as [] not null.
func taExtractDataArray(raw json.RawMessage) []json.RawMessage {
	var wrap struct {
		Data []json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &wrap) == nil && wrap.Data != nil {
		return wrap.Data
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		return arr
	}
	return []json.RawMessage{}
}

// taParseDetail normalizes a location-details response into a taDetail.
func taParseDetail(raw json.RawMessage) taDetail {
	var d struct {
		LocationID  string `json:"location_id"`
		Name        string `json:"name"`
		Rating      string `json:"rating"`
		NumReviews  string `json:"num_reviews"`
		PriceLevel  string `json:"price_level"`
		WebURL      string `json:"web_url"`
		RankingData struct {
			RankingString string          `json:"ranking_string"`
			Ranking       json.RawMessage `json:"ranking"`
			RankingOutOf  json.RawMessage `json:"ranking_out_of"`
		} `json:"ranking_data"`
		AddressObj struct {
			AddressString string `json:"address_string"`
		} `json:"address_obj"`
		TripTypes []struct {
			Name          string `json:"name"`
			LocalizedName string `json:"localized_name"`
			Value         string `json:"value"`
		} `json:"trip_types"`
		Subratings map[string]struct {
			Name          string `json:"name"`
			LocalizedName string `json:"localized_name"`
			Value         string `json:"value"`
		} `json:"subratings"`
	}
	_ = json.Unmarshal(raw, &d)

	det := taDetail{
		LocationID:    d.LocationID,
		Name:          cliutil.CleanText(d.Name),
		Rating:        taParseFloat(d.Rating),
		NumReviews:    taParseInt(d.NumReviews),
		PriceLevel:    d.PriceLevel,
		WebURL:        d.WebURL,
		RankingString: cliutil.CleanText(d.RankingData.RankingString),
		Ranking:       taParseIntRaw(d.RankingData.Ranking),
		RankingOutOf:  taParseIntRaw(d.RankingData.RankingOutOf),
		Address:       cliutil.CleanText(d.AddressObj.AddressString),
		TripTypes:     map[string]int{},
		Subratings:    map[string]float64{},
	}
	for _, tt := range d.TripTypes {
		key := strings.ToLower(strings.TrimSpace(tt.Name))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(tt.LocalizedName))
		}
		if key == "" {
			continue
		}
		det.TripTypes[key] = taParseInt(tt.Value)
	}
	for _, sr := range d.Subratings {
		label := strings.TrimSpace(sr.LocalizedName)
		if label == "" {
			label = strings.TrimSpace(sr.Name)
		}
		if label == "" {
			continue
		}
		det.Subratings[label] = taParseFloat(sr.Value)
	}
	return det
}

// taSearch runs a location search and returns normalized stubs.
func taSearch(ctx context.Context, c *client.Client, query, category, latLong, language string) ([]taStub, error) {
	params := map[string]string{"searchQuery": query}
	if category != "" {
		params["category"] = category
	}
	if latLong != "" {
		params["latLong"] = latLong
	}
	if language != "" {
		params["language"] = language
	}
	raw, err := c.Get(ctx, "/location/search", params)
	if err != nil {
		return nil, err
	}
	return taParseStubs(raw), nil
}

// taNearby runs a nearby search and returns normalized stubs.
func taNearby(ctx context.Context, c *client.Client, latLong, category, radius, radiusUnit, language string) ([]taStub, error) {
	params := map[string]string{"latLong": latLong}
	if category != "" {
		params["category"] = category
	}
	if radius != "" {
		params["radius"] = radius
	}
	if radiusUnit != "" {
		params["radiusUnit"] = radiusUnit
	}
	if language != "" {
		params["language"] = language
	}
	raw, err := c.Get(ctx, "/location/nearby_search", params)
	if err != nil {
		return nil, err
	}
	return taParseStubs(raw), nil
}

// taFetchDetail fetches and normalizes one location's details.
func taFetchDetail(ctx context.Context, c *client.Client, id, language, currency string) (taDetail, error) {
	params := map[string]string{}
	if language != "" {
		params["language"] = language
	}
	if currency != "" {
		params["currency"] = currency
	}
	raw, err := c.Get(ctx, "/location/"+url.PathEscape(id)+"/details", params)
	if err != nil {
		return taDetail{LocationID: id, fetchErr: err}, err
	}
	d := taParseDetail(raw)
	if d.LocationID == "" {
		d.LocationID = id
	}
	return d, nil
}

// taFetchDetailsBounded fans out detail calls (max 5 concurrent), bounded by
// maxScan, and partitions successes from failures so failed fetches never
// pollute downstream aggregates. Order of successful results follows ids.
func taFetchDetailsBounded(ctx context.Context, c *client.Client, ids []string, language, currency string, maxScan int) (ok []taDetail, failures []taFetchFailure, scanned int) {
	if maxScan > 0 && len(ids) > maxScan {
		ids = ids[:maxScan]
	}
	scanned = len(ids)
	type res struct {
		idx int
		det taDetail
	}
	// results is buffered to len(ids) so workers never block on send while the
	// spawn loop is still running; sem caps concurrency at 5.
	results := make(chan res, len(ids))
	sem := make(chan struct{}, 5)
	for i, id := range ids {
		sem <- struct{}{}
		go func(i int, id string) {
			defer func() { <-sem }()
			d, _ := taFetchDetail(ctx, c, id, language, currency)
			results <- res{idx: i, det: d}
		}(i, id)
	}

	ordered := make([]taDetail, len(ids))
	for got := 0; got < len(ids); got++ {
		r := <-results
		ordered[r.idx] = r.det
	}

	ok = make([]taDetail, 0, len(ids))
	failures = make([]taFetchFailure, 0)
	for i, d := range ordered {
		if d.fetchErr != nil {
			failures = append(failures, taFetchFailure{LocationID: ids[i], Error: d.fetchErr.Error()})
			continue
		}
		ok = append(ok, d)
	}
	return ok, failures, scanned
}

// taSortDetails sorts details in place by the named key (rating, reviews,
// ranking). rating/reviews sort descending (better first); ranking sorts
// ascending (rank #1 first), treating 0 (unknown) as last.
func taSortDetails(items []taDetail, sortKey string) {
	less := func(a, b taDetail) bool {
		switch sortKey {
		case "reviews":
			return a.NumReviews > b.NumReviews
		case "ranking":
			ar, br := a.Ranking, b.Ranking
			if ar == 0 {
				ar = 1 << 30
			}
			if br == 0 {
				br = 1 << 30
			}
			return ar < br
		default: // rating
			if a.Rating != b.Rating {
				return a.Rating > b.Rating
			}
			return a.NumReviews > b.NumReviews
		}
	}
	// simple insertion-free stable sort
	sortStable(items, less)
}

// sortStable is a tiny stable sort to avoid importing sort with a closure type
// mismatch; n is small (bounded by --top / scan caps).
func sortStable(items []taDetail, less func(a, b taDetail) bool) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && less(items[j], items[j-1]); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// taLimit clamps a slice of details to n (n<=0 means no limit).
func taLimit(items []taDetail, n int) []taDetail {
	if n > 0 && len(items) > n {
		return items[:n]
	}
	return items
}

// taDogfoodScan curtails scan effort under live-dogfood so fan-out fits the
// matrix's per-command timeout.
func taDogfoodScan(maxScan int) int {
	if cliutil.IsDogfoodEnv() && maxScan > 3 {
		return 3
	}
	return maxScan
}
