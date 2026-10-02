// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPTransportRequiresEnvironmentTokenAndTLSOffLoopback(t *testing.T) {
	if defaultHTTPAddr != "127.0.0.1:7777" {
		t.Fatalf("default HTTP bind = %q, want loopback", defaultHTTPAddr)
	}
	for _, tc := range []struct {
		name      string
		addr      string
		token     string
		cert, key string
		wantTLS   bool
		wantErr   bool
	}{
		{"loopback token", "127.0.0.1:7777", "test-token", "", "", false, false},
		{"localhost token", "localhost:7777", "test-token", "", "", false, false},
		{"missing token", "127.0.0.1:7777", "", "", "", false, true},
		{"wildcard plaintext", ":7777", "test-token", "", "", false, true},
		{"public plaintext", "0.0.0.0:7777", "test-token", "", "", false, true},
		{"public TLS", "0.0.0.0:7777", "test-token", "cert.pem", "key.pem", true, false},
		{"partial TLS", "127.0.0.1:7777", "test-token", "cert.pem", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotTLS, err := validateHTTPTransport(tc.addr, tc.token, tc.cert, tc.key)
			if (err != nil) != tc.wantErr || gotTLS != tc.wantTLS {
				t.Fatalf("TLS=%v err=%v, want TLS=%v error=%v", gotTLS, err, tc.wantTLS, tc.wantErr)
			}
		})
	}
}

func TestBoundListenerRejectsNonLoopbackPlaintext(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1"} {
		if err := validateBoundListener(&net.TCPAddr{IP: net.ParseIP(ip), Port: 7777}, false); err != nil {
			t.Errorf("loopback %s rejected: %v", ip, err)
		}
	}
	for _, ip := range []string{"0.0.0.0", "192.0.2.1", "::"} {
		addr := &net.TCPAddr{IP: net.ParseIP(ip), Port: 7777}
		if err := validateBoundListener(addr, false); err == nil {
			t.Errorf("non-loopback %s accepted without TLS", ip)
		}
	}
}

func TestHTTPBearerGuardRejectsUnauthenticatedRequests(t *testing.T) {
	called := false
	guard := authenticatedMCPHandler("test-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, auth := range []string{"", "Bearer wrong", "Basic test-token"} {
		called = false
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7777/mcp", nil)
		req.Header.Set("Authorization", auth)
		res := httptest.NewRecorder()
		guard.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized || called {
			t.Fatalf("auth=%q status=%d called=%v", auth, res.Code, called)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7777/mcp", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	res := httptest.NewRecorder()
	guard.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent || !called {
		t.Fatalf("valid auth status=%d called=%v", res.Code, called)
	}
	called = false
	req = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7777/other", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	res = httptest.NewRecorder()
	guard.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound || called {
		t.Fatalf("unrelated path status=%d called=%v, want 404 without MCP call", res.Code, called)
	}
}
