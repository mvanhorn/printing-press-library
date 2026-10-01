package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestLoopbackHTTPAddr(t *testing.T) {
	for addr, want := range map[string]string{
		"127.0.0.1:7777": "127.0.0.1:7777",
		"localhost:7777": "127.0.0.1:7777",
		"LOCALHOST:7777": "127.0.0.1:7777",
		"[::1]:7777":     "[::1]:7777",
	} {
		got, err := loopbackHTTPAddr(addr)
		if err != nil || got != want {
			t.Errorf("loopbackHTTPAddr(%q) = %q, %v; want %q", addr, got, err, want)
		}
	}
	for _, addr := range []string{":7777", "0.0.0.0:7777", "[::]:7777", "remote.invalid:7777", "127.0.0.1:0"} {
		if _, err := loopbackHTTPAddr(addr); err == nil {
			t.Errorf("loopbackHTTPAddr(%q) = nil, want refusal", addr)
		}
	}
}

func TestRequireBearerTokenRejectsUnauthenticatedRequests(t *testing.T) {
	called := false
	handler := requireBearerToken("synthetic-test-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, authorization := range []string{"", "wrong-scheme synthetic-test-token", "Bearer wrong-token", "Bearer synthetic-test-token extra"} {
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
	handler := requireBearerToken("synthetic-test-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	request.Header.Set("Authorization", "bearer  synthetic-test-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("authenticated request: status = %d, called = %v", response.Code, called)
	}
}

func TestConfiguredHTTPRouteRequiresBearerToken(t *testing.T) {
	mcpServer := server.NewMCPServer("Axs test", "test")
	httpServer, _ := newAuthenticatedHTTPServer(mcpServer, "synthetic-test-token")
	listener := httptest.NewServer(httpServer.Handler)
	defer listener.Close()

	for _, tc := range []struct {
		name, authorization string
		wantUnauthorized    bool
	}{
		{name: "no token", wantUnauthorized: true},
		{name: "wrong token", authorization: "Bearer wrong-token", wantUnauthorized: true},
		{name: "correct token", authorization: "bearer synthetic-test-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, listener.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			if tc.authorization != "" {
				request.Header.Set("Authorization", tc.authorization)
			}
			response, err := listener.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if tc.wantUnauthorized && response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", response.StatusCode)
			}
			if !tc.wantUnauthorized && response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.StatusCode)
			}
		})
	}
}
