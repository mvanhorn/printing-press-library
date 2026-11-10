// PATCH(retry-policy): tests for the hand-written retry policy; see .printing-press-patches.json.

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	neturl "net/url"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/config"
)

// retryHarness records the waits and zeroes jitter unless a test sets it.
func retryHarness(t *testing.T, jitter float64) *[]time.Duration {
	t.Helper()
	waits := &[]time.Duration{}
	oldS, oldJ := retrySleep, retryJitter
	retrySleep = func(_ context.Context, d time.Duration) error {
		*waits = append(*waits, d)
		return nil
	}
	retryJitter = func() float64 { return jitter }
	t.Cleanup(func() { retrySleep, retryJitter = oldS, oldJ })
	return waits
}

func newRetryClient(url string, headers map[string]string) *Client {
	return New(&config.Config{BaseURL: url, Headers: headers}, 5*time.Second, 0)
}

// allowIdempotent registers an endpoint as honouring Idempotency-Key.
func allowIdempotent(t *testing.T, key string) {
	t.Helper()
	idempotentEndpoints[key] = true
	t.Cleanup(func() { delete(idempotentEndpoints, key) })
}

const gateBusyBody = `{"error":"busy","code":"gate_busy","retryAfter":7}`

func busyServer(t *testing.T, hits *int32, header string, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(hits, 1)
		if n == 1 {
			if header != "" {
				w.Header().Set("Retry-After", header)
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGET503HonoursRetryAfterHeader(t *testing.T) {
	waits := retryHarness(t, 0)
	var hits int32
	srv := busyServer(t, &hits, "4", gateBusyBody)
	c := newRetryClient(srv.URL, nil)
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&hits) != 2 || len(*waits) != 1 || (*waits)[0] != 4*time.Second {
		t.Fatalf("hits=%d waits=%v, want 2 atomic.LoadInt32(&hits) and one 4s wait (header beats body)", atomic.LoadInt32(&hits), *waits)
	}
}

func TestGET503FallsBackToBodyRetryAfter(t *testing.T) {
	waits := retryHarness(t, 0)
	var hits int32
	srv := busyServer(t, &hits, "", gateBusyBody)
	c := newRetryClient(srv.URL, nil)
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(*waits) != 1 || (*waits)[0] != 7*time.Second {
		t.Fatalf("waits=%v, want [7s]", *waits)
	}
}

func TestGET503NoHeaderNoBodyUsesBackoff(t *testing.T) {
	waits := retryHarness(t, 0)
	var hits int32
	srv := busyServer(t, &hits, "", `{"error":"down"}`)
	c := newRetryClient(srv.URL, nil)
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(*waits) != 1 || (*waits)[0] != time.Second {
		t.Fatalf("waits=%v, want the old 1s backoff", *waits)
	}
}

func TestPOSTWithoutKeyNotReplayedOn503(t *testing.T) {
	waits := retryHarness(t, 0)
	var hits int32
	srv := busyServer(t, &hits, "1", gateBusyBody)
	c := newRetryClient(srv.URL, nil)
	_, status, err := c.do(context.Background(), "POST", "/x", nil, map[string]string{"a": "b"}, nil)
	if err == nil || status != 503 {
		t.Fatalf("want 503 error, got status=%d err=%v", status, err)
	}
	if atomic.LoadInt32(&hits) != 1 || len(*waits) != 0 {
		t.Fatalf("hits=%d waits=%v, want exactly one request and no wait", atomic.LoadInt32(&hits), *waits)
	}
}

func TestPATCHAndDELETEWithoutKeyNotReplayed(t *testing.T) {
	retryHarness(t, 0)
	for _, m := range []string{"PATCH", "DELETE", "PUT"} {
		var hits int32
		srv := busyServer(t, &hits, "1", gateBusyBody)
		c := newRetryClient(srv.URL, nil)
		if _, _, err := c.do(context.Background(), m, "/x", nil, nil, nil); err == nil || atomic.LoadInt32(&hits) != 1 {
			t.Fatalf("%s: hits=%d err=%v, want one request and an error", m, atomic.LoadInt32(&hits), err)
		}
	}
}

func TestAllowlistedPOSTGetsCLIKeyAndRetriesWithSameKey(t *testing.T) {
	waits := retryHarness(t, 0)
	allowIdempotent(t, "POST /things/{id}")
	var hits int32
	var keys []string
	var keysMu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keysMu.Lock()
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		keysMu.Unlock()
		if atomic.AddInt32(&hits, 1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	if _, _, err := c.do(context.Background(), "POST", "/things/42", nil, map[string]string{"a": "b"}, nil); err != nil {
		t.Fatal(err)
	}
	keysMu.Lock()
	defer keysMu.Unlock()
	if atomic.LoadInt32(&hits) != 2 || len(*waits) != 1 || len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("hits=%d waits=%v keys=%v, want 2 atomic.LoadInt32(&hits), one wait, same generated key", atomic.LoadInt32(&hits), *waits, keys)
	}
}

func TestUserSuppliedKeyDoesNotMakeMutationReplaySafe(t *testing.T) {
	retryHarness(t, 0)
	for name, tc := range map[string]struct {
		cfg map[string]string
		ovr map[string]string
	}{"config headers": {map[string]string{"Idempotency-Key": "k"}, nil}, "override": {nil, map[string]string{"Idempotency-Key": "k"}}} {
		var hits int32
		srv := busyServer(t, &hits, "1", gateBusyBody)
		c := newRetryClient(srv.URL, tc.cfg)
		if _, _, err := c.do(context.Background(), "POST", "/x", nil, nil, tc.ovr); err == nil || atomic.LoadInt32(&hits) != 1 {
			t.Fatalf("%s: hits=%d err=%v, want one request", name, atomic.LoadInt32(&hits), err)
		}
	}
}

func TestEmptyKeyHeaderIsRemoved(t *testing.T) {
	retryHarness(t, 0)
	for _, h := range []string{"Idempotency-Key", "X-Idempotency-Key"} {
		present := true
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, present = r.Header[http.CanonicalHeaderKey(h)]
			_, _ = w.Write([]byte(`{}`))
		}))
		c := newRetryClient(srv.URL, map[string]string{h: "  "})
		_, _, err := c.do(context.Background(), "POST", "/x", nil, nil, nil)
		srv.Close()
		if err != nil || present {
			t.Fatalf("%s: err=%v present=%v, want header absent", h, err, present)
		}
	}
}

func Test429POSTNotReplayedButGETRetried(t *testing.T) {
	waits := retryHarness(t, 0)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 || r.Method == "POST" {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	if _, status, err := c.do(context.Background(), "POST", "/x", nil, nil, nil); err == nil || status != 429 || atomic.LoadInt32(&hits) != 1 || len(*waits) != 0 {
		t.Fatalf("POST: hits=%d status=%d err=%v waits=%v, want exactly one request", atomic.LoadInt32(&hits), status, err, *waits)
	}
	atomic.StoreInt32(&hits, 0)
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&hits) != 2 || len(*waits) != 1 || (*waits)[0] != 3*time.Second {
		t.Fatalf("GET: hits=%d waits=%v, want retry after 3s", atomic.LoadInt32(&hits), *waits)
	}
}

func TestRedirectPOSTNotResentButGETFollowed(t *testing.T) {
	retryHarness(t, 0)
	var target int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/b" {
			atomic.AddInt32(&target, 1)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		http.Redirect(w, r, "/b", http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	if _, _, err := c.do(context.Background(), "POST", "/a", nil, map[string]string{"a": "b"}, nil); err == nil || atomic.LoadInt32(&target) != 0 {
		t.Fatalf("POST: target hits=%d err=%v, want the redirect refused and nothing resent", atomic.LoadInt32(&target), err)
	}
	if _, _, err := c.do(context.Background(), "GET", "/a", nil, nil, nil); err != nil || atomic.LoadInt32(&target) != 1 {
		t.Fatalf("GET: target hits=%d err=%v, want redirect followed", atomic.LoadInt32(&target), err)
	}
}

func TestGETBodyResetRetriedPOSTNot(t *testing.T) {
	retryHarness(t, 0)
	for _, tc := range []struct {
		method string
		want   int32
	}{{"GET", 2}, {"POST", 1}} {
		var hits int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&hits, 1) == 1 {
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(200)
				_, _ = w.Write([]byte("partial"))
				hj, _ := w.(http.Hijacker)
				conn, _, _ := hj.Hijack()
				_ = conn.Close()
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c := newRetryClient(srv.URL, nil)
		_, _, err := c.do(context.Background(), tc.method, "/x", nil, nil, nil)
		srv.Close()
		if atomic.LoadInt32(&hits) != tc.want || (tc.method == "GET") != (err == nil) {
			t.Fatalf("%s: hits=%d err=%v", tc.method, atomic.LoadInt32(&hits), err)
		}
	}
}

func TestCallBudgetStopsRetriesEarly(t *testing.T) {
	waits := retryHarness(t, 0)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Retry-After", "20")
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.CallBudget = 5 * time.Second
	_, status, err := c.do(context.Background(), "GET", "/x", nil, nil, nil)
	if err == nil || status != 503 || atomic.LoadInt32(&hits) != 1 || len(*waits) != 0 {
		t.Fatalf("hits=%d status=%d err=%v waits=%v, want one request and no wait", atomic.LoadInt32(&hits), status, err, *waits)
	}
}

func TestCallBudgetCoversSlowServer(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.CallBudget = 200 * time.Millisecond
	start := time.Now()
	_, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "call budget") || time.Since(start) > 3*time.Second {
		t.Fatalf("err=%v elapsed=%v, want a call budget error well under 3s", err, time.Since(start))
	}
}

func TestRetriesCapAtThreeAttemptsAfterFirst(t *testing.T) {
	retryHarness(t, 0)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err == nil || atomic.LoadInt32(&hits) != 4 {
		t.Fatalf("hits=%d err=%v, want 4 requests then an error", atomic.LoadInt32(&hits), err)
	}
}

func TestNetworkErrorNotReplayedForPOSTButForGET(t *testing.T) {
	retryHarness(t, 0)
	for _, tc := range []struct {
		method string
		hdr    map[string]string
		want   int32
	}{{"POST", nil, 1}, {"GET", nil, 4}, {"POST", map[string]string{"Idempotency-Key": "k"}, 1}} {
		var hits int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close() // drop the connection after the request arrived
		}))
		c := newRetryClient(srv.URL, tc.hdr)
		_, _, err := c.do(context.Background(), tc.method, "/x", nil, nil, nil)
		srv.Close()
		if err == nil || atomic.LoadInt32(&hits) != tc.want {
			t.Fatalf("%s %v: hits=%d err=%v, want %d requests", tc.method, tc.hdr, atomic.LoadInt32(&hits), err, tc.want)
		}
	}
}

func TestServerRetryDelayJitterBounds(t *testing.T) {
	h := http.Header{"Retry-After": []string{"10"}}
	for _, j := range []float64{0, 0.5, 0.999999} {
		old := retryJitter
		retryJitter = func() float64 { return j }
		d, ok := serverRetryDelay(h, nil)
		retryJitter = old
		if !ok || d < 10*time.Second || d > 15*time.Second {
			t.Fatalf("jitter %v: d=%v ok=%v, want within [10s,15s]", j, d, ok)
		}
	}
	// Cap is 30s including jitter.
	old := retryJitter
	retryJitter = func() float64 { return 0.999 }
	defer func() { retryJitter = old }()
	for _, v := range []string{"30", "31", "3600", "99999999999"} {
		d, ok := serverRetryDelay(http.Header{"Retry-After": []string{v}}, nil)
		if !ok || d != 30*time.Second {
			t.Fatalf("Retry-After %s: d=%v ok=%v, want 30s", v, d, ok)
		}
	}
}

