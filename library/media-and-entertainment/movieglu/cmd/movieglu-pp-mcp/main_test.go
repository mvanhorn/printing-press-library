// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultHTTPAddrIsLoopbackOnly(t *testing.T) {
	if defaultHTTPAddr != "127.0.0.1:7777" {
		t.Fatalf("defaultHTTPAddr = %q, want loopback-only listener", defaultHTTPAddr)
	}
}

func TestRequireBearerToken(t *testing.T) {
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
			if response.Code != tc.wantStatus || called != tc.wantCalled {
				t.Fatalf("status = %d, called = %v; want %d, %v", response.Code, called, tc.wantStatus, tc.wantCalled)
			}
		})
	}
}

func TestValidateHTTPTransport(t *testing.T) {
	for _, tc := range []struct {
		name    string
		addr    string
		cert    string
		key     string
		wantErr bool
	}{
		{name: "IPv4 loopback", addr: "127.0.0.1:7777"},
		{name: "IPv6 loopback", addr: "[::1]:7777"},
		{name: "localhost", addr: "localhost:7777"},
		{name: "wildcard plaintext", addr: ":7777", wantErr: true},
		{name: "all interfaces plaintext", addr: "0.0.0.0:7777", wantErr: true},
		{name: "remote plaintext", addr: "192.0.2.10:7777", wantErr: true},
		{name: "remote TLS", addr: "0.0.0.0:7777", cert: "server.crt", key: "server.key"},
		{name: "certificate without key", addr: "127.0.0.1:7777", cert: "server.crt", wantErr: true},
		{name: "key without certificate", addr: "127.0.0.1:7777", key: "server.key", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHTTPTransport(tc.addr, tc.cert, tc.key)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateHTTPTransport() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
