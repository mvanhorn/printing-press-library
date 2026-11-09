package client

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// PATCH(retry-policy): gocli1219 round 3 (Codex gocli1219-r2 P3): keyed streaming mutations.

// r2 P3: a keyed BINARY (streaming) mutation dropped after the request arrived reaches the server once, on a fresh
// connection (the warm request's connection is not reused), with the key on the wire.
func TestKeyedStreamingMutationDroppedAfterAWarmCallIsSentOnceOnAFreshConnection(t *testing.T) {
	retryHarness(t, 0)
	srv, drops, sawKey, addrs := warmDropServer(t, "Idempotency-Key")
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	ovr := map[string]string{BinaryResponseHeader: "true", "Idempotency-Key": "k-5"}
	// Warm the keyed transport with an ordinary (non-streaming) keyed POST: a pooled transport would park that
	// connection, and the streaming call below wraps the same keyed transport.
	if _, _, err := c.do(context.Background(), "POST", "/warm", nil, map[string]string{"a": "b"}, map[string]string{"Idempotency-Key": "k-5"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := c.do(context.Background(), "POST", "/drop", nil, map[string]string{"a": "b"}, ovr)
	if d, k := atomic.LoadInt32(drops), atomic.LoadInt32(sawKey); err == nil || d != 1 || k != 1 {
		t.Fatalf("keyed streaming POST: drops=%d keyOnWire=%d err=%v, want exactly one /drop hit carrying the key", d, k, err)
	}
	got := addrs()
	if len(got) != 2 {
		t.Fatalf("requests %v, want /warm then one /drop", got)
	}
	warmAddr, dropAddr := got[0][strings.Index(got[0], "@")+1:], got[1][strings.Index(got[1], "@")+1:]
	if warmAddr == dropAddr {
		t.Fatalf("the keyed streaming /drop reused the warm connection %s, want a fresh one", warmAddr)
	}
}

// r2 P3 (direct proof): the client a keyed BINARY mutation streams through has keep-alives off and speaks HTTP/1 only;
// an unkeyed binary mutation keeps the HTTP/1-only mutation transport; a binary read keeps the shared policy. The
// warm/drop test above shows the behaviour end to end (StreamingHTTPClient clones the transport per call, so a pooled
// policy would only show inside one call; this pins the policy itself).
func TestKeyedStreamingMutationClientHasKeepAlivesOffAndIsHTTP1(t *testing.T) {
	c := newRetryClient("http://127.0.0.1:1", nil)
	keyedOvr := map[string]string{BinaryResponseHeader: "true", "Idempotency-Key": "k-6"}
	hc, idle, err := c.attemptHTTPClient("POST", keyedOvr, false, true)
	if err != nil || idle <= 0 {
		t.Fatalf("keyed binary POST: err=%v idle=%s", err, idle)
	}
	tr, ok := hc.Transport.(*http.Transport)
	if !ok || !tr.DisableKeepAlives || tr.ForceAttemptHTTP2 || tr.Protocols == nil || tr.Protocols.HTTP2() {
		t.Fatalf("keyed binary POST transport %#v, want HTTP/1 only with keep-alives off", hc.Transport)
	}
	hc, _, _ = c.attemptHTTPClient("POST", map[string]string{BinaryResponseHeader: "true"}, false, true)
	if tr, ok := hc.Transport.(*http.Transport); !ok || tr.Protocols == nil || tr.Protocols.HTTP2() {
		t.Fatalf("unkeyed binary POST transport %#v, want the HTTP/1-only mutation transport", hc.Transport)
	}
	c2 := newRetryClient("http://127.0.0.1:1", nil)
	c2.HTTPClient.Transport = wrappingTransport{inner: &http.Transport{}}
	if hc, _, err := c2.attemptHTTPClient("POST", keyedOvr, false, true); err == nil || hc != nil {
		t.Fatalf("keyed binary POST through a wrapper: client=%v err=%v, want the refusal and no client", hc, err)
	}
}