func TestParseRetryAfterEdgeCases(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	old := retryNow
	retryNow = func() time.Time { return now }
	defer func() { retryNow = old }()

	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"", 0, false},
		{"  5 ", 5 * time.Second, true},
		{"1.5", 1500 * time.Millisecond, true},
		{"0", 0, true},
		{"-3", 0, false},
		{"NaN", 0, false},
		{"Inf", 0, false},
		{"soon", 0, false},
		{now.Add(12 * time.Second).Format(http.TimeFormat), 12 * time.Second, true},
		{now.Add(-time.Minute).Format(http.TimeFormat), 0, true},
		{strings.Repeat("9", 400), 30 * time.Second, true},
		{"1e3", 0, false},
		{"+5", 0, false},
		{now.Add(time.Hour).Format(http.TimeFormat), 30 * time.Second, true},
	}
	for _, tc := range cases {
		got, ok := parseRetryAfterHeader(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%q: got %v,%v want %v,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseBusyBodyShapes(t *testing.T) {
	for _, tc := range []struct {
		body string
		code string
		ok   bool
	}{
		{`{"code":"gate_busy","retryAfter":5}`, "gate_busy", true},
		{`{"error":{"code":"gate_busy","retryAfter":5}}`, "gate_busy", true},
		{`{"code":"gate_busy","retryAfter":"5"}`, "gate_busy", false},
		{`{"error":"x"}`, "", false},
		{`not json`, "", false},
	} {
		code, _, ok := parseBusyBody([]byte(tc.body))
		if code != tc.code || ok != tc.ok {
			t.Errorf("%s: code=%q ok=%v", tc.body, code, ok)
		}
	}
}

func TestGateBusyMessagePrintedOnce(t *testing.T) {
	retryHarness(t, 0)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) <= 3 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(503)
			_, _ = w.Write([]byte(gateBusyBody))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	out := captureStderr(t, func() {
		c := newRetryClient(srv.URL, nil)
		if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	})
	if n := strings.Count(out, "Conduyt is busy, retrying in 2 s"); n != 1 {
		t.Fatalf("busy message count=%d, stderr=%q", n, out)
	}
	if strings.Contains(out, "server error") || strings.Contains(out, "goroutine") {
		t.Fatalf("unexpected noise: %q", out)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	fn()
	os.Stderr = old
	_ = w.Close()
	return <-done
}

func TestKeyHeadersStrippedFromUnkeyedMutationsOnWarmMutationConnection(t *testing.T) {
	retryHarness(t, 0)
	for _, h := range []string{"Idempotency-Key", "X-Idempotency-Key"} {
		var posts, sawKey int32
		var addrs []string
		var mu sync.Mutex
		note := func(a string) { mu.Lock(); addrs = append(addrs, a); mu.Unlock() }
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/drop" {
				atomic.AddInt32(&posts, 1)
				if r.Header.Get(h) != "" {
					atomic.AddInt32(&sawKey, 1)
				}
				note(r.RemoteAddr)
				conn, _, _ := w.(http.Hijacker).Hijack()
				_ = conn.Close() // drop before the first response byte
				return
			}
			note(r.RemoteAddr)
			_, _ = w.Write([]byte(`{}`))
		}))
		c := newRetryClient(srv.URL, map[string]string{h: "user-key"})
		// Warm the MUTATION transport's keep-alive connection with a POST.
		if _, _, err := c.do(context.Background(), "POST", "/warm", nil, map[string]string{"a": "b"}, nil); err != nil {
			t.Fatal(err)
		}
		_, _, err := c.do(context.Background(), "POST", "/drop", nil, map[string]string{"a": "b"}, nil)
		srv.Close()
		mu.Lock()
		defer mu.Unlock()
		if len(addrs) != 2 || addrs[0] != addrs[1] {
			t.Fatalf("%s: the dropped POST must ride the warm mutation connection (keep-alives on): %v", h, addrs)
		}
		if p, k := atomic.LoadInt32(&posts), atomic.LoadInt32(&sawKey); err == nil || p != 1 || k != 0 {
			t.Fatalf("%s: posts=%d keyOnWire=%d err=%v, want one POST with no key header", h, p, k, err)
		}
	}
}

// Control: the same warm-connection drop WITH the key header on the wire is
// resent by Go's own transport on the pinned toolchain, so the strip is what
// protects mutations. This must fail (not skip) if that behaviour changes, so
// the premise of the strip stays checked.
func TestControlGoTransportReplaysKeyedPOSTOnWarmConnection(t *testing.T) {
	var mu sync.Mutex
	var warmAddr string
	var dropAddrs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/drop" {
			mu.Lock()
			dropAddrs = append(dropAddrs, r.RemoteAddr)
			mu.Unlock()
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
			return
		}
		mu.Lock()
		warmAddr = r.RemoteAddr
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	hc := &http.Client{Transport: &http.Transport{}}
	do := func(path string) error {
		req, _ := http.NewRequest("POST", srv.URL+path, strings.NewReader(`{"a":"b"}`))
		req.Header.Set("Idempotency-Key", "k")
		resp, err := hc.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		return err
	}
	if err := do("/warm"); err != nil {
		t.Fatal(err)
	}
	_ = do("/drop")
	mu.Lock()
	defer mu.Unlock()
	if len(dropAddrs) != 2 || dropAddrs[0] != warmAddr || dropAddrs[1] == warmAddr {
		t.Fatalf("warm=%s drops=%v: Go no longer resends a keyed POST dropped on a warm connection (first on the warm conn, second on a new one); revisit the strip rationale", warmAddr, dropAddrs)
	}
}

func TestMutationOverHTTPSWithH2ServerArrivesAsHTTP1(t *testing.T) {
	retryHarness(t, 0)
	var mu sync.Mutex
	protos := map[string]string{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		protos[r.Method] = r.Proto
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.HTTPClient.Transport = srv.Client().Transport.(*http.Transport).Clone()
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err != nil {
		t.Fatalf("GET: %v", err)
	}
	if _, _, err := c.do(context.Background(), "POST", "/x", nil, map[string]string{"a": "b"}, nil); err != nil {
		t.Fatalf("POST over TLS to an h2-capable server: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if protos["GET"] != "HTTP/2.0" || protos["POST"] != "HTTP/1.1" {
		t.Fatalf("protos=%v, want reads on HTTP/2.0 and mutations on HTTP/1.1", protos)
	}
}

func TestMutationClientIsHTTP1WithKeepAlives(t *testing.T) {
	c := newRetryClient("http://example.invalid", nil)
	mt, ok := c.httpClientFor(false).Transport.(*http.Transport)
	if !ok || mt.DisableKeepAlives || mt.Protocols == nil || !mt.Protocols.HTTP1() || mt.Protocols.HTTP2() || mt.Protocols.UnencryptedHTTP2() {
		t.Fatalf("mutation transport must be HTTP/1-only with keep-alives: %+v", mt)
	}
	if c.httpClientFor(false).Transport == c.HTTPClient.Transport {
		t.Fatal("mutations must not share the read pool")
	}
}

func TestUserKeyStillSentOnGET(t *testing.T) {
	retryHarness(t, 0)
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Idempotency-Key")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, map[string]string{"Idempotency-Key": "k"})
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err != nil || got != "k" {
		t.Fatalf("err=%v got=%q", err, got)
	}
}

func TestBudgetErrorKeepsLastResponseAndError(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "20")
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.CallBudget = 5 * time.Second
	_, status, err := c.do(context.Background(), "GET", "/x", nil, nil, nil)
	var apiErr *APIError
	if status != 503 || err == nil || !strings.Contains(err.Error(), "call budget of 5s exceeded") || !errors.As(err, &apiErr) {
		t.Fatalf("status=%d err=%v, want budget message wrapping the 503 APIError", status, err)
	}
}

func TestBudgetPrecheckOnNetworkErrorKeepsCause(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.CallBudget = 500 * time.Millisecond // first backoff (1s) cannot fit
	_, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "call budget") || !strings.Contains(err.Error(), "GET") {
		t.Fatalf("err=%v, want the budget message with the network error attached", err)
	}
}

func TestParseBusyBodyNullRetryAfterIsAbsent(t *testing.T) {
	for _, b := range []string{`{"retryAfter":null}`, `{"error":{"retryAfter":null}}`} {
		if _, _, ok := parseBusyBody([]byte(b)); ok {
			t.Errorf("%s: null must not be a wait", b)
		}
	}
	if _, d, ok := parseBusyBody([]byte(`{"retryAfter":null,"error":{"retryAfter":4}}`)); !ok || d != 4*time.Second {
		t.Errorf("outer null must not hide a valid nested value: %v %v", d, ok)
	}
	waits := retryHarness(t, 0)
	var hits int32
	srv := busyServer(t, &hits, "", `{"code":"gate_busy","retryAfter":null}`)
	c := newRetryClient(srv.URL, nil)
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err != nil || len(*waits) != 1 || (*waits)[0] != time.Second {
		t.Fatalf("err=%v waits=%v, want the 1s backoff", err, *waits)
	}
}

func TestRetryAfterNumericGrammar(t *testing.T) {
	for _, in := range []string{".5", "1.", "1.2.3", "+5", "1e3", "-1", "0x10"} {
		if d, ok := parseRetryAfterHeader(in); ok {
			t.Errorf("%q accepted as %v", in, d)
		}
	}
	for in, want := range map[string]time.Duration{"5": 5 * time.Second, "0.5": 500 * time.Millisecond, "12.25": 12250 * time.Millisecond} {
		if d, ok := parseRetryAfterHeader(in); !ok || d != want {
			t.Errorf("%q: %v %v", in, d, ok)
		}
	}
}

func TestAllowlistLiteralBeatsTemplate(t *testing.T) {
	allowIdempotent(t, "PATCH /users/{id}")
	idempotentEndpoints["PATCH /users/me"] = false
	t.Cleanup(func() { delete(idempotentEndpoints, "PATCH /users/me") })
	if !endpointHonoursIdempotencyKey("PATCH", "/users/42") {
		t.Fatal("template should match /users/42")
	}
	if endpointHonoursIdempotencyKey("PATCH", "/users/me") {
		t.Fatal("literal /users/me must win over the template")
	}
	if endpointHonoursIdempotencyKey("PUT", "/users/42") || endpointHonoursIdempotencyKey("PATCH", "/users/42/x") {
		t.Fatal("method and depth must match")
	}
}

// specLiteralRoutes scans spec.yaml for literal (no {param}) routes, "METHOD /path".
func specLiteralRoutes(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../../spec.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pathRe := regexp.MustCompile(`^  (/[^:\s]+):\s*$`)
	methodRe := regexp.MustCompile(`^    (get|post|put|patch|delete):`)
	var literals []string
	cur := ""
	for _, line := range strings.Split(string(data), "\n") {
		if m := pathRe.FindStringSubmatch(line); m != nil {
			cur = m[1]
		} else if m := methodRe.FindStringSubmatch(line); m != nil && cur != "" && !strings.Contains(cur, "{") {
			literals = append(literals, strings.ToUpper(m[1])+" "+cur)
		}
	}
	sort.Strings(literals)
	return literals
}

// shadowedLiterals lists spec literal routes that the current allowlist would
// wrongly treat as keyed because a template matches them and they are not
// listed themselves.
func shadowedLiterals(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, lit := range specLiteralRoutes(t) {
		if _, listed := idempotentEndpoints[lit]; listed {
			continue
		}
		m, p, _ := strings.Cut(lit, " ")
		if endpointHonoursIdempotencyKey(m, p) {
			out = append(out, lit)
		}
	}
	return out
}

func TestAllowlistTemplatesDoNotShadowSpecLiterals(t *testing.T) {
	if got := shadowedLiterals(t); len(got) != 0 {
		t.Fatalf("allowlist templates shadow literal routes %v: list each as false", got)
	}
}

