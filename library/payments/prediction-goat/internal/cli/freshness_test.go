// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// fakeFreshnessClient is a hand-written stub so tests don't have to
// spin up a real httptest server for every refresh case. The httptest
// server is exercised by TestDefaultFreshnessClient_FetchPolymarket /
// _FetchKalshi below to pin the URL shape and decoding.
type fakeFreshnessClient struct {
	pmReturn      map[string]liveValues
	pmErr         error
	ksReturn      map[string]liveValues
	ksErr         error
	pmSlugsAsked  []string
	ksTickerAsked []string
}

func (f *fakeFreshnessClient) FetchPolymarket(ctx context.Context, slugs []string) (map[string]liveValues, error) {
	f.pmSlugsAsked = append(f.pmSlugsAsked, slugs...)
	if f.pmErr != nil {
		return nil, f.pmErr
	}
	return f.pmReturn, nil
}

func (f *fakeFreshnessClient) FetchKalshi(ctx context.Context, tickers []string) (map[string]liveValues, error) {
	f.ksTickerAsked = append(f.ksTickerAsked, tickers...)
	if f.ksErr != nil {
		return nil, f.ksErr
	}
	return f.ksReturn, nil
}

// TestRefreshTopicHits_OverwritesPolymarketWithLiveValue covers the
// canonical regression scenario from the plan: a cached Polymarket
// markets row with lastTradePrice=0.05 returns YesProbability matching
// the live API value when topic queries it.
func TestRefreshTopicHits_OverwritesPolymarketWithLiveValue(t *testing.T) {
	t.Parallel()
	hits := []topicHit{
		{Source: "polymarket", Kind: "market", ID: "lakers-tonight", Title: "Lakers win tonight", YesProbability: 0.05, Volume24h: 1000},
	}
	fc := &fakeFreshnessClient{
		pmReturn: map[string]liveValues{
			"lakers-tonight": {YesProbability: 0.55, Volume24h: 9999, Status: "active"},
		},
	}
	outcome := refreshTopicHits(context.Background(), fc, hits)
	if hits[0].YesProbability != 0.55 {
		t.Errorf("YesProbability = %v, want 0.55 (cached value was not overwritten)", hits[0].YesProbability)
	}
	if hits[0].Volume24h != 9999 {
		t.Errorf("Volume24h = %v, want 9999", hits[0].Volume24h)
	}
	if hits[0].PriceSource != priceSourceLive {
		t.Errorf("PriceSource = %q, want %q", hits[0].PriceSource, priceSourceLive)
	}
	if !outcome.PolymarketOK {
		t.Error("outcome.PolymarketOK = false, want true")
	}
	if outcome.priceSourceLabel() != priceSourceLive {
		t.Errorf("priceSourceLabel = %q, want live", outcome.priceSourceLabel())
	}
}

// TestRefreshTopicHits_OverwritesKalshiWithLiveValue mirrors the
// Polymarket case for a Kalshi cached row.
func TestRefreshTopicHits_OverwritesKalshiWithLiveValue(t *testing.T) {
	t.Parallel()
	hits := []topicHit{
		{Source: "kalshi", Kind: "market", ID: "KXPT-26", Title: "Portugal win", YesProbability: 0.085},
	}
	fc := &fakeFreshnessClient{
		ksReturn: map[string]liveValues{
			"KXPT-26": {YesProbability: 0.098, Volume24h: 1234},
		},
	}
	outcome := refreshTopicHits(context.Background(), fc, hits)
	if hits[0].YesProbability != 0.098 {
		t.Errorf("YesProbability = %v, want 0.098", hits[0].YesProbability)
	}
	if hits[0].PriceSource != priceSourceLive {
		t.Errorf("PriceSource = %q, want live", hits[0].PriceSource)
	}
	if !outcome.KalshiOK {
		t.Error("outcome.KalshiOK = false, want true")
	}
}

