// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMutationOversizedResponseIsOutcomeUnknown(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", int(maxResponseBodyBytes+1))))
	}))
	defer server.Close()

	c := newRetryTestClient(server.URL)
	body := map[string]any{"synthetic": true}
	authorizeTestMutation(t, c, http.MethodPost, "/ship/v1/shipments", body)
	var unknown *OutcomeUnknownError
	_, _, err := c.Post("/ship/v1/shipments", body)
	if !errors.As(err, &unknown) {
		t.Fatalf("error=%v, want OutcomeUnknownError", err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d, want one non-retried mutation", calls)
	}
}
