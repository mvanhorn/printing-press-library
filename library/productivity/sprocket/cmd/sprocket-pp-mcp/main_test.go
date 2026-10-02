package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBoundListenerMustResolveToLoopback(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1"} {
		if err := validateBoundListener(&net.TCPAddr{IP: net.ParseIP(ip), Port: 7777}); err != nil {
			t.Fatalf("loopback %s rejected: %v", ip, err)
		}
	}
	for _, ip := range []string{"0.0.0.0", "192.0.2.1", "::"} {
		if err := validateBoundListener(&net.TCPAddr{IP: net.ParseIP(ip), Port: 7777}); err == nil {
			t.Fatalf("non-loopback %s accepted", ip)
		}
	}
}

func TestValidateHTTPAddrAllowsOnlyLoopback(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:7777", "localhost:7777", "[::1]:7777"} {
		if err := validateHTTPAddr(addr); err != nil {
			t.Errorf("validateHTTPAddr(%q): %v", addr, err)
		}
	}
	for _, addr := range []string{":7777", "0.0.0.0:7777", "[::]:7777", "example.com:7777", "127.0.0.1:0"} {
		if err := validateHTTPAddr(addr); err == nil {
			t.Errorf("validateHTTPAddr(%q) = nil, want refusal", addr)
		}
	}
}

func TestRequireBearerTokenRejectsUnauthenticatedRequests(t *testing.T) {
	called := false
	handler := requireBearerToken("test-secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, authorization := range []string{"", "Bearer wrong-secret"} {
		request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("authorization %q: status = %d, want 401", authorization, response.Code)
		}
	}
	if called {
		t.Fatal("downstream MCP handler ran for an unauthenticated request")
	}
}

func TestRequireBearerTokenAllowsAuthenticatedRequest(t *testing.T) {
	called := false
	handler := requireBearerToken("test-secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	request.Header.Set("Authorization", "Bearer test-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("authenticated request: status = %d, called = %v", response.Code, called)
	}
}