// Negative control: the scanner really finds literal routes and the guard
// really fires when a template shadows one.
func TestShadowGuardNegativeControl(t *testing.T) {
	if len(specLiteralRoutes(t)) < 50 {
		t.Fatalf("scanner found only %d literal routes in spec.yaml", len(specLiteralRoutes(t)))
	}
	allowIdempotent(t, "GET /users/{id}")
	got := shadowedLiterals(t)
	found := false
	for _, g := range got {
		if g == "GET /users/me" {
			found = true
		}
	}
	if !found {
		t.Fatalf("guard missed GET /users/me shadowed by GET /users/{id}: %v", got)
	}
	idempotentEndpoints["GET /users/me"] = false
	t.Cleanup(func() { delete(idempotentEndpoints, "GET /users/me") })
	for _, g := range shadowedLiterals(t) {
		if g == "GET /users/me" {
			t.Fatal("listing the literal as false must clear the shadow")
		}
	}
}

func TestCallerDeadlineEarlierThanBudgetIsDeadlineExceeded(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "20")
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.CallBudget = time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := c.do(ctx, "GET", "/x", nil, nil, nil)
	var apiErr *APIError
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "call budget") || !errors.As(err, &apiErr) {
		t.Fatalf("err=%v, want a DeadlineExceeded-wrapping error carrying the 503, not a budget message", err)
	}
}

// A deadline or cancel that fires inside a wait or a request (failCtx path)
// must keep the last response attached and keep the caller's error class.
func TestCallerDeadlineDuringRetrySleepKeepsLastResponse(t *testing.T) {
	retryHarness(t, 0)
	retrySleep = func(ctx context.Context, _ time.Duration) error { <-ctx.Done(); return ctx.Err() }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0.1")
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, _, err := c.do(ctx, "GET", "/x", nil, nil, nil)
	var apiErr *APIError
	if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &apiErr) || strings.Contains(err.Error(), "call budget") {
		t.Fatalf("err=%v, want DeadlineExceeded wrapping the 503 APIError", err)
	}
}

func TestManualCancelDuringRetrySleepIsCanceledNotBudget(t *testing.T) {
	retryHarness(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	retrySleep = func(c context.Context, _ time.Duration) error { cancel(); <-c.Done(); return c.Err() }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	_, _, err := c.do(ctx, "GET", "/x", nil, nil, nil)
	var apiErr *APIError
	if !errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &apiErr) || strings.Contains(err.Error(), "call budget") {
		t.Fatalf("err=%v, want Canceled wrapping the 503 APIError", err)
	}
}

func TestCallerDeadlineDuringLaterRequestReportsThatRequest(t *testing.T) {
	retryHarness(t, 0)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(503)
			return
		}
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, _, err := c.do(ctx, "GET", "/x", nil, nil, nil)
	var apiErr *APIError
	if !errors.Is(err, context.DeadlineExceeded) || errors.As(err, &apiErr) || !strings.Contains(err.Error(), "GET") || strings.Contains(err.Error(), "call budget") {
		t.Fatalf("err=%v, want DeadlineExceeded carrying THIS request's error, not the stale 503", err)
	}
}

// A 503 whose body stalls until the caller's deadline is still an APIError.
func TestStalledErrorBodyAtDeadlineIsAPIError(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":"part`))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, status, err := c.do(ctx, "GET", "/x", nil, nil, nil)
	var apiErr *APIError
	if status != 503 || !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &apiErr) || apiErr.StatusCode != 503 || !strings.Contains(apiErr.Body, "part") {
		t.Fatalf("status=%d err=%v apiErr=%+v, want DeadlineExceeded wrapping the 503 APIError with the partial body", status, err, apiErr)
	}
}

func tlsDialerFor(t *testing.T, srv *httptest.Server, protos []string) func(context.Context, string, string) (net.Conn, error) {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := &tls.Dialer{Config: &tls.Config{RootCAs: pool, NextProtos: protos, ServerName: "example.com"}}
		return d.DialContext(ctx, network, addr)
	}
}

func TestCustomTLSDialerNegotiatingH2IsRefusedForMutations(t *testing.T) {
	retryHarness(t, 0)
	var posts int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			atomic.AddInt32(&posts, 1)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	// h2-negotiating dialer: explicit failure, no request bytes sent.
	c := newRetryClient(srv.URL, nil)
	c.HTTPClient.Transport = &http.Transport{DialTLSContext: tlsDialerFor(t, srv, []string{"h2", "http/1.1"})}
	_, _, err := c.do(context.Background(), "POST", "/x", nil, map[string]string{"a": "b"}, nil)
	if err == nil || !errors.Is(err, errH2OnMutationTransport) || atomic.LoadInt32(&posts) != 0 {
		t.Fatalf("err=%v posts=%d, want the explicit h2 refusal and no request", err, atomic.LoadInt32(&posts))
	}

	// http/1.1 dialer: the mutation goes through.
	c = newRetryClient(srv.URL, nil)
	c.HTTPClient.Transport = &http.Transport{DialTLSContext: tlsDialerFor(t, srv, []string{"http/1.1"})}
	if _, _, err := c.do(context.Background(), "POST", "/x", nil, map[string]string{"a": "b"}, nil); err != nil || atomic.LoadInt32(&posts) != 1 {
		t.Fatalf("err=%v posts=%d, want success over HTTP/1.1", err, atomic.LoadInt32(&posts))
	}
}

func Test429AlwaysSlowsLimiterEvenOnTerminalAttempt(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(429)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.limiter = cliutil.NewAdaptiveLimiter(1000)
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err == nil {
		t.Fatal("want the final 429")
	}
	// 4 responses (3 retries + the terminal one) = 4 halvings.
	if got := c.limiter.Rate(); got != 1000.0/16 {
		t.Fatalf("rate=%v, want %v (the terminal 429 must count)", got, 1000.0/16)
	}
}

func Test429SlowsLimiterWhenRetriesAreOff(t *testing.T) {
	retryHarness(t, 0)
	t.Setenv(cliutil.VerifyEnvVar, "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.limiter = cliutil.NewAdaptiveLimiter(1000)
	if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err == nil {
		t.Fatal("want the 429")
	}
	if got := c.limiter.Rate(); got != 500 {
		t.Fatalf("rate=%v, want 500 with maxRetries == 0", got)
	}
}

func Test429OnUnkeyedMutationStillSlowsTheLimiter(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.limiter = cliutil.NewAdaptiveLimiter(8)
	before := c.limiter.Rate()
	if _, _, err := c.do(context.Background(), "POST", "/x", nil, nil, nil); err == nil {
		t.Fatal("want the 429")
	}
	if after := c.limiter.Rate(); !(after < before) {
		t.Fatalf("rate %v -> %v, want the shared pace reduced by a 429 on a mutation", before, after)
	}
}

// A TLS dialer whose handshake stalls must not outlive the request context.
func TestStalledTLSHandshakeEndsAtTheDeadline(t *testing.T) {
	retryHarness(t, 0)
	ln, err := net.Listen("tcp", "127.0.0.1:0") // accepts, never answers
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	rawDial := func() (net.Conn, error) { return net.Dial("tcp", ln.Addr().String()) }
	block := make(chan struct{})
	defer close(block)
	dialers := map[string]*http.Transport{
		"legacy DialTLS, handshake stalls": {DialTLS: func(network, addr string) (net.Conn, error) { //nolint:staticcheck
			raw, err := rawDial()
			if err != nil {
				return nil, err
			}
			return tls.Client(raw, &tls.Config{InsecureSkipVerify: true}), nil
		}},
		"legacy DialTLS blocks": {DialTLS: func(network, addr string) (net.Conn, error) { //nolint:staticcheck
			<-block
			return nil, errors.New("released")
		}},
		"DialTLSContext, handshake stalls": {DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			raw, err := rawDial()
			if err != nil {
				return nil, err
			}
			return tls.Client(raw, &tls.Config{InsecureSkipVerify: true}), nil
		}},
	}
	for name, tr := range dialers {
		c := newRetryClient("https://example.invalid", nil)
		c.HTTPClient.Transport = tr
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		start := time.Now()
		_, _, err := c.do(ctx, "POST", "/x", nil, map[string]string{"a": "b"}, nil)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 3*time.Second {
			t.Fatalf("%s: err=%v after %v, want DeadlineExceeded at the deadline", name, err, time.Since(start))
		}
	}
}

func Test429WithResettingBodyStillPacesAndIsNotANetworkFailure(t *testing.T) {
	retryHarness(t, 0)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(429)
			_, _ = w.Write([]byte("par"))
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	waits := retryHarness(t, 0)
	out := captureStderr(t, func() {
		c := newRetryClient(srv.URL, nil)
		c.limiter = cliutil.NewAdaptiveLimiter(1000)
		if _, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		if got := c.limiter.Rate(); got != 500 {
			t.Errorf("rate=%v, want 500: a 429 whose body resets must still pace", got)
		}
	})
	if atomic.LoadInt32(&hits) != 2 || len(*waits) != 1 || (*waits)[0] != 2*time.Second || strings.Contains(out, "network error") || !strings.Contains(out, "rate limited") {
		t.Fatalf("hits=%d waits=%v stderr=%q, want the 429 path with the 2s Retry-After", atomic.LoadInt32(&hits), *waits, out)
	}

	// An unkeyed POST gets that 429 as an APIError and one request.
	atomic.StoreInt32(&hits, 0)
	c := newRetryClient(srv.URL, nil)
	c.limiter = cliutil.NewAdaptiveLimiter(1000)
	_, status, err := c.do(context.Background(), "POST", "/x", nil, nil, nil)
	var apiErr *APIError
	if status != 429 || !errors.As(err, &apiErr) || atomic.LoadInt32(&hits) != 1 || c.limiter.Rate() != 500 {
		t.Fatalf("status=%d err=%v hits=%d rate=%v", status, err, atomic.LoadInt32(&hits), c.limiter.Rate())
	}
}

func TestMaskedBodyHidesTokenAcrossTheBoundaryAndInterruptedReads(t *testing.T) {
	const token = "sk-ABCDEFGHIJKLMNOPQRSTUVWX1234"
	c := newRetryClient("http://example.invalid", nil)
	cred := "Bearer " + token

	// Token straddling the 4096-byte cut.
	body := strings.Repeat("a", 4096-10) + token + strings.Repeat("b", 200)
	got := c.maskedBody([]byte(body), false, cred)
	if strings.Contains(got, token[:6]) || strings.Contains(got, "ABCDEF") || !strings.HasSuffix(got, "...") {
		t.Fatalf("straddling token leaked or truncation lost: ...%s", got[len(got)-40:])
	}

	// Interrupted read mid-token: only a prefix of the token arrived.
	for _, n := range []int{6, 9, 20, len(token) - 1} {
		got = c.maskedBody([]byte(`{"echo":"`+token[:n]), true, cred)
		if strings.Contains(got, token[:6]) {
			t.Fatalf("interrupted prefix of %d bytes leaked: %s", n, got)
		}
	}
	// Short cut prefixes (3 bytes and up) are masked too; a coincidental
	// match is an accepted cost.
	for _, n := range []int{3, 4, 5} {
		if got = c.maskedBody([]byte(`{"ok":"`+token[:n]), true, cred); strings.Contains(got, token[:n]) {
			t.Fatalf("%d-byte cut prefix leaked: %s", n, got)
		}
	}
	// Full token anywhere is masked.
	if got = c.maskedBody([]byte(`x `+token+` y`), false, cred); strings.Contains(got, "ABCDEF") {
		t.Fatalf("full token leaked: %s", got)
	}
}

func TestInterruptedErrorBodyDoesNotLeakTokenPrefix(t *testing.T) {
	retryHarness(t, 0)
	const token = "sk-ABCDEFGHIJKLMNOPQRSTUVWX1234"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"echo":"` + token[:14]))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.Config.AccessToken = token
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, status, err := c.do(ctx, "GET", "/x", nil, nil, nil)
	var apiErr *APIError
	if status != 503 || !errors.As(err, &apiErr) || strings.Contains(apiErr.Body, "ABCDEFGH") || strings.Contains(err.Error(), "ABCDEFGH") {
		t.Fatalf("status=%d err=%v, want a masked 503 body", status, err)
	}
}

