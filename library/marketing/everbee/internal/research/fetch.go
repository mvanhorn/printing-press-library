// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/client"
)

// Endpoint paths. These are the endpoints EverBee's own search boxes call.
//
// The published v4.19 CLI used the `default_*` feeds below for search, which is
// the root cause of issue #1492: those feeds ignore the seed entirely and return
// unranked filler (for the seed "dad shirt" they return keywords like
// "gifttings" and listings for African masks). They are legitimate as a browse
// surface and are exposed as `... list`, but they are never the answer to a query.
const (
	PathKeywordSearch = "/keyword_research/keyword_suggestion" // POST, seeded
	PathProductSearch = "/product_analytics"                   // GET, seeded (requires search_term)
	PathShopSearch    = "/shops"                               // GET, seeded (search_term optional)
	PathShopResolve   = "/shops/search"                        // GET, handle -> identity
	PathAccount       = "/users/show"                          // GET, plan + quota
)

// ErrPlanCapReached signals that EverBee refused the research because the
// account's plan quota is exhausted. It must never be reported as "no results":
// an empty answer and a refused answer are different facts.
var ErrPlanCapReached = errors.New("everbee plan research quota reached")

// APIDoer is the subset of the generated client this package needs. Keeping it
// an interface lets the scoring pipeline be tested without a live API.
//
// Rate-limit contract: every production APIDoer is the generated
// *client.Client (enforced by the compile-time assertion below), whose
// cliutil.AdaptiveLimiter paces requests and retries 429s inside do().
// Any 429 that still escapes the retry loop surfaces here as
// ErrPlanCapReached via isPlanCap — EverBee overloads 429 as a plan-quota
// refusal, so that typed error is this package's RateLimitError equivalent
// and throttling is never reported as an empty result.
type APIDoer interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
	PostQueryWithParams(ctx context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error)
}

// Compile-time proof that the generated, rate-limited client satisfies
// APIDoer, so signature drift in the generated client breaks the build here
// instead of at the call sites.
var _ APIDoer = (*client.Client)(nil)

// Now is swappable so tests can pin provenance timestamps.
var Now = time.Now

// keywordResponse is EverBee's seeded keyword search payload.
type keywordResponse struct {
	TotalCount      int             `json:"total_count"`
	SearchedKeyword json.RawMessage `json:"searched_keyword"`
	Results         []struct {
		Keyword     string      `json:"keyword"`
		Vol         json.Number `json:"vol"`
		NewVolume   json.Number `json:"new_volume"`
		Competition json.Number `json:"competition"`
		Score       json.Number `json:"score"`
		CPC         json.Number `json:"cpc"`
		// Trend is EverBee's month-by-month search-volume series. It is
		// frequently null — as of the 2026-07-11 capture it was null for every
		// row — so seasonality must degrade to "unknown" rather than guess.
		Trend json.RawMessage `json:"trend"`
	} `json:"results"`
}

// SeedTrend extracts the seed's own search-volume trend series, if EverBee
// supplied one. Returns nil when no trend data came back, which callers report
// as seasonality "unknown" rather than filling in an invented series.
//
// EverBee's `trend` field is null for most keywords, so nil is the common case
// and is a correct answer, not a failure.
func SeedTrend(rows []KeywordRow, seed string) []float64 {
	for _, r := range rows {
		if len(r.Trend) > 0 && strings.EqualFold(r.Keyword, seed) {
			return r.Trend
		}
	}
	// Fall back to the best-matching row's trend, if any row carried one.
	for _, r := range rows {
		if len(r.Trend) > 0 && r.Relevance >= DefaultMinRelevance {
			return r.Trend
		}
	}
	return nil
}

// parseTrend accepts the shapes EverBee has been observed to use for `trend`:
// a bare array of numbers, or an object whose values are the series. Anything
// else (including null) yields nil.
func parseTrend(raw json.RawMessage) []float64 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var arr []json.Number
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		out := make([]float64, 0, len(arr))
		for _, n := range arr {
			out = append(out, num(n))
		}
		return out
	}
	var obj map[string]json.Number
	if err := json.Unmarshal(raw, &obj); err == nil && len(obj) > 0 {
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]float64, 0, len(keys))
		for _, k := range keys {
			out = append(out, num(obj[k]))
		}
		return out
	}
	return nil
}

