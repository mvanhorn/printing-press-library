package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateHTTPTransport(t *testing.T) {
	tests := []struct {
		name                   string
		addr, token, cert, key string
		wantTLS, wantErr       bool
	}{
		{"loopback authenticated plaintext", "127.0.0.1:7777", "secret", "", "", false, false},
		{"missing token", "127.0.0.1:7777", "", "", "", false, true},
		{"wildcard plaintext", ":7777", "secret", "", "", false, true},
		{"remote plaintext", "0.0.0.0:7777", "secret", "", "", false, true},
		{"remote TLS", "0.0.0.0:7777", "secret", "cert.pem", "key.pem", true, false},
		{"localhost", "localhost:7777", "secret", "", "", false, false},
		{"partial TLS", "127.0.0.1:7777", "secret", "cert.pem", "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTLS, err := validateHTTPTransport(tt.addr, tt.token, tt.cert, tt.key)
			if (err != nil) != tt.wantErr || gotTLS != tt.wantTLS {
				t.Fatalf("validateHTTPTransport() = (%v, %v), want TLS=%v err=%v", gotTLS, err, tt.wantTLS, tt.wantErr)
			}
		})
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
	h := requireHTTPToken("secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	unauthorized := httptest.NewRecorder()
	h.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if unauthorized.Code != http.StatusUnauthorized || called {
		t.Fatalf("unauthorized request: status=%d called=%v", unauthorized.Code, called)
	}

	authorized := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(authorized, req)
	if authorized.Code != http.StatusNoContent || !called {
		t.Fatalf("authorized request: status=%d called=%v", authorized.Code, called)
	}
}

func TestDefaultHTTPAddrIsLoopback(t *testing.T) {
	if !isLoopbackAddr(defaultHTTPAddr) {
		t.Fatalf("defaultHTTPAddr %q must bind loopback", defaultHTTPAddr)
	}
}
