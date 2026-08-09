// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultHTTPAddressIsLoopback(t *testing.T) {
	loopback, err := isLoopbackAddress(defaultHTTPAddr)
	if err != nil {
		t.Fatalf("isLoopbackAddress(%q): %v", defaultHTTPAddr, err)
	}
	if !loopback {
		t.Fatalf("default HTTP address %q is not loopback-only", defaultHTTPAddr)
	}
}

func TestNonLoopbackHTTPRequiresAuthentication(t *testing.T) {
	if _, err := newStreamableHTTPServer(nil, ":7777", ""); err == nil {
		t.Fatal("unauthenticated all-interface bind was accepted")
	}
	if _, err := newStreamableHTTPServer(nil, ":7777", "mcp-secret"); err != nil {
		t.Fatalf("authenticated all-interface bind was rejected: %v", err)
	}
}

func TestBearerTokenMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := requireBearerToken("mcp-secret", next)

	for name, tc := range map[string]struct {
		header string
		want   int
	}{
		"missing": {},
		"wrong":   {header: "Bearer wrong", want: http.StatusUnauthorized},
		"valid":   {header: "Bearer mcp-secret", want: http.StatusNoContent},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			want := tc.want
			if want == 0 {
				want = http.StatusUnauthorized
			}
			if recorder.Code != want {
				t.Fatalf("status = %d, want %d", recorder.Code, want)
			}
		})
	}
}