// productResponse is EverBee's seeded product search payload.
type productResponse struct {
	TotalCount int `json:"total_count"`
	Results    []struct {
		ListingID       json.Number `json:"listing_id"`
		Title           string      `json:"title"`
		ShopName        string      `json:"shop_name"`
		Price           json.Number `json:"price"`
		ListingType     string      `json:"listing_type"`
		Tags            []string    `json:"tags"`
		EstMoRevenue    json.Number `json:"cached_est_mo_revenue"`
		EstMoSales      json.Number `json:"cached_est_mo_sales"`
		EstTotalSales   json.Number `json:"cached_est_total_sales"`
		ReviewCount     json.Number `json:"review_count"`
		ListingAgeMonth json.Number `json:"cached_listing_age_in_months"`
		URL             string      `json:"url"`
	} `json:"results"`
	UsageDetails json.RawMessage `json:"usage_details"`
}

// num coerces EverBee's numeric fields, which arrive as JSON numbers in some
// rows and JSON-encoded strings in others. Returning 0 for an absent field is
// correct here: a missing estimate is not a zero estimate, and callers that
// care check the evidence counts rather than the value.
func num(n json.Number) float64 {
	if n == "" {
		return 0
	}
	f, err := n.Float64()
	if err != nil {
		return 0
	}
	return f
}

// intFromNumber preserves integer identity exactly. EverBee listing IDs arrive
// as JSON numbers; routing them through float64 turns 55043301 into
// 5.5043301e+07 and breaks store identity, so they are parsed as integers.
func intFromNumber(n json.Number) int64 {
	if n == "" {
		return 0
	}
	if i, err := n.Int64(); err == nil {
		return i
	}
	if f, err := n.Float64(); err == nil {
		return int64(f)
	}
	return 0
}

// FetchKeywords runs EverBee's seeded keyword search and annotates every
// returned row with its relevance to the seed. Rows are never dropped.
func FetchKeywords(ctx context.Context, c APIDoer, seed string, page int) ([]KeywordRow, SeedMetrics, Provenance, error) {
	prov := Provenance{
		Source:     "everbee:keyword_suggestion",
		Endpoint:   PathKeywordSearch,
		QueryScope: seed,
		FetchedAt:  Now().UTC(),
	}
	if strings.TrimSpace(seed) == "" {
		return nil, SeedMetrics{}, prov, errors.New("seed keyword is required")
	}
	if page < 1 {
		page = 1
	}
	params := map[string]string{
		"keyword":         seed,
		"type_of_search":  "keywords_suggestion",
		"order_by":        "new_volume",
		"order_direction": "desc",
		"page":            strconv.Itoa(page),
		"count_search":    "true",
		"fromApp":         "true",
	}
	raw, status, err := c.PostQueryWithParams(ctx, PathKeywordSearch, params, nil)
	if err != nil {
		if isPlanCap(status, raw, err) {
			return nil, SeedMetrics{}, prov, fmt.Errorf("%w (seed %q)", ErrPlanCapReached, seed)
		}
		return nil, SeedMetrics{}, prov, fmt.Errorf("keyword search for %q: %w", seed, err)
	}

	var resp keywordResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, SeedMetrics{}, prov, fmt.Errorf("parsing keyword response for %q: %w", seed, err)
	}

	rows := make([]KeywordRow, 0, len(resp.Results))
	for _, r := range resp.Results {
		vol := int(num(r.NewVolume))
		if vol == 0 {
			vol = int(num(r.Vol))
		}
		rows = append(rows, KeywordRow{
			Keyword:     r.Keyword,
			Volume:      vol,
			Competition: int(num(r.Competition)),
			Score:       num(r.Score),
			CPC:         num(r.CPC),
			Trend:       parseTrend(r.Trend),
			Relevance:   Relevance(seed, r.Keyword),
		})
	}
	return rows, parseSeedMetrics(resp.SearchedKeyword, seed), prov, nil
}

