// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
)

// The test binary must never reach a real host: the careers and Oracle base
// URLs default to a dead loopback port (tests that need data point them at
// an httptest server), and every dial outside loopback fails. The generated
// client clones http.DefaultTransport, so the guard reaches it too.
func init() {
	for _, name := range []string{"UBER_JOBS_BASE_URL", "UBER_JOBS_ORACLE_BASE_URL"} {
		if os.Getenv(name) == "" {
			_ = os.Setenv(name, "http://127.0.0.1:9")
		}
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
