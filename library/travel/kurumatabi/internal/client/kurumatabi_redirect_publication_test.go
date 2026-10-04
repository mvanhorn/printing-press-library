// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/config"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type kurumatabiRedirectTransport func(*http.Request) (*http.Response, error)

func (f kurumatabiRedirectTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestForeignSourceRedirectNeverSendsConfiguredOrPerCallHeaders(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(map[bool]string{false: "standard", true: "streaming"}[streaming], func(t *testing.T) {
			c := New(&config.Config{BaseURL: "https://source.example", AuthHeaderVal: "synthetic-auth", Headers: map[string]string{"X-Configured-Key": "synthetic-configured"}}, time.Second, 0)
			c.NoCache = true
			origin, foreign := 0, 0
			c.HTTPClient.Transport = kurumatabiRedirectTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "source.example" {
					origin++
					if req.Header.Get("Authorization") != "synthetic-auth" || req.Header.Get("X-Configured-Key") != "synthetic-configured" || req.Header.Get("X-Endpoint-Key") != "synthetic-endpoint" {
						t.Fatal("test did not exercise both credential header sources")
					}
					return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://foreign.example/park"}}, Body: io.NopCloser(strings.NewReader("redirect")), Request: req}, nil
				}
				foreign++
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: req}, nil
			})
			if streaming {
				c.HTTPClient = StreamingHTTPClient(c.HTTPClient, time.Second)
				if c.HTTPClient.Timeout != 0 {
					t.Fatal("stream timeout changed")
				}
			}
			_, err := c.GetWithHeaders(context.Background(), "/park", nil, map[string]string{"X-Endpoint-Key": "synthetic-endpoint"})
			if !errors.Is(err, ErrRedirectForeignOrigin) || origin != 1 || foreign != 0 {
				t.Fatalf("foreign request sent: origin=%d foreign=%d err=%v", origin, foreign, err)
			}
		})
	}
}

func TestSourceRedirectSameEffectiveOriginAndExistingGuards(t *testing.T) {
	via := func(raw string) []*http.Request { u, _ := url.Parse(raw); return []*http.Request{{URL: u}} }
	for _, tc := range []struct {
		from, to string
		want     error
	}{
		{"https://source.example/park", "https://SOURCE.example:443/canonical", nil},
		{"http://127.0.0.1:80/a", "http://127.0.0.1/b", nil},
		{"https://source.example/a", "https://source.example:444/b", ErrRedirectForeignOrigin},
		{"https://source.example/a", "http://source.example/b", ErrRedirectProtocolDowngrade},
		{"https://source.example/a", "ftp://source.example/b", ErrRedirectUnsupportedScheme},
		{"https://source.example/a", "https://127.0.0.1/b", ErrRedirectPrivateDestination},
	} {
		u, _ := url.Parse(tc.to)
		err := redirectDestinationRefused(u, via(tc.from))
		if !errors.Is(err, tc.want) {
			t.Fatalf("%s -> %s err=%v want=%v", tc.from, tc.to, err, tc.want)
		}
	}
	c := New(&config.Config{BaseURL: "https://source.example", AuthHeaderVal: "synthetic-auth"}, time.Second, 0)
	c.NoCache = true
	calls := 0
	c.HTTPClient.Transport = kurumatabiRedirectTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Path == "/a" {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"/b"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
		if req.Header.Get("Authorization") != "synthetic-auth" || req.Header.Get("X-Endpoint-Key") != "synthetic-endpoint" {
			t.Fatal("same-origin credentials lost")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Request: req}, nil
	})
	data, err := c.GetWithHeaders(context.Background(), "/a", nil, map[string]string{"X-Endpoint-Key": "synthetic-endpoint"})
	if err != nil || calls != 2 || string(data) != `{"ok":true}` {
		t.Fatalf("same-origin err=%v calls=%d data=%s", err, calls, data)
	}
	req, _ := http.NewRequest("GET", "https://source.example/c", nil)
	hops := make([]*http.Request, 10)
	for i := range hops {
		hops[i] = req
	}
	if err := c.HTTPClient.CheckRedirect(req, hops); err == nil || !strings.Contains(err.Error(), "10 redirects") {
		t.Fatal("redirect cap lost")
	}
}
