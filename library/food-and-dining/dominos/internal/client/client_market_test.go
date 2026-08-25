// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/config"
)

func TestCanadianMarketHeadersAreSent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("DPZ-Market"); got != "CANADA" {
			t.Errorf("DPZ-Market = %q", got)
		}
		if got := r.Header.Get("Market"); got != "CANADA" {
			t.Errorf("Market = %q", got)
		}
		if got := r.Header.Get("DPZ-Language"); got != "en" {
			t.Errorf("DPZ-Language = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	cfg := &config.Config{BaseURL: server.URL, Market: config.MarketCanada}
	c := New(cfg, time.Second, 0)
	c.NoCache = true

	_, err := c.Get("/power/test", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUSMarketPreservesExistingHeaderBehavior(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, header := range []string{"DPZ-Market", "Market", "DPZ-Language"} {
			if got := r.Header.Get(header); got != "" {
				t.Errorf("%s = %q", header, got)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	cfg := &config.Config{BaseURL: server.URL, Market: config.MarketUS}
	c := New(cfg, time.Second, 0)
	c.NoCache = true

	_, err := c.Get("/power/test", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEndpointHeadersOverrideCanadianDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("DPZ-Language"); got != "fr" {
			t.Errorf("DPZ-Language = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	cfg := &config.Config{BaseURL: server.URL, Market: config.MarketCanada}
	c := New(cfg, time.Second, 0)
	c.NoCache = true

	_, err := c.GetWithHeaders("/power/test", nil, map[string]string{"DPZ-Language": "fr"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCacheKeyIncludesBaseURLAndMarket(t *testing.T) {
	us := &Client{BaseURL: config.USBaseURL, Config: &config.Config{Market: config.MarketUS}}
	canada := &Client{BaseURL: config.CanadaBaseURL, Config: &config.Config{Market: config.MarketCanada}}
	canadaOnAlternateBase := &Client{BaseURL: "https://alternate.example", Config: &config.Config{Market: config.MarketCanada}}

	usKey := us.cacheKey("/power/store/1/menu", map[string]string{"lang": "en"})
	canadaKey := canada.cacheKey("/power/store/1/menu", map[string]string{"lang": "en"})
	alternateKey := canadaOnAlternateBase.cacheKey("/power/store/1/menu", map[string]string{"lang": "en"})
	if usKey == canadaKey || canadaKey == alternateKey || usKey == alternateKey {
		t.Fatalf("cache keys must differ by market and base URL: us=%q ca=%q alternate=%q", usKey, canadaKey, alternateKey)
	}
}

func TestCacheKeyIsDeterministicAndUnambiguous(t *testing.T) {
	c := &Client{BaseURL: config.CanadaBaseURL, Config: &config.Config{Market: config.MarketCanada}}
	params := map[string]string{"lang": "en", "limit": "5"}
	want := c.cacheKey("/power/customer/id/order", params)
	for range 100 {
		if got := c.cacheKey("/power/customer/id/order", params); got != want {
			t.Fatalf("cache key changed between identical calls: got %q want %q", got, want)
		}
	}
	first := c.cacheKey("/test", map[string]string{"a": "bc", "d": "e"})
	second := c.cacheKey("/test", map[string]string{"a": "b", "cd": "e"})
	if first == second {
		t.Fatalf("distinct query maps produced the same cache key: %q", first)
	}
}

func TestGetUncachedBypassesStoredResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"source":"live"}`))
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL, Market: config.MarketCanada}, time.Second, 0)
	c.cacheDir = t.TempDir()
	c.writeCache("/power/customer/id/card", nil, json.RawMessage(`{"source":"cached","CardID":"must-not-return"}`))

	got, err := c.GetUncached("/power/customer/id/card", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"source":"live"}` {
		t.Fatalf("GetUncached returned %s", got)
	}
}

func TestSensitiveDryRunHeadersAreRecognized(t *testing.T) {
	for _, name := range []string{"Authorization", "Cookie", "X-API-Key", "X-Auth-Token", "Client-Secret"} {
		if !isSensitiveHeader(name) {
			t.Errorf("header %q was not treated as sensitive", name)
		}
	}
	for _, name := range []string{"DPZ-Market", "Market", "DPZ-Language", "Accept"} {
		if isSensitiveHeader(name) {
			t.Errorf("safe header %q was treated as sensitive", name)
		}
	}
}

func TestNilConfigDoesNotPanicWhenApplyingDefaultHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	c := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		NoCache:    true,
		limiter:    newTestLimiter(),
	}
	_, err := c.Get("/test", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func newTestLimiter() *cliutil.AdaptiveLimiter {
	return cliutil.NewAdaptiveLimiter(0)
}