// TestRefreshTopicHits_OneVenueFailureDegradesOnlyThatVenue covers
// the plan's degrade-on-failure contract: when the Polymarket refresh
// fails, only its rows get price_source="stale" and Kalshi rows
// remain "live".
func TestRefreshTopicHits_OneVenueFailureDegradesOnlyThatVenue(t *testing.T) {
	t.Parallel()
	hits := []topicHit{
		{Source: "polymarket", Kind: "market", ID: "trump-2028", YesProbability: 0.42},
		{Source: "kalshi", Kind: "market", ID: "KXPT-26", YesProbability: 0.10},
	}
	fc := &fakeFreshnessClient{
		pmErr: errors.New("upstream 503"),
		ksReturn: map[string]liveValues{
			"KXPT-26": {YesProbability: 0.18},
		},
	}
	outcome := refreshTopicHits(context.Background(), fc, hits)
	// Polymarket row should be flagged stale and keep its cached value.
	if hits[0].PriceSource != priceSourceStale {
		t.Errorf("PM PriceSource = %q, want stale", hits[0].PriceSource)
	}
	if hits[0].YesProbability != 0.42 {
		t.Errorf("PM YesProbability = %v, want 0.42 (cached) (stale flag must not erase the cached value)", hits[0].YesProbability)
	}
	// Kalshi row should be live and overwritten.
	if hits[1].PriceSource != priceSourceLive {
		t.Errorf("KS PriceSource = %q, want live", hits[1].PriceSource)
	}
	if hits[1].YesProbability != 0.18 {
		t.Errorf("KS YesProbability = %v, want 0.18 (live)", hits[1].YesProbability)
	}
	// Envelope-level label should be "mixed" since one venue succeeded
	// and one failed.
	if got := outcome.priceSourceLabel(); got != priceSourceMixed {
		t.Errorf("priceSourceLabel = %q, want mixed", got)
	}
}

// TestRefreshTopicHits_NonMarketKindHasNoPriceSource pins the
// contract that tag/event/series rows leave price_source empty so an
// agent can tell a tag-row apart from a market-row whose refresh
// failed.
func TestRefreshTopicHits_NonMarketKindHasNoPriceSource(t *testing.T) {
	t.Parallel()
	hits := []topicHit{
		{Source: "polymarket", Kind: "tag", ID: "elections"},
		{Source: "kalshi", Kind: "series", ID: "KXMENWORLDCUP"},
	}
	fc := &fakeFreshnessClient{}
	refreshTopicHits(context.Background(), fc, hits)
	for i, h := range hits {
		if h.PriceSource != "" {
			t.Errorf("hits[%d] (kind=%s) PriceSource = %q, want empty", i, h.Kind, h.PriceSource)
		}
	}
}

// TestRefreshTopicHits_AllVenuesFailureLabelsStale covers the
// envelope-level rollup when both venues' refresh attempts fail.
func TestRefreshTopicHits_AllVenuesFailureLabelsStale(t *testing.T) {
	t.Parallel()
	hits := []topicHit{
		{Source: "polymarket", Kind: "market", ID: "a", YesProbability: 0.1},
		{Source: "kalshi", Kind: "market", ID: "b", YesProbability: 0.2},
	}
	fc := &fakeFreshnessClient{pmErr: errors.New("timeout"), ksErr: errors.New("timeout")}
	outcome := refreshTopicHits(context.Background(), fc, hits)
	if got := outcome.priceSourceLabel(); got != priceSourceStale {
		t.Errorf("priceSourceLabel = %q, want stale", got)
	}
	if hits[0].PriceSource != priceSourceStale || hits[1].PriceSource != priceSourceStale {
		t.Errorf("per-row PriceSource not stale: %v / %v", hits[0].PriceSource, hits[1].PriceSource)
	}
}

// TestRefreshOutcome_PriceSourceLabel pins the per-venue label
// aggregate against every (asked, ok) combination.
func TestRefreshOutcome_PriceSourceLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		o    refreshOutcome
		want string
	}{
		{"neither asked", refreshOutcome{}, priceSourceIndex},
		{"only pm asked + ok", refreshOutcome{PolymarketAsked: true, PolymarketOK: true}, priceSourceLive},
		{"only pm asked + failed", refreshOutcome{PolymarketAsked: true, PolymarketOK: false}, priceSourceStale},
		{"both asked + both ok", refreshOutcome{PolymarketAsked: true, PolymarketOK: true, KalshiAsked: true, KalshiOK: true}, priceSourceLive},
		{"both asked + pm failed", refreshOutcome{PolymarketAsked: true, PolymarketOK: false, KalshiAsked: true, KalshiOK: true}, priceSourceMixed},
		{"both asked + both failed", refreshOutcome{PolymarketAsked: true, KalshiAsked: true}, priceSourceStale},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.o.priceSourceLabel(); got != c.want {
				t.Errorf("priceSourceLabel = %q, want %q", got, c.want)
			}
		})
	}
}

