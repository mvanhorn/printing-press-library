// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsLoopbackHost(t *testing.T) {
	cases := []struct {
		addr string
		lb   bool
	}{
		{"127.0.0.1:7777", true},
		{"[::1]:7777", true},
		{"localhost:7777", true},
		{":7777", false},
		{"0.0.0.0:7777", false},
		{"192.168.1.10:7777", false},
	}
	for _, tc := range cases {
		if got := isLoopbackHost(tc.addr); got != tc.lb {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", tc.addr, got, tc.lb)
		}
	}
}

// TestBearerGuard asserts the guard passes through the correct bearer token
// and rejects wrong/missing Authorization headers with 401 without invoking
// the wrapped handler.
func TestBearerGuard(t *testing.T) {
	token := "sekret-token"
	run := func(authHeader string, wantStatus int, wantInner bool) {
		t.Helper()
		innerRan := false
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			innerRan = true
			w.Write([]byte("ran"))
		})
		g := &bearerGuard{inner: inner, token: token}
		req := httptest.NewRequest("GET", mcpEndpointPath, nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, req)
		if rec.Code != wantStatus {
			t.Errorf("auth header %q: status = %d, want %d", authHeader, rec.Code, wantStatus)
		}
		if innerRan != wantInner {
			t.Errorf("auth header %q: inner ran = %v, want %v", authHeader, innerRan, wantInner)
		}
	}
	run("Bearer "+token, http.StatusOK, true)
	run("Bearer wrong-token", http.StatusUnauthorized, false)
	run("", http.StatusUnauthorized, false) // missing header
}

// TestBuildHTTPHandler drives the host-classification gate and the
// non-loopback-without-token refusal, exercising the (http.Handler, bool, error)
// shape so main is the only place that exits.
func TestBuildHTTPHandler(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Loopback binds are unguarded regardless of the env var.
	t.Setenv(mcpAuthEnv, "")
	if h, auth, err := buildHTTPHandler(inner, "127.0.0.1:7777"); err != nil {
		t.Fatalf("loopback: unexpected error: %v", err)
	} else if auth {
		t.Fatal("loopback bind should be unguarded (auth=false)")
	} else if _, ok := h.(*bearerGuard); ok {
		t.Fatal("loopback bind should not be wrapped in bearerGuard")
	}

	// Non-loopback without a token is refused.
	t.Setenv(mcpAuthEnv, "")
	if _, _, err := buildHTTPHandler(inner, "0.0.0.0:7777"); err == nil {
		t.Fatal("non-loopback bind without token should refuse to start")
	}

	// Non-loopback with a token is wrapped in a working bearerGuard and auth=true.
	t.Setenv(mcpAuthEnv, "sekret")
	h, auth, err := buildHTTPHandler(inner, "0.0.0.0:7777")
	if err != nil {
		t.Fatalf("non-loopback with token: unexpected error: %v", err)
	}
	if !auth {
		t.Fatal("non-loopback bind with token should report auth enabled")
	}
	g, ok := h.(*bearerGuard)
	if !ok {
		t.Fatalf("non-loopback bind with token should wrap in bearerGuard, got %T", h)
	}
	if g.token != "sekret" {
		t.Fatalf("guard token = %q, want sekret", g.token)
	}
}

func TestBearerGuardEndToEnd(t *testing.T) {
	// A request against a server mounted at /mcp with the correct token reaches
	// the MCP handler; a wrong one is rejected with 401.
	innerRan := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerRan = true
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(&bearerGuard{inner: inner, token: "t0ken"})
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest("GET", srv.URL+mcpEndpointPath, nil)
	req.Header.Set("Authorization", "Bearer t0ken")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("correct token request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !innerRan {
		t.Fatalf("correct token: status=%d innerRan=%v", resp.StatusCode, innerRan)
	}
}
