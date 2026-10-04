// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/config"
)

type carstayRedirectTransport func(*http.Request) (*http.Response, error)

func (f carstayRedirectTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCarstayForeignRedirectNeverReceivesConfiguredHeaders(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "standard", true: "stream-clone"}[stream], func(t *testing.T) {
			cfg := &config.Config{BaseURL: "https://carstay.example", Headers: map[string]string{"X-Configured-Key": "fixture-config", "Authorization": "Bearer fixture", "Cookie": "fixture=value"}}
			c := New(cfg, 5*time.Second, 0)
			c.NoCache = true
			foreignCalls, originCalls := 0, 0
			c.HTTPClient.Transport = carstayRedirectTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Hostname() != "carstay.example" {
					foreignCalls++
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
				}
				originCalls++
				if r.Header.Get("X-Configured-Key") != "fixture-config" || r.Header.Get("X-Request-Key") != "fixture-request" || r.Header.Get("Cookie") != "fixture=value" {
					t.Errorf("origin request missing configured/per-call fixture headers")
				}
				return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://unrelated.example/next"}}, Body: io.NopCloser(strings.NewReader("redirect")), Request: r}, nil
			})
			if stream {
				c.HTTPClient = StreamingHTTPClient(c.HTTPClient, time.Second)
			}
			data, err := c.GetWithHeaders(context.Background(), "/start", nil, map[string]string{"X-Request-Key": "fixture-request"})
			if !errors.Is(err, errCarstayForeignRedirect) || foreignCalls != 0 || originCalls != 1 || len(data) != 0 {
				t.Fatalf("err=%v foreignCalls=%d originCalls=%d returnedBytes=%d", err, foreignCalls, originCalls, len(data))
			}
		})
	}
}

func TestCarstaySameOriginRedirectPreservesHeaderAndLimitContracts(t *testing.T) {
	c := New(&config.Config{BaseURL: "https://carstay.example", Headers: map[string]string{"X-Configured-Key": "fixture-config"}}, 5*time.Second, 0)
	c.NoCache = true
	calls := 0
	c.HTTPClient.Transport = carstayRedirectTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("X-Configured-Key") != "fixture-config" || r.Header.Get("X-Request-Key") != "fixture-request" {
			t.Errorf("same-origin headers lost")
		}
		if r.URL.Path == "/start" {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"/final"}}, Body: io.NopCloser(strings.NewReader("redirect")), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Request: r}, nil
	})
	data, err := c.GetWithHeaders(context.Background(), "/start", nil, map[string]string{"X-Request-Key": "fixture-request"})
	if err != nil || calls != 2 || string(data) != `{"ok":true}` {
		t.Fatalf("calls=%d data=%s err=%v", calls, data, err)
	}
	calls = 0
	c.HTTPClient.Transport = carstayRedirectTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"/loop"}}, Body: io.NopCloser(strings.NewReader("redirect")), Request: r}, nil
	})
	_, err = c.Get(context.Background(), "/loop", nil)
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") || calls > 10 {
		t.Fatalf("redirect limit calls=%d err=%v", calls, err)
	}
}

func TestCarstayRedirectEffectiveOriginNormalization(t *testing.T) {
	for _, tc := range []struct {
		origin, target string
		same           bool
	}{
		{"https://carstay.example/a", "https://CARSTAY.EXAMPLE:443/b", true},
		{"http://127.0.0.1/a", "http://127.0.0.1:80/b", true},
		{"https://carstay.example/a", "https://carstay.example:444/b", false},
		{"https://carstay.example/a", "http://carstay.example/b", false},
		{"https://carstay.example/a", "https://unrelated.example/b", false},
	} {
		u, err := url.Parse(tc.origin)
		if err != nil {
			t.Fatal(err)
		}
		v, err := url.Parse(tc.target)
		if err != nil {
			t.Fatal(err)
		}
		err = refuseCarstayForeignRedirect(v, []*http.Request{{URL: u}})
		if (err == nil) != tc.same {
			t.Fatalf("origin=%s target=%s err=%v", tc.origin, tc.target, err)
		}
	}
}