// TestParsePolymarketLive_DecodesArray covers the canonical gamma-api
// response shape (bare JSON array) and confirms slug→liveValues
// mapping.
func TestParsePolymarketLive_DecodesArray(t *testing.T) {
	t.Parallel()
	body := []byte(`[{"slug":"foo","lastTradePrice":0.42,"volume24hr":1000,"closed":false,"active":true},{"slug":"bar","lastTradePrice":0.08,"volumeNum":500,"closed":true}]`)
	got, err := parsePolymarketLive(body)
	if err != nil {
		t.Fatalf("parsePolymarketLive: %v", err)
	}
	if got["foo"].YesProbability != 0.42 {
		t.Errorf("foo.YesProbability = %v, want 0.42", got["foo"].YesProbability)
	}
	if got["foo"].Volume24h != 1000 {
		t.Errorf("foo.Volume24h = %v, want 1000", got["foo"].Volume24h)
	}
	if got["bar"].YesProbability != 0.08 {
		t.Errorf("bar.YesProbability = %v, want 0.08", got["bar"].YesProbability)
	}
	if got["bar"].Volume24h != 500 {
		t.Errorf("bar.Volume24h = %v, want 500", got["bar"].Volume24h)
	}
}

// TestParseKalshiLive_DecodesEnvelope covers the canonical Kalshi
// /markets envelope shape and confirms ticker→liveValues mapping.
func TestParseKalshiLive_DecodesEnvelope(t *testing.T) {
	t.Parallel()
	body := []byte(`{"markets":[{"ticker":"KXA","last_price_dollars":0.61,"volume_24h_fp":12345,"status":"active"},{"ticker":"KXB","last_price_dollars":0.09,"volume_24h_fp":777,"status":"settled"}]}`)
	got, err := parseKalshiLive(body)
	if err != nil {
		t.Fatalf("parseKalshiLive: %v", err)
	}
	if got["KXA"].YesProbability != 0.61 {
		t.Errorf("KXA.YesProbability = %v, want 0.61", got["KXA"].YesProbability)
	}
	if got["KXA"].Status != "active" {
		t.Errorf("KXA.Status = %q, want active", got["KXA"].Status)
	}
	if got["KXB"].YesProbability != 0.09 {
		t.Errorf("KXB.YesProbability = %v, want 0.09", got["KXB"].YesProbability)
	}
}

// TestDefaultFreshnessClient_FetchPolymarket_HitsExpectedURL pins the
// URL shape the Polymarket refresh hits — repeated slug= query
// params, not slugs=. Uses httptest so a regression that switches to
// the wrong shape fails here rather than silently returning empty
// from the live API.
func TestDefaultFreshnessClient_FetchPolymarket_HitsExpectedURL(t *testing.T) {
	t.Parallel()
	var captured *url.URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[{"slug":"a","lastTradePrice":0.7},{"slug":"b","lastTradePrice":0.3}]`)
	}))
	defer srv.Close()
	fc := &defaultFreshnessClient{
		polymarketBaseURL: srv.URL,
		httpClient:        srv.Client(),
		kalshiClient:      kalshi.New(),
	}
	got, err := fc.FetchPolymarket(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("FetchPolymarket: %v", err)
	}
	if got["a"].YesProbability != 0.7 || got["b"].YesProbability != 0.3 {
		t.Errorf("returned values wrong: %+v", got)
	}
	if captured == nil {
		t.Fatal("request did not reach test server")
	}
	if !strings.HasPrefix(captured.Path, "/markets") {
		t.Errorf("path = %q, want /markets prefix", captured.Path)
	}
	q := captured.Query()
	slugs := q["slug"]
	if len(slugs) != 2 || slugs[0] != "a" || slugs[1] != "b" {
		t.Errorf("slug query params = %v, want [a b]", slugs)
	}
}

// TestDefaultFreshnessClient_FetchPolymarket_SkipsOn5xx pins the
// degrade-not-fail contract: a 5xx from the upstream returns an
// error so the caller can flag rows as stale rather than the whole
// command failing.
func TestDefaultFreshnessClient_FetchPolymarket_SkipsOn5xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	fc := &defaultFreshnessClient{
		polymarketBaseURL: srv.URL,
		httpClient:        srv.Client(),
		kalshiClient:      kalshi.New(),
	}
	_, err := fc.FetchPolymarket(context.Background(), []string{"a"})
	if err == nil {
		t.Fatal("expected error on 5xx, got nil")
	}
}

// TestDefaultFreshnessClient_FetchKalshi_HitsExpectedURL pins the
// URL shape the Kalshi refresh hits — tickers= comma-separated, not
// repeated ticker= params.
func TestDefaultFreshnessClient_FetchKalshi_HitsExpectedURL(t *testing.T) {
	t.Parallel()
	var captured *url.URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"markets":[{"ticker":"KXA","last_price_dollars":0.5}]}`)
	}))
	defer srv.Close()
	ksClient := kalshi.New()
	ksClient.BaseURL = srv.URL
	ksClient.HTTPClient = srv.Client()
	fc := &defaultFreshnessClient{
		polymarketBaseURL: "unused",
		httpClient:        srv.Client(),
		kalshiClient:      ksClient,
	}
	got, err := fc.FetchKalshi(context.Background(), []string{"KXA", "KXB"})
	if err != nil {
		t.Fatalf("FetchKalshi: %v", err)
	}
	if got["KXA"].YesProbability != 0.5 {
		t.Errorf("KXA.YesProbability = %v, want 0.5", got["KXA"].YesProbability)
	}
	if captured == nil {
		t.Fatal("request did not reach test server")
	}
	if got, want := captured.Query().Get("tickers"), "KXA,KXB"; got != want {
		t.Errorf("tickers query param = %q, want %q", got, want)
	}
}