func TestEchoedRequestHeadersAreMasked(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"auth":"` + r.Header.Get("Authorization") + `","key":"` + r.Header.Get("X-API-Key") + `","cfg":"` + r.Header.Get("X-Session-Token") + `"}`))
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, map[string]string{"X-Session-Token": "cfgtoken-ZZZ-9876543"})
	_, _, err := c.do(context.Background(), "GET", "/x", nil, nil, map[string]string{
		"Authorization": "Bearer ovr-secret-ABCDEF123456",
		"X-API-Key":     "apikey-QWERTY-7654321",
	})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Fatalf("err=%v", err)
	}
	for _, leak := range []string{"ovr-secret-ABCDEF123456", "ABCDEF123456", "apikey-QWERTY-7654321", "QWERTY", "cfgtoken-ZZZ-9876543", "ZZZ-9876"} {
		if strings.Contains(apiErr.Body, leak) || strings.Contains(err.Error(), leak) {
			t.Fatalf("%q leaked: body=%s err=%v", leak, apiErr.Body, err)
		}
	}
}

func TestJSONEscapedSecretIsMasked(t *testing.T) {
	const secret = "sk-live/abc-DEF-12345"
	c := newRetryClient("http://example.invalid", nil)
	marshaled, _ := json.Marshal(secret)
	forms := []string{
		secret,
		string(marshaled),
		`"sk\u002dlive\/abc\u002dDEF\u002d12345"`,
		`"sk-live\/abc-DEF-12345"`,
	}
	for _, f := range forms {
		got := c.maskedBody([]byte(`{"echo":`+f+`}`), false, secret)
		if strings.Contains(got, "DEF") || strings.Contains(got, "abc") {
			t.Errorf("form %s leaked: %s", f, got)
		}
	}
}

// Gate the legacy dialer indefinitely: while gated, each cancelled in-flight
// call may hold at most ONE goroutine (the dialer's own worker, which we cannot
// interrupt). Releasing the gate must then clean everything up and close every
// late connection. Against the round-7 adapter (a waiting cleanup goroutine per
// call on top of the worker) the gated count is about 2 per call and the first
// assertion fails.
func TestLegacyDialerAdapterHoldsOneGoroutinePerGatedCallAndCleansUp(t *testing.T) {
	retryHarness(t, 0)
	const n = 20
	gate := make(chan struct{})
	var gateOnce sync.Once
	openGate := func() { gateOnce.Do(func() { close(gate) }) }
	t.Cleanup(openGate) // a failed assertion must not strand the gated workers
	var closed, started int32
	dial := func(network, addr string) (net.Conn, error) { //nolint:staticcheck
		atomic.AddInt32(&started, 1)
		<-gate
		a, b := net.Pipe()
		_ = b.Close()
		return &closeCounter{Conn: a, n: &closed}, nil
	}
	settle := func(max int) int {
		deadline := time.Now().Add(3 * time.Second)
		for {
			if g := runtime.NumGoroutine(); g <= max || time.Now().After(deadline) {
				return g
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	before := runtime.NumGoroutine()
	// Start barrier: every call is cancelled only after all n workers are
	// inside the dialer, so the measurement never depends on timing.
	callCtx, cancelAll := context.WithCancel(context.Background())
	defer cancelAll()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := newRetryClient("https://example.invalid", nil)
			c.HTTPClient.Transport = &http.Transport{DialTLS: dial} //nolint:staticcheck
			_, _, _ = c.do(callCtx, "POST", "/x", nil, map[string]string{"a": "b"}, nil)
		}()
	}
	barrier := time.Now().Add(10 * time.Second)
	for atomic.LoadInt32(&started) < n && time.Now().Before(barrier) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&started); got != n {
		t.Fatalf("start barrier: %d of %d workers entered the dialer", got, n)
	}
	cancelAll()
	wg.Wait()
	if got := atomic.LoadInt32(&started); got != n {
		t.Fatalf("dialers started = %d, want %d: the test did not exercise every call", got, n)
	}
	if g := settle(before + n + 5); g > before+n+5 {
		t.Fatalf("while gated: goroutines %d -> %d for %d cancelled calls, want at most one per call (+5)", before, g, n)
	}
	openGate()
	if g := settle(before + 3); g > before+3 {
		t.Fatalf("after release: goroutines %d -> %d, want full cleanup", before, g)
	}
	deadline := time.Now().Add(3 * time.Second)
	for atomic.LoadInt32(&closed) < n && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&closed); got != n {
		t.Fatalf("late connections closed = %d, want %d", got, n)
	}
}

type closeCounter struct {
	net.Conn
	n *int32
}

func (c *closeCounter) Close() error { atomic.AddInt32(c.n, 1); return c.Conn.Close() }

func TestSleepCancelReturnsTheResponseStatus(t *testing.T) {
	for _, code := range []int{429, 503} {
		retryHarness(t, 0)
		retrySleep = func(ctx context.Context, _ time.Duration) error { <-ctx.Done(); return ctx.Err() }
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "0.1")
			w.WriteHeader(code)
		}))
		c := newRetryClient(srv.URL, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		_, status, err := c.do(ctx, "GET", "/x", nil, nil, nil)
		cancel()
		srv.Close()
		var apiErr *APIError
		if status != code || !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &apiErr) {
			t.Fatalf("%d: status=%d err=%v, want the response status with DeadlineExceeded", code, status, err)
		}
	}
}

func TestRequestBodyAndQueryCredentialsAreMasked(t *testing.T) {
	retryHarness(t, 0)
	const pw = "hunter2-Secret!"
	const qk = "qkey-123456789"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"you sent":` + string(body) + `,"query":"` + r.URL.RawQuery + `"}`))
	}))
	defer srv.Close()
	var apiErr *APIError
	var err error
	out := captureStderr(t, func() {
		c := newRetryClient(srv.URL, nil)
		_, _, err = c.do(context.Background(), "POST", "/auth/login", map[string]string{"api_key": qk},
			map[string]any{"email": "a@b.co", "password": pw, "nested": map[string]any{"refreshToken": "rt-ABCDEFGH"}}, nil)
	})
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Fatalf("err=%v", err)
	}
	for _, leak := range []string{pw, "hunter2", qk, "123456789", "rt-ABCDEFGH", "ABCDEFGH"} {
		if strings.Contains(apiErr.Body, leak) || strings.Contains(err.Error(), leak) || strings.Contains(out, leak) {
			t.Fatalf("%q leaked: body=%s err=%v stderr=%q", leak, apiErr.Body, err, out)
		}
	}
	if !strings.Contains(apiErr.Body, "a@b.co") {
		t.Fatalf("non-secret fields must stay readable: %s", apiErr.Body)
	}
}

func TestQueryCredentialInNetworkErrorStderrIsMasked(t *testing.T) {
	waits := retryHarness(t, 0)
	_ = waits
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()
	var err error
	out := captureStderr(t, func() {
		c := newRetryClient(srv.URL, nil)
		_, _, err = c.do(context.Background(), "GET", "/x", map[string]string{"access_token": "tok-987654321"}, nil, nil)
	})
	if err == nil || strings.Contains(out, "987654321") || strings.Contains(err.Error(), "987654321") || !strings.Contains(out, "network error") {
		t.Fatalf("err=%v stderr=%q, want the retry line without the query token", err, out)
	}
}

func TestCookieJarAndXConduytKeyAreMaskedFromTheSentRequest(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"cookie":"` + r.Header.Get("Cookie") + `","key":"` + r.Header.Get("X-Conduyt-Key") + `"}`))
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(srv.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "sid", Value: "jarcookie-QQQ-5555", Path: "/"}})
	c.HTTPClient.Jar = jar
	_, _, err := c.do(context.Background(), "GET", "/x", nil, nil, map[string]string{"X-Conduyt-Key": "ck-WWW-77777777"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err=%v", err)
	}
	for _, leak := range []string{"jarcookie-QQQ-5555", "QQQ-5555", "ck-WWW-77777777", "WWW-7777"} {
		if strings.Contains(apiErr.Body, leak) || strings.Contains(err.Error(), leak) {
			t.Fatalf("%q leaked: %s", leak, apiErr.Body)
		}
	}
}

func TestPrefixMaskIsFixedAndStaysInTheBound(t *testing.T) {
	const token = "sk-ABCDEFGHIJKLMNOPQRSTUVWX1234"
	c := newRetryClient("http://example.invalid", nil)
	got := c.maskedBody([]byte(`{"echo":"`+token[:10]), true, token)
	if !strings.HasSuffix(got, `"echo":"****`) || strings.Contains(got, token[len(token)-4:]) {
		t.Fatalf("prefix-only match must become a bare ****, not a fabricated tail: %s", got)
	}
	// A truncated huge body stays within the bound, token or not.
	big := strings.Repeat("a", 1<<20) + token
	got = c.maskedBody([]byte(big), false, token)
	if len(got) > 4096+3 || !strings.HasSuffix(got, "...") {
		t.Fatalf("len=%d, want within 4096+3", len(got))
	}
	// Masking that shrinks the input under the bound still hides a dangling prefix.
	in := strings.Repeat("x", 4096-20) + strings.Repeat(token, 3)[:len(token)*2+9]
	got = c.maskedBody([]byte(in), false, token)
	if strings.Contains(got, "ABCDEFGH") || len(got) > 4096+3 {
		t.Fatalf("len=%d leak=%v", len(got), strings.Contains(got, "ABCDEFGH"))
	}
}

// maskedBody is a test convenience over the per-attempt mask set.
func (c *Client) maskedBody(b []byte, interrupted bool, extraCredentials ...string) string {
	return c.newMaskSet(extraCredentials...).body(b, interrupted)
}

func TestCrossHostRedirectDropsEveryCredentialHeader(t *testing.T) {
	retryHarness(t, 0)
	var mu sync.Mutex
	var got http.Header
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Clone()
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer target.Close()
	var sameHost http.Header
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/back" {
			mu.Lock()
			sameHost = r.Header.Clone()
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
			return
		}
		if r.URL.Path == "/local" {
			http.Redirect(w, r, "/back", http.StatusFound)
			return
		}
		http.Redirect(w, r, target.URL+"/landing", http.StatusFound)
	}))
	defer origin.Close()
	creds := map[string]string{
		"X-API-Key": "apikey-AAAA-1111", "X-Conduyt-Key": "ck-BBBB-2222", "X-Session-Token": "st-CCCC-3333",
		"Authorization": "Bearer ovr-DDDD-4444", "Cookie": "sid=cookie-EEEE-5555", "X-Request-Id": "req-1",
		"X-AccessToken": "at-FFFF-6666", "X-CSRFToken": "csrf-GGGG-7777", "X-APIToken": "apit-HHHH-8888", "X-Custom-Whatever": "w-1",
	}
	c := newRetryClient(origin.URL, nil)
	// gocli1219: Printing Press 4.34.2 refuses a cross-origin hop to a loopback/private literal outright: nothing
	// reaches the target. The allowlist on a followed public hop is TestPublicCrossOriginRedirectCarriesOnlyAllowlistedHeaders.
	if _, _, err := c.do(context.Background(), "GET", "/start", nil, nil, creds); !errors.Is(err, ErrRedirectPrivateDestination) {
		t.Fatalf("cross-origin redirect to a loopback literal: err=%v, want ErrRedirectPrivateDestination", err)
	}
	mu.Lock()
	if got != nil {
		t.Fatalf("refused redirect still reached the target: %v", got)
	}
	mu.Unlock()

	// Control: a same-host redirect keeps the custom credentials.
	if _, _, err := c.do(context.Background(), "GET", "/local", nil, nil, creds); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if sameHost.Get("X-API-Key") == "" || sameHost.Get("X-Conduyt-Key") == "" {
		t.Errorf("same-host redirect lost its credentials: %v", sameHost)
	}
}

