// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"net/http"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/config"
)

func TestCheckRedirectWithholdsCredentialHeadersCrossOrigin(t *testing.T) {
	c := New(&config.Config{
		BaseURL: "https://www.michi-no-eki.jp",
		Headers: map[string]string{
			"X-API-Key":    "secret-api-key-value",
			"X-Auth-Token": "secret-token-value",
			"X-Request-Id": "trace-123",
		},
	}, time.Second, 0)

	next, err := http.NewRequest(http.MethodGet, "https://evil.example/collect", nil)
	if err != nil {
		t.Fatal(err)
	}
	next.Header.Set("Authorization", "Bearer should-be-stripped")
	next.Header.Set("X-API-Key", "secret-api-key-value")
	next.Header.Set("X-Auth-Token", "secret-token-value")
	next.Header.Set("X-Request-Id", "trace-123")
	orig, err := http.NewRequest(http.MethodGet, "https://www.michi-no-eki.jp/notices", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.HTTPClient.CheckRedirect(next, []*http.Request{orig}); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"Authorization", "X-API-Key", "X-Auth-Token"} {
		if got := next.Header.Get(h); got != "" {
			t.Errorf("cross-origin hop leaked %s = %q", h, got)
		}
	}
	if got := next.Header.Get("X-Request-Id"); got != "trace-123" {
		t.Errorf("non-credential header = %q, want trace-123", got)
	}
}

func TestCheckRedirectKeepsCredentialHeadersSameOrigin(t *testing.T) {
	c := New(&config.Config{
		BaseURL: "https://www.michi-no-eki.jp",
		Headers: map[string]string{"X-API-Key": "same-host-keep"},
	}, time.Second, 0)

	next, err := http.NewRequest(http.MethodGet, "https://www.michi-no-eki.jp/notices?page=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	next.Header.Set("X-API-Key", "same-host-keep")
	next.Header.Set("Authorization", "Bearer stay")
	orig, err := http.NewRequest(http.MethodGet, "https://www.michi-no-eki.jp/notices", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.HTTPClient.CheckRedirect(next, []*http.Request{orig}); err != nil {
		t.Fatal(err)
	}
	if got := next.Header.Get("X-API-Key"); got != "same-host-keep" {
		t.Errorf("same-origin hop dropped X-API-Key; got %q", got)
	}
	if got := next.Header.Get("Authorization"); got != "Bearer stay" {
		t.Errorf("same-origin Authorization = %q, want Bearer stay", got)
	}
}