// TestIndexSyncedAt_PopulatesFromMostRecentSyncState writes two rows
// to sync_state and confirms indexSyncedAt returns the later of the
// two timestamps for the price-bearing tables.
func TestIndexSyncedAt_PopulatesFromMostRecentSyncState(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer db.Close()
	// kalshi_markets synced just now; markets synced a long time ago.
	// The later of the two wins.
	if err := db.SaveSyncState("kalshi_markets", "", 100); err != nil {
		t.Fatalf("SaveSyncState kalshi_markets: %v", err)
	}
	now := time.Now()
	got := indexSyncedAt(db)
	if got == nil {
		t.Fatal("indexSyncedAt = nil, want non-nil")
	}
	// Confirm it landed within a small window of now (SaveSyncState
	// uses time.Now()).
	if got.Before(now.Add(-2*time.Minute)) || got.After(now.Add(2*time.Minute)) {
		t.Errorf("indexSyncedAt = %v, want near %v", *got, now)
	}
}

// TestIndexSyncedAt_NilWhenNoSyncStateRows returns nil when no
// sync has run, so the envelope surfaces "never synced" rather than
// the zero-value time.Time.
func TestIndexSyncedAt_NilWhenNoSyncStateRows(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer db.Close()
	if got := indexSyncedAt(db); got != nil {
		t.Errorf("indexSyncedAt = %v, want nil", got)
	}
}

// TestFreshnessFooterLine_IncludesAgeAndSource sanity-checks the
// human-mode footer string.
func TestFreshnessFooterLine_IncludesAgeAndSource(t *testing.T) {
	t.Parallel()
	syncedAt := time.Now().Add(-14 * time.Hour)
	meta := &freshnessMeta{PriceSource: priceSourceLive, IndexSyncedAt: &syncedAt}
	got := freshnessFooterLine(meta)
	if !strings.Contains(got, "Index synced") || !strings.Contains(got, "prices live") {
		t.Errorf("footer = %q, want both 'Index synced' and 'prices live'", got)
	}
}

// TestFreshnessFooterLine_EmptyOnIndexOnly suppresses the footer for
// no-price-bearing-rows results (an index-only response with no
// markets), since the "prices live" suffix would be misleading.
func TestFreshnessFooterLine_EmptyOnIndexOnly(t *testing.T) {
	t.Parallel()
	meta := &freshnessMeta{PriceSource: priceSourceIndex}
	if got := freshnessFooterLine(meta); got != "" {
		t.Errorf("footer = %q, want empty", got)
	}
}

