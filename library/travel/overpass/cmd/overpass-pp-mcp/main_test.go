// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPTransportRequiresTokenAndTLSOffLoopback(t *testing.T) {
	if defaultHTTPAddr != "127.0.0.1:7777" {
		t.Fatalf("defaultHTTPAddr = %q, want loopback", defaultHTTPAddr)
	}

	tests := []struct {
		name      string
		addr      string
		token     string
		cert      string
		key       string
		wantTLS   bool
		wantError bool
	}{
		{name: "loopback token", addr: "127.0.0.1:7777", token: "secret"},
		{name: "localhost token", addr: "localhost:7777", token: "secret"},
		{name: "loopback missing token", addr: "127.0.0.1:7777", wantError: true},
		{name: "wildcard plaintext", addr: ":7777", token: "secret", wantError: true},
		{name: "public plaintext", addr: "0.0.0.0:7777", token: "secret", wantError: true},
		{name: "public tls", addr: "0.0.0.0:7777", token: "secret", cert: "cert.pem", key: "key.pem", wantTLS: true},
		{name: "partial tls", addr: "127.0.0.1:7777", token: "secret", cert: "cert.pem", wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotTLS, err := validateHTTPTransport(tc.addr, tc.token, tc.cert, tc.key)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateHTTPTransport() error = %v, wantError %v", err, tc.wantError)
			}
			if gotTLS != tc.wantTLS {
				t.Errorf("validateHTTPTransport() TLS = %v, want %v", gotTLS, tc.wantTLS)
			}
		})
	}
}

func TestHTTPTokenGuard(t *testing.T) {
	called := false
	handler := requireHTTPToken("secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, auth := range []string{"", "Bearer wrong", "Basic c2VjcmV0"} {
		called = false
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7777/mcp", nil)
		req.Header.Set("Authorization", auth)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized || called {
			t.Errorf("auth %q: status=%d called=%v, want 401/false", auth, res.Code, called)
		}
	}

	called = false
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7777/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent || !called {
		t.Fatalf("valid bearer token: status=%d called=%v, want 204/true", res.Code, called)
	}
}

func TestLoopbackAddressValidation(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:7777", "[::1]:7777"} {
		if !isLoopbackAddr(addr) {
			t.Errorf("isLoopbackAddr(%q) = false, want true", addr)
		}
	}
	for _, addr := range []string{"localhost:7777", "localhost.evil.com:7777", ":7777", "0.0.0.0:7777", "bad"} {
		if isLoopbackAddr(addr) {
			t.Errorf("isLoopbackAddr(%q) = true, want false", addr)
		}
	}
}

func TestHTTPBoundListenerRequiresLoopbackWithoutTLS(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1"} {
		addr := &net.TCPAddr{IP: net.ParseIP(ip), Port: 7777}
		if err := validateBoundListener(addr, false); err != nil {
			t.Errorf("loopback %s rejected: %v", ip, err)
		}
	}
	for _, ip := range []string{"0.0.0.0", "192.0.2.1", "::"} {
		addr := &net.TCPAddr{IP: net.ParseIP(ip), Port: 7777}
		if err := validateBoundListener(addr, false); err == nil {
			t.Errorf("non-loopback %s accepted without TLS", ip)
		}
		if err := validateBoundListener(addr, true); err != nil {
			t.Errorf("TLS bind %s rejected: %v", ip, err)
		}
	}
}
