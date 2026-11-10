// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
)

type campaignErrorTransport func(*http.Request) (*http.Response, error)

func (f campaignErrorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCampaignIDMigrationHintIsScopedToDetail404(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		code         int
		hint         bool
	}{
		{"GET", "/api/campaigns/old-message", 404, true},
		{"GET", "/api/campaigns/old-message/?fields%5Bcampaign%5D=definition", 404, true},
		{"GET", "/api/campaigns/old-message", 403, false},
		{"GET", "/api/campaigns", 404, false},
		{"GET", "/api/campaigns/id/campaign-messages", 404, false},
		{"GET", "/api/campaign-messages/id", 404, false},
		{"DELETE", "/api/campaigns/id", 404, false},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			c := New(&config.Config{BaseURL: "https://fixture.invalid", KlaviyoApiKey: "fixture-key"}, time.Second, 0)
			c.NoCache = true
			calls := 0
			c.HTTPClient.Transport = campaignErrorTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != tc.method || r.Header.Get("revision") != "2026-10-15" {
					t.Errorf("request=%s %s", r.Method, r.URL)
				}
				recorder := httptest.NewRecorder()
				recorder.Header().Set("Content-Type", "application/json")
				recorder.WriteHeader(tc.code)
				fmt.Fprint(recorder, `{"errors":[{"detail":"unavailable"}]}`)
				return recorder.Result(), nil
			})
			var err error
			if tc.method == "GET" {
				_, err = c.Get(context.Background(), tc.path, nil)
			} else {
				_, _, err = c.Delete(context.Background(), tc.path)
			}
			if err == nil || strings.Contains(err.Error(), campaignMessageIDHint) != tc.hint || calls != 1 {
				t.Fatalf("error=%v calls=%d", err, calls)
			}
		})
	}
}
