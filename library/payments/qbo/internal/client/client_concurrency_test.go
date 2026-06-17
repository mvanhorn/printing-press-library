// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/config"
)

func TestClient_ConcurrentTokenRefresh(t *testing.T) {
	// Create a temp directory for the config file and lock file
	tmpDir, err := os.MkdirTemp("", "qbo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.toml")

	// Create initial config with an expired token
	cfg := &config.Config{
		Path:         configPath,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RefreshToken: "initial-refresh-token",
		TokenExpiry:  time.Now().Add(-1 * time.Hour), // Expired
	}
	// Save initial config
	if err := cfg.SaveTokens(cfg.ClientID, cfg.ClientSecret, "old-access-token", "initial-refresh-token", cfg.TokenExpiry); err != nil {
		t.Fatalf("failed to save initial config: %v", err)
	}

	// Create mock token refresh server
	var refreshCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Increment call count atomically
		atomic.AddInt32(&refreshCalls, 1)

		// Simulate minor latency to increase likelihood of overlap
		time.Sleep(50 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	// Initialize the clients
	// Since clients are separate processes in reality, we simulate this by creating two Client instances
	// that share the same configuration path on disk.
	cfg1, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config 1: %v", err)
	}
	cfg1.TokenURL = server.URL
	c1 := New(cfg1, time.Second, 0)

	cfg2, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config 2: %v", err)
	}
	cfg2.TokenURL = server.URL
	c2 := New(cfg2, time.Second, 0)

	// Run concurrent refreshes
	var wg sync.WaitGroup
	wg.Add(2)

	var err1, err2 error

	go func() {
		defer wg.Done()
		err1 = c1.refreshAccessToken(context.Background(), false)
	}()

	go func() {
		defer wg.Done()
		err2 = c2.refreshAccessToken(context.Background(), false)
	}()

	wg.Wait()

	if err1 != nil {
		t.Errorf("client 1 failed to refresh: %v", err1)
	}
	if err2 != nil {
		t.Errorf("client 2 failed to refresh: %v", err2)
	}

	// Verify that the refresh endpoint was only called once!
	calls := atomic.LoadInt32(&refreshCalls)
	if calls != 1 {
		t.Errorf("expected refresh endpoint to be called exactly once, but got %d", calls)
	}

	// Verify the config file contains the new tokens
	finalCfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load final config: %v", err)
	}

	if finalCfg.AccessToken != "new-access-token" {
		t.Errorf("expected access token to be 'new-access-token', got %q", finalCfg.AccessToken)
	}
	if finalCfg.RefreshToken != "new-refresh-token" {
		t.Errorf("expected refresh token to be 'new-refresh-token', got %q", finalCfg.RefreshToken)
	}
	if finalCfg.TokenExpiry.Before(time.Now()) {
		t.Errorf("expected token expiry to be in the future, got %v", finalCfg.TokenExpiry)
	}
}

// TestClient_ForceRefreshBypassesShortCircuit verifies that the reactive/401
// path (forceRefresh=true) always contacts the token endpoint even when the
// on-disk token is still considered valid by local clock. This covers the
// case where a provider revokes or early-expires a token before its stated
// expiry time.
func TestClient_ForceRefreshBypassesShortCircuit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "qbo-force-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.toml")

	// Write a config with a token that is NOT yet expired by local clock
	// but would be rejected by the provider (simulated by our test server).
	cfg := &config.Config{
		Path:         configPath,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RefreshToken: "initial-refresh-token",
	}
	if err := cfg.SaveTokens(cfg.ClientID, cfg.ClientSecret, "stale-access-token", "initial-refresh-token",
		time.Now().Add(1*time.Hour)); err != nil { // token still "valid" by clock
		t.Fatalf("failed to save initial config: %v", err)
	}

	var refreshCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&refreshCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "fresh-access-token",
			"refresh_token": "fresh-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	loadedCfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	loadedCfg.TokenURL = server.URL
	c := New(loadedCfg, time.Second, 0)

	// forceRefresh=true must bypass the short-circuit and hit the server.
	if err := c.refreshAccessToken(context.Background(), true); err != nil {
		t.Fatalf("forceRefresh failed: %v", err)
	}

	if calls := atomic.LoadInt32(&refreshCalls); calls != 1 {
		t.Errorf("expected 1 token-endpoint call with forceRefresh=true, got %d", calls)
	}

	finalCfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load final config: %v", err)
	}
	if finalCfg.AccessToken != "fresh-access-token" {
		t.Errorf("expected fresh-access-token, got %q", finalCfg.AccessToken)
	}
}

// TestClient_ForceRefreshReloadsRotatedToken verifies that even on the
// forceRefresh=true (401) path, the config is reloaded from disk after
// acquiring the lock so that a rotated refresh_token written by a concurrent
// process is used rather than a stale in-memory copy.
func TestClient_ForceRefreshReloadsRotatedToken(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "qbo-rotated-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.toml")

	// Write initial config with refresh token "original-refresh".
	cfg := &config.Config{
		Path:         configPath,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RefreshToken: "original-refresh-token",
	}
	if err := cfg.SaveTokens(cfg.ClientID, cfg.ClientSecret, "stale-access-token", "original-refresh-token",
		time.Now().Add(1*time.Hour)); err != nil {
		t.Fatalf("failed to save initial config: %v", err)
	}

	// Track the refresh_token values posted to the server.
	var postedRefreshTokens []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			mu.Lock()
			postedRefreshTokens = append(postedRefreshTokens, r.FormValue("refresh_token"))
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"refresh_token": "rotated-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	// c1 runs first (it will win the lock) and rotates the refresh token on disk.
	cfg1, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load cfg1: %v", err)
	}
	cfg1.TokenURL = server.URL
	c1 := New(cfg1, time.Second, 0)

	// c2 starts with a stale in-memory refresh token equal to "original-refresh-token".
	cfg2, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load cfg2: %v", err)
	}
	cfg2.TokenURL = server.URL
	c2 := New(cfg2, time.Second, 0)

	// c1 refreshes first, writing "rotated-refresh-token" to disk.
	if err := c1.refreshAccessToken(context.Background(), true); err != nil {
		t.Fatalf("c1 forceRefresh: %v", err)
	}

	// c2 now calls forceRefresh. It should reload from disk and post
	// "rotated-refresh-token", not the stale "original-refresh-token".
	if err := c2.refreshAccessToken(context.Background(), true); err != nil {
		t.Fatalf("c2 forceRefresh: %v", err)
	}

	mu.Lock()
	posted := postedRefreshTokens
	mu.Unlock()

	if len(posted) != 2 {
		t.Fatalf("expected 2 refresh calls, got %d", len(posted))
	}
	if posted[0] != "original-refresh-token" {
		t.Errorf("c1 should post original-refresh-token, got %q", posted[0])
	}
	if posted[1] != "rotated-refresh-token" {
		t.Errorf("c2 should post rotated-refresh-token (from disk reload), got %q", posted[1])
	}
}