// parseSeedMetrics reads the seed's own demand/competition pair out of the
// `searched_keyword` block. EverBee returns it as an object for a single seed
// and as an array when several seeds were comma-joined; both are handled, and
// an absent block yields Present=false rather than a fabricated zero.
func parseSeedMetrics(raw json.RawMessage, seed string) SeedMetrics {
	if len(raw) == 0 || string(raw) == "null" {
		return SeedMetrics{Keyword: seed}
	}
	type sk struct {
		Keyword     string      `json:"keyword"`
		Vol         json.Number `json:"vol"`
		NewVolume   json.Number `json:"new_volume"`
		Competition json.Number `json:"competition"`
	}
	build := func(s sk) SeedMetrics {
		vol := int(num(s.NewVolume))
		if vol == 0 {
			vol = int(num(s.Vol))
		}
		kw := s.Keyword
		if kw == "" {
			kw = seed
		}
		return SeedMetrics{
			Keyword:     kw,
			Volume:      vol,
			Competition: int(num(s.Competition)),
			Present:     true,
		}
	}
	var one sk
	if err := json.Unmarshal(raw, &one); err == nil && (one.Keyword != "" || one.Vol != "" || one.NewVolume != "") {
		return build(one)
	}
	var many []sk
	if err := json.Unmarshal(raw, &many); err == nil && len(many) > 0 {
		return build(many[0])
	}
	return SeedMetrics{Keyword: seed}
}

// FetchProducts runs EverBee's seeded product search and annotates every
// returned row with its relevance to the seed. Rows are never dropped.
func FetchProducts(ctx context.Context, c APIDoer, seed string, perPage int) ([]ProductRow, int, Provenance, error) {
	prov := Provenance{
		Source:     "everbee:product_analytics",
		Endpoint:   PathProductSearch,
		QueryScope: seed,
		FetchedAt:  Now().UTC(),
	}
	if strings.TrimSpace(seed) == "" {
		return nil, 0, prov, errors.New("search term is required")
	}
	if perPage < 1 {
		perPage = 20
	}
	params := map[string]string{
		"search_term":     seed,
		"type_of_search":  "listings",
		"time_range":      "last_1_month",
		"order_by":        "est_mo_revenue",
		"order_direction": "desc",
		"page":            "1",
		"per_page":        strconv.Itoa(perPage),
	}
	raw, err := c.Get(ctx, PathProductSearch, params)
	if err != nil {
		if isPlanCap(0, raw, err) {
			return nil, 0, prov, fmt.Errorf("%w (search %q)", ErrPlanCapReached, seed)
		}
		return nil, 0, prov, fmt.Errorf("product search for %q: %w", seed, err)
	}

	var resp productResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, 0, prov, fmt.Errorf("parsing product response for %q: %w", seed, err)
	}

	rows := make([]ProductRow, 0, len(resp.Results))
	for _, r := range resp.Results {
		title := r.Title
		rows = append(rows, ProductRow{
			ListingID:       intFromNumber(r.ListingID),
			Title:           title,
			ShopName:        r.ShopName,
			Price:           num(r.Price),
			ListingType:     r.ListingType,
			Tags:            r.Tags,
			EstMoRevenue:    num(r.EstMoRevenue),
			EstMoSales:      num(r.EstMoSales),
			EstTotalSales:   num(r.EstTotalSales),
			ReviewCount:     int(num(r.ReviewCount)),
			ListingAgeMonth: num(r.ListingAgeMonth),
			URL:             r.URL,
			// Relevance is scored against the title AND the listing's own tags:
			// a listing titled "Just Resting My Eyes Tee" tagged "funny dad
			// shirt" is genuinely on-target, and title-only scoring would miss it.
			Relevance: maxFloat(
				Relevance(seed, title),
				Relevance(seed, strings.Join(r.Tags, " ")),
			),
		})
	}
	return rows, resp.TotalCount, prov, nil
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// isPlanCap recognizes EverBee's plan-quota refusal so it can be surfaced as a
// typed error instead of an empty result. The free Hobby plan allows only 10
// keyword searches per month.
//
// The generated client returns a nil body alongside an HTTP-status error, so the
// refusal text usually reaches us only inside err.Error(). Both the body and the
// error string are checked; relying on the body alone would silently miss every
// plan cap on the product path. (Generator retro candidate: client should return
// the response body with status errors.)
func isPlanCap(status int, body json.RawMessage, err error) bool {
	if status == 402 || status == 429 {
		return true
	}
	hay := strings.ToLower(string(body))
	if err != nil {
		hay += " " + strings.ToLower(err.Error())
	}
	return strings.Contains(hay, "http 402") ||
		strings.Contains(hay, "http 429") ||
		strings.Contains(hay, "upgrade") ||
		strings.Contains(hay, "limit reached") ||
		strings.Contains(hay, "quota") ||
		strings.Contains(hay, "out of searches")
}
