// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPListenerClosesIncompleteHeaders(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var called atomic.Bool
	srv := newHTTPServer(listener.Addr().String(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusNoContent)
	}))
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(listener) }()
	t.Cleanup(func() {
		_ = srv.Close()
		if err := <-serveDone; err != nil && err != http.ErrServerClosed {
			t.Errorf("HTTP listener: %v", err)
		}
	})
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(8 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "GET /mcp HTTP/1.1\r\nHost: localhost\r\nX-Incomplete:"); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(conn); err != nil {
		t.Fatalf("listener did not close incomplete headers before the client deadline: %v", err)
	}
	if called.Load() {
		t.Fatal("incomplete headers reached the request handler")
	}
}
