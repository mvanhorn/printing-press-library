// Copyright 2026 googio and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/config"
)

func TestCacheKeyPartitionsSearchLocationAndDevice(t *testing.T) {
	t.Parallel()

	c := &Client{BaseURL: "https://api.serply.io", cacheDir: t.TempDir()}
	params := map[string]string{"q": "serp api"}
	base := c.cacheKeyFor(http.MethodGet, "/v1/search", params, nil, nil)
	us := c.cacheKeyFor(http.MethodGet, "/v1/search", params, map[string]string{"X-Proxy-Location": "US"}, nil)
	gb := c.cacheKeyFor(http.MethodGet, "/v1/search", params, map[string]string{"X-Proxy-Location": "GB"}, nil)
	usFolded := c.cacheKeyFor(http.MethodGet, "/v1/search", params, map[string]string{"x-proxy-location": "us"}, nil)
	desktop := c.cacheKeyFor(http.MethodGet, "/v1/search", params, map[string]string{"X-User-Agent": "desktop"}, nil)
	mobile := c.cacheKeyFor(http.MethodGet, "/v1/search", params, map[string]string{"X-User-Agent": "mobile"}, nil)
	transportUA := c.cacheKeyFor(http.MethodGet, "/v1/search", params, map[string]string{"User-Agent": "desktop"}, nil)

	if us == base || us == gb {
		t.Fatalf("X-Proxy-Location did not partition the cache key: base %q us %q gb %q", base, us, gb)
	}
	if us != usFolded {
		t.Fatalf("location header case should share a cache key: %q != %q", us, usFolded)
	}
	if desktop == base || desktop == mobile || desktop == us {
		t.Fatalf("X-User-Agent did not partition the cache key: desktop %q mobile %q us %q", desktop, mobile, us)
	}
	if transportUA != base {
		t.Fatalf("transport User-Agent must not partition the search cache: %q != %q", transportUA, base)
	}

	fromConfig := &Client{
		BaseURL:  c.BaseURL,
		cacheDir: c.cacheDir,
		Config:   &config.Config{Headers: map[string]string{"X-Proxy-Location": "DE"}},
	}
	if fromConfig.cacheKeyFor(http.MethodGet, "/v1/search", params, nil, nil) == base {
		t.Fatal("config X-Proxy-Location did not partition the cache key")
	}

	c.writeCacheWithHeaders("/v1/search", params, map[string]string{"X-Proxy-Location": "US"}, json.RawMessage(`{"where":"US"}`))
	if got, ok := c.readCacheWithHeaders("/v1/search", params, map[string]string{"X-Proxy-Location": "GB"}); ok {
		t.Fatalf("GB read hit the US cache entry: %s", got)
	}
	got, ok := c.readCacheWithHeaders("/v1/search", params, map[string]string{"X-Proxy-Location": "us"})
	if !ok || string(got) != `{"where":"US"}` {
		t.Fatalf("US case variant missed its cache entry: %s ok=%v", got, ok)
	}
	c.writeCacheWithHeaders("/v1/search", params, map[string]string{"X-User-Agent": "desktop"}, json.RawMessage(`{"device":"desktop"}`))
	if _, ok := c.readCacheWithHeaders("/v1/search", params, map[string]string{"X-User-Agent": "mobile"}); ok {
		t.Fatal("mobile read hit the desktop cache entry")
	}
}
