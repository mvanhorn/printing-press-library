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

type formJobTestTransport func(*http.Request) (*http.Response, error)

func (f formJobTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFormPublicationJobLegacyClientPostsOnce(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	for _, name := range []string{"XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	for _, path := range []string{"/api/form-publish-jobs", "/api/form-unpublish-jobs"} {
		for _, code := range []int{0, 202, 307, 308, 429, 502} {
			t.Run(fmt.Sprintf("%s/%d", path, code), func(t *testing.T) {
				c := New(&config.Config{
					BaseURL: "https://klaviyo.test", KlaviyoApiKey: "fixture-key",
					Headers: map[string]string{"Idempotency-Key": "fixture-key"},
				}, time.Second, 10000)
				posts, gets := 0, 0
				c.HTTPClient.Transport = formJobTestTransport(func(r *http.Request) (*http.Response, error) {
					response := httptest.NewRecorder()
					if r.Method == http.MethodGet {
						gets++
						fmt.Fprint(response, `{"data":{"attributes":{"status":"live"}}}`)
						return response.Result(), nil
					}
					posts++
					if r.URL.Path != path || r.Header.Get(ReplaySafeHeader) != "" || r.GetBody != nil {
						t.Errorf("path=%s internal header=%q", r.URL.Path, r.Header.Get(ReplaySafeHeader))
					}
					if code == 0 {
						return nil, errors.New("connection lost after write")
					}
					response.Header().Set("Retry-After", "0")
					if code == 307 || code == 308 {
						response.Header().Set("Location", path+"/")
					}
					response.WriteHeader(code)
					fmt.Fprint(response, `{}`)
					return response.Result(), nil
				})
				_, _, err := c.PostFormPublicationJob(context.Background(), path, map[string]any{})
				if posts != 1 || (code == 202 && err != nil) || ((code == 0 || code >= 400) && err == nil) {
					t.Fatalf("posts=%d error=%v", posts, err)
				}
				if _, err := c.GetNoCache(context.Background(), "/api/forms/form-1", nil); err != nil || gets != 1 {
					t.Fatalf("verification read gets=%d error=%v", gets, err)
				}
			})
		}
	}
}
