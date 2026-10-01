package client

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/config"
)

type writeSafetyTransport func(*http.Request) (*http.Response, error)

func (f writeSafetyTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestAmbiguousWritesNeverReplay(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	paths := []string{"/m/api/orders/braintreeCheckout", "/api/markPromotionUsed", "/m/api/postmicmeshorder"}
	for _, path := range paths {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			for _, status := range []int{0, 429, 500, 503} {
				t.Run(path+"/"+method+"/"+http.StatusText(status), func(t *testing.T) {
					c := New(&config.Config{BaseURL: "http://fixture.invalid", AuthHeaderVal: "fixture-token"}, time.Second, 0)
					c.cacheDir = t.TempDir()
					calls := 0
					c.HTTPClient.Transport = writeSafetyTransport(func(req *http.Request) (*http.Response, error) {
						calls++
						if req.GetBody != nil {
							t.Error("write request permits net/http to replay its body")
						}
						if status == 0 {
							return nil, errors.New("synthetic reset after possible commit")
						}
						return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"synthetic failure"}`)), Request: req}, nil
					})
					_, _, err := c.do(method, path, nil, map[string]any{"value": "once"}, map[string]string{"Authorization": "fixture-token", "Idempotency-Key": "not-proof-of-provider-deduplication"})
					if err == nil || calls != 1 {
						t.Fatalf("error=%v calls=%d; want error and exactly one attempt", err, calls)
					}
				})
			}
		}
	}
}

func TestWriteRedirectNeverReplaysPaymentBody(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			c := New(&config.Config{BaseURL: "http://fixture.invalid", AuthHeaderVal: "fixture-token"}, time.Second, 0)
			c.cacheDir = t.TempDir()
			calls := 0
			c.HTTPClient.Transport = writeSafetyTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{
					StatusCode: status,
					Header:     http.Header{"Location": []string{"http://fixture.invalid/redirected-checkout"}},
					Body:       io.NopCloser(strings.NewReader(`{"error":"redirect"}`)),
					Request:    req,
				}, nil
			})
			_, _, err := c.do(http.MethodPost, "/m/api/postmicmeshorder", nil, map[string]any{"value": "once"}, map[string]string{"Authorization": "fixture-token"})
			if err == nil || calls != 1 {
				t.Fatalf("redirect error=%v calls=%d; want error and exactly one POST", err, calls)
			}
		})
	}
}

func TestCheckoutPostDoesNotReplayAfterReusedConnectionDrop(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	var mu sync.Mutex
	var warmupAddress string
	postCount := 0
	reusedConnection := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/warmup" {
			mu.Lock()
			warmupAddress = r.RemoteAddr
			mu.Unlock()
			_, _ = io.WriteString(w, "ready")
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/m/api/postmicmeshorder" {
			t.Errorf("unexpected mock request %s %s", r.Method, r.URL.Path)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		postCount++
		reusedConnection = reusedConnection || r.RemoteAddr == warmupAddress
		mu.Unlock()
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack mock checkout connection: %v", err)
			return
		}
		_ = connection.Close() // Simulate a lost response after possible commit.
	}))
	defer server.Close()
	c := New(&config.Config{
		BaseURL: server.URL, AuthHeaderVal: "fixture-token",
		Headers: map[string]string{"Idempotency-Key": "fixture-key"},
	}, time.Second, 0)
	c.cacheDir = t.TempDir()
	response, err := c.HTTPClient.Get(server.URL + "/warmup")
	if err != nil {
		t.Fatalf("warm reusable connection: %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	_, _, err = c.do(http.MethodPost, "/m/api/postmicmeshorder", nil, map[string]any{"value": "once"}, nil)
	mu.Lock()
	defer mu.Unlock()
	if err == nil || postCount != 1 || !reusedConnection {
		t.Fatalf("checkout error=%v posts=%d reused_connection=%t; want error after one POST on the warmed connection", err, postCount, reusedConnection)
	}
}

func TestReadRateLimitRecoveryRemainsBounded(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	for _, exhausted := range []bool{false, true} {
		c := New(&config.Config{BaseURL: "http://fixture.invalid", AuthHeaderVal: "fixture-token"}, time.Second, 0)
		c.cacheDir = t.TempDir()
		calls := 0
		firstRequestID := ""
		c.HTTPClient.Transport = writeSafetyTransport(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				firstRequestID = req.Header.Get("__requestid")
			} else if req.Header.Get("__requestid") != firstRequestID {
				t.Error("retry changed request identity")
			}
			status := http.StatusOK
			if calls == 1 || exhausted {
				status = http.StatusTooManyRequests
			}
			return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": []string{"1"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Request: req}, nil
		})
		_, _, err := c.do(http.MethodGet, "/m/api/orders", nil, nil, map[string]string{"Authorization": "fixture-token"})
		wantCalls := 2
		if exhausted {
			wantCalls = 4
		}
		if (err != nil) != exhausted || calls != wantCalls {
			t.Fatalf("exhausted=%v err=%v calls=%d want=%d", exhausted, err, calls, wantCalls)
		}
	}
}

func TestReadOnlyPostRateLimitRecovery(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	for _, path := range []string{"/api/getRestmeshUser", "/api/getUserOrderHistoryByUserid", "/m/api/getmicmeshorders"} {
		t.Run(path, func(t *testing.T) {
			c := New(&config.Config{BaseURL: "http://fixture.invalid", AuthHeaderVal: "fixture-token"}, time.Second, 0)
			c.cacheDir = t.TempDir()
			calls := 0
			c.HTTPClient.Transport = writeSafetyTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				status := http.StatusOK
				if calls == 1 {
					status = http.StatusTooManyRequests
				}
				return &http.Response{
					StatusCode: status,
					Header:     http.Header{"Retry-After": []string{"1"}},
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
					Request:    req,
				}, nil
			})
			_, _, err := c.do(http.MethodPost, path, nil, map[string]any{"userid": 42}, nil)
			if err != nil || calls != 2 {
				t.Fatalf("read-only POST err=%v calls=%d; want one bounded 429 retry", err, calls)
			}
		})
	}
	if readOnlyPostPath("/m/api/postmicmeshorder") || readOnlyPostPath("/api/getUserOrderHistoryByUserid/extra") {
		t.Fatal("read-only POST allowlist matched an unsafe or non-exact endpoint")
	}
}
