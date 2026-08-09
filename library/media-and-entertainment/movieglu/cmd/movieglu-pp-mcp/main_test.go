// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import "testing"

func TestDefaultHTTPAddrIsLoopbackOnly(t *testing.T) {
	if defaultHTTPAddr != "127.0.0.1:7777" {
		t.Fatalf("defaultHTTPAddr = %q, want loopback-only listener", defaultHTTPAddr)
	}
}