// TestTopicResult_MetaSurfacesInJSON pins the JSON envelope shape:
// `meta.price_source` and `meta.index_synced_at` are visible on a
// topicResult marshalled to JSON, so agents can read them without
// the human-mode footer.
func TestTopicResult_MetaSurfacesInJSON(t *testing.T) {
	t.Parallel()
	syncedAt := time.Date(2026, 5, 23, 14, 0, 0, 0, time.UTC)
	result := topicResult{
		Topic: "lakers",
		Count: 0,
		Hits:  []topicHit{},
		Meta:  &freshnessMeta{PriceSource: priceSourceLive, IndexSyncedAt: &syncedAt},
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	meta, ok := decoded["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta not a map, got %T (raw=%s)", decoded["meta"], string(raw))
	}
	if meta["price_source"] != priceSourceLive {
		t.Errorf("meta.price_source = %v, want %q", meta["price_source"], priceSourceLive)
	}
	if _, ok := meta["index_synced_at"]; !ok {
		t.Errorf("meta missing index_synced_at: %v", meta)
	}
}

// TestRefreshComparePairs_RecomputesDelta covers the contract that
// the compare DeltaPct is recomputed from refreshed prices so it
// never reports a stale spread.
func TestRefreshComparePairs_RecomputesDelta(t *testing.T) {
	t.Parallel()
	cachedDelta := 5.0
	pairs := []comparePair{{
		Topic:    "world cup",
		PM:       &compareVenue{ID: "pt-pm", YesProbability: 0.10},
		Kalshi:   &compareVenue{ID: "KXPT-26", YesProbability: 0.05},
		DeltaPct: &cachedDelta,
	}}
	fc := &fakeFreshnessClient{
		pmReturn: map[string]liveValues{"pt-pm": {YesProbability: 0.20}},
		ksReturn: map[string]liveValues{"KXPT-26": {YesProbability: 0.08}},
	}
	refreshComparePairs(context.Background(), fc, pairs)
	if pairs[0].PM.YesProbability != 0.20 {
		t.Errorf("PM = %v, want 0.20", pairs[0].PM.YesProbability)
	}
	if pairs[0].Kalshi.YesProbability != 0.08 {
		t.Errorf("Kalshi = %v, want 0.08", pairs[0].Kalshi.YesProbability)
	}
	if pairs[0].DeltaPct == nil {
		t.Fatal("DeltaPct = nil")
	}
	// New spread should be (0.20 - 0.08) * 100 = 12.0
	if got := *pairs[0].DeltaPct; got < 11.99 || got > 12.01 {
		t.Errorf("DeltaPct = %v, want ~12.0 (live spread)", got)
	}
}

