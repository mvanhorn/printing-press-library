package main

import (
	"net"
	"strings"
	"testing"
)

func TestValidateHTTPAddrLoopbackOnly(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:7777", "localhost:7777", "[::1]:7777"} {
		if err := validateHTTPAddr(addr); err != nil {
			t.Errorf("validateHTTPAddr(%q) = %v, want nil", addr, err)
		}
	}
	// ":7777" is the dangerous one: it binds every interface, which is what
	// exposed the local-data and local-write tools to any reachable peer.
	for _, addr := range []string{":7777", "0.0.0.0:7777", "192.0.2.1:7777", "[::]:7777", "garbage"} {
		if err := validateHTTPAddr(addr); err == nil {
			t.Errorf("validateHTTPAddr(%q) = nil, want rejection", addr)
		}
	}
}

// TestDefaultHTTPAddrIsLoopback guards the constant itself: the flag default is
// what almost every caller gets, so it has to be loopback on its own.
func TestDefaultHTTPAddrIsLoopback(t *testing.T) {
	if err := validateHTTPAddr(defaultHTTPAddr); err != nil {
		t.Fatalf("defaultHTTPAddr %q is not loopback: %v", defaultHTTPAddr, err)
	}
	host, _, err := net.SplitHostPort(defaultHTTPAddr)
	if err != nil {
		t.Fatalf("defaultHTTPAddr %q is not host:port: %v", defaultHTTPAddr, err)
	}
	if strings.TrimSpace(host) == "" {
		t.Fatalf("defaultHTTPAddr %q omits the host, which binds every interface", defaultHTTPAddr)
	}
}
