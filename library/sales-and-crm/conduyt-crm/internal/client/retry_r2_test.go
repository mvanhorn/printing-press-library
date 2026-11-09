package client

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/platform"
)

// PATCH(retry-policy) / PATCH(redirect-and-redaction-policy): gocli1219 round 2 (Codex gocli1219-r1).

// r1 P1: a mutation keyed per call (a spec-declared --idempotency-key flag) dropped after the request arrived on a
// warm connection reaches the server exactly once, with the key on the wire: neither the CLI nor Go's transport
// resends it (the control test above shows Go would on a reused connection).
func TestPerCallKeyedMutationDroppedOnAWarmConnectionIsSentOnce(t *testing.T) {
	retryHarness(t, 0)
	for _, h := range []string{"Idempotency-Key", "X-Idempotency-Key"} {
		var drops, sawKey int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/drop" {
				atomic.AddInt32(&drops, 1)
				if r.Header.Get(h) == "k-1" {
					atomic.AddInt32(&sawKey, 1)
				}
				conn, _, _ := w.(http.Hijacker).Hijack()
				_ = conn.Close() // drop before the first response byte
				return
			}
			_, _ = w.Write([]byte(`{}`))
		}))
		c := newRetryClient(srv.URL, nil)
		keyed := map[string]string{h: "k-1"}
		// Warm the mutation transports with a keyed POST, then drop the next one.
		if _, _, err := c.do(context.Background(), "POST", "/warm", nil, map[string]string{"a": "b"}, keyed); err != nil {
			t.Fatal(err)
		}
		_, _, err := c.do(context.Background(), "POST", "/drop", nil, map[string]string{"a": "b"}, keyed)
		srv.Close()
		if d, k := atomic.LoadInt32(&drops), atomic.LoadInt32(&sawKey); err == nil || d != 1 || k != 1 {
			t.Fatalf("%s: drops=%d keyOnWire=%d err=%v, want exactly one POST carrying the key and an error", h, d, k, err)
		}
	}
}

func TestKeyedMutationClientDisablesKeepAlivesAndStaysHTTP1(t *testing.T) {
	c := newRetryClient("http://127.0.0.1:1", nil)
	keyed, err := c.noReuseMutationClient()
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := keyed.Transport.(*http.Transport)
	if !ok || !tr.DisableKeepAlives || tr.ForceAttemptHTTP2 {
		t.Fatalf("keyed mutation transport = %#v, want an HTTP/1 transport with keep-alives off", keyed.Transport)
	}
	if shared, ok := c.HTTPClient.Transport.(*http.Transport); ok && shared.DisableKeepAlives {
		t.Fatal("the shared client must keep its keep-alives")
	}
}

// r1 P2-2: a binary (streaming) mutation keeps the no-replay transport chosen for it.
func TestStreamingMutationKeepsTheNoReplayTransport(t *testing.T) {
	var protos []string
	var mu sync.Mutex
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		protos = append(protos, r.Proto)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("bytes"))
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.HTTPClient.Transport = srv.Client().Transport
	for _, ovr := range []map[string]string{
		{BinaryResponseHeader: "true"},
		{BinaryResponseHeader: "true", "Idempotency-Key": "k-2"},
	} {
		if _, _, err := c.do(context.Background(), "POST", "/export", nil, map[string]string{"a": "b"}, ovr); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(protos) != 2 || protos[0] != "HTTP/1.1" || protos[1] != "HTTP/1.1" {
		t.Fatalf("streaming POSTs arrived as %v, want HTTP/1.1 (the no-replay mutation transport), never h2", protos)
	}
}

// r1 P2-1: a GET whose body resets mid-read is not retried once the provider retry budget is spent.
func TestBrokenBodyRetryHonoursThePlatformRetryBudget(t *testing.T) {
	retryHarness(t, 0)
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Length", "100")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		conn, _, _ := w.(http.Hijacker).Hijack()
		_ = conn.Close()
	}))
	defer server.Close()
	client, _ := platformRateLimitTestClient(t, server)
	if err := client.SetPlatformEndpointBudget(platform.EndpointBudget{
		Class: "*", Steady: 60, Interval: time.Minute, Burst: 10, MaxAttempts: 3, RetryBudget: time.Nanosecond,
	}); err != nil {
		t.Fatal(err)
	}
	_, _, err := client.do(context.Background(), "GET", "/broken", nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "reading response") {
		t.Fatalf("err=%v, want the body-read failure", err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("calls=%d, want 1: the spent provider retry budget must stop the body-reset retry", n)
	}
}

