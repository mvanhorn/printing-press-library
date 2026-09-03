// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"net/http"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
)

func TestRejectsNonFedExRemoteBaseURLBeforeNetwork(t *testing.T) {
	calls := 0
	cfg := &config.Config{BaseURL: "https://attacker.example", AccessToken: "synthetic-token"}
	c := New(cfg, time.Second, 0)
	c.NoCache = true
	c.cacheDir = ""
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, nil
	})}
	if _, err := c.Get("/rate/v1/rates/quotes", nil); err == nil {
		t.Fatal("non-FedEx remote base URL was accepted")
	}
	if calls != 0 {
		t.Fatalf("non-FedEx remote base URL emitted %d requests, want 0", calls)
	}
}

func TestAcceptsOfficialAndLoopbackBaseURLs(t *testing.T) {
	for _, raw := range []string{
		"https://apis.fedex.com",
		"https://apis-sandbox.fedex.com/",
		"http://127.0.0.1:8080",
		"http://localhost:8080",
		"http://[::1]:8080",
	} {
		if err := validateFedExBaseURL(raw); err != nil {
			t.Errorf("validateFedExBaseURL(%q): %v", raw, err)
		}
	}
}
