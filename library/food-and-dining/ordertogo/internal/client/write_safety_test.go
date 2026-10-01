package client

import (
	"errors"
	"io"
	"net/http"
	"strings"
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
