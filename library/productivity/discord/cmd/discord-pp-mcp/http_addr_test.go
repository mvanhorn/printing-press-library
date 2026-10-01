package main

import "testing"

func TestHTTPAddressRejectsUnauthenticatedRemoteListeners(t *testing.T) {
	for _, addr := range []string{":7777", "0.0.0.0:7777", "[::]:7777", "192.168.1.1:7777", "example.com:7777", "localhost.example.com:7777", "7777"} {
		if _, err := loopbackHTTPAddr(addr); err == nil {
			t.Errorf("accepted remote/ambiguous listener %q", addr)
		}
	}
	for _, addr := range []string{defaultHTTPAddr, "127.0.0.1:7777", "localhost:7777", "[::1]:7777"} {
		if _, err := loopbackHTTPAddr(addr); err != nil {
			t.Errorf("rejected loopback %q: %v", addr, err)
		}
	}
	if defaultHTTPAddr != "127.0.0.1:7777" {
		t.Fatalf("unsafe default listener %q", defaultHTTPAddr)
	}
}
