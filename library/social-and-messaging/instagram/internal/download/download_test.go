// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package download

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
)

// TestParseSlideRange covers the carousel-slide range parser, the helper that
// translates user-typed "1-3" / "2" into an internal [lo, hi] range.
func TestParseSlideRange(t *testing.T) {
	cases := []struct {
		in      string
		wantLo  int
		wantHi  int
		wantErr bool
	}{
		{"", 0, 0, false},
		{"1", 1, 1, false},
		{"3", 3, 3, false},
		{"1-3", 1, 3, false},
		{"2-5", 2, 5, false},
		{"0", 0, 0, true},
		{"-1", 0, 0, true},
		{"3-1", 0, 0, true},
		{"abc", 0, 0, true},
		{"1-", 0, 0, true},
	}
	for _, tc := range cases {
		lo, hi, err := parseSlideRange(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseSlideRange(%q): expected error, got lo=%d hi=%d", tc.in, lo, hi)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSlideRange(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if lo != tc.wantLo || hi != tc.wantHi {
			t.Errorf("parseSlideRange(%q) = (%d,%d); want (%d,%d)", tc.in, lo, hi, tc.wantLo, tc.wantHi)
		}
	}
}

// TestPickBestImage asserts the highest-width candidate wins, regardless of
// the input order. IG normally returns candidates largest-first but we sort
// defensively so a future order change can't silently downgrade.
func TestPickBestImage(t *testing.T) {
	cands := []ImageCandidate{
		{URL: "small", Width: 320, Height: 320},
		{URL: "huge", Width: 1080, Height: 1080},
		{URL: "med", Width: 640, Height: 640},
	}
	best := pickBestImage(cands)
	if best == nil {
		t.Fatal("pickBestImage returned nil for non-empty slice")
	}
	if best.URL != "huge" {
		t.Errorf("pickBestImage chose %q; want huge", best.URL)
	}
	if pickBestImage(nil) != nil {
		t.Error("pickBestImage(nil) should return nil")
	}
}

// TestPickBestVideo asserts the highest-bandwidth video wins, then falls
// through to pixel width when bandwidth is missing.
func TestPickBestVideo(t *testing.T) {
	vs := []VideoVersion{
		{URL: "low", Width: 720, Bandwidth: 500_000},
		{URL: "hi", Width: 1080, Bandwidth: 2_500_000},
		{URL: "mid", Width: 720, Bandwidth: 1_000_000},
	}
	best := pickBestVideo(vs)
	if best == nil || best.URL != "hi" {
		t.Fatalf("pickBestVideo chose %+v; want hi", best)
	}

	// Bandwidth zero — should fall back to width.
	vs2 := []VideoVersion{
		{URL: "smallpx", Width: 480},
		{URL: "bigpx", Width: 1280},
	}
	best2 := pickBestVideo(vs2)
	if best2 == nil || best2.URL != "bigpx" {
		t.Fatalf("pickBestVideo (no bandwidth) chose %+v; want bigpx", best2)
	}
}

// TestCDNExtFromURL covers extension extraction from the CDN URL helper.
// The helper is conservative: any extension over 5 characters (including
// trailing query strings) is treated as unrecognized and returns "".
func TestCDNExtFromURL(t *testing.T) {
	cases := map[string]string{
		"https://cdn.ig/p/AB.jpg":      ".jpg",
		"https://cdn.ig/p/AB":          "",
		"https://cdn.ig/p/AB.heic":     ".heic",
		"https://cdn.ig/no/ext/at/all": "",
	}
	for url, want := range cases {
		got := CDNExtFromURL(url)
		if got != want {
			t.Errorf("CDNExtFromURL(%q) = %q; want %q", url, got, want)
		}
	}
}

// TestFetchToFile429ReturnsRateLimitError verifies the download pipeline
// surfaces a typed *cliutil.RateLimitError on a 429 from the CDN instead of
// silently returning empty bytes. Empty-on-throttle would corrupt downstream
// queries that can't tell "no asset" from "throttled away".
func TestFetchToFile429ReturnsRateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	dir, err := os.MkdirTemp("", "dl-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	d := &Downloader{
		OutputDir:  dir,
		HTTPClient: srv.Client(),
	}
	_, _, err = d.fetchToFile(context.Background(), srv.URL+"/cdn/asset.jpg", filepath.Join(dir, "asset.jpg"), 0)
	if err == nil {
		t.Fatal("expected a typed RateLimitError, got nil")
	}
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected *cliutil.RateLimitError, got %T: %v", err, err)
	}
	if !strings.Contains(rl.URL, "asset.jpg") {
		t.Errorf("RateLimitError.URL = %q; want substring asset.jpg", rl.URL)
	}
	// Local file must NOT exist — we aborted before any write.
	if _, statErr := os.Stat(filepath.Join(dir, "asset.jpg")); !os.IsNotExist(statErr) {
		t.Errorf("local file should not exist after 429; stat err = %v", statErr)
	}
}

// TestFetchToFile200WritesAndHashes covers the happy path: a 200 response
// body is written verbatim and hashed.
func TestFetchToFile200WritesAndHashes(t *testing.T) {
	body := []byte("hello world")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dir, err := os.MkdirTemp("", "dl-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	target := filepath.Join(dir, "asset.bin")

	d := &Downloader{OutputDir: dir, HTTPClient: srv.Client()}
	n, sha, err := d.fetchToFile(context.Background(), srv.URL+"/asset.bin", target, 0)
	if err != nil {
		t.Fatalf("fetchToFile: %v", err)
	}
	if n != int64(len(body)) {
		t.Errorf("wrote %d bytes; want %d", n, len(body))
	}
	if sha == "" {
		t.Error("expected non-empty sha256")
	}
	read, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if string(read) != string(body) {
		t.Errorf("saved bytes = %q; want %q", read, body)
	}
}
