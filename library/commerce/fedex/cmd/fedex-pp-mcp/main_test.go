// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"testing"
)

func TestDefaultTransportIsStdio(t *testing.T) {
	t.Setenv("PP_MCP_TRANSPORT", "")
	if got := defaultTransport(); got != "stdio" {
		t.Fatalf("defaultTransport()=%q, want stdio", got)
	}
}

func TestHTTPTransportRequiresExplicitOptIn(t *testing.T) {
	if err := validateTransport("http", false, "127.0.0.1:7777"); err == nil {
		t.Fatal("HTTP transport without --allow-http must be rejected")
	}
	if err := validateTransport("http", true, "127.0.0.1:7777"); err != nil {
		t.Fatalf("loopback HTTP opt-in rejected: %v", err)
	}
}

func TestHTTPTransportRejectsNonLoopbackBind(t *testing.T) {
	for _, address := range []string{":7777", "0.0.0.0:7777", "192.0.2.10:7777", "example.com:7777"} {
		t.Run(address, func(t *testing.T) {
			if err := validateTransport("http", true, address); err == nil {
				t.Fatalf("non-loopback address %q must be rejected", address)
			}
		})
	}
	for _, address := range []string{"127.0.0.1:7777", "localhost:7777", "[::1]:7777"} {
		t.Run(address, func(t *testing.T) {
			if err := validateTransport("http", true, address); err != nil {
				t.Fatalf("loopback address %q rejected: %v", address, err)
			}
		})
	}
}

func TestStdioTransportNeverRequiresHTTPOptIn(t *testing.T) {
	if err := validateTransport("stdio", false, ""); err != nil {
		t.Fatalf("stdio rejected: %v", err)
	}
}