func TestSentSecretsCoverMultipartContainersNumbersCookies(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "setcookie-9999-ZZZ"})
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"cookie":"` + r.Header.Get("Cookie") + `","body":` + strconvQuote(string(body)) + `,"set":"setcookie-9999-ZZZ"}`))
	}))
	defer srv.Close()
	mask := func(body any, hdr map[string]string) (*APIError, error) {
		c := newRetryClient(srv.URL, nil)
		_, _, err := c.do(context.Background(), "POST", "/x", nil, body, hdr)
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("err=%v", err)
		}
		return apiErr, err
	}
	cases := []struct {
		name  string
		body  any
		hdr   map[string]string
		leaks []string
	}{
		{"multipart fields", multipartRequestBody{Fields: map[string]string{"api_token": "tok-MPMP-123456", "password": "pw-MPMP-654321", "title": "hello-title"}},
			nil, []string{"tok-MPMP-123456", "pw-MPMP-654321", "MPMP-1234", "MPMP-6543"}},
		{"sensitive container of strings", map[string]any{"tokens": []string{"listed-secret-1111"}}, nil, []string{"listed-secret-1111"}},
		{"sensitive container of objects", map[string]any{"credentials": map[string]any{"value": "nested-secret-2222"}}, nil, []string{"nested-secret-2222"}},
		{"numeric secret", map[string]any{"pin": 1, "password": 8675309124}, nil, []string{"8675309124"}},
		{"manual cookie values", map[string]any{"a": "b"}, map[string]string{"Cookie": "a=cookieval-1111-AA; b=cookieval-2222-BB"}, []string{"cookieval-1111-AA", "cookieval-2222-BB"}},
		{"response Set-Cookie", map[string]any{"a": "b"}, nil, []string{"setcookie-9999-ZZZ"}},
	}
	for _, tc := range cases {
		apiErr, err := mask(tc.body, tc.hdr)
		for _, leak := range tc.leaks {
			if strings.Contains(apiErr.Body, leak) || strings.Contains(err.Error(), leak) {
				t.Errorf("%s: %q leaked: %s", tc.name, leak, apiErr.Body)
			}
		}
	}
}

func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestRedactionNeverEmitsAnotherCredential(t *testing.T) {
	c := newRetryClient("http://example.invalid", nil)
	ms := c.newMaskSet("abcdefgh1234", "1234")
	got := ms.mask("x abcdefgh1234 y 1234 z")
	if strings.Contains(got, "1234") || strings.Contains(got, "abcdefgh") {
		t.Fatalf("redaction emitted a credential: %s", got)
	}
}

func TestSensitiveNameRule(t *testing.T) {
	yes := []string{"Authorization", "X-API-Key", "X-Conduyt-Key", "X-Session-Token", "api_key", "apiKey", "accessToken", "refreshToken",
		"password", "newPassword", "client_secret", "clientSecret", "Cookie", "tokens", "credentials", "key", "x-amz-security-token",
		"Proxy-Authorization", "sessionId", "SessionID", "APIToken", "APIKey", "apiKEY", "API_KEY", "apiKey2", "access_key_id", "privateKey",
		"X-AccessToken", "X-CSRFToken", "X-APIToken", "otp", "pin", "passcode", "pwd", "verificationCode", "recoveryCode", "authCode",
		"securityCode", "backup_code", "reset-code", "x-xsrf-token", "twoFactorCode", "mfaCode",
		"apiKeys", "accessKeys", "privateKeys", "clientKeys", "code_verifier", "totp", "sig", "hmac", "x-hub-signature-256", "x-amz-signature", "signatures"}
	no := []string{"fieldKey", "keyword", "passengers", "author", "Content-Type", "Idempotency-Key", "sortKey", "email", "monkey", "compass",
		"X-Request-Id", "zipCode", "statusCode", "countryCode", "code", "pinned", "keys", "sortKeys", "primaryKey", "page", "limit"}
	for _, n := range yes {
		if !isSensitiveName(n) {
			t.Errorf("%q should be sensitive (words %v)", n, nameWords(n))
		}
	}
	for _, n := range no {
		if isSensitiveName(n) {
			t.Errorf("%q should stay readable (words %v)", n, nameWords(n))
		}
	}
}

func TestNameWordsSplitting(t *testing.T) {
	for in, want := range map[string]string{
		"X-API-Key": "x api key", "APIToken": "api token", "apiKEY": "api key", "SessionID": "session id",
		"apiKey2": "api key 2", "keyword": "keyword", "access_key_id": "access key id", "refreshToken": "refresh token",
	} {
		if got := strings.Join(nameWords(in), " "); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestSameOrigin(t *testing.T) {
	u := func(s string) *url.URL { r, _ := url.Parse(s); return r }
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"https://h/a", "https://h/b", true},
		{"https://h/a", "https://H:443/b", true},
		{"http://h/a", "http://h:80/b", true},
		{"https://h/a", "http://h/b", false},
		{"https://h/a", "https://h:8443/b", false},
		{"https://h/a", "https://other/b", false},
	} {
		if got := sameOrigin(u(tc.a), u(tc.b)); got != tc.want {
			t.Errorf("%s vs %s = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestDuplicateJSONKeysAreAllMasked(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"echo":` + strconvQuote(string(body)) + `}`))
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	// The MCP body_json override path hands the client a json.RawMessage.
	raw := json.RawMessage(`{"password":"first-pw-111111","user":{"token":"tok-one-2222","token":"tok-two-3333"},"password":"second-pw-222222"}`)
	_, _, err := c.do(context.Background(), "POST", "/x", nil, raw, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err=%v", err)
	}
	for _, leak := range []string{"first-pw-111111", "second-pw-222222", "tok-one-2222", "tok-two-3333"} {
		if strings.Contains(apiErr.Body, leak) || strings.Contains(err.Error(), leak) {
			t.Fatalf("%q leaked: %s", leak, apiErr.Body)
		}
	}
}

func TestHTTPSToHTTPRedirectOnTheSameHostDropsEverything(t *testing.T) {
	retryHarness(t, 0)
	var mu sync.Mutex
	var got http.Header
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Clone()
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer plain.Close()
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+"/landing", http.StatusFound) // same host (127.0.0.1), https -> http
	}))
	defer tlsSrv.Close()
	c := newRetryClient(tlsSrv.URL, nil)
	c.HTTPClient.Transport = tlsSrv.Client().Transport
	creds := map[string]string{
		"Authorization": "Bearer ovr-aaaa-1111", "X-API-Key": "apikey-bbbb-2222", "X-AccessToken": "at-cccc-3333",
		"X-CSRFToken": "csrf-dddd-4444", "X-APIToken": "apit-eeee-5555", "X-Request-Id": "req-9",
	}
	if _, _, err := c.do(context.Background(), "GET", "/start", nil, nil, creds); err != nil {
		// gocli1219: Printing Press 4.34.2 refuses an https -> http downgrade outright: nothing reaches the plain server.
		if !errors.Is(err, ErrRedirectProtocolDowngrade) {
			t.Fatal(err)
		}
		mu.Lock()
		defer mu.Unlock()
		if got != nil {
			t.Fatalf("refused downgrade still reached the plain server: %v", got)
		}
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for name := range creds {
		if name != "X-Request-Id" && got.Get(name) != "" {
			t.Errorf("https->http redirect target received %s", name)
		}
	}
	if got.Get("X-Request-Id") != "req-9" {
		t.Errorf("allowlisted header must travel: %v", got)
	}
}

func TestBodyStaysWithinTheBoundAfterTailMasking(t *testing.T) {
	const token = "sk-ABCDEFGHIJKLMNOPQRSTUVWX1234"
	c := newRetryClient("http://example.invalid", nil)
	for _, n := range []int{3, 4, 10} {
		in := strings.Repeat("a", 4096-n) + token[:n] + strings.Repeat("b", 500)
		got := c.maskedBody([]byte(in), false, token)
		if len(got) > 4096+3 || strings.Contains(got, token[:3]) {
			t.Fatalf("n=%d len=%d leak=%v", n, len(got), strings.Contains(got, token[:3]))
		}
	}
	// A 3-byte prefix right at the end of an interrupted read: 4093 + "****" would be 4097.
	got := c.maskedBody([]byte(strings.Repeat("a", 4093)+token[:3]), true, token)
	if len(got) > 4096+3 {
		t.Fatalf("len=%d, want within the bound", len(got))
	}
}

func TestLinkCallCtxMirrorsDeadlineAndCause(t *testing.T) {
	// Deadline: the linked context reports DeadlineExceeded, not Canceled.
	callCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	detached := context.WithValue(context.WithoutCancel(callCtx), callCtxKey{}, callCtx)
	linked, release := linkCallCtx(detached)
	<-linked.Done()
	release()
	if !errors.Is(linked.Err(), context.DeadlineExceeded) {
		t.Fatalf("Err=%v, want DeadlineExceeded", linked.Err())
	}
	// Cancellation with a cause is forwarded with the cause.
	cause := errors.New("operator pressed ctrl-c")
	parent, cancelCause := context.WithCancelCause(context.Background())
	detached = context.WithValue(context.WithoutCancel(parent), callCtxKey{}, parent)
	linked, release = linkCallCtx(detached)
	defer release()
	cancelCause(cause)
	select {
	case <-linked.Done():
	case <-time.After(time.Second):
		t.Fatal("cancellation was not forwarded")
	}
	if !errors.Is(linked.Err(), context.Canceled) || !errors.Is(context.Cause(linked), cause) {
		t.Fatalf("Err=%v cause=%v", linked.Err(), context.Cause(linked))
	}
}

func TestMutationNeverRedirectedToAnotherOrigin(t *testing.T) {
	retryHarness(t, 0)
	allowIdempotent(t, "POST /things/{id}")
	var hits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer target.Close()
	for _, code := range []int{http.StatusPermanentRedirect, http.StatusTemporaryRedirect, http.StatusFound, http.StatusSeeOther, http.StatusMovedPermanently} {
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL+"/landing", code)
		}))
		c := newRetryClient(origin.URL, nil)
		// A keyed (replay-safe) POST: still refused across origins.
		_, _, err := c.do(context.Background(), "POST", "/things/1", nil, map[string]string{"a": "b"}, nil)
		origin.Close()
		if err == nil || !strings.Contains(err.Error(), "cross-origin") || atomic.LoadInt32(&hits) != 0 {
			t.Fatalf("%d: err=%v targetHits=%d, want the redirect refused and nothing sent", code, err, atomic.LoadInt32(&hits))
		}
	}
}

func TestRedirectTargetCredentialsAreMasked(t *testing.T) {
	retryHarness(t, 0)
	const loc = "https://user:hunter2pass@127.0.0.1:1/x?access_token=tok-LOCATION-9999"
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", loc)
		w.WriteHeader(http.StatusFound)
	}))
	defer origin.Close()
	for _, method := range []string{"POST", "GET"} {
		var err error
		out := captureStderr(t, func() {
			c := newRetryClient(origin.URL, nil)
			c.CallBudget = 5 * time.Second
			_, _, err = c.do(context.Background(), method, "/start", nil, nil, nil)
		})
		if err == nil {
			t.Fatalf("%s: want an error", method)
		}
		for _, leak := range []string{"hunter2pass", "tok-LOCATION-9999", "LOCATION-9999"} {
			if strings.Contains(err.Error(), leak) || strings.Contains(out, leak) {
				t.Fatalf("%s: %q leaked: err=%v stderr=%q", method, leak, err, out)
			}
		}
	}
}

