// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
)

// The test binary must never reach a real host: every dial outside loopback
// fails, and the Oracle fallback points at a dead loopback port unless a test
// sets its own fake server. Clients built on http.DefaultTransport (or a
// clone of it) inherit the guard.
func init() {
	if os.Getenv("UBER_JOBS_ORACLE_BASE_URL") == "" {
		_ = os.Setenv("UBER_JOBS_ORACLE_BASE_URL", "http://127.0.0.1:9")
	}
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.Proxy = nil
		tr.DialContext = loopbackOnlyDial
	}
}

func loopbackOnlyDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("test net guard: refusing to dial %s (tests use loopback fakes only)", addr)
	}
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}
