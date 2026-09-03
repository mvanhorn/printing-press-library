// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestConfirmedMutationDoesNotFollowRedirect(t *testing.T) {
	var redirectedCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedCalls.Add(1)
	}))
	defer target.Close()

	var originCalls atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		originCalls.Add(1)
		w.Header().Set("Location", target.URL+"/ship/v1/shipments")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	body := map[string]any{"request": "sensitive"}
	client := newRetryTestClient(origin.URL)
	authorizeTestMutation(t, client, http.MethodPost, "/ship/v1/shipments", body)
	_, status, err := client.Post("/ship/v1/shipments", body)
	var unknown *OutcomeUnknownError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected OutcomeUnknownError, got status=%d err=%v", status, err)
	}
	if status != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d, want %d", status, http.StatusTemporaryRedirect)
	}
	if got := originCalls.Load(); got != 1 {
		t.Fatalf("origin calls=%d, want 1", got)
	}
	if got := redirectedCalls.Load(); got != 0 {
		t.Fatalf("redirected calls=%d, want 0", got)
	}
}
