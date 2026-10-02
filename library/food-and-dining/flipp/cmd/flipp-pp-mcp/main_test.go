package main

import (
	"net"
	"testing"
)

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

func TestValidateBoundListenerAllowsOnlyResolvedLoopback(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1"} {
		if err := validateBoundListener(&net.TCPAddr{IP: net.ParseIP(ip), Port: 7777}); err != nil {
			t.Errorf("loopback %s rejected: %v", ip, err)
		}
	}
	for _, ip := range []string{"0.0.0.0", "192.0.2.1", "::"} {
		if err := validateBoundListener(&net.TCPAddr{IP: net.ParseIP(ip), Port: 7777}); err == nil {
			t.Errorf("non-loopback %s accepted", ip)
		}
	}
}
