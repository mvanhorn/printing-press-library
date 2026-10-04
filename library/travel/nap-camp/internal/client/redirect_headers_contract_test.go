// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/config"
)

type redirectContractTransport func(*http.Request) (*http.Response, error)

func (f redirectContractTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestForeignRedirectDropsAllOriginConfiguredHeaders(t *testing.T) {
	c := New(&config.Config{BaseURL: "https://origin.example", Headers: map[string]string{"X-Fixture-Private": "fixture-value", "Authorization": "fixture-auth", "Cookie": "fixture-cookie"}}, time.Second, 0)
	c.NoCache = true
	var seen []struct {
		host, path string
		header     http.Header
	}
	c.HTTPClient.Transport = redirectContractTransport(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, struct {
			host, path string
			header     http.Header
		}{r.URL.Host, r.URL.Path, r.Header.Clone()})
		status := http.StatusFound
		location := ""
		switch r.URL.Host + r.URL.Path {
		case "origin.example/start":
			location = "https://origin.example/same"
		case "origin.example/same":
			location = "https://foreign.example/first"
		case "foreign.example/first":
			location = "https://foreign.example/final"
		default:
			status = http.StatusOK
		}
		h := http.Header{"Content-Type": []string{"application/json"}}
		if location != "" {
			h.Set("Location", location)
		}
		return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(`[]`)), Request: r}, nil
	})
	if _, err := c.Get(context.Background(), "/start", nil); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 4 {
		t.Fatal(seen)
	}
	for _, r := range seen {
		for _, key := range []string{"X-Fixture-Private", "Authorization", "Cookie", "X-Requested-With", "X-VERSION-2"} {
			if r.host == "origin.example" && r.header.Get(key) == "" {
				t.Fatalf("same origin lost %s", key)
			}
			if r.host == "foreign.example" && r.header.Get(key) != "" {
				t.Fatalf("foreign hop retained origin %s at %s", key, r.path)
			}
		}
	}
}
