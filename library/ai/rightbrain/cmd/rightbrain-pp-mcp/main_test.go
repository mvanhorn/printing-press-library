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

func TestHTTPTransportRequiresTokenAndTLSOffLoopback(t *testing.T) {
	tests := []struct {
		addr, token, cert, key string
		wantTLS, wantError     bool
	}{
		{addr: "127.0.0.1:7777", token: "secret"},
		{addr: "localhost:7777", token: "secret"},
		{addr: "127.0.0.1:7777", wantError: true},
		{addr: "0.0.0.0:7777", token: "secret", wantError: true},
		{addr: "0.0.0.0:7777", token: "secret", cert: "cert.pem", key: "key.pem", wantTLS: true},
		{addr: "127.0.0.1:7777", token: "secret", cert: "cert.pem", wantError: true},
	}
	for _, tc := range tests {
		gotTLS, err := validateHTTPTransport(tc.addr, tc.token, tc.cert, tc.key)
		if (err != nil) != tc.wantError || gotTLS != tc.wantTLS {
			t.Errorf("validateHTTPTransport(%q) = %v, %v; want %v, error %v", tc.addr, gotTLS, err, tc.wantTLS, tc.wantError)
		}
	}
}

func TestHTTPBoundListenerRequiresTLSOffLoopback(t *testing.T) {
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
		if err := validateBoundListener(addr, true); err != nil {
			t.Errorf("TLS bind %s rejected: %v", ip, err)
		}
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
