// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"context"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/config"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type applicationTransport func(*http.Request) (*http.Response, error)

func (f applicationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestApplicationFailureAcrossHTTPMethods(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			t.Setenv("PRINTING_PRESS_VERIFY", "")
			calls := 0
			c := New(&config.Config{BaseURL: "https://example.invalid"}, time.Second, 0)
			c.NoCache = true
			c.HTTPClient.Transport = applicationTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"success":false,"error":"Malformed text","errTypes":["invalidQuillContent"]}`))}, nil
			})
			data, status, err := c.do(context.Background(), method, "/test", nil, nil, nil)
			var api *APIError
			if !errors.As(err, &api) || status != 200 || data != nil || calls != 1 || !strings.Contains(api.Body, "invalidQuillContent") {
				t.Fatalf("data=%s status=%d calls=%d err=%v", data, status, calls, err)
			}
		})
	}
}
func TestApplicationFailureEnvelopeOnly(t *testing.T) {
	for _, body := range []string{`{"success":true,"data":{"success":false}}`, `{"data":null}`, `[]`, `hello`, `{"success":true}`} {
		if got := applicationFailure([]byte(body)); got != "" {
			t.Errorf("%s: %s", body, got)
		}
	}
	if got := applicationFailure([]byte(`{"success":false,"messages":["Not allowed"]}`)); got != "Not allowed" {
		t.Fatal(got)
	}
}

func TestApplicationErrorWithoutSuccessFlag(t *testing.T) {
	for _, body := range []string{`{"error":"ApplicationError: Could not determine a source while optimizing the route."}`, `{"error":"ApplicationError: You must be an editor to export expenses"}`} {
		c := New(&config.Config{BaseURL: "https://example.invalid"}, time.Second, 0)
		c.NoCache = true
		c.HTTPClient.Transport = applicationTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		t.Setenv("PRINTING_PRESS_VERIFY", "")
		data, status, err := c.do(context.Background(), "GET", "/test", nil, nil, nil)
		if err == nil || data != nil || status != 200 {
			t.Fatalf("masked application failure: data=%s status=%d err=%v", data, status, err)
		}
	}
	for _, body := range []string{`{"error":null}`, `{"error":""}`, `{"success":true,"data":{"error":"user text"}}`} {
		if err := applicationFailure([]byte(body)); err != "" {
			t.Fatal(err)
		}
	}
}
