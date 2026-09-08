// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsfetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/cliutil"
)

// newTestClient allows the local test server. Production clients keep
// DefaultAllowedHosts; widening it here is what makes the guard testable at all.
func newTestClient() *Client {
	c := New(5*time.Second, 1000)
	c.MaxAttempts = 3
	c.AllowedHosts = []string{"127.0.0.1", "localhost", "::1"}
	return c
}

// TestGetRefusesOffOriginURL pins the guard itself: a URL scraped from a defaced
// index must not be fetched just because it parsed.
func TestGetRefusesOffOriginURL(t *testing.T) {
	c := New(5*time.Second, 1000)
	for _, bad := range []string{
		"http://127.0.0.1:9200/_search",
		"https://evil.example.com/wp-content/uploads/2020/07/Annex.xlsx",
		"file:///etc/passwd",
		"http://pbs.gov.pk.evil.example.com/x.xlsx",
	} {
		res, err := c.Get(context.Background(), bad)
		if err == nil {
			t.Errorf("%s was fetched (res=%+v), want refusal", bad, res)
			continue
		}
		if !errors.Is(err, ErrOffOrigin) {
			t.Errorf("%s: error %v does not wrap ErrOffOrigin", bad, err)
		}
	}
	// The real origin must still be allowed.
	if err := c.checkOrigin("https://www.pbs.gov.pk/price-statistics/"); err != nil {
		t.Errorf("the real origin was refused: %v", err)
	}
}

// TestDefaultClientAllowsOnlyPBS pins that the production constructor does not
// ship a wide allowlist.
func TestDefaultClientAllowsOnlyPBS(t *testing.T) {
	c := New(time.Second, 1)
	for _, h := range c.AllowedHosts {
		if !strings.HasSuffix(strings.ToLower(h), "pbs.gov.pk") {
			t.Errorf("default allowlist contains %q, which is not a PBS host", h)
		}
	}
}

func TestGetSuccessHashesBody(t *testing.T) {
	body := []byte("weekly sensitive price indicator")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Errorf("request carried no User-Agent")
		}
		// A browser-shaped Accept is sent deliberately; */* has produced 403s
		// on sibling Pakistani sources.
		if acc := r.Header.Get("Accept"); !strings.Contains(acc, "text/html") {
			t.Errorf("Accept = %q, want a browser-shaped value", acc)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	res, err := newTestClient().Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res.Status != 200 {
		t.Errorf("status = %d, want 200", res.Status)
	}
	sum := sha256.Sum256(body)
	if res.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("sha256 mismatch")
	}
	if string(res.Body) != string(body) {
		t.Errorf("body mismatch")
	}
}

// TestGetNotFoundIsTyped pins that upstream rot is distinguishable from a
// transport failure. The two mean different things for coverage: a 404 is the
// Bureau having deleted a file, a transport failure is ours to retry.
func TestGetNotFoundIsTyped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	_, err := newTestClient().Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for HTTP 404")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error %v does not wrap ErrNotFound", err)
	}
}

// TestGetNeverReturnsEmptySuccess is the central guarantee: a non-2xx response
// must never come back as a zero-length success. Converting a failed fetch into
// an empty result is what punches silent holes in a panel.
func TestGetNeverReturnsEmptySuccess(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404, 418, 500, 503} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		res, err := newTestClient().Get(context.Background(), srv.URL)
		srv.Close()
		if err == nil {
			t.Errorf("HTTP %d returned no error (res=%+v)", code, res)
		}
		if res != nil {
			t.Errorf("HTTP %d returned a non-nil result alongside an error", code)
		}
	}
}

// TestGetRateLimitSurfacesTypedError pins that an exhausted 429 retry budget
// surfaces *cliutil.RateLimitError rather than an empty result, so a throttled
// sync is distinguishable from "no data exists".
func TestGetRateLimitSurfacesTypedError(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient()
	c.MaxAttempts = 2
	_, err := c.Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected an error when the retry budget is exhausted")
	}
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("error %v is not a *cliutil.RateLimitError", err)
	}
	if got := atomic.LoadInt32(&hits); got < 2 {
		t.Errorf("server saw %d requests, want at least 2 (one retry)", got)
	}
}

// TestGetRetriesServerErrorThenSucceeds pins bounded retry on 5xx.
func TestGetRetriesServerErrorThenSucceeds(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	res, err := newTestClient().Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get after one 502: %v", err)
	}
	if string(res.Body) != "ok" {
		t.Errorf("body = %q, want %q", res.Body, "ok")
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("server saw %d requests, want 2", got)
	}
}

func TestGetRespectsCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newTestClient().Get(ctx, srv.URL); err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
}

// TestContentSniffing pins format detection, which decides whether a release is
// handed to the workbook parser or the PDF parser.
func TestContentSniffing(t *testing.T) {
	cases := []struct {
		name     string
		ct       string
		body     []byte
		wantXLSX bool
		wantPDF  bool
		wantHTML bool
	}{
		{"declared xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte("PK\x03\x04"), true, false, false},
		// Some hosts serve workbooks as octet-stream, so the ZIP magic number
		// is the fallback.
		{"octet-stream zip", "application/octet-stream", []byte("PK\x03\x04rest"), true, false, false},
		{"declared pdf", "application/pdf", []byte("%PDF-1.7"), false, true, false},
		{"octet-stream pdf", "application/octet-stream", []byte("%PDF-1.5 rest"), false, true, false},
		{"html", "text/html; charset=UTF-8", []byte("<html>"), false, false, true},
		{"plain text is none of them", "text/plain", []byte("hello"), false, false, false},
	}
	for _, c := range cases {
		r := &Result{ContentType: c.ct, Body: c.body}
		if got := r.LooksXLSX(); got != c.wantXLSX {
			t.Errorf("%s: LooksXLSX = %v, want %v", c.name, got, c.wantXLSX)
		}
		if got := r.LooksPDF(); got != c.wantPDF {
			t.Errorf("%s: LooksPDF = %v, want %v", c.name, got, c.wantPDF)
		}
		if got := r.LooksHTML(); got != c.wantHTML {
			t.Errorf("%s: LooksHTML = %v, want %v", c.name, got, c.wantHTML)
		}
	}
}

// TestDefaultUserAgentDoesNotImpersonateAnAICrawler pins a governance property:
// PBS's robots.txt carries no AI-crawler directives, but claiming to be one
// would be a false statement about what is making the request.
func TestDefaultUserAgentDoesNotImpersonateAnAICrawler(t *testing.T) {
	for _, bad := range []string{"ClaudeBot", "GPTBot", "CCBot", "Google-Extended", "Bytespider", "meta-externalagent"} {
		if strings.Contains(DefaultUserAgent, bad) {
			t.Errorf("default User-Agent impersonates %q", bad)
		}
	}
	if !strings.Contains(DefaultUserAgent, "pbs-pp-cli") {
		t.Errorf("default User-Agent %q does not identify the CLI", DefaultUserAgent)
	}
}

func TestNewAppliesSaneDefaults(t *testing.T) {
	c := New(0, 0)
	if c.HTTP.Timeout <= 0 {
		t.Error("zero timeout was not replaced with a default")
	}
	if c.Limiter == nil {
		t.Error("no limiter was configured, so requests would be unpaced")
	}
	if c.MaxAttempts < 1 {
		t.Error("MaxAttempts must be at least 1")
	}
}
