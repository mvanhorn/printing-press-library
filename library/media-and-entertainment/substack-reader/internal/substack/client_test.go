// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package substack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGetCapsOversizedBody proves the success-path read is size-capped: a body
// larger than the client's cap returns an error instead of being read whole
// into the heap. Regression for the Greptile P2 "success-path io.ReadAll has no
// size cap" finding — the error paths already used io.LimitReader, but the 2xx
// path read the whole body, so an oversized (or redirected) response could
// balloon the heap.
func TestGetCapsOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 4096)) // well over the tiny cap set below
	}))
	defer srv.Close()

	c := NewClient()
	c.maxBytes = 1024 // shrink the 50 MB production cap so the test stays cheap

	if _, err := c.get(context.Background(), srv.URL); err == nil {
		t.Fatal("expected an error when the response body exceeds the cap, got nil")
	} else if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected a size-cap error mentioning 'exceeds', got: %v", err)
	}
}

// TestGetReadsBodyWithinCap confirms the cap does not truncate a normal body:
// a response comfortably under the cap is returned verbatim.
func TestGetReadsBodyWithinCap(t *testing.T) {
	const body = `[{"id":1,"slug":"hello"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient() // default 50 MB cap
	got, err := c.get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != body {
		t.Errorf("body = %q, want %q", got, body)
	}
}
