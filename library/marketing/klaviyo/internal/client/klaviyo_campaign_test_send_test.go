// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
)

func TestCampaignTestSendNeverReplaysIncludingInheritedIdempotency(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	for _, name := range []string{"XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	for _, code := range []int{0, 202, 307, 308, 429, 502} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			c := New(&config.Config{
				BaseURL: "https://klaviyo.test", KlaviyoApiKey: "fixture-key",
				Headers: map[string]string{"Idempotency-Key": "preview-job"},
			}, time.Second, 10000)
			posts := 0
			c.HTTPClient.Transport = formJobTestTransport(func(r *http.Request) (*http.Response, error) {
				posts++
				if r.Method != http.MethodPost || r.URL.Path != "/api/template-preview-send-jobs" || r.GetBody != nil || r.Header.Get(ReplaySafeHeader) != "" {
					t.Errorf("unexpected preview request: %s %s replayable=%t", r.Method, r.URL.Path, r.GetBody != nil)
				}
				if code == 0 {
					return nil, errors.New("connection reset after write")
				}
				w := httptest.NewRecorder()
				w.Header().Set("Retry-After", "0")
				if code == 307 || code == 308 {
					w.Header().Set("Location", "/api/template-preview-send-jobs/")
				}
				w.WriteHeader(code)
				fmt.Fprint(w, `{}`)
				return w.Result(), nil
			})
			_, _, err := c.PostCampaignTestSend(context.Background(), map[string]any{})
			if posts != 1 || (code == 202 && err != nil) || (code != 202 && err == nil) {
				t.Fatalf("posts=%d error=%v", posts, err)
			}
		})
	}
}
