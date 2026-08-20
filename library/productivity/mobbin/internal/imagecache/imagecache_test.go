// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package imagecache

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// CDN URL rewriting must turn Supabase storage paths into the Bytescale CDN
// path, with the `content/...` suffix preserved verbatim. Drift here silently
// breaks every saved-image command.
func TestToCDNURL_SupabaseRewrite(t *testing.T) {
	in := "https://ujasntkfphywizsdaapi.supabase.co/storage/v1/object/public/content/app_screens/abc.png"
	got := ToCDNURL(in, CDNOpts{Width: 800, Quality: 80, Format: "webp"})
	if !strings.HasPrefix(got, BytescaleBase+"/content/app_screens/abc.png") {
		t.Fatalf("rewrite dropped content prefix: %s", got)
	}
	if !strings.Contains(got, "w=800") || !strings.Contains(got, "q=80") || !strings.Contains(got, "f=webp") {
		t.Fatalf("missing CDN query params: %s", got)
	}
}

// Already-CDN URLs should round-trip with query opts applied, not double-prefix.
func TestToCDNURL_AlreadyCDN(t *testing.T) {
	in := BytescaleBase + "/content/app_screens/x.png"
	got := ToCDNURL(in, CDNOpts{Width: 100})
	if strings.Count(got, BytescaleBase) != 1 {
		t.Fatalf("double-prefixed: %s", got)
	}
}

// Cache paths must be platform-aware and sanitised against path traversal.
func TestCachePath(t *testing.T) {
	c := &Cache{Root: "/tmp/x"}
	p := c.Path("web", "stripe", "id-1", "webp")
	if filepath.Base(p) != "id-1.webp" {
		t.Fatalf("unexpected basename: %s", p)
	}
	// Path traversal in slug must be neutralised.
	p = c.Path("web", "../etc", "id", "png")
	if strings.Contains(p, "..") {
		t.Fatalf("traversal not sanitised: %s", p)
	}
}

// 429 from the image CDN must surface as a typed RateLimitError so callers
// can back off rather than treating throttling as a missing image.
func TestFetch_429ReturnsRateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := &Cache{Root: t.TempDir()}
	_, _, err := c.Fetch(context.Background(), srv.URL+"/img.png", "web", "app", "id", CDNOpts{})
	if err == nil {
		t.Fatal("expected error on 429")
	}
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("not a RateLimitError: %T %v", err, err)
	}
	if rle.RetryAfter != "5" {
		t.Fatalf("Retry-After not preserved: %q", rle.RetryAfter)
	}
}
