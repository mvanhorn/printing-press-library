// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
)

func TestAuthHeaderRejectsTokenFromDifferentFedExEnvironment(t *testing.T) {
	client := New(&config.Config{
		BaseURL:      "https://apis.fedex.com",
		AccessToken:  "sandbox-token",
		TokenBaseURL: "https://apis-sandbox.fedex.com",
		TokenExpiry:  time.Now().Add(time.Hour),
	}, time.Second, 0)

	if _, err := client.authHeaderForPath("/ship/v1/shipments"); err == nil || !strings.Contains(err.Error(), "different FedEx environment") {
		t.Fatalf("mismatched token error=%v", err)
	}
}

func TestAuthHeaderAcceptsTokenForMatchingFedExEnvironment(t *testing.T) {
	client := New(&config.Config{
		BaseURL:      "https://apis.fedex.com",
		AccessToken:  "production-token",
		TokenBaseURL: "https://apis.fedex.com/",
		TokenExpiry:  time.Now().Add(time.Hour),
	}, time.Second, 0)

	header, err := client.authHeaderForPath("/ship/v1/shipments")
	if err != nil {
		t.Fatalf("authHeaderForPath: %v", err)
	}
	if header != "Bearer production-token" {
		t.Fatalf("header=%q", header)
	}
}

func TestAuthHeaderRejectsTokenWithoutExpiry(t *testing.T) {
	client := New(&config.Config{
		BaseURL:      "https://apis.fedex.com",
		AccessToken:  "production-token",
		TokenBaseURL: "https://apis.fedex.com",
	}, time.Second, 0)

	if _, err := client.authHeaderForPath("/ship/v1/shipments"); err == nil || !strings.Contains(err.Error(), "bounded expiry") {
		t.Fatalf("unbounded token error=%v", err)
	}
}

func TestAuthHeaderRejectsUnboundTrackTokenWithoutRuntimeCredentials(t *testing.T) {
	client := New(&config.Config{
		BaseURL:           "https://apis.fedex.com",
		TrackAccessToken:  "sandbox-track-token",
		TrackTokenBaseURL: "https://apis-sandbox.fedex.com",
		TrackTokenExpiry:  time.Now().Add(time.Hour),
	}, time.Second, 0)

	if _, err := client.authHeaderForPath("/track/v1/trackingnumbers"); err == nil || !strings.Contains(err.Error(), "different FedEx environment") {
		t.Fatalf("unbound Track token error=%v", err)
	}
}

func TestAuthHeaderRejectsExpiringTokenWithoutRuntimeCredentials(t *testing.T) {
	clearOAuthEnv(t)
	client := New(&config.Config{
		BaseURL:      "https://apis.fedex.com",
		AccessToken:  "expiring-token",
		TokenBaseURL: "https://apis.fedex.com",
		TokenExpiry:  time.Now().Add(30 * time.Second),
	}, time.Second, 0)

	if _, err := client.authHeaderForPath("/ship/v1/shipments"); err == nil || !strings.Contains(err.Error(), "expired or expiring") {
		t.Fatalf("expiring token error=%v", err)
	}
}

func TestAuthHeaderRejectsExpiringTrackTokenWithoutRuntimeCredentials(t *testing.T) {
	clearOAuthEnv(t)
	client := New(&config.Config{
		BaseURL:           "https://apis.fedex.com",
		TrackAccessToken:  "expiring-track-token",
		TrackTokenBaseURL: "https://apis.fedex.com",
		TrackTokenExpiry:  time.Now().Add(30 * time.Second),
	}, time.Second, 0)

	if _, err := client.authHeaderForPath("/track/v1/trackingnumbers"); err == nil || !strings.Contains(err.Error(), "expired or expiring") {
		t.Fatalf("expiring Track token error=%v", err)
	}
}

func clearOAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("FEDEX_API_KEY", "")
	t.Setenv("FEDEX_SECRET_KEY", "")
	t.Setenv("FEDEX_TRACK_API_KEY", "")
	t.Setenv("FEDEX_TRACK_SECRET_KEY", "")
}
