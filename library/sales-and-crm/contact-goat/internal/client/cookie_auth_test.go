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

// TestRefreshClerkSession_ReadsJWTFromResponseBody guards against the
// regression we hit on 2026-04-20: Clerk's /touch returns the fresh
// JWT in response.last_active_token.jwt, NOT via Set-Cookie: __session.
// Only __client_uat comes back as a Set-Cookie. If the refresher only
// looks at Set-Cookie it keeps the expired __session in the jar and
// every subsequent Happenstance request gets a 204 signed-out.
func TestRefreshClerkSession_ReadsJWTFromResponseBody(t *testing.T) {
	freshJWT := "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjI1MjQ2MDgwMDB9.sig" // exp=2050
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mirror the real Clerk shape: only __client_uat via Set-Cookie.
		http.SetCookie(w, &http.Cookie{Name: "__client_uat", Value: "1776619443", Path: "/"})
		fmt.Fprintf(w, `{"response":{"object":"session","status":"active","last_active_token":{"object":"token","jwt":"%s"}},"client":{}}`, freshJWT)
	}))
	defer srv.Close()

	oldBase := clerkBaseURL
	clerkBaseURL = srv.URL
	defer func() { clerkBaseURL = oldBase }()

	c := seedClient(t, srv.Client())

	if err := c.refreshClerkSession(); err != nil {
		t.Fatalf("refreshClerkSession: %v", err)
	}

	// The fresh JWT must be written into the jar under .happenstance.ai
	// so subsequent POSTs to /api/search carry it.
	got := c.sessionCookieValue()
	if got != freshJWT {
		t.Errorf("jar's __session not updated from response body:\n  got  %q\n  want %q", got, freshJWT)
	}
}

// TestRefreshClerkSession_ErrorsWhenBodyJWTExpired guards the guard:
// a /touch response that (somehow) returns an already-expired JWT must
// surface a clear error rather than silently keeping it, since every
// subsequent request would fail with a cryptic 204.
func TestRefreshClerkSession_ErrorsWhenBodyJWTExpired(t *testing.T) {
	expiredJWT := "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjF9.sig" // exp=1970
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"response":{"last_active_token":{"jwt":"%s"}},"client":{}}`, expiredJWT)
	}))
	defer srv.Close()

	oldBase := clerkBaseURL
	clerkBaseURL = srv.URL
	defer func() { clerkBaseURL = oldBase }()

	c := seedClient(t, srv.Client())

	err := c.refreshClerkSession()
	if err == nil {
		t.Fatal("expected error when refresh returns an expired JWT, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error does not mention expiry: %v", err)
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
