// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type failingResponseBody struct{}

func (failingResponseBody) Read([]byte) (int, error) { return 0, errors.New("synthetic read failure") }
func (failingResponseBody) Close() error             { return nil }

func TestMutationClientErrorWithUnreadableBodyIsDefinitive(t *testing.T) {
	body := map[string]any{"request": "synthetic"}
	c := newRetryTestClient("https://apis-sandbox.fedex.com")
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body:       failingResponseBody{},
		}, nil
	})}
	authorizeTestMutation(t, c, http.MethodPost, "/ship/v1/shipments", body)

	_, status, err := c.Post("/ship/v1/shipments", body)
	if status != http.StatusTooManyRequests {
		t.Fatalf("status=%d", status)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error=%T %v, want APIError", err, err)
	}
	var unknown *OutcomeUnknownError
	if errors.As(err, &unknown) {
		t.Fatalf("definitive 429 was classified outcome_unknown: %v", err)
	}
	if !strings.Contains(apiErr.Body, "unavailable") {
		t.Fatalf("API error body=%q", apiErr.Body)
	}
}

var _ io.ReadCloser = failingResponseBody{}
