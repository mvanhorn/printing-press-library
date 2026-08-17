// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// freshness.go implements live-on-read price refresh for the discovery
// commands (topic, compare, mispriced, trending, movers, resolving,
// liquid, new). The local SQLite store is a discovery index — "which
// markets exist" — but historically it also doubled as the price
// source for yesProbability, yes_ask_dollars, volume_24h_fp, etc. That
// is dangerous: a user asking "what are the odds Lakers win tonight"
// could be served yesterday's cached prices with no signal they were
// stale.
//
// After ranking produces N hits, group them by venue and issue ONE
// batched API call per venue to refresh the price-bearing fields:
//   - Polymarket /markets?slug=a&slug=b&...
//   - Kalshi    /markets?tickers=a,b,c
//
// Replace the cached values on the in-memory hits before serialization.
// If a live fetch fails (timeout, 5xx, etc.), mark the affected rows
// with PriceSource="stale" and continue — the user still gets the
// index answer with an explicit staleness flag rather than the whole
// command failing.
//
// The result envelope on each command gains a `meta` field carrying
// `index_synced_at` (read from sync_state.last_synced_at) and
// `price_source` ("live", "stale", or "mixed" when one venue refreshed
// and the other failed). Human-mode terminal stdout also prints a
// one-line footer; --json/--agent mode does not, since the envelope
// already carries the same information.

package cli

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

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// PriceSource enumerates the freshness state for a single hit (or for
// a result bundle aggregated across hits).
const (
	priceSourceLive  = "live"  // the price-bearing fields were refreshed from the upstream API
	priceSourceStale = "stale" // the refresh attempt failed; the cached value is what is being returned
	priceSourceMixed = "mixed" // some venues refreshed, others did not
	priceSourceIndex = "index" // the hit has no price-bearing fields (e.g., a tags/series row)
)

// freshnessMeta is the envelope-level freshness metadata attached to
// every command that surfaces a price or implied probability. It is
// the additive extension to the per-command result struct that the
// plan calls for.
//
// IndexSyncedAt is the most recent sync_state.last_synced_at across
// the price-bearing resource types ("markets", "kalshi_markets"). It
// is the cache age for the discovery layer — an agent can use this
// to decide whether to trigger a `sync` before a high-stakes query.
//
// PriceSource is "live" when every venue queried in this command
// refreshed successfully; "stale" when none refreshed; "mixed" when
// one venue succeeded and the other failed; "index" when no
// price-bearing rows were returned at all (e.g., a topic query that
// only matched tag rows).
type freshnessMeta struct {
	PriceSource   string     `json:"price_source"`
	IndexSyncedAt *time.Time `json:"index_synced_at,omitempty"`
	// LearningsApplied is the count of search_learnings rules that
	// touched the output of this command. 0 when the rerank layer ran
	// but no rule fired; absent when --no-learn /
	// PREDICTION_GOAT_NO_LEARN disabled the layer. See teach.go.
	LearningsApplied int `json:"learnings_applied,omitempty"`
	// TeachHint is set when the LLM should record a learning for the
	// current query (no high-confidence boost already fired). Empty
	// when no hint is warranted — e.g., a high-confidence row already
	// covers this query, or no hits came back.
	TeachHint string `json:"teach_hint,omitempty"`
}

// liveValues holds the price-bearing fields refreshed from upstream.
// Zero-valued fields signal "no fresh value available" — the caller
// keeps whatever was already on the in-memory hit.
type liveValues struct {
	YesProbability float64
	Volume24h      float64
	Status         string
}

// hasValue reports whether at least one price-bearing field is set.
// Used to suppress overwriting a cached non-zero value with a zero
// live value (which happens when the upstream response omitted the
// field entirely rather than reporting it as 0).
func (v liveValues) hasValue() bool {
	return v.YesProbability != 0 || v.Volume24h != 0 || v.Status != ""
}

