// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestSafeEndpointClassIgnoresQueryAndFragment(t *testing.T) {
	for _, endpoint := range []string{"image-upload", "campaign-message-assign-template"} {
		path := "/api/" + endpoint
		want := "POST " + path
		for _, suffix := range []string{"", "?fields%5Bimage%5D=id", "?fields[campaign-message]=name", "#fragment", "?fields[image]=id#fragment", "#fragment?ignored=true"} {
			t.Run(endpoint+suffix, func(t *testing.T) {
				if got := safeEndpointClass("post", path+suffix); got != want {
					t.Fatalf("safeEndpointClass(%q) = %q, want %q", path+suffix, got, want)
				}
			})
		}
	}
	if got := safeEndpointClass("get", "/api/profiles/profile-1/"); got != "GET /api/profiles" {
		t.Fatalf("plain resource class = %q, want GET /api/profiles", got)
	}
}

func TestEndpointClassSharesBudgetAndPreservesRequestQuery(t *testing.T) {
	paths := []string{
		"/api/campaign-message-assign-template?fields%5Bcampaign-message%5D=id",
		"/api/campaign-message-assign-template?fields[campaign-message]=name",
	}
	const body = `{"data":{"type":"template","id":"template-1"}}`
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(calls.Add(1)) - 1
		if i >= len(paths) {
			t.Error("unexpected retry")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodPost || r.RequestURI != paths[i] {
			t.Errorf("request = %s %s, want POST %s", r.Method, r.RequestURI, paths[i])
		}
		for header, want := range map[string]string{
			"Authorization": "Bearer synthetic", "revision": "2026-04-15", "Content-Type": "application/json",
		} {
			if got := r.Header.Get(header); got != want {
				t.Errorf("%s = %q, want %q", header, got, want)
			}
		}
		if got, err := io.ReadAll(r.Body); err != nil || string(got) != body {
			t.Errorf("body = %s, error = %v, want %s", got, err, body)
		}
		if i == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer server.Close()
	c, _ := platformRateLimitTestClient(t, server)
	const class = "POST /api/campaign-message-assign-template"
	c.platformBudgets = map[string]platform.EndpointBudget{}
	if err := c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: class, Steady: 2, Interval: time.Second, Burst: 2, MaxAttempts: 1, RetryBudget: time.Second}); err != nil {
		t.Fatal(err)
	}
	for i, path := range paths {
		_, status, err := c.doBytes(http.MethodPost, path, nil, []byte(body), nil)
		if i == 0 {
			if err != nil || status != http.StatusOK {
				t.Fatalf("first request = %d, %v", status, err)
			}
		} else {
			var limited *platform.RateLimitedError
			want := "rate limited: endpoint class " + class + " exhausted after 1 attempts"
			if status != http.StatusTooManyRequests || !errors.As(err, &limited) || limited.EndpointClass != class || err.Error() != want {
				t.Fatalf("second request = %d, %v, want 429 and %q", status, err, want)
			}
		}
	}
	if calls.Load() != 2 || len(c.platformLimiters) != 1 || c.platformLimiters[class] == nil {
		t.Fatalf("requests = %d, limiters = %v, want two requests sharing %q", calls.Load(), c.platformLimiters, class)
	}
}

func TestEndpointClassPreservesDryRunQuery(t *testing.T) {
	c := New(&config.Config{BaseURL: "https://example.invalid"}, time.Second, 0)
	c.DryRun = true
	c.HTTPClient = nil
	stderr, err := os.CreateTemp(t.TempDir(), "preview")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	previousStderr := os.Stderr
	os.Stderr = stderr
	defer func() { os.Stderr = previousStderr }()
	const path = "/api/image-upload?fields%5Bimage%5D=id#fragment"
	data, status, err := c.doBytes(http.MethodPost, path, nil, nil, nil)
	if err != nil || status != 0 || string(data) != `{"dry_run": true}` {
		t.Fatalf("dry run = %s, %d, %v", data, status, err)
	}
	preview, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if want := "POST https://example.invalid" + path + "\n"; !strings.HasPrefix(string(preview), want) {
		t.Fatalf("preview = %q, want prefix %q", preview, want)
	}
}
