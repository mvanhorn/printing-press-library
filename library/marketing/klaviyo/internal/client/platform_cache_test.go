// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestPlatformCacheKeyScopesEveryRequestDimension(t *testing.T) {
	session := &platform.Session{
		ProfileName: "tenant-a", Source: "source", CredentialFingerprint: "fingerprint-a",
		ExpectedIdentity: map[string]string{"account_id": "one"}, ObservedIdentity: map[string]string{"account_id": "one"},
		GateOutcome: platform.GateVerified,
	}
	c := New(&config.Config{BaseURL: "https://a.klaviyo.com"}, time.Second, 0)
	if err := c.BindPlatformSession(session); err != nil {
		t.Fatal(err)
	}
	base := c.cacheKeyFor("GET", "/resource", map[string]string{"q": "one"}, map[string]string{"Accept": "application/json"}, []byte(`{"x":1}`))
	variants := []string{
		c.cacheKeyFor("POST", "/resource", map[string]string{"q": "one"}, map[string]string{"Accept": "application/json"}, []byte(`{"x":1}`)),
		c.cacheKeyFor("GET", "/other", map[string]string{"q": "one"}, map[string]string{"Accept": "application/json"}, []byte(`{"x":1}`)),
		c.cacheKeyFor("GET", "/resource", map[string]string{"q": "two"}, map[string]string{"Accept": "application/json"}, []byte(`{"x":1}`)),
		c.cacheKeyFor("GET", "/resource", map[string]string{"q": "one"}, map[string]string{"Accept": "text/plain"}, []byte(`{"x":1}`)),
		c.cacheKeyFor("GET", "/resource", map[string]string{"q": "one"}, map[string]string{"Accept": "application/json"}, []byte(`{"x":2}`)),
	}
	for i, key := range variants {
		if key == base {
			t.Fatalf("request dimension %d did not alter cache key", i)
		}
	}

	other := *session
	other.ProfileName = "tenant-b"
	other.CredentialFingerprint = "fingerprint-b"
	other.ExpectedIdentity = map[string]string{"account_id": "two"}
	other.ObservedIdentity = map[string]string{"account_id": "two"}
	c2 := New(&config.Config{BaseURL: "https://a.klaviyo.com"}, time.Second, 0)
	if err := c2.BindPlatformSession(&other); err != nil {
		t.Fatal(err)
	}
	if c2.cacheKeyFor("GET", "/resource", map[string]string{"q": "one"}, map[string]string{"Accept": "application/json"}, []byte(`{"x":1}`)) == base {
		t.Fatal("tenant and credential dimensions did not alter cache key")
	}
}

func TestPlatformCacheResourceInvalidationAndMetadata(t *testing.T) {
	session := &platform.Session{ProfileName: "tenant-a", Source: "klaviyo", CredentialFingerprint: "fingerprint-a", ExpectedIdentity: map[string]string{"account_id": "one"}, ObservedIdentity: map[string]string{"account_id": "one"}, GateOutcome: platform.GateVerified}
	c := New(&config.Config{BaseURL: "https://a.klaviyo.com"}, time.Second, 0)
	if err := c.BindPlatformSession(session); err != nil {
		t.Fatal(err)
	}
	c.cacheDir = t.TempDir()
	c.writeCache("/api/orders", nil, nil, json.RawMessage(`{"resource":"orders"}`))
	c.writeCache("/api/profiles", nil, nil, json.RawMessage(`{"resource":"profiles"}`))
	if _, ok := c.readCache("/api/orders", nil, nil); !ok {
		t.Fatal("verified cache entry with matching metadata missed")
	}
	c.invalidateCacheResource("/api/orders/123")
	if _, err := os.Stat(c.cacheResourceDir("/api/orders")); !os.IsNotExist(err) {
		t.Fatalf("orders resource cache still exists: %v", err)
	}
	if _, err := os.Stat(c.cacheResourceDir("/api/profiles")); err != nil {
		t.Fatalf("unrelated profiles cache was removed: %v", err)
	}
	c.writeCache("/api/orders", nil, nil, json.RawMessage(`{"resource":"orders"}`))
	c.invalidateCacheAfterMutation("/api/orders/123")
	if _, err := os.Stat(c.cacheResourceDir("/api/orders")); !os.IsNotExist(err) {
		t.Fatalf("platform mutation left the directly affected cache: %v", err)
	}
	if _, err := os.Stat(c.cacheResourceDir("/api/profiles")); !os.IsNotExist(err) {
		t.Fatalf("platform mutation left a potentially related projection cache: %v", err)
	}
	session.CredentialFingerprint = "rotated-fingerprint"
	if _, ok := c.readCache("/api/profiles", nil, nil); ok {
		t.Fatal("cache metadata accepted a rotated credential")
	}
}

func TestLegacyMutationClearsAllHTTPProjections(t *testing.T) {
	c := New(&config.Config{BaseURL: "https://a.klaviyo.com"}, time.Second, 0)
	c.cacheDir = t.TempDir()
	c.writeCache("/api/orders", nil, nil, json.RawMessage(`{"resource":"orders"}`))
	c.writeCache("/api/profiles", nil, nil, json.RawMessage(`{"resource":"profiles"}`))

	c.invalidateCacheAfterMutation("/api/orders/123")

	for _, path := range []string{"/api/orders", "/api/profiles"} {
		if _, err := os.Stat(c.cacheResourceDir(path)); !os.IsNotExist(err) {
			t.Fatalf("legacy mutation left projection %s: %v", path, err)
		}
	}
}
