// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
package main

import "testing"

func TestValidateHTTPAddrAllowsLoopback(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:7777", "[::1]:7777", "localhost:7777"} {
		if err := validateHTTPAddr(addr); err != nil {
			t.Errorf("validateHTTPAddr(%q) returned error: %v", addr, err)
		}
	}
}

func TestValidateHTTPAddrRejectsExternalBinds(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:7777", "[::]:7777", ":7777", "192.0.2.10:7777", "example.test:7777", "not-an-address"} {
		if err := validateHTTPAddr(addr); err == nil {
			t.Errorf("validateHTTPAddr(%q) accepted an external or invalid address", addr)
		}
	}
}
