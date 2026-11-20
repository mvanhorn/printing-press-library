package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/platform"
)

func TestEtsyDailyQuota429(t *testing.T) {
	for _, daily := range []bool{true, false} {
		name := "ordinary"
		if daily {
			name = "daily"
		}
		t.Run(name, func(t *testing.T) {
			isolateEtsyQuota(t)
			var calls atomic.Int32
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if daily || n == 1 {
					w.Header().Set("Retry-After", "0")
					w.Header().Set("X-Request-ID", "synthetic-request")
					if daily {
						w.Header().Set("x-remaining-today", "0")
					}
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer srv.Close()
			c, _ := platformRateLimitTestClient(t, srv)
			ctx, cancel := context.WithTimeout(context.Background(), 1900*time.Millisecond)
			defer cancel()
			start := time.Now()
			_, err := c.Get(ctx, "/quota", nil)
			if daily {
				var limited *platform.RateLimitedError
				var apiErr *APIError
				if !IsDailyQuotaExhausted(err) || !errors.As(err, &limited) || !errors.As(err, &apiErr) {
					t.Fatalf("missing daily quota error chain: %v", err)
				}
				if limited.Attempts != 1 || limited.RequestID != "synthetic-request" || limited.RetryAfter != 0 {
					t.Fatalf("unexpected rate error: %+v", limited)
				}
				if calls.Load() != 1 || time.Since(start) >= 2*time.Second {
					t.Fatalf("daily exhaustion: calls=%d elapsed=%v", calls.Load(), time.Since(start))
				}
			} else if err != nil || calls.Load() != 2 || IsDailyQuotaExhausted(err) {
				t.Fatalf("ordinary retry: calls=%d error=%v", calls.Load(), err)
			}
		})
	}
	if IsDailyQuotaExhausted(errors.New("ordinary")) || IsDailyQuotaExhausted(nil) {
		t.Fatal("unrelated error classified as daily exhaustion")
	}
}