// freshnessClient bundles the HTTP endpoints both venues hit. It is
// an interface rather than a concrete struct so tests can swap in
// httptest fixtures.
type freshnessClient interface {
	// FetchPolymarket returns the refreshed price-bearing fields for the
	// given Polymarket slugs, keyed by slug. A non-nil error from the
	// upstream API (5xx, timeout, parse failure) is treated as a
	// per-venue refresh failure by the caller — slugs that the upstream
	// response simply did not include drop out of the map silently.
	FetchPolymarket(ctx context.Context, slugs []string) (map[string]liveValues, error)
	// FetchKalshi returns the refreshed price-bearing fields for the
	// given Kalshi tickers, keyed by ticker.
	FetchKalshi(ctx context.Context, tickers []string) (map[string]liveValues, error)
}

// defaultFreshnessClient is the production implementation that hits
// the live Polymarket gamma-api and Kalshi trade-api endpoints.
type defaultFreshnessClient struct {
	polymarketBaseURL string
	httpClient        *http.Client
	kalshiClient      *kalshi.Client
}

// newDefaultFreshnessClient returns a freshness client wired against
// the production endpoints. The Kalshi client is the same package
// already used by `kalshi events get --with-markets`; the Polymarket
// path uses a direct net/http GET because the existing
// internal/client.Client only accepts map[string]string for query
// params (no repeated keys), and Polymarket's /markets endpoint
// requires repeated `slug=` params for batch lookup.
func newDefaultFreshnessClient() *defaultFreshnessClient {
	return &defaultFreshnessClient{
		polymarketBaseURL: "https://gamma-api.polymarket.com",
		httpClient:        &http.Client{Timeout: 15 * time.Second},
		kalshiClient:      kalshi.New(),
	}
}

// FetchPolymarket issues a single GET against the Polymarket
// `/markets` endpoint with repeated `slug=` query params and returns
// a map keyed by slug. Slugs not present in the response (closed
// market, typo, partial-page truncation) drop out silently — the
// caller keeps the cached value and flags it as stale only if the
// whole batched call errored.
func (c *defaultFreshnessClient) FetchPolymarket(ctx context.Context, slugs []string) (map[string]liveValues, error) {
	if len(slugs) == 0 {
		return map[string]liveValues{}, nil
	}
	q := url.Values{}
	for _, s := range slugs {
		if s == "" {
			continue
		}
		q.Add("slug", s)
	}
	// Cap limit to len(slugs) so a partial response is still complete
	// for the requested slugs; gamma-api defaults to 100 otherwise.
	q.Set("limit", fmt.Sprintf("%d", len(slugs)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.polymarketBaseURL+"/markets?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("polymarket freshness build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "prediction-goat-pp-cli/1.0 freshness")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("polymarket freshness GET: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("polymarket freshness read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("polymarket freshness HTTP %d", resp.StatusCode)
	}
	return parsePolymarketLive(body)
}

// FetchKalshi issues a single GET against the Kalshi `/markets`
// endpoint with `tickers=` set to a comma-separated list and returns
// a map keyed by ticker. Reuses the existing kalshi.Client so adaptive
// rate-limiting and 429 retry behavior match the sync path.
func (c *defaultFreshnessClient) FetchKalshi(ctx context.Context, tickers []string) (map[string]liveValues, error) {
	if len(tickers) == 0 {
		return map[string]liveValues{}, nil
	}
	cleaned := make([]string, 0, len(tickers))
	for _, t := range tickers {
		if t != "" {
			cleaned = append(cleaned, t)
		}
	}
	if len(cleaned) == 0 {
		return map[string]liveValues{}, nil
	}
	params := url.Values{}
	params.Set("tickers", strings.Join(cleaned, ","))
	params.Set("limit", fmt.Sprintf("%d", len(cleaned)))
	body, err := c.kalshiClient.Get(ctx, "/markets", params)
	if err != nil {
		return nil, fmt.Errorf("kalshi freshness GET: %w", err)
	}
	return parseKalshiLive(body)
}

// parsePolymarketLive decodes the gamma-api /markets response (either
// a top-level array, or a wrapped {markets:[...]} envelope) and
// returns the price-bearing fields keyed by slug.
func parsePolymarketLive(body []byte) (map[string]liveValues, error) {
	// gamma-api currently returns a bare JSON array. Tolerate a
	// wrapped {markets:[...]} envelope too in case the shape changes.
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		var envelope struct {
			Markets []map[string]any `json:"markets"`
		}
		if err2 := json.Unmarshal(body, &envelope); err2 != nil {
			return nil, fmt.Errorf("polymarket freshness decode: %w", err)
		}
		arr = envelope.Markets
	}
	out := make(map[string]liveValues, len(arr))
	for _, obj := range arr {
		slug := jsonString(obj, "slug")
		if slug == "" {
			continue
		}
		out[slug] = liveValues{
			YesProbability: jsonFloat(obj, "lastTradePrice"),
			Volume24h:      firstFloat(obj, "volume24hr", "volumeNum"),
			Status:         pmStatus(obj),
		}
	}
	return out, nil
}

