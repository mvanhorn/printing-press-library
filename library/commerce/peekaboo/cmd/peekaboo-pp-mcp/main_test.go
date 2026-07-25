package main

import "testing"

func TestValidateHTTPAddrLoopbackOnly(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:7777", "localhost:7777", "[::1]:7777"} {
		if err := validateHTTPAddr(addr); err != nil {
			t.Errorf("validateHTTPAddr(%q) = %v, want nil", addr, err)
		}
	}
	for _, addr := range []string{":7777", "0.0.0.0:7777", "192.0.2.1:7777", "[::]:7777"} {
		if err := validateHTTPAddr(addr); err == nil {
			t.Errorf("validateHTTPAddr(%q) = nil, want rejection", addr)
		}
	}
}
