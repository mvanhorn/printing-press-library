// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/chromecookies"
)

const fixtureSessionID = "sess_FIXTURETestSessionID"

// seedClient constructs a Client with WithCookieAuth over a jar
// pre-populated with clerk_active_context and an expired __session JWT.
func seedClient(t *testing.T, httpClient *http.Client) *Client {
	t.Helper()

	c := &Client{BaseURL: "https://happenstance.ai", HTTPClient: httpClient}

	expiredJWT := "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjF9.sig"
	cookies := []chromecookies.Cookie{
		{Name: "clerk_active_context", Value: fixtureSessionID, Domain: ".happenstance.ai", Path: "/"},
		{Name: "__session", Value: expiredJWT, Domain: ".happenstance.ai", Path: "/", HttpOnly: true, Secure: true},
	}
	c.ApplyOptions(WithCookieAuth(cookies))
	c.HTTPClient.Jar = c.cookieAuth.jar
	return c
}

func TestRefreshClerkSession_HitsTouchEndpointNotTokens(t *testing.T) {
	var calls int32
	var capturedPath string
	var capturedAPIVersion string
	var capturedJSVersion string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		capturedPath = r.URL.Path
		capturedAPIVersion = r.URL.Query().Get("__clerk_api_version")
		capturedJSVersion = r.URL.Query().Get("_clerk_js_version")

		freshJWT := "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjI1MjQ2MDgwMDB9.sig" // exp=2050
		http.SetCookie(w, &http.Cookie{
			Name:     "__session",
			Value:    freshJWT,
			Path:     "/",
			Domain:   "127.0.0.1",
			HttpOnly: true,
		})
		_, _ = w.Write([]byte(`{"response":{"object":"session","status":"active"},"client":{}}`))
	}))
	defer srv.Close()

	oldBase := clerkBaseURL
	clerkBaseURL = srv.URL
	defer func() { clerkBaseURL = oldBase }()

	c := seedClient(t, srv.Client())

	if err := c.refreshClerkSession(); err != nil {
		t.Fatalf("refreshClerkSession: %v", err)
	}

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("expected 1 HTTP call, got %d", atomic.LoadInt32(&calls))
	}
	wantPath := fmt.Sprintf("/v1/client/sessions/%s/touch", fixtureSessionID)
	if capturedPath != wantPath {
		t.Errorf("wrong endpoint path: got %q want %q (regression: contact-goat used to call /tokens which is the wrong surface)", capturedPath, wantPath)
	}
	if capturedAPIVersion == "" {
		t.Error("missing __clerk_api_version query param")
	}
	if capturedJSVersion == "" {
		t.Error("missing _clerk_js_version query param — Clerk denies refresh without it")
	}
}

func TestRefreshClerkSession_SurfaceClerkHeadersOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Clerk-Auth-Status", "signed-out")
		w.Header().Set("X-Clerk-Auth-Reason", "session-token-expired-refresh-non-eligible-non-get")
		w.Header().Set("X-Clerk-Auth-Message", "JWT is expired. Expiry date: ...")
		w.WriteHeader(401)
	}))
	defer srv.Close()

	oldBase := clerkBaseURL
	clerkBaseURL = srv.URL
	defer func() { clerkBaseURL = oldBase }()

	c := seedClient(t, srv.Client())

	err := c.refreshClerkSession()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "HTTP 401") {
		t.Errorf("error missing HTTP 401: %v", err)
	}
	if !strings.Contains(msg, "signed-out") {
		t.Errorf("error missing Clerk status header: %v", err)
	}
	if !strings.Contains(msg, "JWT is expired") {
		t.Errorf("error missing Clerk message: %v", err)
	}
}

func TestRefreshClerkSession_MissingSessionID(t *testing.T) {
	c := &Client{BaseURL: "https://happenstance.ai", HTTPClient: &http.Client{}}
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(&url.URL{Scheme: "https", Host: "happenstance.ai"}, []*http.Cookie{
		{Name: "__session", Value: "x"},
	})
	c.HTTPClient.Jar = jar
	c.cookieAuth = &cookieAuthState{jar: jar}

	err := c.refreshClerkSession()
	if err == nil {
		t.Fatal("expected error when clerk_active_context is missing")
	}
	if !strings.Contains(err.Error(), "clerk_active_context") {
		t.Errorf("error does not name the missing cookie: %v", err)
	}
}

func TestRefreshClerkSession_CollapsesConcurrentCalls(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.SetCookie(w, &http.Cookie{
			Name:     "__session",
			Value:    "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjI1MjQ2MDgwMDB9.sig",
			Path:     "/",
			Domain:   "127.0.0.1",
			HttpOnly: true,
		})
		_, _ = w.Write([]byte(`{"response":{},"client":{}}`))
	}))
	defer srv.Close()

	oldBase := clerkBaseURL
	clerkBaseURL = srv.URL
	defer func() { clerkBaseURL = oldBase }()

	c := seedClient(t, srv.Client())

	if err := c.refreshClerkSession(); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if err := c.refreshClerkSession(); err != nil {
		t.Fatalf("second refresh (should collapse): %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 HTTP call (collapse window), got %d", got)
	}

	// Advance past the collapse window and expect a genuine refresh.
	c.cookieAuth.lastRefresh = time.Now().Add(-5 * time.Second)
	if err := c.refreshClerkSession(); err != nil {
		t.Fatalf("third refresh after window: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 calls after window expired, got %d", got)
	}
}