func TestPluralKeyArraysInheritSensitivity(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"echo":` + strconvQuote(string(body)) + `}`))
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	_, _, err := c.do(context.Background(), "POST", "/x", nil, map[string]any{"apiKeys": []string{"key-one-111111", "key-two-222222"}, "keys": []string{"plain-visible"}}, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal(err)
	}
	if strings.Contains(apiErr.Body, "key-one-111111") || strings.Contains(apiErr.Body, "key-two-222222") || !strings.Contains(apiErr.Body, "plain-visible") {
		t.Fatalf("body=%s", apiErr.Body)
	}
}

func TestSensitivePathParameterIsMasked(t *testing.T) {
	retryHarness(t, 0)
	const code = "SCRN-share-ABC987654"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":"no such code ` + code + ` at ` + r.URL.Path + `"}`))
	}))
	defer srv.Close()
	for _, path := range []string{"/public/screen-share/" + code, "/screen-share/session/" + code} {
		c := newRetryClient(srv.URL, nil)
		_, _, err := c.do(context.Background(), "GET", path, nil, nil, nil)
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatal(err)
		}
		if strings.Contains(apiErr.Path, "ABC987654") || strings.Contains(apiErr.Body, "ABC987654") || strings.Contains(err.Error(), "ABC987654") {
			t.Fatalf("%s: code leaked: path=%s body=%s", path, apiErr.Path, apiErr.Body)
		}
	}
	// Transport errors name the path too.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		_ = conn.Close()
	}))
	defer dead.Close()
	c := newRetryClient(dead.URL, nil)
	_, _, err := c.do(context.Background(), "POST", "/public/screen-share/"+code, nil, nil, nil)
	if err == nil || strings.Contains(err.Error(), "ABC987654") {
		t.Fatalf("transport error leaked the path code: %v", err)
	}
}

// Every spec path parameter that looks secret (or is a bare "code") must be
// listed in sensitivePathParams.
func TestSpecSensitivePathParametersAreListed(t *testing.T) {
	data, err := os.ReadFile("../../spec.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pathRe := regexp.MustCompile(`^  (/[^:\s]+):\s*$`)
	paramRe := regexp.MustCompile(`\{([^}]+)\}`)
	seen := 0
	for _, line := range strings.Split(string(data), "\n") {
		m := pathRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, pm := range paramRe.FindAllStringSubmatch(m[1], -1) {
			seen++
			name := pm[1]
			if !(isSensitiveName(name) || strings.EqualFold(name, "code")) {
				continue
			}
			listed := false
			for _, p := range sensitivePathParams[m[1]] {
				if p == name {
					listed = true
				}
			}
			if !listed {
				t.Errorf("spec path %s has sensitive-looking parameter {%s}: add it to sensitivePathParams", m[1], name)
			}
		}
	}
	if seen < 100 {
		t.Fatalf("scanner saw only %d path parameters", seen)
	}
}

func TestSecretBudgetFailsClosedOnErrorsAndCostsNothingOnSuccess(t *testing.T) {
	retryHarness(t, 0)
	tokens := make([]string, 1000)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("tok-%06d-secret", i)
	}
	status := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"echo":` + strconvQuote(string(body)) + `}`))
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	before := atomic.LoadInt32(&maskSetBuilds)
	if _, st, err := c.do(context.Background(), "POST", "/x", nil, map[string]any{"tokens": tokens}, nil); err != nil || st != 200 {
		t.Fatalf("200: st=%d err=%v", st, err)
	}
	if got := atomic.LoadInt32(&maskSetBuilds) - before; got != 0 {
		t.Fatalf("a successful call built %d mask sets, want 0", got)
	}
	status = 400
	_, st, err := c.do(context.Background(), "POST", "/x", nil, map[string]any{"tokens": tokens}, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || st != 400 || !strings.Contains(apiErr.Body, "withheld") || strings.Contains(apiErr.Body, "tok-000") {
		t.Fatalf("st=%d err=%v body=%q, want the status kept and the body withheld", st, err, apiErr)
	}
}

func TestAddURLCollectsPathFragmentRawQueryAndFailsClosed(t *testing.T) {
	sinkFor := func(raw string) *secretSink {
		k := &secretSink{}
		k.addURLValue(raw)
		return k
	}
	has := func(k *secretSink, v string) bool {
		for _, x := range k.list {
			if x == v {
				return true
			}
		}
		return false
	}
	// Secret path parameter from the escaped path.
	if k := sinkFor("https://h.example/public/screen-share/CODE-Path%2Fx-77777?z=1"); !has(k, "CODE-Path%2Fx-77777") {
		t.Errorf("path code missing: %v", k.list)
	}
	// Fragment values.
	if k := sinkFor("https://h.example/x#access_token=FRAG-secret-1111&state=ok"); !has(k, "FRAG-secret-1111") {
		t.Errorf("fragment secret missing: %v", k.list)
	}
	// Raw query pairs: lowercase percent-escapes raw and decoded; semicolons; malformed pairs.
	k := sinkFor("https://h.example/x?api_key=ab%2fcd-RAW-9999&client_secret=semi-1111;state=a%zz")
	if !has(k, "ab%2fcd-RAW-9999") || !has(k, "ab/cd-RAW-9999") || !has(k, "semi-1111") {
		t.Errorf("raw query secrets missing: %v", k.list)
	}
	// Unparseable value: the WHOLE value is treated as a secret.
	bad := "http://[::1/x?token=whatever-1234"
	if k := sinkFor(bad); !has(k, bad) {
		t.Errorf("unparseable Location must be masked whole: %v", k.list)
	}
	// Link headers.
	if k := sinkFor(`<https://h.example/next?access_token=LINK-secret-2222>; rel="next"`); !has(k, "LINK-secret-2222") {
		t.Errorf("Link target secret missing: %v", k.list)
	}
}

func TestRefererOfAnIntermediateHopIsMasked(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/middle?access_token=MIDDLE-secret-5555", http.StatusFound)
		case "/middle":
			http.Redirect(w, r, "/error", http.StatusFound)
		default:
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"referer":"` + r.Header.Get("Referer") + `"}`))
		}
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	_, _, err := c.do(context.Background(), "GET", "/start", nil, nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !strings.Contains(apiErr.Body, "referer") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(apiErr.Body, "MIDDLE-secret-5555") || strings.Contains(err.Error(), "MIDDLE-secret-5555") {
		t.Fatalf("Referer token leaked: %s", apiErr.Body)
	}
}

func TestDryRunAndVerifyHideSecretPathCodes(t *testing.T) {
	const code = "DRY-code-ABC123456"
	c := newRetryClient("https://api.example.invalid/api/v1", nil)
	c.DryRun = true
	out := captureStderr(t, func() {
		_, _, _ = c.do(context.Background(), "GET", "/public/screen-share/"+code, map[string]string{"api_key": "qk-DRYRUN-98765"}, nil, nil)
	})
	if strings.Contains(out, "ABC123456") || strings.Contains(out, "DRYRUN-98765") || !strings.Contains(out, "screen-share") {
		t.Fatalf("dry-run output: %q", out)
	}
	t.Setenv(cliutil.VerifyEnvVar, "1")
	c.DryRun = false
	raw, _, err := c.do(context.Background(), "POST", "/public/screen-share/"+code, nil, nil, nil)
	if err != nil || strings.Contains(string(raw), "ABC123456") || !strings.Contains(string(raw), "screen-share") {
		t.Fatalf("verify envelope: %s err=%v", raw, err)
	}
	if got := MaskPath("/screen-share/session/" + code + "?x=1"); strings.Contains(got, "ABC123456") {
		t.Fatalf("MaskPath: %s", got)
	}
}

func TestOverflowFailsClosedForPathAndErrorTextToo(t *testing.T) {
	retryHarness(t, 0)
	const code = "OVERFLOW-code-ZZZ987654"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":"no ` + code + ` at ` + r.URL.Path + `"}`))
	}))
	defer srv.Close()
	hdr := map[string]string{}
	for i := 0; i < 400; i++ {
		hdr[fmt.Sprintf("X-Token-%03d", i)] = fmt.Sprintf("hdr-secret-%06d", i)
	}
	c := newRetryClient(srv.URL, nil)
	_, _, err := c.do(context.Background(), "GET", "/public/screen-share/"+code, nil, nil, hdr)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal(err)
	}
	for _, leak := range []string{"ZZZ987654", "OVERFLOW-code"} {
		if strings.Contains(apiErr.Path, leak) || strings.Contains(apiErr.Body, leak) || strings.Contains(err.Error(), leak) {
			t.Fatalf("%q leaked under overflow: path=%q body=%q", leak, apiErr.Path, apiErr.Body)
		}
	}
	if !strings.Contains(apiErr.Path, "withheld") || !strings.Contains(apiErr.Body, "withheld") {
		t.Fatalf("overflow must withhold path and body: path=%q body=%q", apiErr.Path, apiErr.Body)
	}
	// The sink collects the request's own URL secrets before header floods.
	sink := c.sentSecrets(httptest.NewRequest("GET", srv.URL+"/public/screen-share/"+code+"?access_token=QUERY-first-4444", nil), nil, nil, nil, nil, nil, "/public/screen-share/"+code, "")
	foundQuery := false
	for _, v := range sink.list {
		if v == "QUERY-first-4444" {
			foundQuery = true
		}
	}
	if !foundQuery {
		t.Fatalf("query secret missing from the sink: %v", sink.list)
	}
}

