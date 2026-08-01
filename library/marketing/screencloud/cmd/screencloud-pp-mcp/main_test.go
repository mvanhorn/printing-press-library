// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticatedLoopbackHandler(t *testing.T) {
	called := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authenticatedLoopbackHandler(next, "dedicated-secret")
	tests := []struct {
		name   string
		host   string
		origin string
		auth   string
		path   string
		want   int
	}{
		{"authorized", "127.0.0.1:7777", "http://localhost:7777", "Bearer dedicated-secret", "/mcp", http.StatusNoContent},
		{"missing bearer", "127.0.0.1:7777", "", "", "/mcp", http.StatusUnauthorized},
		{"dns rebinding host", "attacker.example:7777", "", "Bearer dedicated-secret", "/mcp", http.StatusMisdirectedRequest},
		{"browser origin", "localhost:7777", "https://attacker.example", "Bearer dedicated-secret", "/mcp", http.StatusForbidden},
		{"wrong path", "localhost:7777", "", "Bearer dedicated-secret", "/other", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://"+tc.host+tc.path, nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, tc.want, recorder.Body.String())
			}
		})
	}
	if called != 1 {
		t.Fatalf("protected MCP handler called %d times, want 1", called)
	}
}

func TestTrustedLoopbackAuthority(t *testing.T) {
	for _, authority := range []string{"localhost:7777", "127.0.0.1:7777", "[::1]:7777"} {
		if !trustedLoopbackAuthority(authority) {
			t.Errorf("rejected loopback authority %q", authority)
		}
	}
	for _, authority := range []string{"attacker.example:7777", "192.0.2.10:7777", ""} {
		if trustedLoopbackAuthority(authority) {
			t.Errorf("accepted non-loopback authority %q", authority)
		}
	}
}
