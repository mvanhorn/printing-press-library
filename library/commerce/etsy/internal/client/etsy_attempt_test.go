package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

// etsy-ext: Count retries directly and preserve the failure that preceded a veto.
func TestEtsyAttemptHook(t *testing.T) {
	for _, veto := range []bool{false, true} {
		t.Run(fmt.Sprint(veto), func(t *testing.T) {
			isolateEtsyQuota(t)
			requests, attempts := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if requests == 1 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(429)
					fmt.Fprint(w, `{"error":"synthetic failure"}`)
					return
				}
				fmt.Fprint(w, `{"ok":true}`)
			}))
			defer server.Close()
			c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
			c.NoCache = true
			stop := errors.New("synthetic attempt veto")
			c.OnAttempt = func() error {
				attempts++
				if veto && attempts == 2 {
					return stop
				}
				return nil
			}
			_, err := c.Get(context.Background(), "/attempt", nil)
			if veto {
				if !errors.Is(err, stop) || !strings.Contains(err.Error(), "last attempt:") || !strings.Contains(err.Error(), "429") || requests != 1 || attempts != 2 {
					t.Fatalf("err=%v requests=%d attempts=%d", err, requests, attempts)
				}
			} else if err != nil || requests != 2 || attempts != requests {
				t.Fatalf("err=%v requests=%d attempts=%d", err, requests, attempts)
			}
		})
	}
}
