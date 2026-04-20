// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package deepline

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient wires an HTTP-only Client against the given test server URL,
// bypassing subprocess resolution.
func newTestClient(baseURL string) *Client {
	return &Client{
		apiKey:     "dlp_test",
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func TestHTTPWrapsPayloadInEnvelope(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"completed","result":{"data":{"ok":true}}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.executeHTTP(context.Background(), "apollo_people_match", map[string]any{
		"linkedin_url":           "https://www.linkedin.com/in/mkscrg",
		"reveal_personal_emails": true,
	})
	if err != nil {
		t.Fatalf("executeHTTP error: %v", err)
	}
	inner, ok := captured["payload"].(map[string]any)
	if !ok {
		t.Fatalf("request body missing `payload` envelope: %v", captured)
	}
	if inner["linkedin_url"] != "https://www.linkedin.com/in/mkscrg" {
		t.Errorf("inner payload missing linkedin_url: %v", inner)
	}
	if v, _ := inner["reveal_personal_emails"].(bool); !v {
		t.Errorf("inner payload missing reveal_personal_emails=true: %v", inner)
	}
}

func TestHTTP403AuthCategory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error_category":"auth","code":"AUTH_INVALID_KEY","message":"invalid api key"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.executeHTTP(context.Background(), "apollo_people_match", map[string]any{"x": "y"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrDeeplineAuth) {
		t.Errorf("err = %v, want ErrDeeplineAuth wrapped inside", err)
	}
	var pne *ErrProviderNotEntitled
	if errors.As(err, &pne) {
		t.Errorf("auth error wrongly classified as entitlement: %v", err)
	}
}

func TestHTTP403ProviderNotEnabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"provider":"contactout","error_category":"authorization","message":"Integration not enabled for this account"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.executeHTTP(context.Background(), "contactout_enrich_person", map[string]any{"x": "y"})
	if err == nil {
		t.Fatal("expected error")
	}
	var pne *ErrProviderNotEntitled
	if !errors.As(err, &pne) {
		t.Fatalf("err = %v, want *ErrProviderNotEntitled", err)
	}
	if pne.Provider != "contactout" {
		t.Errorf("pne.Provider = %q, want contactout", pne.Provider)
	}
	if pne.ToolID != "contactout_enrich_person" {
		t.Errorf("pne.ToolID = %q", pne.ToolID)
	}
}

func TestHTTP403NotConnected(t *testing.T) {
	// Live Deepline returns {"error":"Provider not connected."} on 403 when
	// the account simply never connected the integration.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Provider not connected."}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.executeHTTP(context.Background(), "contactout_enrich_person", map[string]any{"x": "y"})
	var pne *ErrProviderNotEntitled
	if !errors.As(err, &pne) {
		t.Fatalf("err = %v, want *ErrProviderNotEntitled", err)
	}
	if pne.Provider != "contactout" {
		t.Errorf("pne.Provider = %q, want contactout (derived from toolID prefix)", pne.Provider)
	}
	if !strings.Contains(pne.Error(), "not connected") {
		t.Errorf("err message should echo upstream `not connected`: %v", pne.Error())
	}
}

func TestHTTP403UnknownBodyFallsThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"something random"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.executeHTTP(context.Background(), "some_tool", map[string]any{})
	if errors.Is(err, ErrDeeplineAuth) {
		t.Errorf("unknown 403 shouldn't classify as auth: %v", err)
	}
	var pne *ErrProviderNotEntitled
	if errors.As(err, &pne) {
		t.Errorf("unknown 403 shouldn't classify as entitlement: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("fallback error should mention 403: %v", err)
	}
}

func TestHTTP401MapsToAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.executeHTTP(context.Background(), "any", map[string]any{})
	if !errors.Is(err, ErrDeeplineAuth) {
		t.Errorf("401 should be ErrDeeplineAuth: %v", err)
	}
}