// r1 P3-1: a call budget that runs out inside the platform limiter on a later attempt keeps the stop classification
// (the caller's deadline wins, the last response stays attached), like the adaptive limiter.
func TestPlatformLimiterWaitOnARetryKeepsTheLastResponse(t *testing.T) {
	retryHarness(t, 0)
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	}))
	defer server.Close()
	client, _ := platformRateLimitTestClient(t, server)
	// One token per minute: the first attempt takes it, the retry waits in the platform limiter.
	if err := client.SetPlatformEndpointBudget(platform.EndpointBudget{
		Class: "*", Steady: 1, Interval: time.Minute, Burst: 1, MaxAttempts: 3, RetryBudget: time.Hour,
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond) // room for the 1s backoff, not for the limiter
	defer cancel()
	_, _, err := client.do(ctx, "GET", "/x", nil, nil, nil)
	var apiErr *APIError
	if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &apiErr) || apiErr.StatusCode != 503 {
		t.Fatalf("err=%v, want DeadlineExceeded wrapping the 503 APIError", err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("calls=%d, want 1 (the retry never left the limiter)", n)
	}
}

// r1 P3-2: a GET redirected to a PUBLIC-looking host on another origin is followed and the target receives only the
// allowlisted headers. The target name resolves to a public address through the test resolver and is dialled to the
// local test server, so the 4.34.2 private-destination guard is not what drops the credentials.
func TestPublicCrossOriginRedirectCarriesOnlyAllowlistedHeaders(t *testing.T) {
	retryHarness(t, 0)
	var mu sync.Mutex
	var got http.Header
	var hits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		mu.Lock()
		got = r.Header.Clone()
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer target.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(target.URL, "http://"))
	const publicHost = "redirect-target.example.com"
	publicIP := net.ParseIP("93.184.216.34")
	restoreResolve := swapDialResolve(func(ctx context.Context, host string) ([]net.IP, error) {
		if host == publicHost {
			return []net.IP{publicIP}, nil
		}
		return net.DefaultResolver.LookupIP(ctx, "ip", host)
	})
	defer restoreResolve()
	restoreDial := swapDialPinned(func(ctx context.Context, network, addr string) (net.Conn, error) {
		if host, p, err := net.SplitHostPort(addr); err == nil && net.ParseIP(host).Equal(publicIP) {
			addr = net.JoinHostPort("127.0.0.1", p)
		}
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	})
	defer restoreDial()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+publicHost+":"+port+"/landing", http.StatusFound)
	}))
	defer origin.Close()
	creds := map[string]string{
		"X-API-Key": "apikey-AAAA-1111", "X-Conduyt-Key": "ck-BBBB-2222", "X-Session-Token": "st-CCCC-3333",
		"Authorization": "Bearer ovr-DDDD-4444", "Cookie": "sid=cookie-EEEE-5555", "X-Request-Id": "req-1",
		"X-AccessToken": "at-FFFF-6666", "X-Custom-Whatever": "w-1",
	}
	c := New(&config.Config{BaseURL: origin.URL}, 5*time.Second, 0)
	if _, _, err := c.do(context.Background(), "GET", "/start", nil, nil, creds); err != nil {
		t.Fatalf("public cross-origin GET redirect: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if atomic.LoadInt32(&hits) != 1 || got == nil {
		t.Fatalf("target hits=%d: the redirect was not followed to the public-looking host", hits)
	}
	for name := range creds {
		if name != "X-Request-Id" && got.Get(name) != "" {
			t.Errorf("cross-origin target received %s", name)
		}
	}
	if got.Get("X-Request-Id") != "req-1" {
		t.Errorf("an allowlisted header must still travel: %v", got)
	}
}
