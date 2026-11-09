package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// PATCH(retry-policy): gocli1219 round 4 (Codex gocli1219-r3): the no-reuse decision keys off the call's intent
// (not replay-safe), never off the wire method.

// A keyed state-changing GET (GetMutatingWithHeaders) dropped after it arrived, behind a warm call, reaches the server
// exactly once, carrying the key, on a fresh connection.
func TestKeyedMutatingGETDroppedAfterAWarmCallIsSentOnce(t *testing.T) {
	retryHarness(t, 0)
	srv, drops, sawKey, addrs := warmDropServer(t, "Idempotency-Key")
	defer srv.Close()
	c := newRetryClient(srv.URL, nil)
	keyed := map[string]string{"Idempotency-Key": "k-7", "Accept": "application/json"}
	// /warm answers "ok" with an octet-stream type; a plain body is enough for the client to return.
	_, _ = c.GetMutatingWithHeaders(context.Background(), "/warm", nil, keyed)
	_, err := c.GetMutatingWithHeaders(context.Background(), "/drop", nil, keyed)
	if d, k := atomic.LoadInt32(drops), atomic.LoadInt32(sawKey); err == nil || d != 1 || k != 1 {
		t.Fatalf("keyed mutating GET: drops=%d keyOnWire=%d err=%v, want exactly one /drop hit carrying the key", d, k, err)
	}
	got := addrs()
	if len(got) != 2 || !strings.HasPrefix(got[0], "/warm@") || !strings.HasPrefix(got[1], "/drop@") {
		t.Fatalf("requests %v, want /warm then one /drop", got)
	}
	if got[0][strings.Index(got[0], "@")+1:] == got[1][strings.Index(got[1], "@")+1:] {
		t.Fatalf("the keyed mutating GET reused the warm connection: %v", got)
	}
}

// Every method the client can send with mutation intent (doMutation / a call that is not replay-safe): keyed, it
// takes the keep-alive-free client; a method Go's transport resends on its own (GET, HEAD, OPTIONS, TRACE) takes it
// even without a key. A replay-safe read keeps the shared pooled client.
func TestNoReuseClientIsChosenByIntentForEveryMethod(t *testing.T) {
	c := newRetryClient("http://127.0.0.1:1", nil)
	noReuse := func(hc *http.Client) bool {
		tr, ok := hc.Transport.(*http.Transport)
		return ok && tr.DisableKeepAlives
	}
	key := map[string]string{"Idempotency-Key": "k-8"}
	for _, m := range []string{"GET", "HEAD", "OPTIONS", "TRACE", "POST", "PUT", "PATCH", "DELETE", "get", "post"} {
		hc, _, err := c.attemptHTTPClient(m, key, false, false)
		if err != nil || !noReuse(hc) {
			t.Errorf("%s keyed, not replay-safe: client %#v err=%v, want the no-reuse client", m, hc.Transport, err)
		}
		hc, _, err = c.attemptHTTPClient(m, key, false, true)
		if err != nil || !noReuse(hc) {
			t.Errorf("%s keyed streaming, not replay-safe: client %#v err=%v, want the no-reuse client", m, hc.Transport, err)
		}
	}
	for _, m := range []string{"GET", "HEAD", "OPTIONS", "TRACE"} {
		if hc, _, err := c.attemptHTTPClient(m, nil, false, false); err != nil || !noReuse(hc) {
			t.Errorf("%s unkeyed, not replay-safe: client %#v err=%v, want the no-reuse client (Go replays it on its own)", m, hc.Transport, err)
		}
	}
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		hc, _, err := c.attemptHTTPClient(m, nil, false, false)
		if err != nil || noReuse(hc) || hc == c.HTTPClient {
			t.Errorf("%s unkeyed, not replay-safe: want the pooled HTTP/1 mutation client (Go never replays it), got %#v err=%v", m, hc.Transport, err)
		}
	}
	if hc, _, _ := c.attemptHTTPClient("GET", key, true, false); hc != c.HTTPClient {
		t.Error("a replay-safe read must keep the shared client")
	}
}

// A state-changing GET is never redirected to another origin (its intent is a mutation even though the wire method
// is GET).
func TestMutatingGETIsNeverRedirectedCrossOrigin(t *testing.T) {
	retryHarness(t, 0)
	var targetHits int32
	target := newCountingServer(t, &targetHits)
	origin := newRedirectServer(t, target.URL+"/landing", http.StatusFound)
	c := newRetryClient(origin.URL, nil)
	_, err := c.GetMutatingWithHeaders(context.Background(), "/start", nil, nil)
	if err == nil || atomic.LoadInt32(&targetHits) != 0 {
		t.Fatalf("mutating GET cross-origin redirect: err=%v targetHits=%d, want a refusal and nothing sent", err, targetHits)
	}
}

func newCountingServer(t *testing.T, hits *int32) *httptestServer {
	t.Helper()
	s := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(hits, 1)
		_, _ = w.Write([]byte(`{}`))
	}))
	return s
}

func newRedirectServer(t *testing.T, to string, code int) *httptestServer {
	t.Helper()
	return newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, code)
	}))
}

type httptestServer = httptest.Server

func newTestServer(t *testing.T, h http.Handler) *httptestServer {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s
}