// parseKalshiLive decodes the Kalshi /markets response and returns
// the price-bearing fields keyed by ticker.
func parseKalshiLive(body []byte) (map[string]liveValues, error) {
	var resp struct {
		Markets []map[string]any `json:"markets"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kalshi freshness decode: %w", err)
	}
	out := make(map[string]liveValues, len(resp.Markets))
	for _, obj := range resp.Markets {
		ticker := jsonString(obj, "ticker")
		if ticker == "" {
			continue
		}
		out[ticker] = liveValues{
			YesProbability: jsonFloat(obj, "last_price_dollars"),
			Volume24h:      jsonFloat(obj, "volume_24h_fp"),
			Status:         jsonString(obj, "status"),
		}
	}
	return out, nil
}

// refreshOutcome captures the per-venue refresh result. Each command
// reads this to set the envelope-level price_source string and to
// decide which hits to mark as stale.
type refreshOutcome struct {
	Polymarket map[string]liveValues
	Kalshi     map[string]liveValues
	// PolymarketOK / KalshiOK report whether the batched fetch
	// succeeded for the venue. A nil/empty map with PolymarketOK=true
	// means "we asked and the upstream returned nothing", which is
	// not a refresh failure — the cached value stays.
	PolymarketOK bool
	KalshiOK     bool
	// PolymarketAsked / KalshiAsked report whether the command had any
	// hits for the venue at all. A venue that was not asked does not
	// contribute to the price_source aggregate.
	PolymarketAsked bool
	KalshiAsked     bool
}

// priceSourceLabel computes the envelope-level price_source for the
// command from the per-venue outcome. Rules:
//   - No venue asked → "index" (the result has no price-bearing rows).
//   - Every asked venue refreshed → "live".
//   - No asked venue refreshed → "stale".
//   - Some asked venues refreshed, others didn't → "mixed".
func (o refreshOutcome) priceSourceLabel() string {
	asked := 0
	ok := 0
	if o.PolymarketAsked {
		asked++
		if o.PolymarketOK {
			ok++
		}
	}
	if o.KalshiAsked {
		asked++
		if o.KalshiOK {
			ok++
		}
	}
	switch {
	case asked == 0:
		return priceSourceIndex
	case ok == asked:
		return priceSourceLive
	case ok == 0:
		return priceSourceStale
	default:
		return priceSourceMixed
	}
}

// refreshVenues runs the per-venue refresh fetches in parallel and
// returns the merged outcome. A nil client falls back to the
// production-wired implementation.
func refreshVenues(ctx context.Context, fc freshnessClient, polySlugs, kalshiTickers []string) refreshOutcome {
	if fc == nil {
		fc = newDefaultFreshnessClient()
	}
	out := refreshOutcome{
		PolymarketAsked: len(polySlugs) > 0,
		KalshiAsked:     len(kalshiTickers) > 0,
	}
	var wg sync.WaitGroup
	if out.PolymarketAsked {
		wg.Add(1)
		go func() {
			defer wg.Done()
			values, err := fc.FetchPolymarket(ctx, polySlugs)
			if err == nil {
				out.Polymarket = values
				out.PolymarketOK = true
			}
		}()
	}
	if out.KalshiAsked {
		wg.Add(1)
		go func() {
			defer wg.Done()
			values, err := fc.FetchKalshi(ctx, kalshiTickers)
			if err == nil {
				out.Kalshi = values
				out.KalshiOK = true
			}
		}()
	}
	wg.Wait()
	return out
}

// indexSyncedAt returns the most recent sync_state.last_synced_at
// across the price-bearing resource types. Used to populate
// meta.index_synced_at on the envelope. Returns nil when no sync has
// run yet (the price-bearing tables are empty, so a topic command
// would have returned zero hits anyway — but the envelope still
// surfaces the absence as null rather than synthesizing a fake age).
func indexSyncedAt(db *store.Store) *time.Time {
	if db == nil {
		return nil
	}
	var latest time.Time
	for _, rt := range []string{"markets", "kalshi_markets"} {
		_, ts, _, err := db.GetSyncState(rt)
		if err != nil {
			continue
		}
		if ts.IsZero() {
			continue
		}
		if latest.IsZero() || ts.After(latest) {
			latest = ts
		}
	}
	if latest.IsZero() {
		return nil
	}
	return &latest
}

// indexSyncedAtFromPath opens the local store at the given path (or
// the default) just long enough to read the most recent sync_state
// timestamp. Used by commands that already closed their store handle
// before building the response envelope.
func indexSyncedAtFromPath(ctx context.Context, dbPath string) *time.Time {
	if dbPath == "" {
		dbPath = defaultDBPath("prediction-goat-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()
	return indexSyncedAt(db)
}

// buildFreshnessMeta returns the envelope-level metadata for a
// command given the venue-refresh outcome and the local store's
// sync state.
func buildFreshnessMeta(outcome refreshOutcome, syncedAt *time.Time) *freshnessMeta {
	return &freshnessMeta{
		PriceSource:   outcome.priceSourceLabel(),
		IndexSyncedAt: syncedAt,
	}
}

// freshnessFooterLine returns the human-mode "Index synced 14h ago,
// prices live" footer for the given meta. Returns the empty string
// when the meta carries no useful information (no sync, no price
// source) — callers should suppress the print rather than emitting a
// confusing "Index never synced, prices index" line.
//
// Machine-format modes (--json, --csv, --quiet, --plain, --compact,
// piped stdout) MUST NOT print this footer; the JSON envelope's meta
// already carries the same information. See each command's render
// path for the gate.
func freshnessFooterLine(meta *freshnessMeta) string {
	if meta == nil {
		return ""
	}
	switch meta.PriceSource {
	case priceSourceLive, priceSourceStale, priceSourceMixed:
	default:
		return ""
	}
	age := "never synced"
	if meta.IndexSyncedAt != nil {
		age = humanIndexAge(time.Since(*meta.IndexSyncedAt)) + " ago"
	}
	label := "prices live"
	switch meta.PriceSource {
	case priceSourceStale:
		label = "prices stale (live refresh failed)"
	case priceSourceMixed:
		label = "prices mixed (one venue refresh failed)"
	}
	return fmt.Sprintf("Index synced %s, %s", age, label)
}

// humanIndexAge renders a duration in a one-line human-readable form
// for the cache-age footer. Tracks the existing convention in
// printProvenance for consistency.
func humanIndexAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// applyLiveValuesIfPresent updates dst with v's price-bearing fields
// when v carries fresh data. Callers pass per-field pointers so the
// command's per-shape hit struct stays the source of truth for what
// is a price-bearing field on this command. A zero v (no fresh
// value) leaves dst untouched.
func applyLiveValuesIfPresent(v liveValues, yesProb, volume *float64, status *string) {
	if !v.hasValue() {
		return
	}
	if v.YesProbability != 0 && yesProb != nil {
		*yesProb = v.YesProbability
	}
	if v.Volume24h != 0 && volume != nil {
		*volume = v.Volume24h
	}
	if v.Status != "" && status != nil {
		*status = v.Status
	}
}
