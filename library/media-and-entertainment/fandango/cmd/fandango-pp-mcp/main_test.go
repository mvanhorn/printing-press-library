// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPDefaultsToLoopback(t *testing.T) {
	if defaultHTTPAddr != "127.0.0.1:7777" {
		t.Fatalf("defaultHTTPAddr = %q, want loopback-only address", defaultHTTPAddr)
	}
}

func TestRequireBearerToken(t *testing.T) {
	t.Parallel()

	called := false
	handler := requireBearerToken("test-secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		name       string
		authorize  string
		wantStatus int
		wantCalled bool
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", authorize: "Basic test-secret", wantStatus: http.StatusUnauthorized},
		{name: "wrong token", authorize: "Bearer wrong", wantStatus: http.StatusUnauthorized},
		{name: "valid", authorize: "Bearer test-secret", wantStatus: http.StatusNoContent, wantCalled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.authorize != "" {
				req.Header.Set("Authorization", tc.authorize)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, req)

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tc.wantStatus)
			}
			if called != tc.wantCalled {
				t.Fatalf("downstream called = %v, want %v", called, tc.wantCalled)
			}
		})
	}
}