func TestWholeRedirectChainIsCollected(t *testing.T) {
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/a?access_token=CHAIN-secret-6666", http.StatusFound)
		case "/a":
			http.Redirect(w, r, "/b", http.StatusFound)
		case "/b":
			http.Redirect(w, r, "/error", http.StatusFound)
		default:
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"seen":"` + r.Header.Get("Referer") + ` ` + r.URL.String() + `","tok":"CHAIN-secret-6666"}`))
		}
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	_, _, err := c.do(context.Background(), "GET", "/start", nil, nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !strings.Contains(apiErr.Body, "seen") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(apiErr.Body, "CHAIN-secret-6666") || strings.Contains(err.Error(), "CHAIN-secret-6666") {
		t.Fatalf("a token from a hop two redirects back leaked: %s", apiErr.Body)
	}
}

func TestURLFormsBareUserinfoEscapedFragmentAndRefresh(t *testing.T) {
	has := func(k *secretSink, v string) bool {
		for _, x := range k.list {
			if x == v {
				return true
			}
		}
		return false
	}
	k := &secretSink{}
	k.addURLValue("https://BARE-token-1111@host.example/x")
	if !has(k, "BARE-token-1111") {
		t.Errorf("bare userinfo missing: %v", k.list)
	}
	k = &secretSink{}
	k.addURLValue("https://user:pw-secret-2222@host.example/x")
	if has(k, "user") || !has(k, "pw-secret-2222") {
		t.Errorf("user:password form: %v", k.list)
	}
	k = &secretSink{}
	k.addURLValue("https://host.example/x#access_token=ab%2fcd-FRAG-3333")
	if !has(k, "ab%2fcd-FRAG-3333") || !has(k, "ab/cd-FRAG-3333") {
		t.Errorf("escaped fragment: %v", k.list)
	}
	k = &secretSink{}
	k.addURLValue("0; url=/public/screen-share/REFRESH-code-4444")
	if !has(k, "REFRESH-code-4444") {
		t.Errorf("Refresh target path code: %v", k.list)
	}
	k = &secretSink{}
	k.addURLValue(`5; URL='https://h.example/y?api_key=REFRESH-key-5555'`)
	if !has(k, "REFRESH-key-5555") {
		t.Errorf("Refresh target query: %v", k.list)
	}
}

func TestDryRunBodiesAreRedacted(t *testing.T) {
	c := newRetryClient("https://api.example.invalid/api/v1", nil)
	c.DryRun = true
	out := captureStderr(t, func() {
		_, _, _ = c.do(context.Background(), "POST", "/auth/login", nil,
			map[string]any{"email": "a@b.co", "password": "hunter2-DRY", "nested": map[string]any{"refreshToken": "rt-DRYRUN-1", "keep": "visible"}, "tokens": []string{"t-one-DRY"}, "pin": 123456}, nil)
	})
	for _, leak := range []string{"hunter2-DRY", "rt-DRYRUN-1", "t-one-DRY", "123456"} {
		if strings.Contains(out, leak) {
			t.Fatalf("%q leaked in dry-run body: %s", leak, out)
		}
	}
	if !strings.Contains(out, "a@b.co") || !strings.Contains(out, "visible") {
		t.Fatalf("non-secret fields must stay: %s", out)
	}
}

func TestMaskURL(t *testing.T) {
	for in, leaks := range map[string][]string{
		"https://user:pw-1234@h.example/api?token=TOK-abcd&page=2": {"pw-1234", "TOK-abcd"},
		"https://BARE-token@h.example/api#access_token=FRAG-abcd":  {"BARE-token", "FRAG-abcd"},
		"https://h.example/public/screen-share/CODE-secret-9?x=1":  {"CODE-secret-9"},
		"http://[::1/x?token=whatever-1234":                        {"whatever-1234"},
	} {
		got := MaskURL(in)
		for _, l := range leaks {
			if strings.Contains(got, l) {
				t.Errorf("MaskURL(%q) = %q leaks %q", in, got, l)
			}
		}
	}
	if got := MaskURL("https://h.example/api/v1?page=2"); got != "https://h.example/api/v1?page=2" {
		t.Errorf("a clean URL must be unchanged: %s", got)
	}
}

// Redaction keeps the original's CLASS (so callers can fall back to local
// data) without handing out the original's text.
func TestMaskedErrorKeepsNetworkIdentityWithoutText(t *testing.T) {
	retryHarness(t, 0)
	const code = "IDENT-code-ABC123456"
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := dead.URL
	dead.Close() // connection refused
	c := newRetryClient(url, nil)
	_, _, err := c.do(context.Background(), "GET", "/public/screen-share/"+code, nil, nil, nil)
	if err == nil || strings.Contains(err.Error(), "ABC123456") {
		t.Fatalf("err=%v", err)
	}
	var ue *neturl.Error
	var oe *net.OpError
	if !errors.As(err, &ue) || strings.Contains(ue.URL, "ABC123456") || strings.Contains(ue.Error(), "ABC123456") {
		t.Fatalf("errors.As *url.Error must work with a masked URL: %v", ue)
	}
	if !errors.As(err, &oe) || strings.Contains(oe.Error(), "ABC123456") {
		t.Fatalf("errors.As *net.OpError must work: %v", oe)
	}
	// Sentinels still match.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = c.do(ctx, "GET", "/public/screen-share/"+code, nil, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(Canceled) must survive masking: %v", err)
	}
}

type secretAddr struct{ s string }

func (a secretAddr) Network() string { return "tcp" }
func (a secretAddr) String() string  { return a.s }

func TestMaskedErrorClonesAreFullySanitized(t *testing.T) {
	const secret = "SECRET-code-9999"
	c := newRetryClient("http://example.invalid", nil)
	ms := c.newMaskSet(secret)
	orig := &neturl.Error{Op: "Get", URL: "http://h/public/screen-share/" + secret, Err: &net.OpError{
		Op: "dial", Net: "tcp", Source: secretAddr{"src-" + secret}, Addr: secretAddr{"addr-" + secret},
		Err: &net.DNSError{Err: "no such host " + secret, Name: secret + ".example", Server: "srv-" + secret,
			UnwrapErr: errors.New("lookup " + secret + ": boom")},
	}}
	err := ms.maskErr(orig)
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("text leaked: %v", err)
	}
	var ue *neturl.Error
	var oe *net.OpError
	var de *net.DNSError
	if !errors.As(err, &ue) || !errors.As(err, &oe) || !errors.As(err, &de) {
		t.Fatalf("classification lost: url=%v op=%v dns=%v", ue != nil, oe != nil, de != nil)
	}
	for name, text := range map[string]string{
		"url.Error": ue.Error(), "url.URL": ue.URL, "url.Err": ue.Err.Error(),
		"op.Error": oe.Error(), "op.Source": oe.Source.String(), "op.Addr": oe.Addr.String(), "op.Err": oe.Err.Error(),
		"dns.Error": de.Error(), "dns.Name": de.Name, "dns.Server": de.Server, "dns.Err": de.Err, "dns.Unwrap": de.Unwrap().Error(),
	} {
		if strings.Contains(text, secret) {
			t.Errorf("%s leaked: %q", name, text)
		}
	}
	// No path back to the original through Unwrap.
	var walk func(error, int)
	walk = func(e error, depth int) {
		if e == nil || depth > 20 {
			return
		}
		if e == error(orig) || e == orig.Err {
			t.Fatalf("the original is reachable through Unwrap")
		}
		switch u := e.(type) {
		case interface{ Unwrap() error }:
			walk(u.Unwrap(), depth+1)
		case interface{ Unwrap() []error }:
			for _, x := range u.Unwrap() {
				walk(x, depth+1)
			}
		}
	}
	walk(err, 0)
}

func TestMaskedErrorUnwrapExposesSafeClasses(t *testing.T) {
	c := newRetryClient("http://example.invalid", nil)
	ms := c.newMaskSet("whatever-secret")
	orig := fmt.Errorf("read tcp whatever-secret: %w", syscall.ECONNRESET)
	err := ms.maskErr(orig)
	var errno syscall.Errno
	if !errors.As(err, &errno) || errno != syscall.ECONNRESET {
		t.Fatalf("errors.As(&errno) = %v (%v)", errno, err)
	}
	if !errors.Is(err, syscall.ECONNRESET) || strings.Contains(err.Error(), "whatever-secret") {
		t.Fatalf("err=%v", err)
	}
	var plain interface{ Timeout() bool }
	_ = plain
}

func TestDryRunTargetURLIsMasked(t *testing.T) {
	c := newRetryClient("https://user:pw-DRY-1234@api.example.invalid/api/v1?access_token=TOK-DRY-5555", nil)
	c.DryRun = true
	out := captureStderr(t, func() {
		_, _, _ = c.do(context.Background(), "GET", "/public/screen-share/DRYURL-code-ABC123456", nil, nil, nil)
	})
	for _, leak := range []string{"pw-DRY-1234", "TOK-DRY-5555", "ABC123456"} {
		if strings.Contains(out, leak) {
			t.Fatalf("%q leaked in the dry-run target: %s", leak, out)
		}
	}
	if !strings.Contains(out, "api.example.invalid") {
		t.Fatalf("host must stay visible: %s", out)
	}
}

func TestMaskURLSeparatorsAndEscapedPaths(t *testing.T) {
	for in, leaks := range map[string][]string{
		"https://h.example/x?a=1;token=SEMI-secret-1;b=2":                 {"SEMI-secret-1"},
		"https://h.example/x#a=1;access_token=FRAG-semi-2&b=3":            {"FRAG-semi-2"},
		"https://h.example/public/screen-share/SECRET%2Fpart-ABC1234?x=1": {"SECRET", "ABC1234"},
		"https://h.example/screen-share/session/AA%3FBB-code-777":         {"AA%3FBB", "code-777"},
	} {
		got := MaskURL(in)
		for _, l := range leaks {
			if strings.Contains(got, l) {
				t.Errorf("MaskURL(%q) = %q leaks %q", in, got, l)
			}
		}
	}
	// Valid escaping of ordinary paths is preserved, separators and benign pairs stay.
	for _, in := range []string{"https://h.example/a%2Fb/c?x=1;y=2", "https://h.example/files/a%20b?q=a%26b"} {
		if got := MaskURL(in); got != in {
			t.Errorf("MaskURL(%q) = %q, want it unchanged", in, got)
		}
	}
	if got := MaskURL("https://h.example/public/screen-share/AB%2FCD-9999"); !strings.Contains(got, "screen-share/****") {
		t.Errorf("escaped segment must be replaced whole: %s", got)
	}
}

func TestVeryLongRedirectChainIsFullyWalked(t *testing.T) {
	retryHarness(t, 0)
	const hops = 25
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var n int
		if _, err := fmt.Sscanf(r.URL.Path, "/h%d", &n); err != nil {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"tok":"LONG-secret-7777"}`))
			return
		}
		if n >= hops {
			http.Redirect(w, r, "/done", http.StatusFound)
			return
		}
		next := fmt.Sprintf("/h%d", n+1)
		if n == 1 {
			next += "?access_token=LONG-secret-7777"
		}
		if n == 2 {
			next = "/h3"
		}
		http.Redirect(w, r, next, http.StatusFound)
	}))
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.HTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 40 {
			return errors.New("too many")
		}
		return nil
	}
	_, _, err := c.do(context.Background(), "GET", "/h1", nil, nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(apiErr.Body, "LONG-secret-7777") || strings.Contains(err.Error(), "LONG-secret-7777") {
		t.Fatalf("a secret from hop 2 of a %d-hop chain leaked: %s", hops, apiErr.Body)
	}
}

type fieldAddr struct{ network, text string }

func (a fieldAddr) Network() string { return a.network }
func (a fieldAddr) String() string  { return a.text }

// Each variant puts the secret in exactly ONE field, including fields that
// Error() never prints, so a mutant that stops masking any one field fails.
func TestEveryClonedFieldIsMaskedOnItsOwn(t *testing.T) {
	const secret = "FIELDSECRET-7777"
	c := newRetryClient("http://example.invalid", nil)
	ms := c.newMaskSet(secret)
	build := func(variant string) error {
		pick := func(name string) string {
			if name == variant {
				return secret
			}
			return "plain"
		}
		dns := &net.DNSError{Err: pick("dns.Err"), Name: pick("dns.Name"), Server: pick("dns.Server")}
		if variant == "dns.UnwrapErr" {
			dns.UnwrapErr = errors.New(secret)
		}
		return &neturl.Error{Op: pick("url.Op"), URL: "http://h/" + pick("url.URL"), Err: &net.OpError{
			Op: pick("op.Op"), Net: pick("op.Net"),
			Source: fieldAddr{pick("src.Network"), pick("src.String")},
			Addr:   fieldAddr{pick("addr.Network"), pick("addr.String")},
			Err:    dns,
		}}
	}
	for _, variant := range []string{"url.Op", "url.URL", "op.Op", "op.Net", "src.Network", "src.String", "addr.Network", "addr.String", "dns.Err", "dns.Name", "dns.Server", "dns.UnwrapErr"} {
		err := ms.maskErr(build(variant))
		var ue *neturl.Error
		var oe *net.OpError
		var de *net.DNSError
		if !errors.As(err, &ue) || !errors.As(err, &oe) || !errors.As(err, &de) {
			t.Fatalf("%s: classification lost", variant)
		}
		texts := []string{err.Error(), ue.Op, ue.URL, ue.Error(), oe.Op, oe.Net, oe.Error(), de.Err, de.Name, de.Server, de.Error()}
		for _, a := range []net.Addr{oe.Source, oe.Addr} {
			texts = append(texts, a.Network(), a.String())
		}
		if inner := de.Unwrap(); inner != nil {
			texts = append(texts, inner.Error())
		}
		for _, text := range texts {
			if strings.Contains(text, secret) {
				t.Errorf("variant %s leaked %q", variant, text)
			}
		}
	}
}

type flagErr struct{ timeout, temporary bool }

func (e flagErr) Error() string   { return "flag error" }
func (e flagErr) Timeout() bool   { return e.timeout }
func (e flagErr) Temporary() bool { return e.temporary }

func TestMaskedErrorForwardsTimeoutAndTemporary(t *testing.T) {
	c := newRetryClient("http://example.invalid", nil)
	ms := c.newMaskSet("x-secret-1234")
	for _, tc := range []flagErr{{true, false}, {false, true}, {true, true}, {false, false}} {
		err := ms.maskErr(fmt.Errorf("wrapped: %w", tc))
		to, _ := err.(interface{ Timeout() bool })
		te, _ := err.(interface{ Temporary() bool })
		if to == nil || te == nil || to.Timeout() != tc.timeout || te.Temporary() != tc.temporary {
			t.Errorf("flags %+v: Timeout/Temporary not forwarded", tc)
		}
	}
}

type cycleErr struct{}

func (e *cycleErr) Error() string { return "cycle" }
func (e *cycleErr) Unwrap() error { return e }

