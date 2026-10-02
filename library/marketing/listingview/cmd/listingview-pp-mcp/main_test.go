package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultHTTPAddrIsLoopback(t *testing.T) {
	if !isLoopbackAddr(defaultHTTPAddr) {
		t.Fatalf("default HTTP address %q must be loopback", defaultHTTPAddr)
	}
	for _, addr := range []string{"127.0.0.1:7777", "[::1]:7777"} {
		if !isLoopbackAddr(addr) {
			t.Fatalf("expected %q to be loopback", addr)
		}
	}
	for _, addr := range []string{":7777", "0.0.0.0:7777", "localhost:7777"} {
		if isLoopbackAddr(addr) {
			t.Fatalf("expected %q to be treated as non-loopback", addr)
		}
	}
}

func TestValidateBoundListener(t *testing.T) {
	if err := validateBoundListener(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777}, false); err != nil {
		t.Fatal(err)
	}
	if err := validateBoundListener(&net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 7777}, false); err == nil {
		t.Fatal("plaintext listener must refuse a public bind")
	}
}

func TestRequireHTTPToken(t *testing.T) {
	called := false
	handler := requireHTTPToken("secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, auth := range []string{"", "Bearer wrong", "Basic secret"} {
		called = false
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", auth)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized || called {
			t.Fatalf("auth %q: status=%d called=%v", auth, resp.Code, called)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent || !called {
		t.Fatalf("valid token: status=%d called=%v", resp.Code, called)
	}
}
