// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPListenerClosesIncompleteHeaders(t *testing.T) {
	for _, secure := range []bool{false, true} {
		name := "http"
		if secure {
			name = "tls"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var called atomic.Bool
			listener := newHTTPServer("127.0.0.1:0", "test-secret", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			s := httptest.NewUnstartedServer(listener.Handler)
			s.Config = listener
			if secure {
				s.StartTLS()
			} else {
				s.Start()
			}
			t.Cleanup(s.Close)
			var conn net.Conn
			var err error
			if secure {
				cfg := s.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
				conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", s.Listener.Addr().String(), cfg)
			} else {
				conn, err = net.DialTimeout("tcp", s.Listener.Addr().String(), 2*time.Second)
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			if err := conn.SetDeadline(time.Now().Add(7 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(conn, "POST /mcp HTTP/1.1\r\nHost: example\r\n"); err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(conn)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				t.Fatal("listener left incomplete unauthenticated headers open")
			}
			if called.Load() || strings.Contains(string(body), "200 OK") {
				t.Fatal("incomplete headers reached the MCP handler")
			}
		})
	}
}
