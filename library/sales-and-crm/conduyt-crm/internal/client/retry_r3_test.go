package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// PATCH(retry-policy): gocli1219 round 3 (Codex gocli1219-r2).

// wrappingTransport is a RoundTripper around a real (pooled) transport: the client cannot inspect or clone it.
type wrappingTransport struct{ inner http.RoundTripper }

func (w wrappingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return w.inner.RoundTrip(r)
}

// warmDropServer answers /warm and drops /drop after the request arrived; it records each request's remote address.
func warmDropServer(t *testing.T, keyHeader string) (srv *httptest.Server, drops *int32, sawKey *int32, addrs func() []string) {
	t.Helper()
	drops, sawKey = new(int32), new(int32)
	var mu sync.Mutex
	var seen []string
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Path+"@"+r.RemoteAddr)
		mu.Unlock()
		if r.URL.Path == "/drop" {
			atomic.AddInt32(drops, 1)
			if r.Header.Get(keyHeader) != "" {
				atomic.AddInt32(sawKey, 1)
			}
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("ok"))
	}))
	return srv, drops, sawKey, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), seen...) }
}

// r2 P1: a nil transport (net/http's DefaultTransport) gets the keep-alive-free clone, never the pooled default.
func TestKeyedMutationOnANilTransportIsSentOnce(t *testing.T) {
	retryHarness(t, 0)
	srv, drops, sawKey, _ := warmDropServer(t, "Idempotency-Key")
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.HTTPClient.Transport = nil
	keyed := map[string]string{"Idempotency-Key": "k-3"}
	if _, _, err := c.do(context.Background(), "POST", "/warm", nil, map[string]string{"a": "b"}, keyed); err != nil {
		t.Fatal(err)
	}
	_, _, err := c.do(context.Background(), "POST", "/drop", nil, map[string]string{"a": "b"}, keyed)
	if d, k := atomic.LoadInt32(drops), atomic.LoadInt32(sawKey); err == nil || d != 1 || k != 1 {
		t.Fatalf("nil transport: drops=%d keyOnWire=%d err=%v, want exactly one keyed POST", d, k, err)
	}
	client, kerr := c.noReuseMutationClient()
	if kerr != nil || client == c.HTTPClient {
		t.Fatalf("nil transport: keyed client=%p shared=%p err=%v, want a dedicated client", client, c.HTTPClient, kerr)
	}
	if tr, ok := client.Transport.(*http.Transport); !ok || !tr.DisableKeepAlives || tr == http.DefaultTransport {
		t.Fatalf("nil transport: keyed transport %#v, want a keep-alive-free clone of DefaultTransport", client.Transport)
	}
}

// r2 P1: a wrapper RoundTripper (here around a real pooled transport) cannot be proven non-reusing, so a keyed
// mutation is refused before anything is sent; reads and unkeyed mutations still go through it.
func TestKeyedMutationThroughAWrapperTransportIsRefusedAndSendsNothing(t *testing.T) {
	retryHarness(t, 0)
	srv, _, _, addrs := warmDropServer(t, "Idempotency-Key")
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	c.HTTPClient.Transport = wrappingTransport{inner: &http.Transport{}}
	_, _, err := c.do(context.Background(), "POST", "/warm", nil, map[string]string{"a": "b"}, map[string]string{"Idempotency-Key": "k-4"})
	var refusal errKeyedMutationTransport
	if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "wrappingTransport") || !strings.Contains(err.Error(), "nothing was sent") {
		t.Fatalf("keyed POST through a wrapper: err=%v, want the refusal naming the transport", err)
	}
	if got := addrs(); len(got) != 0 {
		t.Fatalf("refused keyed POST still reached the server: %v", got)
	}
	if client, kerr := c.noReuseMutationClient(); client != nil || kerr == nil {
		t.Fatalf("wrapper transport: noReuseMutationClient returned client=%v err=%v, want no client and the refusal", client, kerr)
	}
	if _, _, err := c.do(context.Background(), "POST", "/warm", nil, map[string]string{"a": "b"}, nil); err != nil {
		t.Fatalf("unkeyed POST through the wrapper: %v", err)
	}
	if _, _, err := c.do(context.Background(), "GET", "/warm", nil, nil, nil); err != nil {
		t.Fatalf("GET through the wrapper: %v", err)
	}
}
