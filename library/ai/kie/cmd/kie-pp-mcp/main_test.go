package main

import (
	"strings"
	"testing"
)

func TestValidateHTTPBindAddrAcceptsLoopback(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{
		"127.0.0.1:7777",
		"127.255.255.254:7777",
		"[::1]:7777",
		"localhost:7777",
		"LOCALHOST:7777",
		"localhost:0",
	} {
		addr := addr
		t.Run(addr, func(t *testing.T) {
			t.Parallel()
			if err := validateHTTPBindAddr(addr); err != nil {
				t.Fatalf("validateHTTPBindAddr(%q) returned error: %v", addr, err)
			}
		})
	}
}

func TestValidateHTTPBindAddrRejectsRemoteOrMalformedAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
	}{
		{name: "empty host", addr: ":7777"},
		{name: "wildcard ipv4", addr: "0.0.0.0:7777"},
		{name: "wildcard ipv6", addr: "[::]:7777"},
		{name: "public ipv4", addr: "8.8.8.8:7777"},
		{name: "private ipv4", addr: "192.168.1.10:7777"},
		{name: "public ipv6", addr: "[2001:4860:4860::8888]:7777"},
		{name: "hostname", addr: "example.com:7777"},
		{name: "missing port", addr: "127.0.0.1"},
		{name: "missing host and port", addr: ""},
		{name: "malformed ipv6", addr: "::1:7777"},
		{name: "non numeric port", addr: "127.0.0.1:http"},
		{name: "negative port", addr: "127.0.0.1:-1"},
		{name: "oversized port", addr: "127.0.0.1:65536"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateHTTPBindAddr(tt.addr)
			if err == nil {
				t.Fatalf("validateHTTPBindAddr(%q) returned nil error", tt.addr)
			}
			msg := err.Error()
			for _, want := range []string{
				"remote MCP exposure is refused",
				"no client authentication",
				"authenticated TLS reverse proxy",
				"loopback",
			} {
				if !strings.Contains(msg, want) {
					t.Fatalf("error %q does not contain %q", msg, want)
				}
			}
		})
	}
}
