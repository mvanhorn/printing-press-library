// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
)

func TestEveryMutationRequiresBoundPermit(t *testing.T) {
	calls := 0
	c := New(&config.Config{
		BaseURL:      "https://apis-sandbox.fedex.com",
		AccessToken:  "synthetic-token",
		TokenBaseURL: "https://apis-sandbox.fedex.com",
		TokenExpiry:  time.Now().Add(time.Hour),
	}, time.Second, 0)
	c.NoCache = true
	c.cacheDir = ""
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("network should not be reached")
	})}

	_, _, err := c.Post("/ship/v1/consolidations", map[string]any{"consolidation": "synthetic"})
	if !errors.Is(err, ErrBoundConfirmationRequired) {
		t.Fatalf("expected bound confirmation error, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("unpermitted mutation reached network %d times", calls)
	}
}
