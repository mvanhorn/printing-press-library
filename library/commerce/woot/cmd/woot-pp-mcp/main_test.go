// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPBindDefaultsToLoopback(t *testing.T) {
	if defaultHTTPAddr != "127.0.0.1:7777" {
		t.Fatalf("defaultHTTPAddr = %q, want loopback", defaultHTTPAddr)
	}
	for _, addr := range []string{"127.0.0.1:7777", "localhost:7777", "[::1]:7777"} {
		if !isLoopbackAddress(addr) {
			t.Errorf("isLoopbackAddress(%q) = false, want true", addr)
		}
	}
	for _, addr := range []string{":7777", "0.0.0.0:7777", "192.0.2.10:7777", "bad-address"} {
		if isLoopbackAddress(addr) {
			t.Errorf("isLoopbackAddress(%q) = true, want false", addr)
		}
	}
}

func TestLoopbackRequestGuardAllowsLocalClients(t *testing.T) {
	for _, tc := range []struct {
		host   string
		origin string
	}{
		{host: "127.0.0.1:7777"},
		{host: "localhost:7777", origin: "http://localhost:3000"},
		{host: "[::1]:7777", origin: "https://[::1]:3000"},
	} {
		called := false
		handler := loopbackRequestGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}))
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7777/mcp", nil)
		req.Host = tc.host
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusNoContent || !called {
			t.Errorf("host=%q origin=%q: status=%d called=%v", tc.host, tc.origin, res.Code, called)
		}
	}
}

func TestLoopbackRequestGuardRejectsDNSRebinding(t *testing.T) {
	for _, tc := range []struct {
		host   string
		origin string
	}{
		{host: "attacker.example:7777"},
		{host: "127.0.0.1:7777", origin: "https://attacker.example"},
		{host: "127.0.0.1:7777", origin: "null"},
		{host: "127.0.0.1.evil.example:7777"},
	} {
		called := false
		handler := loopbackRequestGuard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called = true
		}))
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7777/mcp", nil)
		req.Host = tc.host
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusForbidden || called {
			t.Errorf("host=%q origin=%q: status=%d called=%v", tc.host, tc.origin, res.Code, called)
		}
	}
}
