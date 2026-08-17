// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownOAuthCallbackServerIsBounded(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	started := make(chan struct{})
	release := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	})}
	go server.Serve(listener)

	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		resp, _ := http.Get("http://" + addr)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("callback handler did not start")
	}

	shutdownOAuthCallbackServer(server, 10*time.Millisecond)
	close(release)
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("callback request did not stop after forced shutdown")
	}

	rebound, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("callback address was not released: %v", err)
	}
	_ = rebound.Close()
}