// TestRefreshMispricedPairs_DropsPairsThatFallBelowThresholdLive
// covers the contract that pairs whose spread falls below the
// user-supplied threshold under refreshed prices are filtered out
// of the result.
func TestRefreshMispricedPairs_DropsPairsThatFallBelowThresholdLive(t *testing.T) {
	t.Parallel()
	result := &mispricedResult{
		Threshold: 0.10,
		Pairs: []mispricedPair{
			{PM: compareVenue{ID: "a-pm", YesProbability: 0.30}, Kalshi: compareVenue{ID: "a-ks", YesProbability: 0.10}, Delta: 0.20},
			{PM: compareVenue{ID: "b-pm", YesProbability: 0.50}, Kalshi: compareVenue{ID: "b-ks", YesProbability: 0.40}, Delta: 0.10},
		},
	}
	fc := &fakeFreshnessClient{
		pmReturn: map[string]liveValues{
			"a-pm": {YesProbability: 0.35}, // refreshed: still over threshold (0.35 - 0.10 = 0.25)
			"b-pm": {YesProbability: 0.41}, // refreshed: closes to 0.01 spread → drop
		},
		ksReturn: map[string]liveValues{
			"a-ks": {YesProbability: 0.10},
			"b-ks": {YesProbability: 0.40},
		},
	}
	refreshMispricedPairs(context.Background(), fc, result, 0.10)
	if len(result.Pairs) != 1 {
		t.Fatalf("Pairs len = %d, want 1 (the b-pm row should be dropped under refreshed prices)", len(result.Pairs))
	}
	if result.Pairs[0].PM.ID != "a-pm" {
		t.Errorf("surviving pair PM.ID = %q, want a-pm", result.Pairs[0].PM.ID)
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// TestRefreshMarketScreenItems_FlagsStaleWhenVenueFails covers the
// degrade-not-fail contract on the shared trending/movers/resolving/
// liquid/new path.
func TestRefreshMarketScreenItems_FlagsStaleWhenVenueFails(t *testing.T) {
	t.Parallel()
	items := []marketScreenItem{
		{Source: "polymarket", ID: "pm-1", YesProbability: 0.1},
		{Source: "kalshi", ID: "kx-1", YesProbability: 0.2},
	}
	fc := &fakeFreshnessClient{
		pmErr: errors.New("upstream timeout"),
		ksReturn: map[string]liveValues{
			"kx-1": {YesProbability: 0.99},
		},
	}
	outcome := refreshMarketScreenItems(context.Background(), fc, items)
	if items[0].PriceSource != priceSourceStale {
		t.Errorf("pm-1 PriceSource = %q, want stale", items[0].PriceSource)
	}
	if items[0].YesProbability != 0.1 {
		t.Errorf("pm-1 YesProbability = %v, want 0.1 (cached)", items[0].YesProbability)
	}
	if items[1].PriceSource != priceSourceLive {
		t.Errorf("kx-1 PriceSource = %q, want live", items[1].PriceSource)
	}
	if items[1].YesProbability != 0.99 {
		t.Errorf("kx-1 YesProbability = %v, want 0.99", items[1].YesProbability)
	}
	if got := outcome.priceSourceLabel(); got != priceSourceMixed {
		t.Errorf("priceSourceLabel = %q, want mixed", got)
	}
}

// TestRefreshMoversItems_OverwritesCurrentPrice covers the movers
// shape: CurrentPrice (not YesProbability) is the price-bearing
// field; the refresh must overwrite it.
func TestRefreshMoversItems_OverwritesCurrentPrice(t *testing.T) {
	t.Parallel()
	items := []moversItem{
		{Source: "polymarket", ID: "x", CurrentPrice: 0.1, Volume24h: 100},
	}
	fc := &fakeFreshnessClient{
		pmReturn: map[string]liveValues{"x": {YesProbability: 0.42, Volume24h: 200}},
	}
	refreshMoversItems(context.Background(), fc, items)
	if items[0].CurrentPrice != 0.42 {
		t.Errorf("CurrentPrice = %v, want 0.42 (movers maps YesProbability live value to CurrentPrice)", items[0].CurrentPrice)
	}
	if items[0].Volume24h != 200 {
		t.Errorf("Volume24h = %v, want 200", items[0].Volume24h)
	}
	if items[0].PriceSource != priceSourceLive {
		t.Errorf("PriceSource = %q, want live", items[0].PriceSource)
	}
}

// TestRefreshVenues_EmptySlicesNeverCallTheClient suppresses upstream
// traffic when there are no hits for a venue.
func TestRefreshVenues_EmptySlicesNeverCallTheClient(t *testing.T) {
	t.Parallel()
	fc := &fakeFreshnessClient{
		pmReturn: map[string]liveValues{},
		ksReturn: map[string]liveValues{},
	}
	o := refreshVenues(context.Background(), fc, nil, nil)
	if o.PolymarketAsked || o.KalshiAsked {
		t.Errorf("asked flags = (%v, %v), want both false", o.PolymarketAsked, o.KalshiAsked)
	}
	if got := o.priceSourceLabel(); got != priceSourceIndex {
		t.Errorf("priceSourceLabel = %q, want index", got)
	}
	if len(fc.pmSlugsAsked) != 0 || len(fc.ksTickerAsked) != 0 {
		t.Errorf("client was called: pm=%v ks=%v", fc.pmSlugsAsked, fc.ksTickerAsked)
	}
}

// TestApplyLiveValuesIfPresent_KeepsCachedWhenLiveZero pins that a
// zero live value (upstream omitted the field) does not overwrite a
// cached non-zero value.
func TestApplyLiveValuesIfPresent_KeepsCachedWhenLiveZero(t *testing.T) {
	t.Parallel()
	yes := 0.5
	vol := 1000.0
	status := "active"
	applyLiveValuesIfPresent(liveValues{}, &yes, &vol, &status)
	if yes != 0.5 || vol != 1000.0 || status != "active" {
		t.Errorf("zero liveValues overwrote cached values: yes=%v vol=%v status=%q", yes, vol, status)
	}
}