func TestMaskedErrorUnwrapBudget(t *testing.T) {
	c := newRetryClient("http://example.invalid", nil)
	ms := c.newMaskSet("budget-secret-1")
	// An errno placed late in a wide join is still found.
	errs := make([]error, 0, 1500)
	for i := 0; i < 1499; i++ {
		errs = append(errs, fmt.Errorf("plain %d", i))
	}
	errs = append(errs, syscall.ECONNRESET)
	var errno syscall.Errno
	if err := ms.maskErr(errors.Join(errs...)); !errors.As(err, &errno) || errno != syscall.ECONNRESET {
		t.Fatalf("late errno in a wide join not found: %v", errno)
	}
	// A cycle terminates within the budget.
	done := make(chan struct{})
	go func() { _ = ms.maskErr(&cycleErr{}).(interface{ Unwrap() []error }).Unwrap(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Unwrap did not terminate on a cyclic error")
	}
}

func TestUnparseableAndOpaqueURLValuesFailClosed(t *testing.T) {
	has := func(k *secretSink, v string) bool {
		for _, x := range k.list {
			if x == v {
				return true
			}
		}
		return false
	}
	bad := "http://[::1/x?token=whatever-1234&other=plain-5678;third=semi-9999"
	k := &secretSink{}
	k.addURLValue(bad)
	if !has(k, bad) || !has(k, "whatever-1234") || !has(k, "plain-5678") || !has(k, "semi-9999") {
		t.Errorf("unparseable value: whole value and every pair value must be secrets: %v", k.list)
	}
	k = &secretSink{}
	k.addURLValue("mailto:a@b.example?token=OPAQ-secret-1234&subject=hello-world")
	if !has(k, "OPAQ-secret-1234") || !has(k, "hello-world") {
		t.Errorf("opaque value: %v", k.list)
	}
	if got := MaskURL("mailto:a@b.example?token=OPAQ-secret-1234"); strings.Contains(got, "OPAQ") || strings.Contains(got, "a@b") {
		t.Errorf("opaque URL must be withheld: %q", got)
	}
	// End to end: an echoed bare value from an unparseable Location is masked.
	retryHarness(t, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Refresh", "0; url="+bad)
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"echo":"whatever-1234 semi-9999"}`))
	}))
	defer srv.Close()
	cl := newRetryClient(srv.URL, nil)
	_, _, err := cl.do(context.Background(), "GET", "/x", nil, nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || strings.Contains(apiErr.Body, "whatever-1234") || strings.Contains(apiErr.Body, "semi-9999") {
		t.Fatalf("bare echoed value leaked: err=%v", err)
	}
}

func TestRequestConstructionErrorsNeverCarryTheURL(t *testing.T) {
	retryHarness(t, 0)
	for _, base := range []string{
		"http://user:pw-INVALID-1234@127.0.0.1:badport/api?token=TOK-INVALID-5555",
		"http://127.0.0.1/api\x7f?access_token=CTRL-secret-9999",
	} {
		c := newRetryClient(base, nil)
		_, _, err := c.do(context.Background(), "GET", "/public/screen-share/CODE-INVALID-ABC123456", nil, nil, nil)
		if err == nil {
			t.Fatalf("%q: want a construction error", base)
		}
		for _, leak := range []string{"pw-INVALID-1234", "TOK-INVALID-5555", "CTRL-secret-9999", "ABC123456"} {
			if strings.Contains(err.Error(), leak) {
				t.Errorf("%q: %q leaked in %v", base, leak, err)
			}
		}
		if !strings.Contains(err.Error(), "creating request") {
			t.Errorf("error context lost: %v", err)
		}
	}
}

func TestMaskURLKeepsEmptyDelimiters(t *testing.T) {
	for _, in := range []string{"https://h.example/x#", "https://h.example/x?", "https://h.example/x?#", "https://h.example/x?a=1#"} {
		if got := MaskURL(in); got != in {
			t.Errorf("MaskURL(%q) = %q, want it byte-identical", in, got)
		}
	}
}

func TestSelfUnwrappingOriginalTerminatesThroughIsAsAndFlags(t *testing.T) {
	c := newRetryClient("http://example.invalid", nil)
	ms := c.newMaskSet("selfunwrap-secret-1")
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := ms.maskErr(&cycleErr{})
		var ue *neturl.Error
		var oe *net.OpError
		var de *net.DNSError
		if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || errors.As(err, &ue) || errors.As(err, &oe) || errors.As(err, &de) {
			t.Error("a self-unwrapping error must not match anything")
		}
		if te, ok := err.(interface{ Timeout() bool }); !ok || te.Timeout() {
			t.Error("Timeout() must be false")
		}
		if te, ok := err.(interface{ Temporary() bool }); !ok || te.Temporary() {
			t.Error("Temporary() must be false")
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Is/As/Timeout/Temporary hung on a self-unwrapping original")
	}
	// The bounded walk still finds real classes in a normal chain.
	real := fmt.Errorf("outer: %w", &neturl.Error{Op: "Get", URL: "http://h/x", Err: syscall.ECONNRESET})
	masked := ms.maskErr(real)
	var ue *neturl.Error
	if !errors.Is(masked, syscall.ECONNRESET) || !errors.As(masked, &ue) {
		t.Fatalf("classification lost: %v", masked)
	}
}

func TestConstructionErrorOverBudgetIsWithheld(t *testing.T) {
	marker := strings.Repeat("ZQXJ", 20000) // 80 KB token, past the 64 KiB budget
	c := newRetryClient("http://127.0.0.1:badport/api?access_token="+marker, nil)
	_, _, err := c.do(context.Background(), "GET", "/x", nil, nil, nil)
	if err == nil {
		t.Fatal("want a construction error")
	}
	if strings.Contains(err.Error(), "ZQXJZQXJ") {
		t.Fatalf("an over-budget invalid URL was returned raw (%d bytes)", len(err.Error()))
	}
	if !strings.Contains(err.Error(), "withheld") || !strings.Contains(err.Error(), "creating request") {
		t.Fatalf("err=%v", err)
	}
}

func TestMaskErrorForURL(t *testing.T) {
	raw := "http://user:pw-HOOK-1234@127.0.0.1:1/hook?access_token=TOK-HOOK-5555"
	_, doErr := http.Post(raw, "application/json", nil)
	if doErr == nil {
		t.Fatal("want a dial error")
	}
	got := MaskErrorForURL(doErr, raw)
	for _, leak := range []string{"pw-HOOK-1234", "TOK-HOOK-5555"} {
		if strings.Contains(got.Error(), leak) {
			t.Fatalf("%q leaked: %v", leak, got)
		}
	}
	var oe *net.OpError
	if !errors.As(got, &oe) {
		t.Fatalf("network class lost: %v", got)
	}
	if MaskErrorForURL(nil, raw) != nil {
		t.Fatal("nil in, nil out")
	}
}

// asErr implements a shallow custom As that fills a *neturl.Error carrying a
// secret, without that error being reachable through Unwrap.
type asErr struct{ secret string }

func (a *asErr) Error() string { return "custom as error" }
func (a *asErr) As(target any) bool {
	if t, ok := target.(**neturl.Error); ok {
		*t = &neturl.Error{Op: "Get", URL: "http://h/" + a.secret, Err: errors.New("boom " + a.secret)}
		return true
	}
	return false
}

func TestMaskedErrorAsHonoursCustomAsMethods(t *testing.T) {
	const secret = "SECRET-as-4242"
	c := newRetryClient("http://example.invalid", nil)
	ms := c.newMaskSet(secret)
	cases := map[string]error{
		"direct":       &asErr{secret},
		"wrapped":      fmt.Errorf("outer: %w", &asErr{secret}),
		"inside join":  errors.Join(errors.New("a"), fmt.Errorf("b: %w", &asErr{secret})),
		"behind chain": fmt.Errorf("x: %w", fmt.Errorf("y: %w", &asErr{secret})),
	}
	for name, orig := range cases {
		t.Run(name, func(t *testing.T) {
			err := ms.maskErr(orig)
			var ue *neturl.Error
			if !errors.As(err, &ue) {
				t.Fatalf("custom-As *url.Error not found through the masked chain")
			}
			for field, text := range map[string]string{"Op": ue.Op, "URL": ue.URL, "Error": ue.Error(), "Err": ue.Err.Error()} {
				if strings.Contains(text, secret) {
					t.Fatalf("%s leaked the secret: %q", field, text)
				}
			}
			var oe *net.OpError
			if errors.As(err, &oe) {
				t.Fatalf("unrelated target must not match")
			}
		})
	}
}

type wideJoin struct{ kids []error }

func (w wideJoin) Error() string   { return "wide" }
func (w wideJoin) Unwrap() []error { return w.kids }

func TestWalkChainWideJoinStopsAtBudget(t *testing.T) {
	kids := make([]error, 10000)
	for i := range kids {
		kids[i] = fmt.Errorf("kid %d", i)
	}
	visited := 0
	found, peak := walkChainPeak(wideJoin{kids}, func(error) bool { visited++; return false })
	if found {
		t.Fatal("unexpected match")
	}
	if visited != maxUnwrapNodes {
		t.Fatalf("visited %d nodes, want exactly the budget %d", visited, maxUnwrapNodes)
	}
	if peak != 1 {
		t.Fatalf("frame stack peaked at %d, want exactly one frame for the single join", peak)
	}
	// DFS order is kept: the first kids are the ones reached.
	var order []string
	walkChain(wideJoin{kids}, func(n error) bool {
		if len(order) < 3 {
			order = append(order, n.Error())
		}
		return false
	})
	if got := strings.Join(order, ","); got != "wide,kid 0,kid 1" {
		t.Fatalf("DFS order changed: %s", got)
	}
}

// firstKidWraps builds a wide join whose FIRST child wraps leaf two levels
// down and which is followed by thousands of plain siblings.
func firstKidWraps(leaf error) error {
	kids := make([]error, 6000)
	kids[0] = fmt.Errorf("a: %w", fmt.Errorf("b: %w", leaf))
	for i := 1; i < len(kids); i++ {
		kids[i] = fmt.Errorf("kid %d", i)
	}
	return wideJoin{kids}
}

type timeoutLeaf struct{}

func (timeoutLeaf) Error() string   { return "tl" }
func (timeoutLeaf) Timeout() bool   { return true }
func (timeoutLeaf) Temporary() bool { return true }

func TestMaskedErrorWideJoinFirstChildIsUnwrapped(t *testing.T) {
	const secret = "SECRET-wide-777"
	c := newRetryClient("http://example.invalid", nil)
	ms := c.newMaskSet(secret)

	t.Run("Is parity with errors.Is", func(t *testing.T) {
		orig := firstKidWraps(io.EOF)
		if !errors.Is(orig, io.EOF) {
			t.Fatal("test setup: errors.Is must find io.EOF")
		}
		if !errors.Is(ms.maskErr(orig), io.EOF) {
			t.Fatal("masked error missed io.EOF under a wide join")
		}
	})
	t.Run("custom As", func(t *testing.T) {
		var ue *neturl.Error
		err := ms.maskErr(firstKidWraps(&asErr{secret}))
		if !errors.As(err, &ue) {
			t.Fatal("custom-As error not found under a wide join")
		}
		if strings.Contains(ue.Error(), secret) || strings.Contains(ue.URL, secret) {
			t.Fatalf("leaked secret: %q", ue.Error())
		}
	})
	t.Run("timeout and temporary", func(t *testing.T) {
		err := ms.maskErr(firstKidWraps(timeoutLeaf{}))
		var ne net.Error
		if !errors.As(err, &ne) || !ne.Timeout() {
			t.Fatalf("timeout classification lost under a wide join: %v", err)
		}
		if tmp, ok := err.(interface{ Temporary() bool }); !ok || !tmp.Temporary() {
			t.Fatal("temporary classification lost under a wide join")
		}
	})
	t.Run("budget still holds", func(t *testing.T) {
		visited := 0
		walkChain(firstKidWraps(errors.New("x")), func(error) bool { visited++; return false })
		if visited != maxUnwrapNodes {
			t.Fatalf("visited %d, want %d", visited, maxUnwrapNodes)
		}
	})
}
